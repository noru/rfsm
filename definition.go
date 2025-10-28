package rfsm

import "fmt"

// Builder interfaces
type DefinitionBuilder interface {
	State(id StateID, opts ...StateOption) DefinitionBuilder
	On(event string) TransitionBuilder
	Initial(id StateID) DefinitionBuilder
	Build() (*Definition, error)
}

type StateOption func(*StateDef)

type TransitionBuilder interface {
	From(id StateID) TransitionBuilder
	To(id StateID) TransitionBuilder
	Guard(fn GuardFunc) TransitionBuilder
	Action(fn ActionFunc) TransitionBuilder
	Done() DefinitionBuilder
}

// Internal implementation
type builder struct {
	name        string
	states      map[StateID]StateDef
	transitions []TransitionDef
	initial     *StateID

	// Temporary transition being built
	cur TransitionDef
}

func NewDef(name string) DefinitionBuilder {
	return &builder{
		name:        name,
		states:      make(map[StateID]StateDef),
		transitions: make([]TransitionDef, 0, 8),
	}
}

func WithEntry(h HookFunc) StateOption { return func(s *StateDef) { s.OnEntry = h } }
func WithExit(h HookFunc) StateOption  { return func(s *StateDef) { s.OnExit = h } }

func (b *builder) State(id StateID, opts ...StateOption) DefinitionBuilder {
	def := StateDef{ID: id}
	for _, opt := range opts {
		opt(&def)
	}
	b.states[id] = def
	return b
}

func (b *builder) On(event string) TransitionBuilder {
	b.cur = TransitionDef{Event: event}
	return b
}

func (b *builder) From(id StateID) TransitionBuilder {
	b.cur.From = id
	return b
}

func (b *builder) To(id StateID) TransitionBuilder {
	b.cur.To = id
	return b
}

func (b *builder) Guard(fn GuardFunc) TransitionBuilder {
	b.cur.Guard = fn
	return b
}

func (b *builder) Action(fn ActionFunc) TransitionBuilder {
	b.cur.Action = fn
	return b
}

func (b *builder) Done() DefinitionBuilder {
	b.transitions = append(b.transitions, b.cur)
	b.cur = TransitionDef{}
	return b
}

func (b *builder) Initial(id StateID) DefinitionBuilder {
	b.initial = &id
	return b
}

func (b *builder) Build() (*Definition, error) {
	if b.initial == nil {
		return nil, fmt.Errorf("initial state not set")
	}
	// Validate: initial exists
	if _, ok := b.states[*b.initial]; !ok {
		return nil, fmt.Errorf("initial state %q not defined", *b.initial)
	}
	// Validate: all transitions reference defined states
	for _, t := range b.transitions {
		if _, ok := b.states[t.From]; !ok {
			return nil, fmt.Errorf("transition from undefined state %q", t.From)
		}
		if _, ok := b.states[t.To]; !ok {
			return nil, fmt.Errorf("transition to undefined state %q", t.To)
		}
		if t.Event == "" {
			return nil, fmt.Errorf("transition event is empty")
		}
	}
	d := &Definition{
		Name:        b.name,
		States:      b.states,
		Transitions: b.transitions,
		Initial:     *b.initial,
	}
	return d, nil
}
