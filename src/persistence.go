package rfsm

import (
	"encoding/json"
	"fmt"
)

// Snapshot captures the minimal runtime needed to resume a machine
type Snapshot struct {
	Current    StateID   `json:"current"`
	ActivePath []StateID `json:"active_path"`
	Visited    []StateID `json:"visited,omitempty"`
}

// Snapshot returns an in-memory snapshot of the current machine runtime state.
func (m *Machine[C]) Snapshot() *Snapshot {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	visited := make([]StateID, 0, len(m.visited))
	for s := range m.visited {
		visited = append(visited, s)
	}
	cp := make([]StateID, len(m.activePath))
	copy(cp, m.activePath)
	return &Snapshot{
		Current:    m.current,
		ActivePath: cp,
		Visited:    visited,
	}
}

// SnapshotJSON serializes Snapshot to JSON.
func (m *Machine[C]) SnapshotJSON() ([]byte, error) {
	snap := m.Snapshot()
	return json.Marshal(snap)
}

// RestoreSnapshot restores machine runtime from snapshot and starts the event loop.
// It does not call entry/exit hooks during restoration.
// buf controls the capacity of the internal events queue; if <=0, defaults to 64.
func (m *Machine[C]) RestoreSnapshot(snap *Snapshot, buf int) error {
	if snap == nil {
		return fmt.Errorf("nil snapshot")
	}
	// Validate states exist
	if _, ok := m.def.States[snap.Current]; !ok {
		return fmt.Errorf("snapshot refers to unknown current state %q", snap.Current)
	}
	for _, s := range snap.ActivePath {
		if _, ok := m.def.States[s]; !ok {
			return fmt.Errorf("snapshot refers to unknown state in active_path: %q", s)
		}
	}
	// Validate path consistency: recompute expected path to leaf
	expected := m.pathTo(snap.Current)
	if len(expected) != len(snap.ActivePath) {
		return fmt.Errorf("active_path inconsistent with current")
	}
	for i := range expected {
		if expected[i] != snap.ActivePath[i] {
			return fmt.Errorf("active_path does not match hierarchy")
		}
	}

	// Apply under lock
	m.statusMu.Lock()
	// rebuild channels
	if buf <= 0 {
		buf = 64
	}
	m.events = make(chan Event, buf)
	m.done = make(chan struct{})
	m.current = snap.Current
	m.activePath = make([]StateID, len(snap.ActivePath))
	copy(m.activePath, snap.ActivePath)
	m.visited = make(map[StateID]bool, len(snap.Visited))
	for _, s := range snap.Visited {
		m.visited[s] = true
	}
	m.started = true
	m.statusMu.Unlock()

	// start loop
	m.wg.Add(1)
	go m.loop()
	return nil
}

// RestoreSnapshotJSON restores from JSON snapshot
func (m *Machine[C]) RestoreSnapshotJSON(data []byte, buf int) error {
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	return m.RestoreSnapshot(&snap, buf)
}
