#!/usr/bin/env bash
# benchmark_build.sh — measure build phase timing before/after concurrency changes
#
# Usage:
#   ./scripts/benchmark_build.sh <target-repo-path> [runs]
#
# Examples:
#   ./scripts/benchmark_build.sh /tmp/linux 3
#   ./scripts/benchmark_build.sh . 5
#
# Output:
#   Prints a results table to stdout AND appends it to scripts/benchmark_results.md

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${1:-}"
RUNS="${2:-3}"
RESULTS_FILE="$REPO_ROOT/scripts/benchmark_results.md"
BENCH_BINARY="$REPO_ROOT/scripts/_benchcmd"

die()  { echo "ERROR: $*" >&2; exit 1; }
require() { for cmd in "$@"; do command -v "$cmd" >/dev/null 2>&1 || die "'$cmd' not found"; done; }

if [[ -z "$TARGET" ]]; then
  cat >&2 <<'USAGE'
Usage: ./scripts/benchmark_build.sh <target-repo-path> [runs]

Need a large repo to index. Suggestions:
  git clone --depth=1 https://github.com/torvalds/linux     /tmp/linux
  git clone --depth=1 https://github.com/kubernetes/kubernetes /tmp/k8s
  git clone --depth=1 https://github.com/golang/go           /tmp/go-src

Then run:
  ./scripts/benchmark_build.sh /tmp/linux 3
USAGE
  exit 1
fi

[[ -d "$TARGET" ]] || die "Target directory not found: $TARGET"
require go python3

# ── build the benchmark binary ────────────────────────────────────────────────

echo "→ Compiling benchmark binary..."
go build -o "$BENCH_BINARY" "$REPO_ROOT/scripts/benchcmd/"
echo "  done: $BENCH_BINARY"
echo ""

# ── metadata ──────────────────────────────────────────────────────────────────

GIT_SHA=$(cd "$REPO_ROOT" && git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH=$(cd "$REPO_ROOT" && git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
TARGET_ABS=$(cd "$TARGET" && pwd)
TARGET_NAME=$(basename "$TARGET_ABS")

echo "Target : $TARGET_ABS"
echo "Runs   : $RUNS (+ 1 warmup)"
echo "Commit : $GIT_SHA ($GIT_BRANCH)"
echo ""

# ── run benchmark ─────────────────────────────────────────────────────────────

IDX_DIR=$(mktemp -d /tmp/grepturbo_bench_XXXXXX)
trap "rm -rf $IDX_DIR $BENCH_BINARY" EXIT

declare -a walk_times=()
declare -a finalize_times=()
declare -a write_times=()
declare -a total_times=()
FILES=0
TRIGRAMS=0

parse() { python3 -c "import sys,json; d=json.loads(sys.stdin.read()); print(d['$1'])"; }

for i in $(seq 0 "$RUNS"); do
  rm -rf "$IDX_DIR" && mkdir -p "$IDX_DIR"

  RESULT=$("$BENCH_BINARY" -root "$TARGET_ABS" -out "$IDX_DIR" 2>/dev/null)

  FILES=$(echo "$RESULT"    | parse files)
  TRIGRAMS=$(echo "$RESULT" | parse trigrams)
  WALK=$(echo "$RESULT"     | parse walk_extract_ms)
  FINALIZE=$(echo "$RESULT" | parse finalize_ms)
  WRITE=$(echo "$RESULT"    | parse write_ms)
  TOTAL=$(echo "$RESULT"    | parse total_ms)

  if [[ $i -eq 0 ]]; then
    printf "  run %-2d (warmup, discarded) — total=%s ms\n" "$i" "$TOTAL"
    continue
  fi

  walk_times+=("$WALK")
  finalize_times+=("$FINALIZE")
  write_times+=("$WRITE")
  total_times+=("$TOTAL")

  printf "  run %-2d  walk=%-10s  finalize=%-10s  write=%-10s  total=%s ms\n" \
    "$i" "$WALK" "$FINALIZE" "$WRITE" "$TOTAL"
done

echo ""

# ── averages ──────────────────────────────────────────────────────────────────

avg() {
  python3 -c "
nums = [float(x) for x in '''$*'''.split()]
print(f'{sum(nums)/len(nums):.1f}')
"
}

AVG_WALK=$(avg "${walk_times[@]}")
AVG_FINALIZE=$(avg "${finalize_times[@]}")
AVG_WRITE=$(avg "${write_times[@]}")
AVG_TOTAL=$(avg "${total_times[@]}")

# ── print & save table ────────────────────────────────────────────────────────

TABLE="## Run: $TIMESTAMP

| Field              | Value                      |
|--------------------|----------------------------|
| Commit             | \`$GIT_SHA\` ($GIT_BRANCH) |
| Target repo        | $TARGET_NAME               |
| Files indexed      | $FILES                     |
| Unique trigrams    | $TRIGRAMS                  |
| Runs (excl warmup) | $RUNS                      |

| Phase          | Avg (ms)          | What happens here                  |
|----------------|-------------------|------------------------------------|
| Walk + Extract | $AVG_WALK         | ReadFile + trigram.Extract per file |
| Finalize       | $AVG_FINALIZE     | sort + dedup all posting lists     |
| Write          | $AVG_WRITE        | write 3 index files to disk        |
| **Total**      | **$AVG_TOTAL**    |                                    |
"

echo "$TABLE"

if [[ ! -f "$RESULTS_FILE" ]]; then
  cat > "$RESULTS_FILE" <<'HDR'
# Build Phase Benchmark Results

Run `./scripts/benchmark_build.sh <repo> <n>` before and after concurrency
changes to compare. Each run appends a new section.

---

HDR
fi

printf '%s\n---\n\n' "$TABLE" >> "$RESULTS_FILE"
echo "Results appended to: $RESULTS_FILE"
