package index

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitSync(t *testing.T) {
	// 1. Create a temporary git repo
	repoDir := t.TempDir()
	runCmd(t, repoDir, "git", "init")
	runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, repoDir, "git", "config", "user.name", "Test User")

	// 2. Add some files and commit
	file1 := filepath.Join(repoDir, "file1.go")
	os.WriteFile(file1, []byte("package main\nfunc A() {}\n"), 0644)
	runCmd(t, repoDir, "git", "add", "file1.go")
	runCmd(t, repoDir, "git", "commit", "-m", "initial commit")

	// 3. Build index
	idxDir := t.TempDir()
	b := NewBuilder()
	if err := b.Build(repoDir); err != nil {
		t.Fatal(err)
	}
	if err := Write(b, idxDir); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(idxDir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	// 4. Verify initial state (no changes)
	overlay, drift, err := r.Sync()
	if err != nil {
		t.Fatal(err)
	}
	if drift {
		t.Error("expected no drift")
	}
	if len(overlay.Files) != 0 || len(overlay.Tombstones) != 0 {
		t.Errorf("expected empty overlay, got files=%d tombstones=%d", len(overlay.Files), len(overlay.Tombstones))
	}

	// 5. Modify a file
	os.WriteFile(file1, []byte("package main\nfunc B() {}\n"), 0644)

		// 6. Verify sync detects modification
	overlay, drift, err = r.Sync()
	if err != nil {
		t.Fatal(err)
	}
	if len(overlay.Files) != 1 || overlay.Files[0] != file1 {
		t.Errorf("expected %s in overlay, got %v", file1, overlay.Files)
	}
	if !overlay.Tombstones[file1] {
		t.Errorf("expected %s in tombstones", file1)
	}

	// 7. Add an untracked file
	file2 := filepath.Join(repoDir, "file2.go")
	os.WriteFile(file2, []byte("package main\nfunc C() {}\n"), 0644)

	// 8. Verify sync detects untracked file
	overlay, _, _ = r.Sync()
	foundFile2 := false
	for _, f := range overlay.Files {
		if f == file2 {
			foundFile2 = true
			break
		}
	}
	if !foundFile2 {
		t.Errorf("expected %s in overlay", file2)
	}

	// 9. Detect Commit Drift (commit the changes)
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "second commit")

	_, drift, _ = r.Sync()
	if !drift {
		t.Error("expected drift after new commit")
	}
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cmd failed: %s\nOutput: %s", err, string(out))
	}
}
