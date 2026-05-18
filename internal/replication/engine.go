package replication

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

type Disk interface {
	Size() uint64
	ReadRange(BlockRange) ([]byte, error)
	WriteRange(BlockRange, []byte) error
	DirtyRanges() []BlockRange
	ClearDirtyRanges([]BlockRange)
}

type dirtySnapshot struct {
	ranges  []BlockRange
	clear   func([]BlockRange)
	discard func()
}

type dirtySnapshotter interface {
	snapshotDirtyRanges() dirtySnapshot
}

type dirtySyncLocker interface {
	lockDirtySync()
	unlockDirtySync()
}

type Engine struct {
	ChunkSize uint64
}

type ResumeState struct {
	VerifiedChunks map[uint64]bool
}

type SyncReport struct {
	BytesCopied  uint64
	RangesCopied []BlockRange
}

func (e Engine) BaseSync(ctx context.Context, source Disk, target Disk, resume ResumeState) (SyncReport, error) {
	chunkSize, err := e.validChunkSize()
	if err != nil {
		return SyncReport{}, err
	}
	sourceSize, err := validateDiskPair(source, target)
	if err != nil {
		return SyncReport{}, err
	}
	var report SyncReport
	for offset, index := uint64(0), uint64(0); offset < sourceSize; index++ {
		select {
		case <-ctx.Done():
			return report, ctx.Err()
		default:
		}
		length := chunkSize
		if remaining := sourceSize - offset; remaining < chunkSize {
			length = remaining
		}
		if resume.VerifiedChunks != nil && resume.VerifiedChunks[index] {
			offset += length
			continue
		}
		r := BlockRange{Offset: offset, Length: length}
		if err := copyRange(source, target, r); err != nil {
			return report, err
		}
		report.BytesCopied += r.Length
		report.RangesCopied = append(report.RangesCopied, r)
		offset += length
	}
	return report, nil
}

func (e Engine) IncrementalSync(ctx context.Context, source Disk, target Disk) (SyncReport, error) {
	return e.syncDirtyRanges(ctx, source, target, true)
}

func (e Engine) FinalSync(ctx context.Context, source Disk, target Disk) (SyncReport, error) {
	return e.syncDirtyRanges(ctx, source, target, true)
}

func (e Engine) syncRanges(ctx context.Context, source Disk, target Disk, ranges []BlockRange, clearDirty bool) (SyncReport, error) {
	if _, err := e.validChunkSize(); err != nil {
		return SyncReport{}, err
	}
	if _, err := validateDiskPair(source, target); err != nil {
		return SyncReport{}, err
	}
	report, normalized, err := copyRangesAfterValidation(ctx, source, target, ranges)
	if err != nil {
		return report, err
	}
	if clearDirty {
		source.ClearDirtyRanges(normalized)
	}
	return report, nil
}

func (e Engine) syncDirtyRanges(ctx context.Context, source Disk, target Disk, clearDirty bool) (SyncReport, error) {
	if _, err := e.validChunkSize(); err != nil {
		return SyncReport{}, err
	}
	if _, err := validateDiskPair(source, target); err != nil {
		return SyncReport{}, err
	}

	unlock := lockDirtySyncFor(source)
	defer unlock()

	snapshot := dirtySnapshotForSync(source)
	report, normalized, err := copyRangesAfterValidation(ctx, source, target, snapshot.ranges)
	if err != nil {
		if snapshot.discard != nil {
			snapshot.discard()
		}
		return report, err
	}
	if clearDirty {
		if snapshot.clear != nil {
			snapshot.clear(normalized)
		}
	} else if snapshot.discard != nil {
		snapshot.discard()
	}
	return report, nil
}

func copyRangesAfterValidation(ctx context.Context, source Disk, target Disk, ranges []BlockRange) (SyncReport, []BlockRange, error) {
	if err := validateDirtyRanges(ranges); err != nil {
		return SyncReport{}, nil, err
	}
	normalized := NormalizeRanges(ranges)
	var report SyncReport
	for _, r := range normalized {
		select {
		case <-ctx.Done():
			return report, normalized, ctx.Err()
		default:
		}
		if err := copyRange(source, target, r); err != nil {
			return report, normalized, err
		}
		report.BytesCopied += r.Length
		report.RangesCopied = append(report.RangesCopied, r)
	}
	return report, normalized, nil
}

func (e Engine) validChunkSize() (uint64, error) {
	if e.ChunkSize == 0 {
		return 0, errors.New("chunk size must be greater than zero")
	}
	return e.ChunkSize, nil
}

func validateDirtyRanges(ranges []BlockRange) error {
	for _, r := range ranges {
		if r.Length == 0 {
			continue
		}
		if _, ok := checkedEnd(r); !ok {
			return fmt.Errorf("dirty range overflows uint64: %+v", r)
		}
	}
	return nil
}

func dirtySnapshotForSync(source Disk) dirtySnapshot {
	if snapshotter, ok := source.(dirtySnapshotter); ok {
		return snapshotter.snapshotDirtyRanges()
	}
	return dirtySnapshot{
		ranges:  source.DirtyRanges(),
		clear:   source.ClearDirtyRanges,
		discard: func() {},
	}
}

func lockDirtySyncFor(source Disk) func() {
	if locker, ok := source.(dirtySyncLocker); ok {
		locker.lockDirtySync()
		return locker.unlockDirtySync
	}
	return func() {}
}

func validateDiskPair(source Disk, target Disk) (uint64, error) {
	if err := validateDiskRefs(source, target); err != nil {
		return 0, err
	}
	sourceSize := source.Size()
	if sourceSize != target.Size() {
		return 0, errors.New("source and target disk sizes differ")
	}
	return sourceSize, nil
}

func validateDiskRefs(source Disk, target Disk) error {
	if isNilDisk(source) {
		return errors.New("source disk is nil")
	}
	if isNilDisk(target) {
		return errors.New("target disk is nil")
	}
	return nil
}

func isNilDisk(d Disk) bool {
	if d == nil {
		return true
	}
	value := reflect.ValueOf(d)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func copyRange(source Disk, target Disk, r BlockRange) error {
	data, err := source.ReadRange(r)
	if err != nil {
		return err
	}
	return target.WriteRange(r, data)
}
