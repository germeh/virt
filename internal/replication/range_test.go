package replication

import (
	"math"
	"reflect"
	"testing"
)

func TestNormalizeRangesSortsAndMerges(t *testing.T) {
	input := []BlockRange{
		{Offset: 30, Length: 10},
		{Offset: 0, Length: 10},
		{Offset: 10, Length: 5},
		{Offset: 14, Length: 10},
	}

	got := NormalizeRanges(input)
	want := []BlockRange{
		{Offset: 0, Length: 24},
		{Offset: 30, Length: 10},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestNormalizeRangesDropsZeroLength(t *testing.T) {
	got := NormalizeRanges([]BlockRange{
		{Offset: 0, Length: 0},
		{Offset: 5, Length: 5},
	})

	want := []BlockRange{{Offset: 5, Length: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestNormalizeRangesDropsOverflowingRanges(t *testing.T) {
	got := NormalizeRanges([]BlockRange{
		{Offset: math.MaxUint64 - 1, Length: 2},
		{Offset: 5, Length: 5},
	})

	want := []BlockRange{{Offset: 5, Length: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestNormalizeRangesReturnsNilForEmptyOrZeroOnly(t *testing.T) {
	tests := []struct {
		name  string
		input []BlockRange
	}{
		{name: "nil"},
		{name: "empty", input: []BlockRange{}},
		{name: "zero only", input: []BlockRange{{Offset: 0, Length: 0}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeRanges(tt.input); got != nil {
				t.Fatalf("got %#v, want nil", got)
			}
		})
	}
}

func TestNormalizeRangesMergesAdjacentRanges(t *testing.T) {
	got := NormalizeRanges([]BlockRange{
		{Offset: 0, Length: 10},
		{Offset: 10, Length: 5},
	})

	want := []BlockRange{{Offset: 0, Length: 15}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
