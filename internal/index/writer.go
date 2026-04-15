package index

import (
	"bufio"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"

	"grepturbo/internal/trigram"
)

const slotSize = 8 // 4 bytes trigram value + 4 bytes offset into postings.dat

// Write serializes the built index to 3 files in dir:
//
//	postings.dat — sequential posting list records: [count uint32][fileID uint32 ...]
//	lookup.idx   — fixed-size open-addressing hash table: [trigram uint32][offset uint32]
//	files.idx    — one filepath per line; line number (0-based) == fileID
func Write(b *Builder, dir string) (err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// ── Step 1: write postings.dat, collect trigram → byte offset ──────────
	postingsPath := filepath.Join(dir, "postings.dat")
	pf, err := os.Create(postingsPath)
	if err != nil {
		return err
	}
	defer pf.Close()

	// Use buffered writer to batch small writes into larger kernel calls.
	// Without this, each 4-byte write (count + each fileID) is a separate syscall.
	w := bufio.NewWriter(pf)

	// offsets maps each trigram to its starting byte offset in postings.dat
	offsets := make(map[trigram.T]uint32)
	var cursor uint32

	buf := make([]byte, 4)

	for t, ids := range b.Posts {
		offsets[t] = cursor

		// Write count
		binary.LittleEndian.PutUint32(buf, uint32(len(ids)))
		if _, err := w.Write(buf); err != nil {
			return err
		}
		cursor += 4

		// Write each fileID
		for _, id := range ids {
			binary.LittleEndian.PutUint32(buf, id)
			if _, err := w.Write(buf); err != nil {
				return err
			}
			cursor += 4
		}
	}

	// Flush the buffered writer to ensure all postings data is written to disk.
	// This must happen before we start writing lookup.idx, since we need the
	// final file offsets to be stable.
	if err := w.Flush(); err != nil {
		return err
	}

	// ── Step 2: write lookup.idx (open-addressing hash table) ──────────────
	//
	// numSlots is 1.5x the number of unique trigrams so the load factor stays
	// around 0.67, keeping average probe length short.
	numSlots := uint32(len(offsets)*3/2) + 1

	lookupPath := filepath.Join(dir, "lookup.idx")
	lf, err := os.Create(lookupPath)
	if err != nil {
		return err
	}
	defer lf.Close()

	// Allocate the table in memory, then write it out in one pass.
	// Each slot: [trigramValue uint32][offset uint32] = 8 bytes.
	// A zero trigramValue means the slot is empty (trigram 0x000000 = NUL NUL NUL
	// never appears in source files, so this is safe).
	table := make([]byte, numSlots*slotSize)

	for t, off := range offsets {
		slot := uint32(t) % numSlots

		// Linear probing: find the next empty slot
		for {
			base := slot * slotSize
			if binary.LittleEndian.Uint32(table[base:]) == 0 {
				// Empty slot — claim it
				binary.LittleEndian.PutUint32(table[base:], uint32(t))
				binary.LittleEndian.PutUint32(table[base+4:], off)
				break
			}
			slot = (slot + 1) % numSlots
		}
	}

	if _, err := lf.Write(table); err != nil {
		return err
	}

	// Write numSlots as a 4-byte footer so the reader knows the table size
	binary.LittleEndian.PutUint32(buf, numSlots)
	if _, err := lf.Write(buf); err != nil {
		return err
	}

	// ── Step 3: write files.idx ─────────────────────────────────────────────
	filesPath := filepath.Join(dir, "files.idx")
	ff, err := os.Create(filesPath)
	if err != nil {
		return err
	}
	defer ff.Close()

	if _, err := ff.WriteString(strings.Join(b.Files, "\n")); err != nil {
		return err
	}

	return nil
}
