package rfsm

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestBasicTransition(t *testing.T) {
	def, err := NewDef("turnstile").
		State("Locked").
		State("Unlocked").
		Initial("Locked").
		On("coin").From("Locked").To("Unlocked").Done().
		On("push").From("Unlocked").To("Locked").Done().
		Build()
	if err != nil {
		t.Fatalf("build err: %v", err)
	}

	m := NewMachine(def, nil, 8)
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
		State("A").State("B").
		Initial("A").
		On("go").From("A").To("B").Guard(func(e Event, ctx any) bool { return allow }).Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine(def, nil, 8)
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
		State("A", WithEntry(func(e Event, ctx any) error { atomic.AddInt32(&entryA, 1); return nil }), WithExit(func(e Event, ctx any) error { atomic.AddInt32(&exitA, 1); return nil })).
		State("B", WithEntry(func(e Event, ctx any) error { atomic.AddInt32(&entryB, 1); return nil })).
		Initial("A").
		On("go").From("A").To("B").Action(func(e Event, ctx any) error {
		if fail {
			return errors.New("boom")
		}
		return nil
	}).Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine(def, nil, 8)
	_ = m.Start()
	defer m.Stop()

	// 初始进入 A 一次
	if entryA != 1 {
		t.Fatalf("entryA want 1 got %d", entryA)
	}

	// 失败触发：应回滚，仍在 A，且 entryA 再次被调用以恢复
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

	// 成功后进入 B
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
		State("A").State("B").
		Initial("A").
		On("go").From("A").To("B").Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine(def, nil, 1)
	sub := &recSub{}
	m.Subscribe(sub)
	_ = m.Start()
	defer m.Stop()

	if err := m.DispatchAsync(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}
	// 等待异步处理
	time.Sleep(50 * time.Millisecond)
	if got := m.Current(); got != "B" {
		t.Fatalf("want B got %v", got)
	}
	if atomic.LoadInt32(&sub.count) == 0 {
		t.Fatalf("want subscriber called")
	}
}
