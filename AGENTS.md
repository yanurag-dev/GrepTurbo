# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/trigram/...
go test ./internal/posting/...

# Run a single test
go test ./internal/trigram/... -run TestExtract

# Run benchmarks
go test ./... -bench=.
```

---

## Architecture

FastRegex builds a local inverted index over a codebase so regex queries skip irrelevant files instead of scanning everything like `ripgrep`. See `ARCHITECTURE.md` for full diagrams.

**Query flow:** regex → trigram decomposition → posting list lookup → intersect → candidate files → run regex engine only on candidates.

### Build order (phases)

| Phase | Package | Purpose |
|---|---|---|
| 1 ✅ | `internal/trigram` | Pack 3-char sequences into `uint32`, extract from strings |
| 2 ✅ | `internal/posting` | Inverted index in memory: trigram → sorted `[]uint32` fileIDs, plus `Intersect` |
| 3 | `internal/index` | `builder.go` walks files + builds posting list; `writer.go` serializes to disk; `reader.go` mmap's lookup table |
| 4 | `internal/query` | `decompose.go` extracts required trigrams from a regex; `search.go` runs the full pipeline |
| 5 | `internal/sync` | Git commit baseline + dirty file overlay for incremental updates |
| 6 | `cmd/fastregex` | CLI entrypoint |

### On-disk format (3 files)

- `lookup.idx` — mmap'd fixed-size hash table: trigram value → byte offset in postings file. Each slot is 8 bytes (4 hash + 4 offset).
- `postings.dat` — sequential records: `[count uint32][fileID uint32 ...]`
- `files.idx` — fileID → file path mapping

### Key invariants

- **No false negatives**: if a file contains a regex match, it must appear in the candidate set. False positives (candidates that don't match) are acceptable.
- Hash collisions in `lookup.idx` are acceptable — they widen the candidate set but never lose results.
- Posting lists in `internal/posting` are always kept sorted to enable O(n) two-pointer intersection.
