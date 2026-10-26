package cmd

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// After the timeout, the parent must kill the child. Otherwise the parent
// reports a failure, but the daemon can still start later. The child here never
// answers and never exits on its own. XDG_RUNTIME_DIR points to an empty
// directory, so SendStatus() cannot reach a daemon that already runs.
func TestWaitForDaemonKillsChildAfterTimeout(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	oldTimeout := startTimeout
	startTimeout = 200 * time.Millisecond
	t.Cleanup(func() { startTimeout = oldTimeout })

	child := exec.Command("sleep", "3600")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { child.Process.Kill() })

	done := make(chan error, 1)
	go func() { done <- WaitForDaemon(child.Process) }()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "did not answer") {
			t.Fatalf("WaitForDaemon() error = %v, want the timeout error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForDaemon() did not return after the timeout")
	}

	if err := child.Process.Signal(syscall.Signal(0)); !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("the child still runs: Signal(0) = %v", err)
	}
}
