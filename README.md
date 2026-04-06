# fastregex

Index-accelerated regex search for large codebases. Builds a local trigram index over your codebase so regex queries skip irrelevant files entirely — instead of scanning every file like `grep`.

## Benchmark

Tested on the Go standard library source (~10,000 files):

| Tool | Time |
|---|---|
| `grep -rn` | 2.4 – 3.1s |
| `fastregex search` | 0.4 – 0.9s |

**~6–7x faster**, growing with codebase size. Repeated queries get faster as the OS caches the mmap'd index.

## Install

```bash
go install fastregex/cmd/fastregex@latest
```

Or build from source:

```bash
git clone https://github.com/yourname/fastregex
cd fastregex
go build -o fastregex ./cmd/fastregex
```

## Usage

**Step 1 — build the index** (once, or when files change):

```bash
fastregex build -root ./myproject -out .fastregex
```

**Step 2 — search:**

```bash
fastregex search -index .fastregex 'func.*Error'
```

Output format is `file:line:text`, same as `grep -n`:

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

## How it works

**Query flow:**
```
regex → extract trigrams → lookup index → intersect posting lists → candidate files → run regex on candidates only
```

1. **Trigram decomposition** — the regex `func.*Error` contains literals `func` and `Error`. These produce trigrams: `fun unc` and `Err rro ror`.
2. **Index lookup** — each trigram maps to a sorted list of file IDs that contain it (a posting list).
3. **Intersection** — only files containing *all* trigrams are candidates. On a 10k-file codebase this typically reduces candidates from 10,000 → ~50.
4. **Verification** — the real regex engine runs only on those 50 files.

**No false negatives** — if a file matches the regex, it will always appear in the candidate set. The index may produce false positives (extra candidates), but the regex engine filters those out.

### Index files

```
.fastregex/
  lookup.idx    mmap'd hash table: trigram → byte offset in postings.dat
  postings.dat  posting lists: [count][fileID, fileID, ...]
  files.idx     fileID → filepath mapping
```

## Development

```bash
# Run tests
go test ./...

# Run a specific test
go test ./internal/query/... -run TestCorrectnessVsGrep -v

# Run benchmarks
go test ./... -bench=.
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for full diagrams and design decisions.
