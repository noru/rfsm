package main

import (
	"fmt"

	rfsm "github.com/ethan/rfsm/src"
)

func Transit() {
	// Composite groups: FIAT / HEDGE / CRYPTO with single internal state
	def := DefineFiatCryptoFlow()

	// Simulate state machine execution
	fmt.Println("=== State Machine Execution Simulation ===")
	simulateMachine(def)
}

type transitionLogger struct{}

func (l *transitionLogger) OnTransition(from rfsm.StateID, to rfsm.StateID, e rfsm.Event, err error) {
	if err != nil {
		fmt.Printf("  ❌ Transition failed: %s -> %s (event: %s, error: %v)\n", from, to, e.Name, err)
	} else {
		fmt.Printf("  ✓ %s -> %s (event: %s)\n", from, to, e.Name)
	}
}

func simulateMachine(def *rfsm.Definition) {
	m := rfsm.NewMachine(def, nil, 10)
	logger := &transitionLogger{}
	m.Subscribe(logger)

	// Start machine
	if err := m.Start(); err != nil {
		fmt.Printf("Failed to start machine: %v\n", err)
		return
	}
	fmt.Printf("Machine started at: %s\n", m.Current())
	fmt.Println()

	// Success path simulation
	events := []struct {
		name string
		desc string
	}{
		{"init_next", "Initialize flow"},
		{"start_fiat", "Start FIAT deposit process"},
		{"success", "FIAT deposit succeeded"},
		{"to_hedge", "Move to HEDGE stage"},
		{"executed", "HEDGE executed successfully"},
		{"to_crypto", "Move to CRYPTO stage"},
		{"start_crypto", "Start CRYPTO withdrawal"},
		{"success", "CRYPTO withdrawal succeeded"},
		{"to_success", "Complete flow"},
	}

	for i, evt := range events {
		fmt.Printf("Step %d: %s\n", i+1, evt.desc)
		fmt.Printf("  Current state: %s\n", m.Current())
		fmt.Printf("  Active path: %v\n", m.CurrentPath())

		if err := m.Dispatch(rfsm.Event{Name: evt.name}); err != nil {
			fmt.Printf("  ❌ Error: %v\n", err)
			break
		}

		fmt.Printf("  New state: %s\n", m.Current())
		fmt.Printf("  New path: %v\n", m.CurrentPath())
		fmt.Println()

		// Check if reached final state
		if st, ok := def.States[m.Current()]; ok && st.Final {
			if st.Parent == "" {
				fmt.Printf("✅ Reached final state: %s\n", m.Current())
				break
			} else {
				fmt.Printf("✅ Reached final state in composite: %s\n", m.Current())
			}
		}
	}

	// Stop machine
	if err := m.Stop(); err != nil {
		fmt.Printf("Failed to stop machine: %v\n", err)
	}
}
