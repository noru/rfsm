package rfsm

import (
	"sync"
)

// Subscriber interface
type Subscriber interface {
	OnTransition(from StateID, to StateID, e Event, err error)
}

type Machine struct {
	def    *Definition
	ctx    any
	events chan Event
	done   chan struct{}
	wg     sync.WaitGroup

	statusMu sync.RWMutex
	current  StateID
	started  bool

	subsMu      sync.RWMutex
	subscribers []Subscriber
}

func NewMachine(def *Definition, ctx any, buf int) *Machine {
	if buf <= 0 {
		buf = 64
	}
	return &Machine{
		def:    def,
		ctx:    ctx,
		events: make(chan Event, buf),
		done:   make(chan struct{}),
	}
}

func (m *Machine) Start() error {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	if m.started {
		return nil
	}
	m.current = m.def.Initial
	m.started = true
	// Execute initial state's entry hook
	if st, ok := m.def.States[m.current]; ok && st.OnEntry != nil {
		if err := st.OnEntry(Event{}, m.ctx); err != nil {
			m.started = false
			return err
		}
	}
	m.wg.Add(1)
	go m.loop()
	return nil
}

func (m *Machine) Stop() error {
	m.statusMu.Lock()
	if !m.started {
		m.statusMu.Unlock()
		return nil
	}
	m.started = false
	close(m.done)
	m.statusMu.Unlock()
	m.wg.Wait()
	// Execute current state's exit hook
	if st, ok := m.def.States[m.Current()]; ok && st.OnExit != nil {
		if err := st.OnExit(Event{}, m.ctx); err != nil {
			return err
		}
	}
	return nil
}

func (m *Machine) Current() StateID {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	return m.current
}

func (m *Machine) Subscribe(s Subscriber) {
	m.subsMu.Lock()
	m.subscribers = append(m.subscribers, s)
	m.subsMu.Unlock()
}

func (m *Machine) Dispatch(e Event) error {
	m.statusMu.RLock()
	started := m.started
	m.statusMu.RUnlock()
	if !started {
		return ErrMachineNotStarted
	}
	// 使用一条结果通道等待完成
	done := make(chan error, 1)
	// 通过一个特殊事件封装同步等待
	wrapper := Event{
		Name: e.Name,
		Args: append([]any{}, e.Args...),
	}
	// 利用 goroutine 等待处理完成信号
	// 这里的完成信号通过订阅或内联处理返回
	// 简化：直接在事件处理完成后写入 done（见 loop 内）
	wrapper.Args = append(wrapper.Args, done)
	select {
	case m.events <- wrapper:
		return <-done
	case <-m.done:
		return ErrMachineStopped
	}
}

func (m *Machine) DispatchAsync(e Event) error {
	m.statusMu.RLock()
	started := m.started
	m.statusMu.RUnlock()
	if !started {
		return ErrMachineNotStarted
	}
	select {
	case m.events <- e:
		return nil
	case <-m.done:
		return ErrMachineStopped
	}
}

func (m *Machine) loop() {
	defer m.wg.Done()
	for {
		select {
		case <-m.done:
			return
		case e := <-m.events:
			var syncCh chan error
			// If the last arg is chan error, treat this as sync dispatch
			if n := len(e.Args); n > 0 {
				if ch, ok := e.Args[n-1].(chan error); ok {
					syncCh = ch
					e.Args = e.Args[:n-1]
				}
			}
			err := m.handleEvent(e)
			if syncCh != nil {
				syncCh <- err
			}
		}
	}
}

func (m *Machine) notify(from, to StateID, e Event, err error) {
	m.subsMu.RLock()
	subs := append([]Subscriber(nil), m.subscribers...)
	m.subsMu.RUnlock()
	for _, s := range subs {
		s.OnTransition(from, to, e, err)
	}
}

func (m *Machine) handleEvent(e Event) error {
	m.statusMu.RLock()
	if !m.started {
		m.statusMu.RUnlock()
		return ErrMachineStopped
	}
	from := m.current
	m.statusMu.RUnlock()

	// Find first matching transition
	var matched *TransitionDef
	for i := range m.def.Transitions {
		t := &m.def.Transitions[i]
		if t.From == from && t.Event == e.Name {
			if t.Guard == nil || t.Guard(e, m.ctx) {
				matched = t
				break
			}
		}
	}
	if matched == nil {
		m.notify(from, from, e, ErrNoTransition)
		return ErrNoTransition
	}

	// Execute Exit -> Action -> Entry with rollback on error
	stFrom := m.def.States[from]
	if stFrom.OnExit != nil {
		if err := stFrom.OnExit(e, m.ctx); err != nil {
			m.notify(from, from, e, ErrHookFailed)
			return ErrHookFailed
		}
	}

	if matched.Action != nil {
		if err := matched.Action(e, m.ctx); err != nil {
			// Rollback: re-run from.OnEntry to keep semantics (still in from)
			if stFrom.OnEntry != nil {
				_ = stFrom.OnEntry(Event{}, m.ctx)
			}
			m.notify(from, from, e, ErrActionFailed)
			return ErrActionFailed
		}
	}

	to := matched.To
	stTo := m.def.States[to]
	if stTo.OnEntry != nil {
		if err := stTo.OnEntry(e, m.ctx); err != nil {
			// Rollback: try to recover by re-entering from
			if stFrom.OnEntry != nil {
				_ = stFrom.OnEntry(Event{}, m.ctx)
			}
			m.notify(from, from, e, ErrHookFailed)
			return ErrHookFailed
		}
	}

	// Commit new state
	m.statusMu.Lock()
	m.current = to
	m.statusMu.Unlock()

	m.notify(from, to, e, nil)
	return nil
}
