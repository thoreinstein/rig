package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"thoreinstein.com/rig/pkg/config"
)

func TestLocalProvider(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Notes: config.NotesConfig{
			Path:     tmpDir,
			DailyDir: "daily",
		},
	}

	provider := NewLocalProvider(cfg, false)
	ctx := t.Context()

	t.Run("CreateTicketNote", func(t *testing.T) {
		data := &NoteData{
			Ticket:     "PROJ-123",
			TicketType: "proj",
		}

		result, err := provider.CreateTicketNote(ctx, data)
		if err != nil {
			t.Fatalf("Failed to create note: %v", err)
		}

		if !result.Created {
			t.Error("Expected Created to be true")
		}

		expectedPath := filepath.Join(tmpDir, "proj", "PROJ-123.md")
		if result.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, result.Path)
		}

		if _, err := os.Stat(result.Path); os.IsNotExist(err) {
			t.Error("Note file was not created")
		}
	})

	t.Run("GetNotePath", func(t *testing.T) {
		path, err := provider.GetNotePath(ctx, "proj", "PROJ-123")
		if err != nil {
			t.Fatalf("Failed to get note path: %v", err)
		}

		expectedPath := filepath.Join(tmpDir, "proj", "PROJ-123.md")
		if path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, path)
		}
	})

	t.Run("UpdateDailyNote", func(t *testing.T) {
		err := provider.UpdateDailyNote(ctx, "PROJ-123", "proj")
		if err != nil {
			t.Fatalf("Failed to update daily note: %v", err)
		}

		// Check if daily note exists (path is non-deterministic due to date, but we can check the directory)
		dailyDir := filepath.Join(tmpDir, "daily")
		entries, err := os.ReadDir(dailyDir)
		if err != nil {
			t.Fatalf("Failed to read daily directory: %v", err)
		}

		if len(entries) == 0 {
			t.Error("Daily note was not created")
		}
	})

	t.Run("GetDailyNotePath", func(t *testing.T) {
		path, err := provider.GetDailyNotePath(ctx)
		if err != nil {
			t.Fatalf("Failed to get daily note path: %v", err)
		}

		if path == "" {
			t.Error("Expected non-empty daily note path")
		}

		// Should be under the daily directory
		dailyDir := filepath.Join(tmpDir, "daily")
		rel, err := filepath.Rel(dailyDir, path)
		if err != nil {
			t.Fatalf("Path %s is not relative to daily dir %s: %v", path, dailyDir, err)
		}
		if filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
			t.Errorf("Daily note path %s is not under daily dir %s", path, dailyDir)
		}
	})
}
