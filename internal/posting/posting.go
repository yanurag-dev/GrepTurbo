package posting

import (
	"sort"

	"grepturbo/internal/trigram"
)

// List maps each trigram to a sorted slice of file IDs that contain it.
type List map[trigram.T][]uint32

// AddBatch appends fileIDs without sorting.
// Call Finalize() after all AddBatch calls to sort and dedup.
func (l List) AddBatch(t trigram.T, fileIDs []uint32) {
	existing := l[t]
	l[t] = append(existing, fileIDs...)
}

// Finalize sorts and deduplicates all posting lists in-place.
// Call after all AddBatch calls are done.
func (l List) Finalize() {
	for t, ids := range l {
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		out := ids[:0]
		for _, id := range ids {
			if len(out) == 0 || out[len(out)-1] != id {
				out = append(out, id)
			}
		}
		l[t] = out
	}
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
