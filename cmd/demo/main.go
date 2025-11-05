package main

import (
	"fmt"

	rfsm "github.com/ethan/rfsm/src"
)

func main() {
	// Composite groups: FIAT / HEDGE / CRYPTO with single internal state
	fiatSub, _ := rfsm.NewDef("FIAT_SUB").
		State("fiat_internal").
		Initial("fiat_internal").
		Build()

	hedgeSub, _ := rfsm.NewDef("HEDGE_SUB").
		State("hedge_internal").
		Initial("hedge_internal").
		Build()

	cryptoSub, _ := rfsm.NewDef("CRYPTO_SUB").
		State("crypto_internal").
		Initial("crypto_internal").
		Build()

	def, _ := rfsm.NewDef("FiatCryptoFlow").
		// Terminal states
		State("SUCCESS", rfsm.WithFinal()).
		State("FAILED", rfsm.WithFinal()).
		State("REFUNDED", rfsm.WithFinal()).
		State("EXPIRED", rfsm.WithFinal()).
		// Groups
		State("FIAT", rfsm.WithSubDef(fiatSub)).
		State("HEDGE", rfsm.WithSubDef(hedgeSub)).
		State("CRYPTO", rfsm.WithSubDef(cryptoSub)).
		// Other states
		State("INIT").
		State("PENDING_FIAT_DEPOSIT").
		State("PENDING_FIAT_EXPIRED").
		State("PENDING_FIAT_REFUND").
		State("PENDING_FIAT_DEPOSITED").
		State("PENDING_FIAT_DEPOSIT_FAILED").
		State("PENDING_HEDGE_REQUOTE").
		State("PENDING_HEDGE_EXECUTED").
		State("PENDING_HEDGE_FAILED").
		State("PENDING_HEDGE_UNWIND").
		State("PENDING_CRYPTO_WITHDRAW").
		State("PENDING_CRYPTO_WITHDRAW_FAILED").
		State("PENDING_CRYPTO_WITHDRAWN").
		Initial("INIT").

		// ---- FIAT Stage ----
		On("start_fiat", rfsm.WithFrom("PENDING_FIAT_DEPOSIT"), rfsm.WithTo("FIAT")).
		On("overdue", rfsm.WithFrom("FIAT"), rfsm.WithTo("PENDING_FIAT_EXPIRED")).
		On("manual_refund", rfsm.WithFrom("FIAT"), rfsm.WithTo("PENDING_FIAT_REFUND")).
		On("success", rfsm.WithFrom("FIAT"), rfsm.WithTo("PENDING_FIAT_DEPOSITED")).
		On("failed", rfsm.WithFrom("FIAT"), rfsm.WithTo("PENDING_FIAT_DEPOSIT_FAILED")).
		On("refund", rfsm.WithFrom("PENDING_FIAT_REFUND"), rfsm.WithTo("REFUNDED")).
		On("to_failed", rfsm.WithFrom("PENDING_FIAT_DEPOSIT_FAILED"), rfsm.WithTo("FAILED")).
		On("expire", rfsm.WithFrom("PENDING_FIAT_EXPIRED"), rfsm.WithTo("EXPIRED")).

		// ---- HEDGE Stage ----
		On("to_hedge", rfsm.WithFrom("PENDING_FIAT_DEPOSITED"), rfsm.WithTo("HEDGE")).
		On("requote", rfsm.WithFrom("PENDING_HEDGE_REQUOTE"), rfsm.WithTo("HEDGE")).
		On("executed", rfsm.WithFrom("HEDGE"), rfsm.WithTo("PENDING_HEDGE_EXECUTED")).
		On("failed", rfsm.WithFrom("HEDGE"), rfsm.WithTo("PENDING_HEDGE_FAILED")).
		On("retry_hedge", rfsm.WithFrom("PENDING_HEDGE_FAILED"), rfsm.WithTo("PENDING_HEDGE_REQUOTE")).
		// Optional unwind paths
		On("revert_unwind", rfsm.WithFrom("PENDING_HEDGE_EXECUTED"), rfsm.WithTo("PENDING_HEDGE_UNWIND")).
		On("requote", rfsm.WithFrom("PENDING_HEDGE_UNWIND"), rfsm.WithTo("PENDING_HEDGE_REQUOTE")).
		On("cancel_trade", rfsm.WithFrom("PENDING_HEDGE_UNWIND"), rfsm.WithTo("PENDING_FIAT_REFUND")).

		// Proceed to crypto
		On("to_crypto", rfsm.WithFrom("PENDING_HEDGE_EXECUTED"), rfsm.WithTo("PENDING_CRYPTO_WITHDRAW")).

		// ---- CRYPTO Stage ----
		On("start_crypto", rfsm.WithFrom("PENDING_CRYPTO_WITHDRAW"), rfsm.WithTo("CRYPTO")).
		On("failed", rfsm.WithFrom("CRYPTO"), rfsm.WithTo("PENDING_CRYPTO_WITHDRAW_FAILED")).
		On("success", rfsm.WithFrom("CRYPTO"), rfsm.WithTo("PENDING_CRYPTO_WITHDRAWN")).
		On("retry", rfsm.WithFrom("PENDING_CRYPTO_WITHDRAW_FAILED"), rfsm.WithTo("CRYPTO")).
		On("unwind", rfsm.WithFrom("PENDING_CRYPTO_WITHDRAW_FAILED"), rfsm.WithTo("PENDING_HEDGE_UNWIND")).
		On("to_success", rfsm.WithFrom("PENDING_CRYPTO_WITHDRAWN"), rfsm.WithTo("SUCCESS")).

		// ---- Initial fan-in/out ----
		On("init_next", rfsm.WithFrom("INIT"), rfsm.WithTo("PENDING_FIAT_DEPOSIT")).
		Build()

	// Print Mermaid with labels
	fmt.Println("=== State Machine Definition ===")
	fmt.Println(def.ToMermaidOpts(rfsm.VisualOptions{ShowGuards: true, ShowActions: true}))
	fmt.Println()

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
			fmt.Printf("✅ Reached final state: %s\n", m.Current())
			break
		}
	}

	// Stop machine
	if err := m.Stop(); err != nil {
		fmt.Printf("Failed to stop machine: %v\n", err)
	}
}
