package rfsm

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestBasicTransition(t *testing.T) {
	def, err := NewDef("turnstile").
		State("Locked", WithInitial()).
		State("Unlocked", WithFinal()).
		Current("Locked").
		On("coin", "Locked", "Unlocked").
		On("push", "Unlocked", "Locked").
		Build()
	if err != nil {
		t.Fatalf("build err: %v", err)
	}

	m := NewMachine[any](def, nil, 8)
	if err := m.Start(); err != nil {
		t.Fatalf("start err: %v", err)
	}
	defer m.Stop()

	if got := m.Current(); got != "Locked" {
		t.Fatalf("want Locked, got %v", got)
	}

	if err := m.Dispatch(Event{Name: "coin"}); err != nil {
		t.Fatalf("dispatch coin: %v", err)
	}
	if got := m.Current(); got != "Unlocked" {
		t.Fatalf("want Unlocked, got %v", got)
	}

	if err := m.Dispatch(Event{Name: "push"}); err != nil {
		t.Fatalf("dispatch push: %v", err)
	}
	if got := m.Current(); got != "Locked" {
		t.Fatalf("want Locked, got %v", got)
	}
}

func TestGuardAndNoTransition(t *testing.T) {
	allow := false
	def, err := NewDef("g").
		State("A", WithInitial()).State("B", WithFinal()).
		Current("A").
		On("go", "A", "B", WithGuard[any](func(e Event, ctx any) bool { return allow })).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil, 8)
	_ = m.Start()
	defer m.Stop()

	if err := m.Dispatch(Event{Name: "go"}); !errors.Is(err, ErrNoTransition) {
		t.Fatalf("expect ErrNoTransition, got %v", err)
	}
	if got := m.Current(); got != "A" {
		t.Fatalf("still A, got %v", got)
	}

	allow = true
	if err := m.Dispatch(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}
	if got := m.Current(); got != "B" {
		t.Fatalf("to B, got %v", got)
	}
}

func TestActionRollback(t *testing.T) {
	var entryA, exitA, entryB int32
	fail := true
	def, err := NewDef("r").
		State("A", WithEntry[any](func(e Event, ctx any) error { atomic.AddInt32(&entryA, 1); return nil }), WithExit[any](func(e Event, ctx any) error { atomic.AddInt32(&exitA, 1); return nil }), WithInitial()).
		State("B", WithEntry[any](func(e Event, ctx any) error { atomic.AddInt32(&entryB, 1); return nil }), WithFinal()).
		Current("A").
		On("go", "A", "B", WithAction[any](func(e Event, ctx any) error {
			if fail {
				return errors.New("boom")
			}
			return nil
		})).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil, 8)
	_ = m.Start()
	defer m.Stop()

	// Initial entry into A once
	if entryA != 1 {
		t.Fatalf("entryA want 1 got %d", entryA)
	}

	// Failure: should rollback, stay in A, and A's entry called again
	if err := m.Dispatch(Event{Name: "go"}); !errors.Is(err, ErrActionFailed) {
		t.Fatalf("expect ErrActionFailed, got %v", err)
	}
	if got := m.Current(); got != "A" {
		t.Fatalf("still A, got %v", got)
	}
	if entryA != 2 {
		t.Fatalf("entryA want 2 got %d", entryA)
	}
	if exitA != 1 {
		t.Fatalf("exitA want 1 got %d", exitA)
	}
	if entryB != 0 {
		t.Fatalf("entryB want 0 got %d", entryB)
	}

	// Success: enter B
	fail = false
	if err := m.Dispatch(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}
	if got := m.Current(); got != "B" {
		t.Fatalf("to B, got %v", got)
	}
	if entryB != 1 {
		t.Fatalf("entryB want 1 got %d", entryB)
	}
}

type recSub struct {
	from, to StateID
	count    int32
}

func (r *recSub) OnTransition(from StateID, to StateID, e Event, err error) {
	r.from, r.to = from, to
	atomic.AddInt32(&r.count, 1)
}

func TestAsyncAndSubscriber(t *testing.T) {
	def, err := NewDef("t").
		State("A", WithInitial()).State("B", WithFinal()).
		Current("A").
		On("go", "A", "B").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil, 1)
	sub := &recSub{}
	m.Subscribe(sub)
	_ = m.Start()
	defer m.Stop()

	if err := m.DispatchAsync(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}
	// Wait for async processing
	time.Sleep(50 * time.Millisecond)
	if got := m.Current(); got != "B" {
		t.Fatalf("want B got %v", got)
	}
	if atomic.LoadInt32(&sub.count) == 0 {
		t.Fatalf("want subscriber called")
	}
}

func TestBuildValidation_CurrentNotSet(t *testing.T) {
	_, err := NewDef("test").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Build()
	if err == nil {
		t.Fatal("expected error for missing current state")
	}
	if err.Error() != "current state not set" {
		t.Fatalf("expected 'current state not set', got %q", err.Error())
	}
}

func TestBuildValidation_CurrentNotDefined(t *testing.T) {
	_, err := NewDef("test").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("C").
		Build()
	if err == nil {
		t.Fatal("expected error for undefined current state")
	}
	if err.Error() != "current state \"C\" not defined" {
		t.Fatalf("expected 'current state \"C\" not defined', got %q", err.Error())
	}
}

func TestBuildValidation_NoInitialState(t *testing.T) {
	_, err := NewDef("test").
		State("A").
		State("B", WithFinal()).
		Current("A").
		Build()
	if err == nil {
		t.Fatal("expected error for missing initial state")
	}
	if err.Error() != "at least one state must be marked with WithInitial()" {
		t.Fatalf("expected 'at least one state must be marked with WithInitial()', got %q", err.Error())
	}
}

func TestBuildValidation_NoFinalState(t *testing.T) {
	_, err := NewDef("test").
		State("A", WithInitial()).
		State("B").
		Current("A").
		Build()
	if err == nil {
		t.Fatal("expected error for missing final state")
	}
	if err.Error() != "at least one state must be marked with WithFinal()" {
		t.Fatalf("expected 'at least one state must be marked with WithFinal()', got %q", err.Error())
	}
}

func TestBuildValidation_TransitionFromUndefined(t *testing.T) {
	_, err := NewDef("test").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", "C", "B").
		Build()
	if err == nil {
		t.Fatal("expected error for transition from undefined state")
	}
	if err.Error() != "transition from undefined state \"C\"" {
		t.Fatalf("expected 'transition from undefined state \"C\"', got %q", err.Error())
	}
}

func TestBuildValidation_TransitionToUndefined(t *testing.T) {
	_, err := NewDef("test").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", "A", "C").
		Build()
	if err == nil {
		t.Fatal("expected error for transition to undefined state")
	}
	if err.Error() != "transition to undefined state \"C\"" {
		t.Fatalf("expected 'transition to undefined state \"C\"', got %q", err.Error())
	}
}
