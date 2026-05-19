package replication

import (
	"math"
	"sort"
)

type BlockRange struct {
	Offset uint64 `json:"offset"`
	Length uint64 `json:"length"`
}

func (r BlockRange) End() uint64 {
	return r.Offset + r.Length
}

func checkedEnd(r BlockRange) (uint64, bool) {
	if r.Length > math.MaxUint64-r.Offset {
		return 0, false
	}
	return r.End(), true
}

func NormalizeRanges(input []BlockRange) []BlockRange {
	filtered := make([]BlockRange, 0, len(input))
	for _, r := range input {
		if r.Length > 0 {
			if _, ok := checkedEnd(r); !ok {
				continue
			}
			filtered = append(filtered, r)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Offset == filtered[j].Offset {
			return filtered[i].Length < filtered[j].Length
		}
		return filtered[i].Offset < filtered[j].Offset
	})
	if len(filtered) == 0 {
		return nil
	}
	merged := []BlockRange{filtered[0]}
	lastEnd, _ := checkedEnd(merged[0])
	for _, current := range filtered[1:] {
		currentEnd, _ := checkedEnd(current)
		last := &merged[len(merged)-1]
		if current.Offset <= lastEnd {
			if currentEnd > lastEnd {
				last.Length = currentEnd - last.Offset
				lastEnd = currentEnd
			}
			continue
		}
		merged = append(merged, current)
		lastEnd = currentEnd
	}
	return merged
}
