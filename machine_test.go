package rfsm

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestMachine_StartStop_Hooks(t *testing.T) {
	var entry, exit int32
	def, err := NewDef("hooks").
		State("S", WithEntry[any](func(e Event, ctx any) error { atomic.AddInt32(&entry, 1); return nil }), WithExit[any](func(e Event, ctx any) error { atomic.AddInt32(&exit, 1); return nil }), WithInitial(), WithFinal()).
		Current("S").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&entry); got != 1 {
		t.Fatalf("entry want 1 got %d", got)
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&exit); got != 1 {
		t.Fatalf("exit want 1 got %d", got)
	}
}

func TestMachine_Dispatch_BeforeStart_AfterStop(t *testing.T) {
	def, err := NewDef("b").
		State("A", WithInitial(), WithFinal()).Current("A").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	m := NewMachine[any](def, nil)

	if err := m.Dispatch(Event{Name: "x"}); !errors.Is(err, ErrMachineNotStarted) {
		t.Fatalf("before start want ErrMachineNotStarted got %v", err)
	}
	if err := m.DispatchAsync(Event{Name: "x"}); !errors.Is(err, ErrMachineNotStarted) {
		t.Fatalf("before start async want ErrMachineNotStarted got %v", err)
	}

	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}

	if err := m.Dispatch(Event{Name: "x"}); !errors.Is(err, ErrMachineNotStarted) {
		t.Fatalf("after stop want ErrMachineNotStarted got %v", err)
	}
	if err := m.DispatchAsync(Event{Name: "x"}); !errors.Is(err, ErrMachineNotStarted) {
		t.Fatalf("after stop async want ErrMachineNotStarted got %v", err)
	}
}

func TestMachine_DispatchSync_WaitsForAction(t *testing.T) {
	var stamp int64
	def, err := NewDef("sync").
		State("A", WithInitial()).State("B", WithFinal()).
		Current("A").
		On("go", "A", "B", WithAction[any](func(e Event, ctx any) error {
			time.Sleep(40 * time.Millisecond)
			atomic.StoreInt64(&stamp, time.Now().UnixNano())
			return nil
		})).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	_ = m.Start()
	defer m.Stop()

	start := time.Now()
	if err := m.Dispatch(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed < 35*time.Millisecond {
		t.Fatalf("dispatch returned too early: %v", elapsed)
	}
	if atomic.LoadInt64(&stamp) == 0 {
		t.Fatalf("action not executed before return")
	}
	if m.Current() != "B" {
		t.Fatalf("want B got %v", m.Current())
	}
}

func TestMachine_IsActive(t *testing.T) {
	def, err := NewDef("visit").
		State("A", WithInitial()).State("B", WithFinal()).
		Current("A").
		On("go", "A", "B").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	if !m.IsActive("A") {
		t.Fatalf("A should be active after start")
	}
	if m.IsActive("B") {
		t.Fatalf("B should not be active yet")
	}
	if err := m.Dispatch(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}
	if !m.IsActive("B") {
		t.Fatalf("B should be active after transition")
	}
}

type errSub struct {
	lastErr error
}

func (s *errSub) OnTransition(from StateID, to StateID, e Event, err error) {
	s.lastErr = err
}

func TestMachine_Subscriber_OnError_NoTransition(t *testing.T) {
	def, err := NewDef("sub").
		State("A", WithInitial(), WithFinal()).Current("A").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	m := NewMachine[any](def, nil)
	sub := &errSub{}
	m.Subscribe(sub)
	_ = m.Start()
	defer m.Stop()

	if err := m.Dispatch(Event{Name: "unknown"}); !errors.Is(err, ErrNoTransition) {
		t.Fatalf("want ErrNoTransition got %v", err)
	}
	if !errors.Is(sub.lastErr, ErrNoTransition) {
		t.Fatalf("subscriber did not receive ErrNoTransition, got %v", sub.lastErr)
	}
}

func TestMachine_Start_EntryHookFailure(t *testing.T) {
	def, err := NewDef("test").
		State("A", WithInitial(), WithEntry[any](func(e Event, ctx any) error {
			return errors.New("entry failed")
		})).
		State("B", WithFinal()).
		Current("A").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err == nil {
		t.Fatal("expected error for entry hook failure")
	} else if err.Error() != "entry failed" {
		t.Fatalf("expected 'entry failed', got %q", err.Error())
	}
	// Verify machine is not started by trying to dispatch
	if err := m.Dispatch(Event{Name: "test"}); !errors.Is(err, ErrMachineNotStarted) {
		t.Fatal("machine should not be started after entry hook failure")
	}
}

func TestMachine_Stop_ExitHookFailure(t *testing.T) {
	def, err := NewDef("test").
		State("A", WithInitial(), WithExit[any](func(e Event, ctx any) error {
			return errors.New("exit failed")
		})).
		State("B", WithFinal()).
		Current("A").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	stopErr := m.Stop()
	if stopErr == nil {
		t.Fatal("expected error for exit hook failure")
	}
	if stopErr.Error() != "exit failed" {
		t.Fatalf("expected 'exit failed', got %q", stopErr.Error())
	}
}

func TestMachine_StartTwice(t *testing.T) {
	def, err := NewDef("test").
		State("A", WithInitial(), WithFinal()).
		Current("A").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal("starting twice should not error")
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestMachine_StopTwice(t *testing.T) {
	def, err := NewDef("test").
		State("A", WithInitial(), WithFinal()).
		Current("A").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(); err != nil {
		t.Fatal("stopping twice should not error")
	}
}

func TestMachine_EntryHookFailure_Rollback(t *testing.T) {
	var entryA, exitA, entryB int32
	def, err := NewDef("test").
		State("A", WithInitial(), WithEntry[any](func(e Event, ctx any) error { atomic.AddInt32(&entryA, 1); return nil }), WithExit[any](func(e Event, ctx any) error { atomic.AddInt32(&exitA, 1); return nil })).
		State("B", WithEntry[any](func(e Event, ctx any) error {
			atomic.AddInt32(&entryB, 1)
			return errors.New("entry failed")
		}), WithFinal()).
		Current("A").
		On("go", "A", "B").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	// Initial entry into A
	if entryA != 1 {
		t.Fatalf("entryA want 1 got %d", entryA)
	}

	// Transition to B, but B's entry hook fails
	if err := m.Dispatch(Event{Name: "go"}); !errors.Is(err, ErrHookFailed) {
		t.Fatalf("expected ErrHookFailed, got %v", err)
	}

	// Should still be in A
	if m.Current() != "A" {
		t.Fatalf("expected A, got %v", m.Current())
	}

	// A should have exited and re-entered
	if exitA != 1 {
		t.Fatalf("exitA want 1 got %d", exitA)
	}
	if entryA != 2 {
		t.Fatalf("entryA want 2 got %d", entryA)
	}

	// B should have been entered but failed
	if entryB != 1 {
		t.Fatalf("entryB want 1 got %d", entryB)
	}
}

func TestMachine_ExitHookFailure(t *testing.T) {
	var exitA int32
	def, err := NewDef("test").
		State("A", WithInitial(), WithExit[any](func(e Event, ctx any) error {
			atomic.AddInt32(&exitA, 1)
			return errors.New("exit failed")
		})).
		State("B", WithFinal()).
		Current("A").
		On("go", "A", "B").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	if err := m.Dispatch(Event{Name: "go"}); !errors.Is(err, ErrHookFailed) {
		t.Fatalf("expected ErrHookFailed, got %v", err)
	}

	// Should still be in A
	if m.Current() != "A" {
		t.Fatalf("expected A, got %v", m.Current())
	}

	if exitA != 1 {
		t.Fatalf("exitA want 1 got %d", exitA)
	}
}

func TestMachine_TransitionToCompositeState(t *testing.T) {
	// Test transition to a composite state (should drill down to initial child)
	sub, err := NewDef("sub").
		State("B1", WithInitial()).
		State("B2", WithFinal()).
		Current("B1").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	def, err := NewDef("test").
		State("A", WithInitial()).
		State("B", WithSubDef(sub), WithFinal()).
		Current("A").
		On("go", "A", "B").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	if m.Current() != "A" {
		t.Fatalf("initial current want A got %v", m.Current())
	}

	if err := m.Dispatch(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}

	// Should drill down to B1 (initial child of B)
	if m.Current() != "B1" {
		t.Fatalf("current want B1 got %v", m.Current())
	}

	path := m.CurrentPath()
	if len(path) != 2 || path[0] != "B" || path[1] != "B1" {
		t.Fatalf("path want [B B1] got %v", path)
	}
}

func TestMachine_TransitionSameLevel(t *testing.T) {
	// Test transition between states at the same level (no exit/entry of parent)
	sub, err := NewDef("sub").
		State("A1", WithInitial()).
		State("A2", WithFinal()).
		Current("A1").
		On("switch", "A1", "A2").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	def, err := NewDef("test").
		State("A", WithSubDef(sub), WithInitial()).
		State("B", WithFinal()).
		Current("A").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	// Initial state should be A1
	if m.Current() != "A1" {
		t.Fatalf("initial current want A1 got %v", m.Current())
	}

	if err := m.Dispatch(Event{Name: "switch"}); err != nil {
		t.Fatal(err)
	}

	// Should transition to A2 (same parent A)
	if m.Current() != "A2" {
		t.Fatalf("current want A2 got %v", m.Current())
	}

	// A should still be active
	if !m.IsActive("A") {
		t.Fatal("A should still be active")
	}
}

func TestMachine_EventWithArgs(t *testing.T) {
	var receivedArgs []any
	def, err := NewDef("test").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", "A", "B", WithAction[any](func(e Event, ctx any) error {
			receivedArgs = e.Args
			return nil
		})).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	args := []any{42, "test", true}
	if err := m.Dispatch(Event{Name: "go", Args: args}); err != nil {
		t.Fatal(err)
	}

	if len(receivedArgs) != len(args) {
		t.Fatalf("args length want %d got %d", len(args), len(receivedArgs))
	}
	if receivedArgs[0] != 42 {
		t.Fatalf("arg[0] want 42 got %v", receivedArgs[0])
	}
	if receivedArgs[1] != "test" {
		t.Fatalf("arg[1] want 'test' got %v", receivedArgs[1])
	}
	if receivedArgs[2] != true {
		t.Fatalf("arg[2] want true got %v", receivedArgs[2])
	}
}

func TestMachine_GuardBlocksTransition(t *testing.T) {
	var guardCalled int32
	allow := false
	def, err := NewDef("test").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", "A", "B", WithGuard[any](func(e Event, ctx any) bool {
			atomic.AddInt32(&guardCalled, 1)
			return allow
		})).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	// Guard returns false, should block transition
	if err := m.Dispatch(Event{Name: "go"}); !errors.Is(err, ErrNoTransition) {
		t.Fatalf("expected ErrNoTransition, got %v", err)
	}

	if atomic.LoadInt32(&guardCalled) != 1 {
		t.Fatalf("guard should be called once, got %d", guardCalled)
	}

	if m.Current() != "A" {
		t.Fatalf("should still be in A, got %v", m.Current())
	}

	// Allow transition
	allow = true
	if err := m.Dispatch(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}

	if atomic.LoadInt32(&guardCalled) != 2 {
		t.Fatalf("guard should be called twice, got %d", guardCalled)
	}

	if m.Current() != "B" {
		t.Fatalf("should be in B, got %v", m.Current())
	}
}

func TestMachine_Next_SingleTransition(t *testing.T) {
	def, err := NewDef("next").
		State("A", WithInitial()).
		State("B").
		State("C", WithFinal()).
		Current("A").
		On("go", "A", "B").
		On("next", "B", "C").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	// Should advance from A to B
	if err := m.Next(); err != nil {
		t.Fatalf("Next() should succeed, got %v", err)
	}
	if got := m.Current(); got != "B" {
		t.Fatalf("expected state B, got %v", got)
	}

	// Should advance from B to C
	if err := m.Next(); err != nil {
		t.Fatalf("Next() should succeed, got %v", err)
	}
	if got := m.Current(); got != "C" {
		t.Fatalf("expected state C, got %v", got)
	}
}

func TestMachine_Next_NoTransition(t *testing.T) {
	def, err := NewDef("next").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", "A", "B").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	// Move to final state
	if err := m.Dispatch(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}

	// No transitions from final state
	if err := m.Next(); !errors.Is(err, ErrNoAvailableTransition) {
		t.Fatalf("expected ErrNoAvailableTransition, got %v", err)
	}
}

func TestMachine_Next_MultipleTransitions(t *testing.T) {
	def, err := NewDef("next").
		State("A", WithInitial()).
		State("B", WithFinal()).
		State("C", WithFinal()).
		Current("A").
		On("go1", "A", "B").
		On("go2", "A", "C").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	// Multiple transitions available, should fail
	if err := m.Next(); !errors.Is(err, ErrMultipleTransitions) {
		t.Fatalf("expected ErrMultipleTransitions, got %v", err)
	}
	if got := m.Current(); got != "A" {
		t.Fatalf("state should not change, expected A, got %v", got)
	}
}

func TestMachine_Next_WithGuard(t *testing.T) {
	allow := false
	def, err := NewDef("next").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", "A", "B", WithGuard[any](func(e Event, ctx any) bool { return allow })).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	// Guard blocks transition
	if err := m.Next(); !errors.Is(err, ErrNoAvailableTransition) {
		t.Fatalf("expected ErrNoAvailableTransition, got %v", err)
	}

	// Allow transition
	allow = true
	if err := m.Next(); err != nil {
		t.Fatalf("Next() should succeed, got %v", err)
	}
	if got := m.Current(); got != "B" {
		t.Fatalf("expected state B, got %v", got)
	}
}

func TestMachine_Next_BeforeStart(t *testing.T) {
	def, err := NewDef("next").
		State("A", WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", "A", "B").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine[any](def, nil)

	// Should fail before start
	if err := m.Next(); !errors.Is(err, ErrMachineNotStarted) {
		t.Fatalf("expected ErrMachineNotStarted, got %v", err)
	}
}
