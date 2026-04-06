package query

import (
	"bufio"
	"fmt"
	"os"
	"regexp"

	"fastregex/internal/index"
	"fastregex/internal/posting"
)

// Match represents a single line match within a file.
type Match struct {
	File string
	Line int
	Text string
}

// Search runs a full regex query against the index:
//  1. Decompose pattern into trigrams
//  2. Look up each trigram in the index → posting lists
//  3. Intersect posting lists → candidate file IDs
//  4. Run the compiled regex only on candidate files
//  5. Return all line-level matches
func Search(r *index.Reader, pattern string) ([]Match, error) {
	// ── Step 1: decompose regex into trigrams ────────────────────────────────
	decomp, err := Decompose(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	// ── Step 2: compile the real regex ──────────────────────────────────────
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile pattern: %w", err)
	}

	// ── Step 3: determine candidate files ───────────────────────────────────
	var candidates []string

	if decomp.Wildcard || len(decomp.Trigrams) == 0 {
		// No trigrams extracted — must scan every file
		candidates = r.Files
	} else {
		// Look up each trigram and collect its posting list
		var lists [][]uint32
		for _, t := range decomp.Trigrams {
			ids, err := r.Lookup(t)
			if err != nil {
				return nil, fmt.Errorf("lookup trigram %s: %w", t, err)
			}
			if len(ids) == 0 {
				// This trigram appears in no file — no candidates possible
				return nil, nil
			}
			lists = append(lists, ids)
		}

		// Intersect all posting lists → file IDs that contain every trigram
		fileIDs := posting.Intersect(lists...)
		if len(fileIDs) == 0 {
			return nil, nil
		}

		// Resolve file IDs to paths
		candidates = make([]string, 0, len(fileIDs))
		for _, id := range fileIDs {
			if int(id) < len(r.Files) {
				candidates = append(candidates, r.Files[id])
			}
		}
	}

	// ── Step 4: run regex on candidate files only ────────────────────────────
	var matches []Match
	for _, path := range candidates {
		ms, err := grep(re, path)
		if err != nil {
			// File may have been deleted since indexing — skip it
			continue
		}
		matches = append(matches, ms...)
	}

	return matches, nil
}

// grep scans file at path line by line and returns all lines matching re.
func grep(re *regexp.Regexp, path string) ([]Match, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []Match
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, Match{
				File: path,
				Line: lineNum,
				Text: line,
			})
		}
	}

	return matches, scanner.Err()
}
