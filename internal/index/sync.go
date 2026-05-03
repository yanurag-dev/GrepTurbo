package index

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"grepturbo/internal/posting"
	"grepturbo/internal/trigram"
)

// GitStatus holds the lists of files that have changed since the baseline.
type GitStatus struct {
	Modified  []string
	Untracked []string
	Deleted   []string
}

// Overlay holds the transient in-memory index of dirty files.
type Overlay struct {
	Posts      posting.List
	Files      []string        // fileID → filepath (starts from len(Baseline.Files))
	Tombstones map[string]bool // paths that should be ignored from Baseline
}

// Sync performs a Git-based synchronization.
func (r *Reader) Sync() (*Overlay, bool, error) {
	current, err := CurrentCommit(r.Meta.RootDir)
	if err != nil {
		// Not in a git repo (or git not installed) — nothing to sync.
		return &Overlay{
			Posts:      make(posting.List),
			Tombstones: make(map[string]bool),
		}, false, nil
	}

	// Commit Drift detected
	if r.Meta.Commit != current && r.Meta.Commit != "unknown" {
		return nil, true, nil
	}

	status, err := GetGitStatus(r.Meta.RootDir)
	if err != nil {
		return nil, false, err
	}

	overlay := &Overlay{
		Posts:      make(posting.List),
		Tombstones: make(map[string]bool),
	}

	// Deleted files are Tombstones
	for _, p := range status.Deleted {
		overlay.Tombstones[filepath.Join(r.Meta.RootDir, p)] = true
	}

	// Modified files are both Tombstones (hide old version) and Indexed (show new version)
	dirtyFiles := append(status.Modified, status.Untracked...)
	for _, p := range status.Modified {
		overlay.Tombstones[filepath.Join(r.Meta.RootDir, p)] = true
	}

	// Index dirty files in memory
	for _, relPath := range dirtyFiles {
		absPath := filepath.Join(r.Meta.RootDir, relPath)
		data, err := os.ReadFile(absPath)
		if err != nil {
			continue // skip files we can't read
		}
		if !utf8.Valid(data) || len(data) > maxFileSize {
			continue
		}

		fileID := uint32(len(r.Files) + len(overlay.Files))
		overlay.Files = append(overlay.Files, absPath)

		for _, t := range trigram.Extract(string(data)) {
			overlay.Posts.AddBatch(t, []uint32{fileID})
		}
	}
	overlay.Posts.Finalize()

	return overlay, false, nil
}

// GetGitStatus runs 'git status --porcelain' in dir and returns the categorized files.
func GetGitStatus(dir string) (*GitStatus, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	status := &GitStatus{}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}

		// git status --porcelain format: "XY PATH"
		xy := line[:2]
		path := line[3:]

		switch {
		case xy == "??":
			status.Untracked = append(status.Untracked, path)
		case strings.Contains(xy, "D"):
			status.Deleted = append(status.Deleted, path)
		case strings.Contains(xy, "M") || strings.Contains(xy, "A"):
			status.Modified = append(status.Modified, path)
		}
	}

	return status, scanner.Err()
}
