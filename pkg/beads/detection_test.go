package beads

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsBeadsProject(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Test case: no beads project
	if IsBeadsProject(tmpDir) {
		t.Error("IsBeadsProject() should return false for directory without .beads")
	}

	// Create .beads directory
	beadsDir := filepath.Join(tmpDir, BeadsDirName)
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("failed to create .beads directory: %v", err)
	}

	// Test case: .beads exists but no issues.jsonl
	if IsBeadsProject(tmpDir) {
		t.Error("IsBeadsProject() should return false without issues.jsonl")
	}

	// Create issues.jsonl file
	beadsFile := filepath.Join(beadsDir, BeadsFileName)
	if err := os.WriteFile(beadsFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create issues.jsonl: %v", err)
	}

	// Test case: valid beads project
	if !IsBeadsProject(tmpDir) {
		t.Error("IsBeadsProject() should return true for valid beads project")
	}

	// Test case: non-existent directory
	if IsBeadsProject("/nonexistent/path/xyz") {
		t.Error("IsBeadsProject() should return false for non-existent path")
	}
}

func TestIsBeadsProject_FileInsteadOfDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .beads as a file instead of directory
	beadsPath := filepath.Join(tmpDir, BeadsDirName)
	if err := os.WriteFile(beadsPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create .beads file: %v", err)
	}

	if IsBeadsProject(tmpDir) {
		t.Error("IsBeadsProject() should return false when .beads is a file")
	}
}

func TestFindBeadsRoot(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create nested directories: tmpDir/level1/level2/level3
	level3 := filepath.Join(tmpDir, "level1", "level2", "level3")
	if err := os.MkdirAll(level3, 0o755); err != nil {
		t.Fatalf("failed to create nested directories: %v", err)
	}

	// Test case: no beads project found
	root, found := FindBeadsRoot(level3)
	if found {
		t.Errorf("FindBeadsRoot() should not find project, got %q", root)
	}

	// Create beads project at level1
	level1 := filepath.Join(tmpDir, "level1")
	beadsDir := filepath.Join(level1, BeadsDirName)
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("failed to create .beads directory: %v", err)
	}
	beadsFile := filepath.Join(beadsDir, BeadsFileName)
	if err := os.WriteFile(beadsFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create issues.jsonl: %v", err)
	}

	// Test case: find beads root from level3
	root, found = FindBeadsRoot(level3)
	if !found {
		t.Error("FindBeadsRoot() should find beads project")
	}
	if root != level1 {
		t.Errorf("FindBeadsRoot() = %q, want %q", root, level1)
	}

	// Test case: find beads root from level1 itself
	root, found = FindBeadsRoot(level1)
	if !found {
		t.Error("FindBeadsRoot() should find beads project at same level")
	}
	if root != level1 {
		t.Errorf("FindBeadsRoot() = %q, want %q", root, level1)
	}
}

func TestFindBeadsRoot_RelativePath(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create beads project
	beadsDir := filepath.Join(tmpDir, BeadsDirName)
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("failed to create .beads directory: %v", err)
	}
	beadsFile := filepath.Join(beadsDir, BeadsFileName)
	if err := os.WriteFile(beadsFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create issues.jsonl: %v", err)
	}

	// Change to tmpDir to test relative path handling
	t.Chdir(tmpDir)

	// Test with relative path "."
	root, found := FindBeadsRoot(".")
	if !found {
		t.Error("FindBeadsRoot(\".\") should find beads project")
	}

	// The returned path should be absolute
	if !filepath.IsAbs(root) {
		t.Errorf("FindBeadsRoot() should return absolute path, got %q", root)
	}
}

func TestFindBeadsRoot_InvalidPath(t *testing.T) {
	// Test with a path that doesn't exist
	root, found := FindBeadsRoot("/nonexistent/path/that/does/not/exist")
	if found {
		t.Errorf("FindBeadsRoot() should not find project for invalid path, got %q", root)
	}
}

func TestConstants(t *testing.T) {
	// Verify constants have expected values
	if BeadsDirName != ".beads" {
		t.Errorf("BeadsDirName = %q, want %q", BeadsDirName, ".beads")
	}
	if BeadsFileName != "issues.jsonl" {
		t.Errorf("BeadsFileName = %q, want %q", BeadsFileName, "issues.jsonl")
	}
}
