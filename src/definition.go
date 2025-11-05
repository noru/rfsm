package rfsm

import "fmt"

// Builder interfaces
type DefinitionBuilder interface {
	State(id StateID, opts ...StateOption) DefinitionBuilder
	On(event string, opts ...TransitionOption) DefinitionBuilder
	Initial(id StateID) DefinitionBuilder
	InitialChild(parent StateID, child StateID) DefinitionBuilder
	Build() (*Definition, error)
}

type StateOption func(*StateDef)
type TransitionOption func(*TransitionDef)

// Internal implementation
type builder struct {
	name        string
	states      map[StateID]StateDef
	transitions []TransitionDef
	initial     *StateID
}

func NewDef(name string) DefinitionBuilder {
	return &builder{
		name:        name,
		states:      make(map[StateID]StateDef),
		transitions: make([]TransitionDef, 0, 8),
	}
}

func WithEntry(h HookFunc) StateOption        { return func(s *StateDef) { s.OnEntry = h } }
func WithExit(h HookFunc) StateOption         { return func(s *StateDef) { s.OnExit = h } }
func WithDescription(desc string) StateOption { return func(s *StateDef) { s.Description = desc } }
func WithSubDef(sub *Definition) StateOption  { return func(s *StateDef) { s.SubDef = sub } }
func WithFinal() StateOption                  { return func(s *StateDef) { s.Final = true } }

// Transition options (With* for naming consistency)
func WithFrom(id StateID) TransitionOption      { return func(t *TransitionDef) { t.From = id } }
func WithTo(id StateID) TransitionOption        { return func(t *TransitionDef) { t.To = id } }
func WithGuard(fn GuardFunc) TransitionOption   { return func(t *TransitionDef) { t.Guard = fn } }
func WithAction(fn ActionFunc) TransitionOption { return func(t *TransitionDef) { t.Action = fn } }

func (b *builder) State(id StateID, opts ...StateOption) DefinitionBuilder {
	def := StateDef{ID: id}
	for _, opt := range opts {
		opt(&def)
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
			def.InitialChild = sub.Initial
		}
		// merge transitions
		b.transitions = append(b.transitions, sub.Transitions...)
		// clear build-time field
		def.SubDef = nil
	}
	b.states[id] = def
	return b
}

func (b *builder) On(event string, opts ...TransitionOption) DefinitionBuilder {
	t := TransitionDef{Event: event}
	for _, opt := range opts {
		opt(&t)
	}
	b.transitions = append(b.transitions, t)
	return b
}

// removed chained TransitionBuilder in favor of option-style On

func (b *builder) Initial(id StateID) DefinitionBuilder {
	b.initial = &id
	return b
}

func (b *builder) InitialChild(parent StateID, child StateID) DefinitionBuilder {
	st := b.states[parent]
	st.InitialChild = child
	b.states[parent] = st
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
		Initial:     *b.initial,
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
