<div align="center">

# fastregex

*Index-accelerated regex search. Skip irrelevant files entirely.*

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat)](LICENSE)
[![Build](https://img.shields.io/badge/Build-Passing-brightgreen?style=flat)]()
[![Speedup](https://img.shields.io/badge/Speedup-6--7x_faster-orange?style=flat)]()

</div>

---

> **fastregex** builds a local trigram index over your codebase so regex queries skip irrelevant files entirely — instead of scanning every byte like `grep`. The bigger your codebase, the bigger the win.

---

## Benchmark

Tested on the Go standard library source (~10,000 files):

| Tool | Time | Files Scanned |
|---|---|---|
| `grep -rn` | 2.4 – 3.1s | All 10,000 |
| `fastregex search` | 0.4 – 0.9s | ~50 candidates |

**6–7x faster** on 10k files. Grows with codebase size. Repeated queries get faster as the OS caches the mmap'd index in the page cache.

---

## Install

```bash
git clone https://github.com/yanurag-dev/fastregex
cd fastregex
go build -o fastregex ./cmd/fastregex
```

---

## Usage

**Step 1 — build the index** (once, or when files change):

```bash
fastregex build -root ./myproject -out .fastregex
```

**Step 2 — search:**

```bash
fastregex search -index .fastregex 'func.*Error'
```

Output is `file:line:text`, same as `grep -n`:

```
internal/index/reader.go:25:func NewReader(dir string) (*Reader, error) {
internal/query/search.go:26:func Search(r *index.Reader, pattern string) ([]Match, error) {
```

### Flags

```
fastregex build
  -root   <dir>    Directory to index (default: .)
  -out    <dir>    Where to write the index (default: .fastregex)

fastregex search
  -index  <dir>    Index directory to query (default: .fastregex)
```

---

## How It Works

```
regex → trigram decomposition → index lookup → intersect posting lists → candidate files → verify with regex
```

1. **Trigram decomposition** — `func.*Error` contains literals `func` and `Error`, producing trigrams `fun unc` and `Err rro ror`
2. **Index lookup** — each trigram maps to a sorted posting list of file IDs that contain it
3. **Intersection** — only files containing *all* required trigrams become candidates (10,000 → ~50)
4. **Verification** — the real regex engine runs only on those ~50 files

**The golden invariant:** if a file matches the regex, it will always appear in the candidate set. No false negatives, ever.

### Index on Disk

```
.fastregex/
  lookup.idx    mmap'd hash table — trigram → byte offset in postings.dat
  postings.dat  posting lists — [count][fileID, fileID, ...]
  files.idx     fileID → filepath mapping
```

Only `lookup.idx` is loaded into memory (mmap'd). Posting lists are read from disk on demand.

---

## Testing

Run the dynamic test script to benchmark and verify correctness against `grep`:

```bash
# Test default patterns on this repo
./scripts/test.sh

# Test a single pattern
./scripts/test.sh 'func.*Error'

# Test on any large codebase
./scripts/test.sh 'func.*Error' /path/to/large/repo
```

The script builds the binary, indexes the target directory, runs each pattern through both `grep` and `fastregex`, compares results, and reports speedup + any false negatives.

```bash
# Run unit + integration tests
go test ./...

# Run a specific test
go test ./internal/query/... -run TestCorrectnessVsGrep -v

# Run benchmarks
go test ./... -bench=.
```

---

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for full diagrams covering the index build pipeline, query flow, on-disk format, regex decomposition rules, and incremental sync strategy.

---

<div align="center">

Built with Go · MIT License

</div>
