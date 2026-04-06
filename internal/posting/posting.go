package posting

import (
	"sort"

	"fastregex/internal/trigram"
)

// List maps each trigram to a sorted slice of file IDs that contain it.
type List map[trigram.T][]uint32

// Add records that fileID contains trigram t.
// The slice is kept sorted and deduplicated.
func (l List) Add(t trigram.T, fileID uint32) {
	ids := l[t]

	// Find the insertion point using binary search
	pos := sort.Search(len(ids), func(i int) bool {
		return ids[i] >= fileID
	})

	// Already present — skip
	if pos < len(ids) && ids[pos] == fileID {
		return
	}

	// Insert at pos, shifting elements right
	ids = append(ids, 0)
	copy(ids[pos+1:], ids[pos:])
	ids[pos] = fileID
	l[t] = ids
}

// Get returns the sorted file IDs for trigram t, or nil if none.
func (l List) Get(t trigram.T) []uint32 {
	return l[t]
}

// Intersect returns file IDs present in ALL provided slices.
// All input slices must be sorted. Uses a two-pointer merge for O(n) per pair.
func Intersect(lists ...[]uint32) []uint32 {
	if len(lists) == 0 {
		return nil
	}
	if len(lists) == 1 {
		out := make([]uint32, len(lists[0]))
		copy(out, lists[0])
		return out
	}

	// Start with the shortest list to minimise work
	sort.Slice(lists, func(i, j int) bool {
		return len(lists[i]) < len(lists[j])
	})

	result := lists[0]
	for _, next := range lists[1:] {
		result = intersectTwo(result, next)
		if len(result) == 0 {
			return nil
		}
	}
	return result
}

// intersectTwo returns elements present in both a and b (both sorted).
func intersectTwo(a, b []uint32) []uint32 {
	out := make([]uint32, 0, min(len(a), len(b)))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, a[i])
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return out
}
