package index

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Metadata contains information about the built Baseline index.
type Metadata struct {
	Commit  string   `json:"commit"`
	RootDir string   `json:"root_dir"`
	Skip    []string `json:"skip"`
}

// WriteMetadata saves index metadata to metadata.json in dir.
func WriteMetadata(dir, rootDir string, skip []string) error {
	commit, err := CurrentCommit(rootDir)
	if err != nil {
		commit = "unknown"
	}

	m := Metadata{
		Commit:  commit,
		RootDir: rootDir,
		Skip:    skip,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "metadata.json"), data, 0644)
}

// ReadMetadata loads index metadata from metadata.json in dir.
func ReadMetadata(dir string) (*Metadata, error) {
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return nil, err
	}

	var m Metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// CurrentCommit returns the current Git HEAD commit hash in the given dir.
func CurrentCommit(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
