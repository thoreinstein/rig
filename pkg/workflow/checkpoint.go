package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// Checkpoint stores workflow state for resuming interrupted workflows.
type Checkpoint struct {
	PRNumber       int              `json:"pr_number"`
	Ticket         string           `json:"ticket,omitempty"`
	Worktree       string           `json:"worktree,omitempty"`
	CompletedSteps []Step           `json:"completed_steps"`
	CurrentStep    Step             `json:"current_step"`
	Context        *WorkflowContext `json:"context,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

const (
	// rigDir is the directory within a worktree where rig stores state.
	rigDir = ".rig"
	// checkpointFile is the name of the checkpoint file.
	checkpointFile = "checkpoint.json"
)

// checkpointPath returns the full path to the checkpoint file.
func checkpointPath(worktree string) string {
	return filepath.Join(worktree, rigDir, checkpointFile)
}

// SaveCheckpoint saves workflow state to .rig/checkpoint.json.
//
// The checkpoint allows resuming a workflow after interruption.
// The file is created with restricted permissions (0600) since it may
// contain sensitive context information.
func SaveCheckpoint(worktree string, checkpoint *Checkpoint) error {
	if worktree == "" {
		return rigerrors.NewWorkflowError("save_checkpoint", "worktree path is required")
	}

	if checkpoint == nil {
		return rigerrors.NewWorkflowError("save_checkpoint", "checkpoint is nil")
	}

	// Ensure .rig directory exists
	dir := filepath.Join(worktree, rigDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return rigerrors.Wrapf(err, "failed to create .rig directory")
	}

	// Update timestamp
	checkpoint.UpdatedAt = time.Now()

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return rigerrors.Wrapf(err, "failed to marshal checkpoint")
	}

	// Write checkpoint file with restricted permissions
	path := checkpointPath(worktree)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return rigerrors.Wrapf(err, "failed to write checkpoint")
	}

	return nil
}

// LoadCheckpoint loads checkpoint from .rig/checkpoint.json.
//
// Returns nil, nil if no checkpoint exists (not an error).
// Returns an error only if the checkpoint exists but cannot be read.
func LoadCheckpoint(worktree string) (*Checkpoint, error) {
	if worktree == "" {
		return nil, rigerrors.NewWorkflowError("load_checkpoint", "worktree path is required")
	}

	path := checkpointPath(worktree)

	// Check if checkpoint exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	// Read checkpoint file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, rigerrors.Wrapf(err, "failed to read checkpoint")
	}

	// Unmarshal JSON
	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, rigerrors.Wrapf(err, "failed to parse checkpoint")
	}

	return &checkpoint, nil
}

// ClearCheckpoint removes the checkpoint file.
//
// This should be called after a workflow completes successfully.
// If the file doesn't exist, this is a no-op (not an error).
func ClearCheckpoint(worktree string) error {
	if worktree == "" {
		return rigerrors.NewWorkflowError("clear_checkpoint", "worktree path is required")
	}

	path := checkpointPath(worktree)

	// Remove file (ignore error if it doesn't exist)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return rigerrors.Wrapf(err, "failed to remove checkpoint")
	}

	return nil
}

// HasCheckpoint checks if a checkpoint exists for the worktree.
func HasCheckpoint(worktree string) bool {
	if worktree == "" {
		return false
	}

	path := checkpointPath(worktree)
	_, err := os.Stat(path)
	return err == nil
}

// GetCheckpointAge returns how old the checkpoint is.
// Returns 0 if no checkpoint exists.
func GetCheckpointAge(worktree string) time.Duration {
	checkpoint, err := LoadCheckpoint(worktree)
	if err != nil || checkpoint == nil {
		return 0
	}
	return time.Since(checkpoint.UpdatedAt)
}

// IsCheckpointStale returns true if the checkpoint is older than the given duration.
// A stale checkpoint might indicate an abandoned workflow.
func IsCheckpointStale(worktree string, maxAge time.Duration) bool {
	age := GetCheckpointAge(worktree)
	if age == 0 {
		return false
	}
	return age > maxAge
}
