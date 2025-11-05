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
	m1 := NewMachine[any](def, nil, 4)
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
	m2 := NewMachine[any](def, nil, 4)
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
	m1 := NewMachine[*testContext](def, ctx1, 4)
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
	m2 := NewMachine[*testContext](def, ctx2, 4)
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
