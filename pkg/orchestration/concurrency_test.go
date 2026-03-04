package orchestration

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestDoltConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "data")

	// Create first manager
	dm1, err := NewDatabaseManager(dataPath, "Rig Test 1", "test1@rig.sh", true)
	if err != nil {
		t.Fatalf("failed to create dm1: %v", err)
	}
	defer dm1.Close()
	if err := dm1.InitDatabase(); err != nil {
		t.Fatalf("dm1 InitDatabase failed: %v", err)
	}

	// Create second manager pointing to the same data
	dm2, err := NewDatabaseManager(dataPath, "Rig Test 2", "test2@rig.sh", true)
	if err != nil {
		t.Fatalf("failed to create dm2: %v", err)
	}
	defer dm2.Close()

	ctx := t.Context()
	var wg sync.WaitGroup
	wg.Add(2)

	errs := make(chan error, 400) // Buffer for all possible errors

	// Thread 1: Keep updating the same workflow
	sharedID := "shared-wf-id"
	go func() {
		defer wg.Done()
		w := &Workflow{ID: sharedID, Name: "shared-wf"}
		if err := dm1.CreateWorkflow(ctx, w); err != nil {
			errs <- fmt.Errorf("dm1 CreateWorkflow failed: %v", err)
			return
		}
		for i := range 100 {
			w.Description = fmt.Sprintf("updated by dm1 - %d", i)
			if err := dm1.UpdateWorkflow(ctx, w); err != nil {
				errs <- fmt.Errorf("dm1 failed at %d: %v", i, err)
				return
			}
		}
	}()

	// Thread 2: Keep updating the same workflow
	go func() {
		defer wg.Done()
		// Wait for wf to be created
		time.Sleep(100 * time.Millisecond)
		for i := range 100 {
			wf, err := dm2.GetWorkflow(ctx, sharedID)
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			wf.Description = fmt.Sprintf("updated by dm2 - %d", i)
			if err := dm2.UpdateWorkflow(ctx, wf); err != nil {
				errs <- fmt.Errorf("dm2 failed at %d: %v", i, err)
				return
			}
		}
	}()

	wg.Wait()
	close(errs)

	count := 0
	for err := range errs {
		t.Errorf("Concurrency error detected: %v", err)
		count++
	}
	if count == 0 {
		t.Log("No concurrency errors detected after stress test with retries.")
	}
}
