package stop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Identity uses Linux /proc start ticks to distinguish a live process from a
// recycled PID. Provider handles are never used as operating-system PIDs.
type Identity struct {
	PID   int    `json:"pid"`
	Start uint64 `json:"start"`
}

type processRow struct {
	Identity
	Parent int
	State  byte
}

type Probe struct {
	mu    sync.Mutex
	root  Identity
	owned map[Identity]bool
}

func NewProbe(providerPID int) (*Probe, error) {
	rows, err := processTable()
	if err != nil {
		return nil, err
	}
	row, ok := rows[providerPID]
	if !ok || row.State == 'Z' {
		return nil, fmt.Errorf("provider process %d is not live", providerPID)
	}
	return &Probe{root: row.Identity, owned: make(map[Identity]bool)}, nil
}

func (p *Probe) Root() Identity { return p.root }

// Capture records only descendants of the exact Concoct-owned provider. A
// sampler calls this throughout a turn so reparented descendants remain known.
func (p *Probe) Capture() ([]Identity, error) {
	rows, err := processTable()
	if err != nil {
		return nil, err
	}
	root, ok := rows[p.root.PID]
	if !ok || root.Start != p.root.Start || root.State == 'Z' {
		return nil, fmt.Errorf("Concoct provider process identity changed")
	}
	descendants := map[int]bool{p.root.PID: true}
	for changed := true; changed; {
		changed = false
		for pid, row := range rows {
			if !descendants[pid] && descendants[row.Parent] {
				descendants[pid], changed = true, true
			}
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for pid := range descendants {
		if pid != p.root.PID && rows[pid].State != 'Z' && !(rows[pid].Parent == p.root.PID && persistentProviderHelper(pid)) {
			p.owned[rows[pid].Identity] = true
		}
	}
	result := make([]Identity, 0, len(p.owned))
	for id := range p.owned {
		result = append(result, id)
	}
	return result, nil
}

// The installed runner keeps its code-mode transport helper alive between
// turns. Its descendants are still traversed and captured as execution work.
func persistentProviderHelper(pid int) bool {
	exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	return err == nil && filepath.Base(exe) == "codex-code-mode-host"
}

// Quiet requires exact owned identities to remain absent for the full grace
// interval. It never signals any process.
func (p *Probe) Quiet(ctx context.Context, grace time.Duration) error {
	if grace <= 0 {
		return fmt.Errorf("positive cessation grace is required")
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var clearSince time.Time
	for {
		if _, err := p.Capture(); err != nil {
			return err
		}
		rows, err := processTable()
		if err != nil {
			return err
		}
		live := false
		p.mu.Lock()
		for id := range p.owned {
			if _, ok := rows[id.PID]; !ok {
				if _, statErr := os.Stat(filepath.Join("/proc", strconv.Itoa(id.PID))); !os.IsNotExist(statErr) {
					p.mu.Unlock()
					return fmt.Errorf("owned process %d cannot be inspected", id.PID)
				}
			}
			if row, ok := rows[id.PID]; ok && row.Start == id.Start && row.State != 'Z' {
				live = true
				break
			}
		}
		p.mu.Unlock()
		if live {
			clearSince = time.Time{}
		} else if clearSince.IsZero() {
			clearSince = time.Now()
		} else if time.Since(clearSince) >= grace {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func processTable() (map[int]processRow, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	rows := make(map[int]processRow)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		body, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if os.IsNotExist(err) || os.IsPermission(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		end := strings.LastIndexByte(string(body), ')')
		if end < 0 {
			return nil, fmt.Errorf("malformed proc stat for %d", pid)
		}
		fields := strings.Fields(string(body[end+1:]))
		if len(fields) < 20 || len(fields[0]) != 1 {
			return nil, fmt.Errorf("malformed proc fields for %d", pid)
		}
		parent, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, err
		}
		start, err := strconv.ParseUint(fields[19], 10, 64)
		if err != nil {
			return nil, err
		}
		rows[pid] = processRow{Identity: Identity{PID: pid, Start: start}, Parent: parent, State: fields[0][0]}
	}
	return rows, nil
}

// TerminateOwned signals only captured child identities that still match their
// Linux start ticks. It is the fallback when provider cleanup cannot establish
// cessation. The app-server root itself is closed by its Process owner.
func (p *Probe) TerminateOwned(ctx context.Context) error {
	if _, err := p.Capture(); err != nil {
		return err
	}
	p.mu.Lock()
	ids := make([]Identity, 0, len(p.owned))
	for id := range p.owned {
		ids = append(ids, id)
	}
	p.mu.Unlock()
	for _, signal := range []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL} {
		rows, err := processTable()
		if err != nil {
			return err
		}
		live := false
		for _, id := range ids {
			row, ok := rows[id.PID]
			if !ok || row.Start != id.Start || row.State == 'Z' {
				continue
			}
			live = true
			if err := syscall.Kill(id.PID, signal); err != nil && err != syscall.ESRCH {
				return err
			}
		}
		if !live {
			return nil
		}
		wait := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			wait.Stop()
			return ctx.Err()
		case <-wait.C:
		}
	}
	rows, err := processTable()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if row, ok := rows[id.PID]; ok && row.Start == id.Start && row.State != 'Z' {
			return fmt.Errorf("owned process %d remains live", id.PID)
		}
	}
	return nil
}
