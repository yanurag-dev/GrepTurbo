package index

import (
	"io/fs"
	"os"
	"path/filepath"
	"unicode/utf8"

	"fastregex/internal/posting"
	"fastregex/internal/trigram"
)

const maxFileSize = 1 << 20 // 1 MB — skip files larger than this

// Builder walks a directory, extracts trigrams from each file,
// and accumulates an in-memory posting list.
type Builder struct {
	Posts posting.List // trigram → sorted []fileID
	Files []string     // fileID → filepath (index == fileID)
}

func NewBuilder() *Builder {
	return &Builder{
		Posts: make(posting.List),
	}
}

// Add indexes a single file. It reads the file content, extracts trigrams,
// and records the trigram → fileID mapping in the posting list.
// Returns the fileID assigned to this file, or an error.
func (b *Builder) Add(path string) (uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	// Skip binary files — valid source files are UTF-8 text
	if !utf8.Valid(data) {
		return 0, nil
	}

	// Skip large files — they blow up the index with common trigrams
	if len(data) > maxFileSize {
		return 0, nil
	}

	fileID := uint32(len(b.Files))
	b.Files = append(b.Files, path)

	for _, t := range trigram.Extract(string(data)) {
		b.Posts.Add(t, fileID)
	}

	return fileID, nil
}

// defaultSkipDirs are directories that should never be indexed.
var defaultSkipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".hg":          true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".fastregex":   true,
}

// Build walks all files under rootDir and indexes each one.
// Directories listed in skip are skipped entirely (e.g. "node_modules").
// Directories and files that fail to read are silently skipped.
func (b *Builder) Build(rootDir string, skip ...string) error {
	skipSet := make(map[string]bool)
	for k, v := range defaultSkipDirs {
		skipSet[k] = v
	}
	for _, s := range skip {
		skipSet[s] = true
	}

	return filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipSet[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		b.Add(path)
		return nil
	})
}
