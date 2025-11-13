package rfsm

import "fmt"

// Builder interfaces
type DefinitionBuilder interface {
	State(id StateID, opts ...StateOption) DefinitionBuilder
	On(event string, from, to StateID, opts ...TransitionOption) DefinitionBuilder
	Current(id StateID) DefinitionBuilder
	InitialChild(parent StateID, child StateID) DefinitionBuilder
	Build() (*Definition, error)
}

type StateOption func(*StateDef)
type TransitionOption func(*TransitionDef)

// Internal implementation
type builder struct {
	name        string
	states      map[StateID]StateDef
	transitions map[TransitionKey]TransitionDef
	current     *StateID
	hasInitial  bool
	hasFinal    bool
}

func NewDef(name string) DefinitionBuilder {
	return &builder{
		name:        name,
		states:      make(map[StateID]StateDef),
		transitions: make(map[TransitionKey]TransitionDef),
	}
}

// State options
func WithEntry[C any](h HookFunc[C]) StateOption {
	return func(s *StateDef) {
		s.OnEntry = func(e Event, ctx any) error {
			var c C
			if ctx != nil {
				c = ctx.(C)
			}
			return h(e, c)
		}
	}
}

func WithExit[C any](h HookFunc[C]) StateOption {
	return func(s *StateDef) {
		s.OnExit = func(e Event, ctx any) error {
			var c C
			if ctx != nil {
				c = ctx.(C)
			}
			return h(e, c)
		}
	}
}

func WithDescription(desc string) StateOption { return func(s *StateDef) { s.Description = desc } }
func WithSubDef(sub *Definition) StateOption  { return func(s *StateDef) { s.SubDef = sub } }
func WithFinal() StateOption                  { return func(s *StateDef) { s.Final = true } }
func WithInitial() StateOption                { return func(s *StateDef) { s.Initial = true } }

// Transition options
func WithGuard[C any](fn GuardFunc[C]) TransitionOption {
	return func(t *TransitionDef) {
		t.Guard = func(e Event, ctx any) bool {
			var c C
			if ctx != nil {
				c = ctx.(C)
			}
			return fn(e, c)
		}
	}
}
func WithAction[C any](fn ActionFunc[C]) TransitionOption {
	return func(t *TransitionDef) {
		t.Action = func(e Event, ctx any) error {
			var c C
			if ctx != nil {
				c = ctx.(C)
			}
			return fn(e, c)
		}
	}
}

func (b *builder) State(id StateID, opts ...StateOption) DefinitionBuilder {
	var def StateDef
	if existing, ok := b.states[id]; ok {
		def = existing
	} else {
		def = StateDef{ID: id}
	}

	for _, opt := range opts {
		opt(&def)
	}

	if def.Initial {
		b.hasInitial = true
	}
	if def.Final {
		b.hasFinal = true
	}

	// If SubDef is provided, merge sub-definition into composite state
	if def.SubDef != nil {
		sub := def.SubDef
		var children []StateID
		// merge states
		for sid, s := range sub.States {
			if _, ok := b.states[sid]; ok {
				panic(fmt.Sprintf("duplicate state id %q when merging sub definition into %q", sid, id))
			}
			if s.Parent == "" {
				s.Parent = id
				children = append(children, sid)
			}
			b.states[sid] = s
		}
		def.Children = append(def.Children, children...)
		if def.InitialChild == "" {
			def.InitialChild = sub.Current
		}
		// merge transitions
		for _, t := range sub.Transitions {
			if _, ok := b.transitions[t.Key]; ok {
				panic(fmt.Sprintf("duplicate transition key %q when merging sub definition into %q", t.Key, id))
			}
			b.transitions[t.Key] = t
		}
		// clear build-time field
		def.SubDef = nil
	}

	b.states[id] = def
	return b
}

func (b *builder) On(event string, from, to StateID, opts ...TransitionOption) DefinitionBuilder {
	tk := TransitionKey{From: from, Event: event}
	t, ok := b.transitions[tk]
	if !ok {
		t = TransitionDef{Key: tk, To: to}
	} else {
		// fail early
		if t.To != to {
			panic(fmt.Sprintf("transition with event %q from %q already exists with different target state: %q != %q", event, from, t.To, to))
		}
	}
	for _, opt := range opts {
		opt(&t)
	}
	b.transitions[tk] = t
	return b
}

func (b *builder) Current(id StateID) DefinitionBuilder {
	b.current = &id
	return b
}

func (b *builder) InitialChild(parent StateID, child StateID) DefinitionBuilder {
	st := b.states[parent]
	st.InitialChild = child
	b.states[parent] = st
	return b
}

func (b *builder) Build() (*Definition, error) {
	if b.current == nil {
		return nil, fmt.Errorf("current state not set")
	}
	// Validate: current exists
	if _, ok := b.states[*b.current]; !ok {
		return nil, fmt.Errorf("current state %q not defined", *b.current)
	}
	if !b.hasInitial {
		return nil, fmt.Errorf("at least one state must be marked with WithInitial()")
	}
	if !b.hasFinal {
		return nil, fmt.Errorf("at least one state must be marked with WithFinal()")
	}
	// Validate: all transitions reference defined states
	for k, t := range b.transitions {
		if k != t.Key {
			return nil, fmt.Errorf("transition key mismatch for transition %q from %q", t.Key.Event, k.From)
		}
		if _, ok := b.states[k.From]; !ok {
			return nil, fmt.Errorf("transition from undefined state %q", k.From)
		}
		if _, ok := b.states[t.To]; !ok {
			return nil, fmt.Errorf("transition to undefined state %q", t.To)
		}
		if t.Key.Event == "" {
			return nil, fmt.Errorf("transition event is empty")
		}
	}
	// Validate: hierarchy
	for id, st := range b.states {
		if len(st.Children) > 0 {
			// initial child must be one of children
			if st.InitialChild == "" {
				return nil, fmt.Errorf("composite state %q requires InitialChild", id)
			}
			okChild := false
			for _, c := range st.Children {
				if c == st.InitialChild {
					okChild = true
					break
				}
			}
			if !okChild {
				return nil, fmt.Errorf("InitialChild %q not in children of %q", st.InitialChild, id)
			}
			// children must exist and parent must be set to this id
			for _, c := range st.Children {
				cst, ok := b.states[c]
				if !ok {
					return nil, fmt.Errorf("child state %q of %q not defined", c, id)
				}
				if cst.Parent != id {
					return nil, fmt.Errorf("child %q of %q has wrong parent %q", c, id, cst.Parent)
				}
			}
		}
		// parent exists if set
		if st.Parent != "" {
			if _, ok := b.states[st.Parent]; !ok {
				return nil, fmt.Errorf("state %q references missing parent %q", id, st.Parent)
			}
		}
	}
	d := &Definition{
		Name:        b.name,
		States:      b.states,
		Transitions: b.transitions,
		Current:     *b.current,
	}
	return d, nil
}

// IsBefore reports whether a appears before b in the definition's topological order.
// Returns false with error if a cycle exists or states are missing.
func (d *Definition) IsBefore(a, b StateID) (bool, error) {
	topo, err := d.ensureTopology()
	if err != nil {
		return false, err
	}
	return topo.IsBefore(a, b), nil
}

// IsAfter reports whether a appears after b in the definition's topological order.
// Returns false with error if a cycle exists or states are missing.
func (d *Definition) IsAfter(a, b StateID) (bool, error) {
	topo, err := d.ensureTopology()
	if err != nil {
		return false, err
	}
	return topo.IsAfter(a, b), nil
}
