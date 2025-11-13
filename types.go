package rfsm

import "errors"

// Basic event type
type Event struct {
	Name string
	Args []any
}

// Hooks, actions, and guards (generic for type-safe context)
type GuardFunc[C any] func(e Event, ctx C) bool
type ActionFunc[C any] func(e Event, ctx C) error
type HookFunc[C any] func(e Event, ctx C) error

// Internal storage uses any for compatibility across different context types
type guardFuncAny func(e Event, ctx any) bool
type actionFuncAny func(e Event, ctx any) error
type hookFuncAny func(e Event, ctx any) error

// State ID
type StateID = string
type EventID = string

// State definition (immutable)
type StateDef struct {
	ID          StateID
	Description string
	OnEntry     hookFuncAny
	OnExit      hookFuncAny
	// Hierarchy
	Parent       StateID   // empty means no parent (top-level)
	Children     []StateID // non-empty => composite state
	InitialChild StateID   // valid only if Children non-empty
	// Build-time: optional sub-definition to merge into this composite state
	SubDef *Definition
	// Initial indicates this is a entry state (no incoming transitions by convention)
	Initial bool
	// Final indicates this is a terminal state (no outgoing transitions by convention)
	Final bool
}

type TransitionKey struct {
	From  StateID
	Event EventID
}

// Transition definition (immutable)
type TransitionDef struct {
	Key    TransitionKey
	To     StateID
	Guard  guardFuncAny
	Action actionFuncAny
}

// Definition is the built, read-only state machine definition
type Definition struct {
	Name        string
	States      map[StateID]StateDef
	Transitions map[TransitionKey]TransitionDef
	Current     StateID
	// cached topology (computed on demand)
	topology *GraphTopology
	// OutgoingTransitions maps each state to its outgoing transition keys for fast lookup
	OutgoingTransitions map[StateID][]TransitionKey
}

// Runtime errors
var (
	ErrMachineNotStarted     = errors.New("machine not started")
	ErrMachineStopped        = errors.New("machine stopped")
	ErrNoTransition          = errors.New("no transition matched")
	ErrHookFailed            = errors.New("hook failed")
	ErrActionFailed          = errors.New("action failed")
	ErrMultipleTransitions   = errors.New("multiple transitions available, cannot auto-advance")
	ErrNoAvailableTransition = errors.New("no available transition from current state")
)
