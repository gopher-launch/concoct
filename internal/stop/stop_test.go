package stop

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

type fakeControl struct {
	child   Identity
	fail    bool
	methods []string
	cleaned bool
}

func (f *fakeControl) Interrupt(_ context.Context, thread, turn string) error {
	f.methods = append(f.methods, "interrupt")
	return nil
}
func (f *fakeControl) ReadTurn(_ context.Context, thread, turn string) (codexapp.ObservedTurn, error) {
	f.methods = append(f.methods, "read")
	return codexapp.ObservedTurn{ID: turn, Status: "interrupted"}, nil
}
func (f *fakeControl) BackgroundTerminals(_ context.Context, thread string) ([]codexapp.BackgroundTerminal, error) {
	f.methods = append(f.methods, "list")
	if f.cleaned {
		return nil, nil
	}
	return []codexapp.BackgroundTerminal{{ProcessID: "opaque-handle", ItemID: "item", OSPID: nil}}, nil
}
func (f *fakeControl) TerminateBackgroundTerminal(_ context.Context, thread, handle string) (bool, error) {
	f.methods = append(f.methods, "terminate")
	if handle != "opaque-handle" || f.fail {
		return false, nil
	}
	return true, syscall.Kill(f.child.PID, syscall.SIGKILL)
}
func (f *fakeControl) CleanBackgroundTerminals(context.Context, string) error {
	f.methods = append(f.methods, "clean")
	f.cleaned = true
	return nil
}

func TestOwnedChildStopAndUnrelatedProcess(t *testing.T) {
	provider := exec.Command("bash", "-c", "sleep 30 & wait; while :; do :; done")
	if err := provider.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Process.Kill(); _ = provider.Wait() })
	unrelated := exec.Command("sleep", "30")
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unrelated.Process.Kill(); _ = unrelated.Wait() })
	probe, err := NewProbe(provider.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	var owned []Identity
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		owned, err = probe.Capture()
		if err != nil {
			t.Fatal(err)
		}
		if len(owned) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(owned) != 1 || owned[0].PID == unrelated.Process.Pid {
		t.Fatalf("owned descendants: %+v", owned)
	}
	control := &fakeControl{child: owned[0]}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result := Run(ctx, control, probe, "thread", "turn", 120*time.Millisecond)
	if result.Status != "stopped_safely" || len(result.Handles) != 1 || result.Handles[0] != "opaque-handle" {
		t.Fatalf("stop result: %+v", result)
	}
	if stringJoin(control.methods) != "interrupt,read,list,terminate,clean,list" {
		t.Fatalf("control order: %v", control.methods)
	}
	if err := unrelated.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("unrelated process was affected: %v", err)
	}
}

func TestFailedCleanupIsAbnormal(t *testing.T) {
	provider := exec.Command("bash", "-c", "sleep 30 & wait; while :; do :; done")
	if err := provider.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Process.Kill(); _ = provider.Wait() })
	probe, err := NewProbe(provider.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	var owned []Identity
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		owned, err = probe.Capture()
		if err != nil {
			t.Fatal(err)
		}
		if len(owned) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(owned) == 0 {
		t.Fatal("child was not observed")
	}
	control := &fakeControl{child: owned[0], fail: true}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	result := Run(ctx, control, probe, "thread", "turn", 100*time.Millisecond)
	if result.Status != "stopped_abnormally" || result.Problem == "" {
		t.Fatalf("failed cleanup was accepted: %+v", result)
	}
	if _, err := os.Stat("/proc/" + itoa(owned[0].PID)); err != nil {
		t.Fatalf("child unexpectedly absent: %v", err)
	}
}

func stringJoin(items []string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += ","
		}
		result += item
	}
	return result
}
func itoa(value int) string { return strconv.Itoa(value) }
