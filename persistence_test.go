package rfsm

import (
	"encoding/json"
	"sync/atomic"
	"testing"
)

func TestPersistence_SnapshotAndRestore(t *testing.T) {
	var entryA, entryB int32
	def, err := NewDef("p").
		State("A", WithEntry[any](func(e Event, ctx any) error { atomic.AddInt32(&entryA, 1); return nil }), WithInitial()).
		State("B", WithEntry[any](func(e Event, ctx any) error { atomic.AddInt32(&entryB, 1); return nil }), WithFinal()).
		Current("A").
		On("go", "A", "B").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// run first machine and transition to B
	m1 := NewMachine[any](def, nil)
	if err := m1.Start(); err != nil {
		t.Fatal(err)
	}
	if entryA != 1 {
		t.Fatalf("entryA want 1 got %d", entryA)
	}
	if err := m1.Dispatch(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}
	if m1.Current() != "B" {
		t.Fatalf("want B got %v", m1.Current())
	}

	// take snapshot and stop
	snapData, err := m1.SnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := m1.Stop(); err != nil {
		t.Fatal(err)
	}

	// restore into a new machine without triggering entry hooks again
	m2 := NewMachine[any](def, nil)
	if err := m2.RestoreSnapshotJSON(snapData, 4); err != nil {
		t.Fatal(err)
	}
	if m2.Current() != "B" {
		t.Fatalf("restored current want B got %v", m2.Current())
	}
	// entryB should NOT increment due to restore
	if entryB != 1 {
		t.Fatalf("entryB should remain 1 after restore, got %d", entryB)
	}
	// visited should contain A and B
	if !m2.HasVisited("A") || !m2.HasVisited("B") {
		t.Fatalf("visited should include A and B")
	}
}

func TestPersistence_ContextSerialization(t *testing.T) {
	type testContext struct {
		Counter int    `json:"counter"`
		Message string `json:"message"`
	}

	def, err := NewDef("ctx_test").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", "A", "B").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Create machine with context
	ctx1 := &testContext{
		Counter: 42,
		Message: "test",
	}
	m1 := NewMachine[*testContext](def, ctx1)
	if err := m1.Start(); err != nil {
		t.Fatal(err)
	}
	if err := m1.Dispatch(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}

	// Take snapshot
	snapData, err := m1.SnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}

	// Verify snapshot contains context
	var snap Snapshot
	if err := json.Unmarshal(snapData, &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.ContextJSON) == 0 {
		t.Fatal("snapshot should contain context")
	}

	// Restore into new machine
	ctx2 := &testContext{}
	m2 := NewMachine[*testContext](def, ctx2)
	if err := m2.RestoreSnapshotJSON(snapData, 4); err != nil {
		t.Fatal(err)
	}

	// Verify context was restored
	if ctx2.Counter != 42 {
		t.Fatalf("context counter want 42 got %d", ctx2.Counter)
	}
	if ctx2.Message != "test" {
		t.Fatalf("context message want 'test' got %q", ctx2.Message)
	}
	if m2.Current() != "B" {
		t.Fatalf("current want B got %v", m2.Current())
	}
}

func TestPersistence_RestoreSnapshot_Errors(t *testing.T) {
	def, err := NewDef("test").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", "A", "B").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)

	// Test nil snapshot
	if err := m.RestoreSnapshot(nil, 4); err == nil {
		t.Fatal("expected error for nil snapshot")
	}

	// Test invalid current state
	snap := &Snapshot{
		Current:    "C",
		ActivePath: []StateID{"C"},
		Visited:    []StateID{"C"},
	}
	if err := m.RestoreSnapshot(snap, 4); err == nil {
		t.Fatal("expected error for invalid current state")
	}

	// Test invalid state in active_path
	snap2 := &Snapshot{
		Current:    "A",
		ActivePath: []StateID{"A", "C"},
		Visited:    []StateID{"A"},
	}
	if err := m.RestoreSnapshot(snap2, 4); err == nil {
		t.Fatal("expected error for invalid state in active_path")
	}

	// Test inconsistent active_path
	snap3 := &Snapshot{
		Current:    "B",
		ActivePath: []StateID{"A"}, // Should be [A, B]
		Visited:    []StateID{"A", "B"},
	}
	if err := m.RestoreSnapshot(snap3, 4); err == nil {
		t.Fatal("expected error for inconsistent active_path")
	}
}

func TestPersistence_RestoreSnapshot_InvalidContext(t *testing.T) {
	type testContext struct {
		Value int `json:"value"`
	}

	def, err := NewDef("test").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("A").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Create snapshot with invalid context JSON
	ctx1 := &testContext{Value: 42}
	m1 := NewMachine[*testContext](def, ctx1)
	if err := m1.Start(); err != nil {
		t.Fatal(err)
	}
	snapData, err := m1.SnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the context JSON
	corrupted := []byte(`{"current":"A","active_path":["A"],"context":"invalid json"}`)

	// Restore into new machine
	ctx2 := &testContext{}
	m2 := NewMachine[*testContext](def, ctx2)
	if err := m2.RestoreSnapshotJSON(corrupted, 4); err == nil {
		t.Fatal("expected error for invalid context JSON")
	}

	// Test with valid data
	if err := m2.RestoreSnapshotJSON(snapData, 4); err != nil {
		t.Fatal(err)
	}
	if ctx2.Value != 42 {
		t.Fatalf("context value want 42 got %d", ctx2.Value)
	}
}
