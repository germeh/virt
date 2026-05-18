package replication

import (
	"errors"
	"sync"
)

type MemoryDisk struct {
	mu              sync.Mutex
	dirtySyncMu     sync.Mutex
	data            []byte
	dirty           []dirtyEntry
	writeSeq        uint64
	nextSnapshotID  uint64
	activeSnapshots map[uint64]uint64
}

type dirtyEntry struct {
	Range BlockRange
	Seq   uint64
}

func NewMemoryDisk(data []byte) *MemoryDisk {
	copyData := append([]byte(nil), data...)
	return &MemoryDisk{data: copyData}
}

func (d *MemoryDisk) Size() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return uint64(len(d.data))
}

func (d *MemoryDisk) Bytes() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]byte(nil), d.data...)
}

func (d *MemoryDisk) ReadRange(r BlockRange) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	end, ok := checkedEnd(r)
	if !ok || end > uint64(len(d.data)) {
		return nil, errors.New("read range exceeds disk size")
	}
	return append([]byte(nil), d.data[r.Offset:end]...), nil
}

func (d *MemoryDisk) WriteRange(r BlockRange, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	end, ok := checkedEnd(r)
	if !ok || end > uint64(len(d.data)) {
		return errors.New("write range exceeds disk size")
	}
	if uint64(len(data)) != r.Length {
		return errors.New("write data length does not match range length")
	}
	copy(d.data[r.Offset:end], data)
	return nil
}

func (d *MemoryDisk) WriteAtBytes(data []byte, offset uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r := BlockRange{Offset: offset, Length: uint64(len(data))}
	end, ok := checkedEnd(r)
	if !ok || end > uint64(len(d.data)) {
		panic("write exceeds disk size")
	}
	if r.Length == 0 {
		return
	}
	if d.writeSeq == ^uint64(0) {
		panic("dirty write sequence exhausted")
	}
	copy(d.data[offset:end], data)
	d.writeSeq++
	d.dirty = append(d.dirty, dirtyEntry{Range: r, Seq: d.writeSeq})
}

func (d *MemoryDisk) DirtyRanges() []BlockRange {
	d.mu.Lock()
	defer d.mu.Unlock()
	return dirtyEntryRanges(d.dirty)
}

func (d *MemoryDisk) lockDirtySync() {
	d.dirtySyncMu.Lock()
}

func (d *MemoryDisk) unlockDirtySync() {
	d.dirtySyncMu.Unlock()
}

func (d *MemoryDisk) snapshotDirtyRanges() dirtySnapshot {
	d.mu.Lock()
	ranges := dirtyEntryRanges(d.dirty)
	if len(ranges) == 0 {
		d.mu.Unlock()
		return dirtySnapshot{ranges: ranges, discard: func() {}}
	}
	if d.nextSnapshotID == ^uint64(0) {
		d.mu.Unlock()
		panic("dirty snapshot sequence exhausted")
	}
	cutoff := d.writeSeq
	lowerBound := d.activeSnapshotLowerBoundLocked()
	d.nextSnapshotID++
	snapshotID := d.nextSnapshotID
	if d.activeSnapshots == nil {
		d.activeSnapshots = make(map[uint64]uint64)
	}
	d.activeSnapshots[snapshotID] = cutoff
	d.mu.Unlock()

	return dirtySnapshot{
		ranges: ranges,
		clear: func(ranges []BlockRange) {
			d.clearDirtySnapshot(snapshotID, ranges, lowerBound, cutoff)
		},
		discard: func() {
			d.discardSnapshot(snapshotID)
		},
	}
}

func (d *MemoryDisk) activeSnapshotLowerBoundLocked() uint64 {
	var lowerBound uint64
	for _, cutoff := range d.activeSnapshots {
		if cutoff > lowerBound {
			lowerBound = cutoff
		}
	}
	return lowerBound
}

func (d *MemoryDisk) clearDirtySnapshot(snapshotID uint64, ranges []BlockRange, lowerBound uint64, cutoff uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.activeSnapshots[snapshotID]; !ok {
		return
	}
	delete(d.activeSnapshots, snapshotID)
	d.dirty = clearDirtyEntriesBetween(d.dirty, ranges, lowerBound, cutoff)
}

func (d *MemoryDisk) discardSnapshot(snapshotID uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.activeSnapshots, snapshotID)
}

func (d *MemoryDisk) ClearDirtyRanges(ranges []BlockRange) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dirty = clearDirtyEntriesBetween(d.dirty, ranges, 0, d.writeSeq)
}

func dirtyEntryRanges(entries []dirtyEntry) []BlockRange {
	ranges := make([]BlockRange, 0, len(entries))
	for _, entry := range entries {
		ranges = append(ranges, entry.Range)
	}
	return NormalizeRanges(ranges)
}

func clearDirtyEntriesBetween(entries []dirtyEntry, cleared []BlockRange, lowerBound uint64, cutoff uint64) []dirtyEntry {
	acknowledged := NormalizeRanges(cleared)
	if len(entries) == 0 || len(acknowledged) == 0 {
		return entries
	}

	remaining := make([]dirtyEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Seq <= lowerBound || entry.Seq > cutoff {
			remaining = append(remaining, entry)
			continue
		}

		parts := []BlockRange{entry.Range}
		for _, clear := range acknowledged {
			parts = subtractOneRange(parts, clear)
			if len(parts) == 0 {
				break
			}
		}
		for _, part := range parts {
			remaining = append(remaining, dirtyEntry{Range: part, Seq: entry.Seq})
		}
	}
	return remaining
}

func subtractOneRange(ranges []BlockRange, clear BlockRange) []BlockRange {
	clearEnd, ok := checkedEnd(clear)
	if !ok {
		return ranges
	}
	result := make([]BlockRange, 0, len(ranges))
	for _, current := range ranges {
		currentEnd, ok := checkedEnd(current)
		if !ok {
			continue
		}
		if clearEnd <= current.Offset || clear.Offset >= currentEnd {
			result = append(result, current)
			continue
		}
		if clear.Offset > current.Offset {
			result = append(result, BlockRange{Offset: current.Offset, Length: clear.Offset - current.Offset})
		}
		if clearEnd < currentEnd {
			result = append(result, BlockRange{Offset: clearEnd, Length: currentEnd - clearEnd})
		}
	}
	return result
}
