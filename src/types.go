package rfsm

import "errors"

// Basic event type
type Event struct {
	Name string
	Args []any
}

// Hooks, actions, and guards
type GuardFunc func(e Event, ctx any) bool
type ActionFunc func(e Event, ctx any) error
type HookFunc func(e Event, ctx any) error

// State ID
type StateID string

// State definition (immutable)
type StateDef struct {
	ID          StateID
	Description string
	OnEntry     HookFunc
	OnExit      HookFunc
	// Hierarchy
	Parent       StateID   // empty means no parent (top-level)
	Children     []StateID // non-empty => composite state
	InitialChild StateID   // valid only if Children non-empty
	// Build-time: optional sub-definition to merge into this composite state
	SubDef *Definition
}

// Transition definition (immutable)
type TransitionDef struct {
	From   StateID
	Event  string
	To     StateID
	Guard  GuardFunc
	Action ActionFunc
}

// Definition is the built, read-only state machine definition
type Definition struct {
	Name        string
	States      map[StateID]StateDef
	Transitions []TransitionDef
	Initial     StateID
	// cached topology (computed on demand)
	topology *GraphTopology
}

// Runtime errors
var (
	ErrMachineNotStarted = errors.New("machine not started")
	ErrMachineStopped    = errors.New("machine stopped")
	ErrNoTransition      = errors.New("no transition matched")
	ErrHookFailed        = errors.New("hook failed")
	ErrActionFailed      = errors.New("action failed")
)


