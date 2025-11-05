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
		State("S", WithEntry(func(e Event, ctx any) error { atomic.AddInt32(&entry, 1); return nil }), WithExit(func(e Event, ctx any) error { atomic.AddInt32(&exit, 1); return nil })).
		Current("S").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine(def, nil, 4)
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
		State("A").Current("A").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	m := NewMachine(def, nil, 1)

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
		State("A").State("B").
		Current("A").
		On("go", WithFrom("A"), WithTo("B"), WithAction(func(e Event, ctx any) error {
			time.Sleep(40 * time.Millisecond)
			atomic.StoreInt64(&stamp, time.Now().UnixNano())
			return nil
		})).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine(def, nil, 2)
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
		State("A").State("B").
		Current("A").
		On("go", WithFrom("A"), WithTo("B")).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine(def, nil, 2)
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
		State("A").Current("A").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	m := NewMachine(def, nil, 2)
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
