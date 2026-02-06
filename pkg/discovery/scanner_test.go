package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanner(t *testing.T) {
	// Setup temp directory structure
	tmpDir := t.TempDir()

	// Structure:
	// /src
	//   /project-a (git)
	//   /project-b (bare)
	//   /group
	//     /project-c (git)
	//   /node_modules
	//     /ignored-project (git)
	//   /symlink-to-a -> project-a

	srcDir := filepath.Join(tmpDir, "src")
	mustMkdir(t, srcDir)

	// project-a (standard)
	projA := filepath.Join(srcDir, "project-a")
	mustMkdir(t, projA)
	mustMkdir(t, filepath.Join(projA, ".git"))

	// project-b (bare)
	projB := filepath.Join(srcDir, "project-b")
	mustMkdir(t, projB)
	mustMkdir(t, filepath.Join(projB, "objects"))
	mustCreateFile(t, filepath.Join(projB, "HEAD"))
	mustCreateFile(t, filepath.Join(projB, "config"))

	// project-c (nested)
	group := filepath.Join(srcDir, "group")
	mustMkdir(t, group)
	projC := filepath.Join(group, "project-c")
	mustMkdir(t, projC)
	mustMkdir(t, filepath.Join(projC, ".git"))

	// ignored
	ignored := filepath.Join(srcDir, "node_modules", "ignored-project")
	mustMkdir(t, filepath.Dir(ignored))
	mustMkdir(t, ignored)
	mustMkdir(t, filepath.Join(ignored, ".git"))

	// symlink (named z-link-to-a to ensure project-a is visited first lexically)
	symlink := filepath.Join(srcDir, "z-link-to-a")
	if err := os.Symlink(filepath.Join(srcDir, "project-a"), symlink); err != nil {
		t.Logf("Skipping symlink test on platform: %v", err)
	}

	// Scanner
	scanner := NewScanner([]string{srcDir}, 3)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify
	found := make(map[string]bool)
	for _, p := range result.Projects {
		found[filepath.Base(p.Path)] = true
	}

	if !found["project-a"] {
		t.Error("Did not find project-a")
	}
	if !found["project-b"] {
		t.Error("Did not find project-b")
	}
	if !found["project-c"] {
		t.Error("Did not find project-c")
	}
	if found["ignored-project"] {
		t.Error("Found ignored-project")
	}

	// Duplicate check
	// Since project-a is visited first, it should be found.
	// z-link-to-a points to the same real path, so it should be skipped by visited check.
	if found["z-link-to-a"] {
		t.Error("Found duplicate via symlink z-link-to-a")
	}
}

func mustMkdir(t *testing.T, path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
}

func mustCreateFile(t *testing.T, path string) {
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}
