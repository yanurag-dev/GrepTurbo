package index

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"fastregex/internal/trigram"
)

// Reader holds the mmap'd lookup table and an open handle to postings.dat.
// Use NewReader to open, and Close when done.
type Reader struct {
	table    []byte   // mmap'd contents of lookup.idx
	numSlots uint32   // number of slots in the hash table
	postings *os.File // open handle to postings.dat for random reads
	Files    []string // fileID → filepath
}

// NewReader opens the index written by Write and mmap's the lookup table.
func NewReader(dir string) (*Reader, error) {
	// ── lookup.idx ──────────────────────────────────────────────────────────
	lookupPath := filepath.Join(dir, "lookup.idx")
	lf, err := os.Open(lookupPath)
	if err != nil {
		return nil, fmt.Errorf("open lookup.idx: %w", err)
	}
	defer lf.Close()

	info, err := lf.Stat()
	if err != nil {
		return nil, err
	}
	size := int(info.Size())
	if size < 4 {
		return nil, fmt.Errorf("lookup.idx too small")
	}

	// mmap the entire file into our address space.
	// PROT_READ: read-only. MAP_SHARED: changes (none) would be visible to other processes.
	table, err := unix.Mmap(int(lf.Fd()), 0, size, unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("mmap lookup.idx: %w", err)
	}

	// numSlots is stored as the last 4 bytes of the file
	numSlots := binary.LittleEndian.Uint32(table[size-4:])

	// ── postings.dat ────────────────────────────────────────────────────────
	postingsPath := filepath.Join(dir, "postings.dat")
	pf, err := os.Open(postingsPath)
	if err != nil {
		unix.Munmap(table)
		return nil, fmt.Errorf("open postings.dat: %w", err)
	}

	// ── files.idx ───────────────────────────────────────────────────────────
	filesPath := filepath.Join(dir, "files.idx")
	data, err := os.ReadFile(filesPath)
	if err != nil {
		unix.Munmap(table)
		pf.Close()
		return nil, fmt.Errorf("open files.idx: %w", err)
	}

	var files []string
	if len(data) > 0 {
		files = strings.Split(string(data), "\n")
	}

	return &Reader{
		table:    table,
		numSlots: numSlots,
		postings: pf,
		Files:    files,
	}, nil
}

// Lookup returns the sorted file IDs for trigram t, or nil if not found.
func (r *Reader) Lookup(t trigram.T) ([]uint32, error) {
	if r.numSlots == 0 {
		return nil, nil
	}

	slot := uint32(t) % r.numSlots

	// Linear probe until we find the trigram or an empty slot
	for {
		base := slot * slotSize
		stored := binary.LittleEndian.Uint32(r.table[base:])

		if stored == 0 {
			// Empty slot — trigram not in index
			return nil, nil
		}

		if trigram.T(stored) == t {
			// Found — read the posting list at this offset
			offset := int64(binary.LittleEndian.Uint32(r.table[base+4:]))
			return r.readPostings(offset)
		}

		// Collision — probe next slot
		slot = (slot + 1) % r.numSlots
	}
}

// readPostings reads a posting list from postings.dat at the given byte offset.
// Format: [count uint32][fileID uint32 ...]
func (r *Reader) readPostings(offset int64) ([]uint32, error) {
	buf := make([]byte, 4)

	if _, err := r.postings.ReadAt(buf, offset); err != nil {
		return nil, fmt.Errorf("read postings count at %d: %w", offset, err)
	}
	count := binary.LittleEndian.Uint32(buf)

	if count == 0 {
		return nil, nil
	}

	ids := make([]uint32, count)
	data := make([]byte, count*4)
	if _, err := r.postings.ReadAt(data, offset+4); err != nil {
		return nil, fmt.Errorf("read postings data at %d: %w", offset+4, err)
	}
	for i := range ids {
		ids[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	return ids, nil
}

// Close unmaps the lookup table and closes the postings file.
func (r *Reader) Close() error {
	r.postings.Close()
	return unix.Munmap(r.table)
}
