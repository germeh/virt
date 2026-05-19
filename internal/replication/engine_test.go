package replication

import (
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var errWriteFailed = errors.New("write failed")

type writeFailDisk struct {
	*MemoryDisk
}

func (d writeFailDisk) WriteRange(BlockRange, []byte) error {
	return errWriteFailed
}

type hookedWriteDisk struct {
	*MemoryDisk
	beforeWrite func(BlockRange, []byte)
}

func (d *hookedWriteDisk) WriteRange(r BlockRange, data []byte) error {
	if d.beforeWrite != nil {
		d.beforeWrite(r, data)
	}
	return d.MemoryDisk.WriteRange(r, data)
}

type blockingWriteDisk struct {
	*MemoryDisk
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (d *blockingWriteDisk) WriteRange(r BlockRange, data []byte) error {
	d.once.Do(func() {
		close(d.entered)
		<-d.release
	})
	return d.MemoryDisk.WriteRange(r, data)
}

type signalingWriteDisk struct {
	*MemoryDisk
	wrote chan struct{}
	once  sync.Once
}

func (d *signalingWriteDisk) WriteRange(r BlockRange, data []byte) error {
	d.once.Do(func() {
		close(d.wrote)
	})
	return d.MemoryDisk.WriteRange(r, data)
}

type dirtyRangeDisk struct {
	base        *MemoryDisk
	dirty       []BlockRange
	clearCalled bool
}

func (d *dirtyRangeDisk) Size() uint64 {
	return d.base.Size()
}

func (d *dirtyRangeDisk) ReadRange(r BlockRange) ([]byte, error) {
	return d.base.ReadRange(r)
}

func (d *dirtyRangeDisk) WriteRange(r BlockRange, data []byte) error {
	return d.base.WriteRange(r, data)
}

func (d *dirtyRangeDisk) DirtyRanges() []BlockRange {
	return append([]BlockRange(nil), d.dirty...)
}

func (d *dirtyRangeDisk) ClearDirtyRanges([]BlockRange) {
	d.clearCalled = true
}

type snapshotDirtyRangeDisk struct {
	base        *MemoryDisk
	dirty       []BlockRange
	clearCalled bool
}

func (d *snapshotDirtyRangeDisk) Size() uint64 {
	return d.base.Size()
}

func (d *snapshotDirtyRangeDisk) ReadRange(r BlockRange) ([]byte, error) {
	return d.base.ReadRange(r)
}

func (d *snapshotDirtyRangeDisk) WriteRange(r BlockRange, data []byte) error {
	return d.base.WriteRange(r, data)
}

func (d *snapshotDirtyRangeDisk) DirtyRanges() []BlockRange {
	return append([]BlockRange(nil), d.dirty...)
}

func (d *snapshotDirtyRangeDisk) ClearDirtyRanges([]BlockRange) {
	d.clearCalled = true
}

func (d *snapshotDirtyRangeDisk) snapshotDirtyRanges() dirtySnapshot {
	return dirtySnapshot{
		ranges: append([]BlockRange(nil), d.dirty...),
		clear: func([]BlockRange) {
			d.clearCalled = true
		},
		discard: func() {},
	}
}

func TestEngineBaseSyncCopiesFullDisk(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk(make([]byte, 10))
	engine := Engine{ChunkSize: 4}

	report, err := engine.BaseSync(context.Background(), source, target, ResumeState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(target.Bytes(), source.Bytes()) {
		t.Fatalf("target = %q, source = %q", target.Bytes(), source.Bytes())
	}
	if report.BytesCopied != 10 {
		t.Fatalf("copied %d bytes, want 10", report.BytesCopied)
	}
}

func TestEngineIncrementalSyncCopiesDirtyRangesOnly(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk([]byte("abcdefghij"))
	source.WriteAtBytes([]byte("XYZ"), 2)

	engine := Engine{ChunkSize: 4}
	report, err := engine.IncrementalSync(context.Background(), source, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(target.Bytes(), []byte("abXYZfghij")) {
		t.Fatalf("target = %q", target.Bytes())
	}
	if report.BytesCopied != 3 {
		t.Fatalf("copied %d bytes, want 3", report.BytesCopied)
	}
	if len(source.DirtyRanges()) != 0 {
		t.Fatalf("dirty ranges were not cleared: %#v", source.DirtyRanges())
	}
}

func TestEngineBaseSyncResumeSkipsVerifiedChunks(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk(make([]byte, 10))
	engine := Engine{ChunkSize: 4}

	state := ResumeState{VerifiedChunks: map[uint64]bool{0: true}}
	report, err := engine.BaseSync(context.Background(), source, target, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(target.Bytes()[0:4]) != "\x00\x00\x00\x00" {
		t.Fatalf("verified chunk should not have been copied")
	}
	if string(target.Bytes()[4:]) != "efghij" {
		t.Fatalf("remaining chunks were not copied: %q", target.Bytes())
	}
	if report.BytesCopied != 6 {
		t.Fatalf("copied %d bytes, want 6", report.BytesCopied)
	}
}

func TestMemoryDiskRejectsOverflowingReadWriteRanges(t *testing.T) {
	disk := NewMemoryDisk([]byte("abc"))
	overflowing := BlockRange{Offset: math.MaxUint64, Length: 1}

	if _, err := disk.ReadRange(overflowing); err == nil {
		t.Fatalf("ReadRange returned nil error for overflowing range")
	}
	if err := disk.WriteRange(overflowing, []byte("x")); err == nil {
		t.Fatalf("WriteRange returned nil error for overflowing range")
	}
}

func TestMemoryDiskWriteAtBytesRejectsOverflowBeforeSlice(t *testing.T) {
	disk := NewMemoryDisk([]byte("abc"))

	defer func() {
		got := recover()
		if got == nil {
			t.Fatalf("WriteAtBytes did not panic")
		}
		if got != "write exceeds disk size" {
			t.Fatalf("panic = %v, want %q", got, "write exceeds disk size")
		}
	}()

	disk.WriteAtBytes([]byte("x"), math.MaxUint64)
}

func TestMemoryDiskClearDirtyRangesSubtractsOnlyAcknowledgedRanges(t *testing.T) {
	disk := NewMemoryDisk([]byte("abcdefghij"))
	disk.WriteAtBytes([]byte("XY"), 1)
	disk.WriteAtBytes([]byte("ZZ"), 7)

	disk.ClearDirtyRanges([]BlockRange{{Offset: 1, Length: 2}})

	want := []BlockRange{{Offset: 7, Length: 2}}
	if got := disk.DirtyRanges(); !reflect.DeepEqual(got, want) {
		t.Fatalf("dirty ranges = %#v, want %#v", got, want)
	}
}

func TestMemoryDiskSnapshotClearUsesOwnCutoffWhenCompletedOutOfOrder(t *testing.T) {
	disk := NewMemoryDisk([]byte("abcdefghij"))
	disk.WriteAtBytes([]byte("XY"), 1)
	snapshotA := disk.snapshotDirtyRanges()
	disk.WriteAtBytes([]byte("ZZ"), 7)
	snapshotB := disk.snapshotDirtyRanges()

	snapshotB.clear(snapshotB.ranges)

	want := []BlockRange{{Offset: 1, Length: 2}}
	if got := disk.DirtyRanges(); !reflect.DeepEqual(got, want) {
		t.Fatalf("dirty ranges after snapshot B clear = %#v, want %#v", got, want)
	}

	snapshotA.clear(snapshotA.ranges)
	if got := disk.DirtyRanges(); len(got) != 0 {
		t.Fatalf("dirty ranges after snapshot A clear = %#v, want empty", got)
	}
}

func TestMemoryDiskDirtyRangesInspectionDoesNotPoisonNextSync(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk([]byte("abcdefghij"))
	if got := source.DirtyRanges(); len(got) != 0 {
		t.Fatalf("initial dirty ranges = %#v, want empty", got)
	}
	source.WriteAtBytes([]byte("XYZ"), 2)
	engine := Engine{ChunkSize: 4}

	if _, err := engine.IncrementalSync(context.Background(), source, target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(target.Bytes(), []byte("abXYZfghij")) {
		t.Fatalf("target = %q", target.Bytes())
	}
	if got := source.DirtyRanges(); len(got) != 0 {
		t.Fatalf("dirty ranges = %#v, want empty", got)
	}
}

func TestEngineIncrementalSyncFailurePreservesDirtyRanges(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	source.WriteAtBytes([]byte("XYZ"), 2)
	before := source.DirtyRanges()
	target := writeFailDisk{MemoryDisk: NewMemoryDisk([]byte("abcdefghij"))}
	engine := Engine{ChunkSize: 4}

	if _, err := engine.IncrementalSync(context.Background(), source, target); !errors.Is(err, errWriteFailed) {
		t.Fatalf("error = %v, want %v", err, errWriteFailed)
	}
	if got := source.DirtyRanges(); !reflect.DeepEqual(got, before) {
		t.Fatalf("dirty ranges = %#v, want %#v", got, before)
	}
}

func TestEngineFailedSyncDoesNotPoisonNextSuccessfulSync(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk([]byte("abcdefghij"))
	source.WriteAtBytes([]byte("XYZ"), 2)
	engine := Engine{ChunkSize: 4}

	if _, err := engine.IncrementalSync(context.Background(), source, writeFailDisk{MemoryDisk: NewMemoryDisk([]byte("abcdefghij"))}); !errors.Is(err, errWriteFailed) {
		t.Fatalf("error = %v, want %v", err, errWriteFailed)
	}

	source.WriteAtBytes([]byte("123"), 2)
	if _, err := engine.IncrementalSync(context.Background(), source, target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(target.Bytes(), source.Bytes()) {
		t.Fatalf("target = %q, source = %q", target.Bytes(), source.Bytes())
	}
	if got := source.DirtyRanges(); len(got) != 0 {
		t.Fatalf("dirty ranges = %#v, want empty", got)
	}
}

func TestEngineValidationFailureDoesNotPoisonNextSync(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk([]byte("abcdefghij"))
	source.WriteAtBytes([]byte("XYZ"), 2)

	if _, err := (Engine{}).IncrementalSync(context.Background(), source, target); err == nil {
		t.Fatalf("IncrementalSync returned nil error for zero chunk size")
	}

	source.WriteAtBytes([]byte("123"), 2)
	if _, err := (Engine{ChunkSize: 4}).IncrementalSync(context.Background(), source, target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(target.Bytes(), []byte("ab123fghij")) {
		t.Fatalf("target = %q", target.Bytes())
	}
	if got := source.DirtyRanges(); len(got) != 0 {
		t.Fatalf("dirty ranges = %#v, want empty", got)
	}
}

func TestEngineCanceledSyncDoesNotPoisonNextSuccessfulSync(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk([]byte("abcdefghij"))
	source.WriteAtBytes([]byte("XYZ"), 2)
	engine := Engine{ChunkSize: 4}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := engine.IncrementalSync(ctx, source, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want %v", err, context.Canceled)
	}

	source.WriteAtBytes([]byte("123"), 2)
	if _, err := engine.IncrementalSync(context.Background(), source, target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(target.Bytes(), source.Bytes()) {
		t.Fatalf("target = %q, source = %q", target.Bytes(), source.Bytes())
	}
	if got := source.DirtyRanges(); len(got) != 0 {
		t.Fatalf("dirty ranges = %#v, want empty", got)
	}
}

func TestEngineIncrementalSyncPreservesOverlappingWriteDuringCopy(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	source.WriteAtBytes([]byte("XYZ"), 2)
	target := &hookedWriteDisk{MemoryDisk: NewMemoryDisk([]byte("abcdefghij"))}
	var hooked bool
	target.beforeWrite = func(_ BlockRange, data []byte) {
		if hooked {
			return
		}
		hooked = true
		if !bytes.Equal(data, []byte("XYZ")) {
			t.Fatalf("copied data = %q, want %q", data, "XYZ")
		}
		source.WriteAtBytes([]byte("123"), 2)
	}
	engine := Engine{ChunkSize: 4}

	if _, err := engine.IncrementalSync(context.Background(), source, target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hooked {
		t.Fatalf("target write hook was not called")
	}
	if !bytes.Equal(target.Bytes(), []byte("abXYZfghij")) {
		t.Fatalf("target = %q", target.Bytes())
	}
	if !bytes.Equal(source.Bytes(), []byte("ab123fghij")) {
		t.Fatalf("source = %q", source.Bytes())
	}
	wantDirty := []BlockRange{{Offset: 2, Length: 3}}
	if got := source.DirtyRanges(); !reflect.DeepEqual(got, wantDirty) {
		t.Fatalf("dirty ranges = %#v, want %#v", got, wantDirty)
	}
}

func TestEngineSerializesDirtySyncForMemoryDisk(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk([]byte("abcdefghij"))
	source.WriteAtBytes([]byte("XYZ"), 2)
	engine := Engine{ChunkSize: 4}

	blockingTarget := &blockingWriteDisk{
		MemoryDisk: target,
		entered:    make(chan struct{}),
		release:    make(chan struct{}),
	}
	firstDone := make(chan error, 1)
	go func() {
		_, err := engine.IncrementalSync(context.Background(), source, blockingTarget)
		firstDone <- err
	}()

	<-blockingTarget.entered
	source.WriteAtBytes([]byte("123"), 2)

	secondTarget := &signalingWriteDisk{
		MemoryDisk: target,
		wrote:      make(chan struct{}),
	}
	secondStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondStarted)
		_, err := engine.IncrementalSync(context.Background(), source, secondTarget)
		secondDone <- err
	}()
	<-secondStarted

	select {
	case <-secondTarget.wrote:
		t.Fatalf("second dirty sync wrote before first dirty sync completed")
	case <-time.After(50 * time.Millisecond):
	}

	close(blockingTarget.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first sync error: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second sync error: %v", err)
	}
	if !bytes.Equal(target.Bytes(), source.Bytes()) {
		t.Fatalf("target = %q, source = %q", target.Bytes(), source.Bytes())
	}
	if got := source.DirtyRanges(); len(got) != 0 {
		t.Fatalf("dirty ranges = %#v, want empty", got)
	}
}

func TestEngineIncrementalSyncRejectsOverflowingDirtyRange(t *testing.T) {
	source := &snapshotDirtyRangeDisk{
		base:  NewMemoryDisk([]byte("abc")),
		dirty: []BlockRange{{Offset: math.MaxUint64, Length: 1}},
	}
	target := NewMemoryDisk([]byte("abc"))
	engine := Engine{ChunkSize: 4}

	_, err := engine.IncrementalSync(context.Background(), source, target)
	if err == nil {
		t.Fatalf("IncrementalSync returned nil error for overflowing dirty range")
	}
	if !strings.Contains(err.Error(), "dirty range overflows uint64") {
		t.Fatalf("error = %v, want dirty range overflow error", err)
	}
	if source.clearCalled {
		t.Fatalf("ClearDirtyRanges was called for overflowing dirty range")
	}
}

func TestEngineIncrementalSyncRejectsUnsafeDirtyRangeSource(t *testing.T) {
	source := &dirtyRangeDisk{
		base:  NewMemoryDisk([]byte("abc")),
		dirty: []BlockRange{{Offset: 0, Length: 1}},
	}
	target := NewMemoryDisk([]byte("abc"))
	engine := Engine{ChunkSize: 4}

	_, err := engine.IncrementalSync(context.Background(), source, target)
	if err == nil {
		t.Fatalf("IncrementalSync returned nil error for source without safe dirty snapshots")
	}
	if !strings.Contains(err.Error(), "safe dirty snapshots") {
		t.Fatalf("error = %v, want safe dirty snapshot error", err)
	}
	if source.clearCalled {
		t.Fatalf("ClearDirtyRanges was called for unsafe dirty range source")
	}
}

func TestEngineRejectsNilDisks(t *testing.T) {
	engine := Engine{ChunkSize: 4}
	disk := NewMemoryDisk([]byte("abc"))
	var typedNil *MemoryDisk

	if _, err := engine.BaseSync(context.Background(), nil, disk, ResumeState{}); err == nil {
		t.Fatalf("BaseSync returned nil error for nil source")
	}
	if _, err := engine.BaseSync(context.Background(), disk, nil, ResumeState{}); err == nil {
		t.Fatalf("BaseSync returned nil error for nil target")
	}
	if _, err := engine.IncrementalSync(context.Background(), nil, disk); err == nil {
		t.Fatalf("IncrementalSync returned nil error for nil source")
	}
	if _, err := engine.IncrementalSync(context.Background(), disk, nil); err == nil {
		t.Fatalf("IncrementalSync returned nil error for nil target")
	}
	if _, err := engine.IncrementalSync(context.Background(), typedNil, disk); err == nil {
		t.Fatalf("IncrementalSync returned nil error for typed nil source")
	}
	if _, err := engine.FinalSync(context.Background(), typedNil, disk); err == nil {
		t.Fatalf("FinalSync returned nil error for typed nil source")
	}
}

func TestEngineBaseSyncRejectsSizeMismatch(t *testing.T) {
	engine := Engine{ChunkSize: 4}
	source := NewMemoryDisk([]byte("abc"))
	target := NewMemoryDisk([]byte("abcd"))

	if _, err := engine.BaseSync(context.Background(), source, target, ResumeState{}); err == nil {
		t.Fatalf("BaseSync returned nil error for size mismatch")
	}
}

func TestEngineRejectsZeroChunkSize(t *testing.T) {
	engine := Engine{}
	source := NewMemoryDisk([]byte("abc"))
	target := NewMemoryDisk([]byte("abc"))

	if _, err := engine.BaseSync(context.Background(), source, target, ResumeState{}); err == nil {
		t.Fatalf("BaseSync returned nil error for zero chunk size")
	}
	if _, err := engine.IncrementalSync(context.Background(), source, target); err == nil {
		t.Fatalf("IncrementalSync returned nil error for zero chunk size")
	}
}
