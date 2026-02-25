package project

import (
	"os"
	"path/filepath"
	"testing"
)

// evalSymlinks resolves symlinks for path comparison
func evalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

func TestDiscover(t *testing.T) {
	tmpDir := evalSymlinks(t, t.TempDir())

	tests := []struct {
		name        string
		setup       func(t *testing.T, base string) string
		wantMarkers []MarkerKind
		wantRootRel string
		wantErr     bool
	}{
		{
			name: ".git root",
			setup: func(t *testing.T, base string) string {
				dir := filepath.Join(base, "repo")
				if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				return dir
			},
			wantMarkers: []MarkerKind{MarkerGit},
			wantRootRel: "repo",
		},
		{
			name: ".git file (worktree)",
			setup: func(t *testing.T, base string) string {
				dir := filepath.Join(base, "worktree")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ..."), 0644); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				return dir
			},
			wantMarkers: []MarkerKind{MarkerGit},
			wantRootRel: "worktree",
		},
		{
			name: ".rig.toml in subfolder of .git",
			setup: func(t *testing.T, base string) string {
				root := filepath.Join(base, "cascading")
				if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				sub := filepath.Join(root, "pkg", "cmd")
				if err := os.MkdirAll(sub, 0755); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				if err := os.WriteFile(filepath.Join(sub, ".rig.toml"), []byte(""), 0644); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				return sub
			},
			wantMarkers: []MarkerKind{MarkerGit, MarkerRigToml},
			wantRootRel: "cascading", // Stopped at .git
		},
		{
			name: ".beads project",
			setup: func(t *testing.T, base string) string {
				dir := filepath.Join(base, "beads-proj")
				if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0755); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte(""), 0644); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				return dir
			},
			wantMarkers: []MarkerKind{MarkerBeads},
			wantRootRel: "beads-proj",
		},
		{
			name: "no markers found",
			setup: func(t *testing.T, base string) string {
				dir := filepath.Join(base, "none")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				return dir
			},
			wantErr: true,
		},
		{
			name: "nested .rig.toml within .git boundary",
			setup: func(t *testing.T, base string) string {
				root := filepath.Join(base, "nested")
				if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				if err := os.WriteFile(filepath.Join(root, ".rig.toml"), []byte("root"), 0644); err != nil {
					t.Fatalf("setup failed: %v", err)
				}

				sub := filepath.Join(root, "sub")
				if err := os.MkdirAll(sub, 0755); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
				if err := os.WriteFile(filepath.Join(sub, ".rig.toml"), []byte("sub"), 0644); err != nil {
					t.Fatalf("setup failed: %v", err)
				}

				return sub
			},
			wantMarkers: []MarkerKind{MarkerGit, MarkerRigToml},
			wantRootRel: "nested",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startDir := tt.setup(t, tmpDir)
			ctx, err := Discover(startDir)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Discover() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if !IsNoProjectContext(err) {
					t.Errorf("Expected ErrNoProjectContext, got %v", err)
				}
				return
			}

			for _, m := range tt.wantMarkers {
				if !ctx.HasMarker(m) {
					t.Errorf("Expected marker %v not found", m)
				}
			}

			wantRoot := filepath.Join(tmpDir, tt.wantRootRel)
			if ctx.RootPath != wantRoot {
				t.Errorf("RootPath = %q, want %q", ctx.RootPath, wantRoot)
			}
		})
	}
}

func TestCachedDiscover(t *testing.T) {
	tmpDir := evalSymlinks(t, t.TempDir())
	dir := filepath.Join(tmpDir, "cached")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	ResetCache()

	// First call - cache miss
	ctx1, err := CachedDiscover(dir)
	if err != nil {
		t.Fatalf("First CachedDiscover failed: %v", err)
	}

	// Second call - cache hit
	ctx2, err := CachedDiscover(dir)
	if err != nil {
		t.Fatalf("Second CachedDiscover failed: %v", err)
	}

	if ctx1 != ctx2 {
		t.Error("CachedDiscover returned different pointers for the same directory")
	}

	// Reset and call again
	ResetCache()
	ctx3, err := CachedDiscover(dir)
	if err != nil {
		t.Fatalf("Third CachedDiscover failed: %v", err)
	}

	if ctx1 == ctx3 {
		t.Error("CachedDiscover returned same pointer after cache reset")
	}
}

func TestDiscover_Symlinks(t *testing.T) {
	// Create structure: /base/real/.git
	// Create symlink: /base/link -> /base/real

	tmpDir := evalSymlinks(t, t.TempDir())
	realDir := filepath.Join(tmpDir, "real")
	if err := os.MkdirAll(filepath.Join(realDir, ".git"), 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	linkDir := filepath.Join(tmpDir, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("Symlinks not supported: %v", err)
	}

	ctx, err := Discover(linkDir)
	if err != nil {
		t.Fatalf("Discover through symlink failed: %v", err)
	}

	if ctx.RootPath != realDir {
		t.Errorf("RootPath = %q, want %q (physical path)", ctx.RootPath, realDir)
	}

	if ctx.Origin != realDir {
		t.Errorf("Origin = %q, want %q (resolved path)", ctx.Origin, realDir)
	}
}
