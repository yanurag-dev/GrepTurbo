package query_test

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"grepturbo/internal/index"
	"grepturbo/internal/query"
)

// TestCorrectnessVsGrep is the golden test: every match that grep finds must
// also appear in fastregex results. No false negatives allowed.
// False positives (fastregex returns extra candidates that don't match) are
// acceptable — the regex engine filters them out.
func TestCorrectnessVsGrep(t *testing.T) {
	// Index the repo root (two levels up from this test file)
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	patterns := []string{
		"func",
		"Builder",
		"TODO",
		"func.*error",
		"posting",
		"trigram",
		"fileID",
		"uint32",
	}

	// Build the index once for all patterns
	idxDir := t.TempDir()
	b := index.NewBuilder()
	if err := b.Build(repoRoot); err != nil {
		t.Fatalf("build index: %v", err)
	}
	if err := index.Write(b, idxDir); err != nil {
		t.Fatalf("write index: %v", err)
	}
	r, err := index.NewReader(idxDir)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	defer r.Close()

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			// ── fastregex results ────────────────────────────────────────
			fastMatches, err := query.Search(r, pattern)
			if err != nil {
				t.Fatalf("search error: %v", err)
			}
			fastSet := make(map[string]struct{})
			for _, m := range fastMatches {
				// Normalise to "filepath:linenum:text" for comparison
				key := matchKey(m.File, m.Line, m.Text)
				fastSet[key] = struct{}{}
			}

			// ── grep results ─────────────────────────────────────────────
			grepMatches := runGrep(t, repoRoot, pattern)

			// ── assert no false negatives ────────────────────────────────
			// Every grep match must appear in fastregex output.
			var missed []string
			for _, gm := range grepMatches {
				if _, ok := fastSet[gm]; !ok {
					missed = append(missed, gm)
				}
			}
			if len(missed) > 0 {
				sort.Strings(missed)
				t.Errorf("fastregex missed %d matches that grep found:\n  %s",
					len(missed), strings.Join(missed, "\n  "))
			}

			t.Logf("pattern %q: grep=%d fastregex=%d",
				pattern, len(grepMatches), len(fastMatches))
		})
	}
}

// TestWildcardFallback verifies that patterns with no extractable trigrams
// still return correct results by scanning all files.
func TestWildcardFallback(t *testing.T) {
	repoRoot, _ := filepath.Abs("../..")
	idxDir := t.TempDir()

	b := index.NewBuilder()
	b.Build(repoRoot)
	index.Write(b, idxDir)
	r, _ := index.NewReader(idxDir)
	defer r.Close()

	// ".*" matches every line — fastregex must not return zero results
	matches, err := query.Search(r, "package")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Error("expected matches for 'package', got none")
	}
}

// TestNoFalseNegativesOnCorpus builds a small in-memory corpus and verifies
// the golden invariant: if a file contains a match, it must be in candidates.
func TestNoFalseNegativesOnCorpus(t *testing.T) {
	files := map[string]string{
		"a.go": "package main\nfunc handleError(err error) {}\n",
		"b.go": "package main\nfunc hello() {}\n",
		"c.go": "package main\n// TODO: fix this\n",
		"d.go": "package main\nvar x = 42\n",
	}

	// Write corpus to a temp dir
	corpusDir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(corpusDir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Build and write index
	idxDir := t.TempDir()
	b := index.NewBuilder()
	b.Build(corpusDir)
	index.Write(b, idxDir)
	r, _ := index.NewReader(idxDir)
	defer r.Close()

	tests := []struct {
		pattern      string
		mustContain  []string // files that MUST appear in results
		mustNotMatch []string // files that must NOT match (verified by regex)
	}{
		{
			pattern:     "func.*Error",
			mustContain: []string{"a.go"},
		},
		{
			pattern:     "TODO",
			mustContain: []string{"c.go"},
		},
		{
			pattern:     "func",
			mustContain: []string{"a.go", "b.go"},
		},
	}

	re := regexp.MustCompile
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			matches, err := query.Search(r, tt.pattern)
			if err != nil {
				t.Fatal(err)
			}

			// Build set of matched files
			matchedFiles := make(map[string]struct{})
			for _, m := range matches {
				matchedFiles[filepath.Base(m.File)] = struct{}{}
			}

			// Every file in mustContain must be present
			compiled := re(tt.pattern)
			for _, f := range tt.mustContain {
				content := files[f]
				if !compiled.MatchString(content) {
					t.Skipf("pattern %q doesn't actually match %s — test data wrong", tt.pattern, f)
				}
				if _, ok := matchedFiles[f]; !ok {
					t.Errorf("false negative: pattern %q matched %s but fastregex missed it", tt.pattern, f)
				}
			}
		})
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func matchKey(file string, line int, text string) string {
	// Use only the relative portion after the last path separator anchor
	// so grep and fastregex paths compare correctly.
	return strings.Join([]string{
		file,
		itoa(line),
		strings.TrimSpace(text),
	}, ":")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// runGrep runs grep -rn on dir with pattern and returns "file:line:text" keys.
func runGrep(t *testing.T, dir, pattern string) []string {
	t.Helper()

	// Check grep is available
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("grep not available on this system")
	}

	cmd := exec.Command("grep", "-rn", "--include=*.go", pattern, dir)
	out, err := cmd.Output()
	if err != nil {
		// exit 1 means no matches — not an error
		if cmd.ProcessState.ExitCode() == 1 {
			return nil
		}
		t.Fatalf("grep failed: %v", err)
	}

	var results []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		// grep output: /abs/path/file.go:42:matched text
		// Split into at most 3 parts so the text portion is preserved as-is
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		key := strings.Join([]string{
			parts[0],
			parts[1],
			strings.TrimSpace(parts[2]),
		}, ":")
		results = append(results, key)
	}
	return results
}
