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

	statusMu   sync.RWMutex
	current    StateID
	activePath []StateID
	started    bool

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
	// compute initial active path and enter hooks from root to leaf
	root := m.def.Initial
	path := []StateID{root}
	cur := root
	for {
		st := m.def.States[cur]
		if len(st.Children) == 0 {
			break
		}
		child := st.InitialChild
		path = append(path, child)
		cur = child
	}
	m.current = cur
	m.activePath = path
	// recreate channels to support restart; clear any stale events
	buf := cap(m.events)
	if buf <= 0 {
		buf = 64
	}
	m.events = make(chan Event, buf)
	m.done = make(chan struct{})
	m.started = true
	for _, sid := range m.activePath {
		if st, ok := m.def.States[sid]; ok && st.OnEntry != nil {
			if err := st.OnEntry(Event{}, m.ctx); err != nil {
				m.started = false
				return err
			}
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
	// Execute exit hooks from leaf to root
	path := m.CurrentPath()
	for i := len(path) - 1; i >= 0; i-- {
		if st, ok := m.def.States[path[i]]; ok && st.OnExit != nil {
			if err := st.OnExit(Event{}, m.ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Machine) Current() StateID {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	return m.current
}

// CurrentPath returns the active path from root to leaf
func (m *Machine) CurrentPath() []StateID {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	cp := make([]StateID, len(m.activePath))
	copy(cp, m.activePath)
	return cp
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

	// Bubble from leaf to root to find matching transition
	path := m.CurrentPath()
	var matched *TransitionDef
	var source StateID
	for i := len(path) - 1; i >= 0 && matched == nil; i-- {
		s := path[i]
		for j := range m.def.Transitions {
			t := &m.def.Transitions[j]
			if t.From == s && t.Event == e.Name {
				if t.Guard == nil || t.Guard(e, m.ctx) {
					matched = t
					source = s
					break
				}
			}
		}
	}
	if matched == nil {
		m.notify(from, from, e, ErrNoTransition)
		return ErrNoTransition
	}

	// Compute sequences via LCA between source and target
	to := matched.To
	exitSeq, entrySeq := m.computeTransitionSequences(source, to)
	// Exit
	for _, sid := range exitSeq {
		if st, ok := m.def.States[sid]; ok && st.OnExit != nil {
			if err := st.OnExit(e, m.ctx); err != nil {
				m.notify(from, from, e, ErrHookFailed)
				return ErrHookFailed
			}
		}
	}

	if matched.Action != nil {
		if err := matched.Action(e, m.ctx); err != nil {
			// Rollback: re-enter exited states in reverse order
			for i := len(exitSeq) - 1; i >= 0; i-- {
				if st, ok := m.def.States[exitSeq[i]]; ok && st.OnEntry != nil {
					_ = st.OnEntry(Event{}, m.ctx)
				}
			}
			m.notify(from, from, e, ErrActionFailed)
			return ErrActionFailed
		}
	}

	// Entry
	for _, sid := range entrySeq {
		if st, ok := m.def.States[sid]; ok && st.OnEntry != nil {
			if err := st.OnEntry(e, m.ctx); err != nil {
				// Rollback: exit entered and re-enter exited
				for i := len(entrySeq) - 1; i >= 0; i-- {
					if entrySeq[i] == sid {
						break
					}
					if st2, ok2 := m.def.States[entrySeq[i]]; ok2 && st2.OnExit != nil {
						_ = st2.OnExit(e, m.ctx)
					}
				}
				for i := len(exitSeq) - 1; i >= 0; i-- {
					if st2, ok2 := m.def.States[exitSeq[i]]; ok2 && st2.OnEntry != nil {
						_ = st2.OnEntry(Event{}, m.ctx)
					}
				}
				m.notify(from, from, e, ErrHookFailed)
				return ErrHookFailed
			}
		}
	}

	// Commit new state
	m.statusMu.Lock()
	// final leaf is the last in entrySeq
	leaf := entrySeq[len(entrySeq)-1]
	m.current = leaf
	m.activePath = m.pathTo(leaf)
	m.statusMu.Unlock()

	m.notify(from, m.current, e, nil)
	return nil
}

// computeTransitionSequences returns exit sequence (leaf->up excluding LCA)
// and entry sequence (LCA->down including drilling to leaf)
func (m *Machine) computeTransitionSequences(from StateID, to StateID) ([]StateID, []StateID) {
	fromPath := m.pathTo(from)
	toPath := m.pathTo(to)
	// find LCA index
	i := 0
	for i < len(fromPath) && i < len(toPath) && fromPath[i] == toPath[i] {
		i++
	}
	var exitSeq []StateID
	for x := len(fromPath) - 1; x >= i; x-- {
		exitSeq = append(exitSeq, fromPath[x])
	}
	var entrySeq []StateID
	// start from LCA towards target
	if i < len(toPath) {
		for x := i; x < len(toPath); x++ {
			entrySeq = append(entrySeq, toPath[x])
		}
	} else {
		entrySeq = append(entrySeq, to)
	}
	// drill down from target to its initial descendants
	cur := to
	for {
		st := m.def.States[cur]
		if len(st.Children) == 0 {
			break
		}
		child := st.InitialChild
		entrySeq = append(entrySeq, child)
		cur = child
	}
	return exitSeq, entrySeq
}

// pathTo returns path from root to s (inclusive)
func (m *Machine) pathTo(s StateID) []StateID {
	// climb to root
	var rev []StateID
	cur := s
	for {
		rev = append(rev, cur)
		p := m.def.States[cur].Parent
		if p == "" {
			break
		}
		cur = p
	}
	// reverse
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}
