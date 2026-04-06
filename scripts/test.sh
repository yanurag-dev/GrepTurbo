#!/usr/bin/env bash
# test.sh — build, index, search, and compare fastregex against grep
#
# Usage:
#   ./scripts/test.sh                          # run default patterns on this repo
#   ./scripts/test.sh 'func.*Error'            # single pattern on this repo
#   ./scripts/test.sh 'func.*Error' /some/dir  # single pattern on any directory
#   ./scripts/test.sh '' /some/dir             # default patterns on any directory

set -euo pipefail

# ── colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

# ── args ──────────────────────────────────────────────────────────────────────
PATTERN="${1:-}"
TARGET_DIR="${2:-$(pwd)}"
INDEX_DIR="$(mktemp -d)/fastregex-index"

# Default patterns when none provided
DEFAULT_PATTERNS=(
  "func"
  "error"
  "TODO"
  "func.*error"
  "return nil"
  "uint32"
)

# ── helpers ───────────────────────────────────────────────────────────────────
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BINARY="$REPO_ROOT/fastregex"

cleanup() {
  rm -rf "$INDEX_DIR" "$BINARY" 2>/dev/null || true
}
trap cleanup EXIT

elapsed() {
  # Returns wall time in seconds for a command
  { time "$@" > /tmp/fr_out 2>/dev/null ; } 2>&1 | grep real | awk '{print $2}' \
    | sed 's/m/*60+/' | sed 's/s//' | bc
}

print_header() {
  echo -e "\n${BOLD}${CYAN}══════════════════════════════════════════${RESET}"
  echo -e "${BOLD}${CYAN}  fastregex test runner${RESET}"
  echo -e "${BOLD}${CYAN}══════════════════════════════════════════${RESET}"
  echo -e "  Target : ${BOLD}$TARGET_DIR${RESET}"
  echo -e "  Index  : ${BOLD}$INDEX_DIR${RESET}"
  echo ""
}

# ── step 1: build binary ──────────────────────────────────────────────────────
build_binary() {
  echo -e "${BOLD}[1/3] Building fastregex...${RESET}"
  cd "$REPO_ROOT"
  if go build -o "$BINARY" ./cmd/fastregex 2>&1; then
    echo -e "${GREEN}      ✓ Build succeeded${RESET}"
  else
    echo -e "${RED}      ✗ Build failed${RESET}"
    exit 1
  fi
}

# ── step 2: build index ───────────────────────────────────────────────────────
build_index() {
  echo -e "\n${BOLD}[2/3] Building index for $TARGET_DIR...${RESET}"
  local output
  output=$("$BINARY" build -root "$TARGET_DIR" -out "$INDEX_DIR" 2>&1)
  echo -e "${GREEN}      ✓ $output${RESET}"
}

# ── step 3: run pattern tests ─────────────────────────────────────────────────
run_pattern() {
  local pattern="$1"
  local pass=true

  echo -e "\n${BOLD}  Pattern: ${YELLOW}$pattern${RESET}"
  echo -e "  ────────────────────────────────────"

  # ── fastregex ──
  local fr_start fr_end fr_time fr_count
  fr_start=$(date +%s%N)
  "$BINARY" search -index "$INDEX_DIR" "$pattern" > /tmp/fr_results 2>/dev/null || true
  fr_end=$(date +%s%N)
  fr_time=$(( (fr_end - fr_start) / 1000000 )) # ms
  fr_count=$(wc -l < /tmp/fr_results | tr -d ' ')

  # ── grep ──
  local grep_start grep_end grep_time grep_count
  grep_start=$(date +%s%N)
  grep -rn "$pattern" --include='*.go' "$TARGET_DIR" > /tmp/grep_results 2>/dev/null || true
  grep_end=$(date +%s%N)
  grep_time=$(( (grep_end - grep_start) / 1000000 )) # ms

  # Normalise grep output to match fastregex format for comparison
  grep_count=$(wc -l < /tmp/grep_results | tr -d ' ')

  # ── false negative check ──
  # Every grep match must appear in fastregex output
  local missed=0
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    # Extract file:linenum from grep output
    file=$(echo "$line" | cut -d: -f1)
    linenum=$(echo "$line" | cut -d: -f2)
    if ! grep -q "^$file:$linenum:" /tmp/fr_results 2>/dev/null; then
      missed=$((missed + 1))
    fi
  done < /tmp/grep_results

  # ── speedup ──
  local speedup="N/A"
  if [[ $fr_time -gt 0 && $grep_time -gt 0 ]]; then
    speedup=$(echo "scale=1; $grep_time / $fr_time" | bc)x
  fi

  # ── print results ──
  printf "  %-12s %6s ms   %5s matches\n" "grep"      "$grep_time" "$grep_count"
  printf "  %-12s %6s ms   %5s matches\n" "fastregex" "$fr_time"   "$fr_count"
  echo ""

  if [[ $fr_time -lt $grep_time ]]; then
    echo -e "  Speedup  : ${GREEN}${BOLD}${speedup} faster${RESET}"
  elif [[ $fr_time -eq $grep_time ]]; then
    echo -e "  Speedup  : ${YELLOW}same speed${RESET}"
  else
    echo -e "  Speedup  : ${RED}${speedup} slower${RESET} (index overhead on small corpus)"
  fi

  if [[ $missed -eq 0 ]]; then
    echo -e "  Correctness: ${GREEN}✓ no false negatives${RESET}"
  else
    echo -e "  Correctness: ${RED}✗ $missed false negatives (missed matches!)${RESET}"
    pass=false
  fi

  $pass
}

run_all_patterns() {
  echo -e "\n${BOLD}[3/3] Running pattern tests...${RESET}"

  local total=0 passed=0 failed=0

  if [[ -n "$PATTERN" ]]; then
    patterns=("$PATTERN")
  else
    patterns=("${DEFAULT_PATTERNS[@]}")
  fi

  for p in "${patterns[@]}"; do
    total=$((total + 1))
    if run_pattern "$p"; then
      passed=$((passed + 1))
    else
      failed=$((failed + 1))
    fi
  done

  # ── summary ──
  echo -e "\n${BOLD}${CYAN}══════════════════════════════════════════${RESET}"
  echo -e "${BOLD}  Summary: $total patterns tested${RESET}"
  echo -e "  ${GREEN}✓ $passed passed${RESET}"
  if [[ $failed -gt 0 ]]; then
    echo -e "  ${RED}✗ $failed failed (false negatives detected)${RESET}"
    echo -e "${BOLD}${CYAN}══════════════════════════════════════════${RESET}\n"
    exit 1
  else
    echo -e "${BOLD}${CYAN}══════════════════════════════════════════${RESET}\n"
  fi
}

# ── main ──────────────────────────────────────────────────────────────────────
print_header
build_binary
build_index
run_all_patterns
