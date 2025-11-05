package main

import (
	"fmt"
	"time"

	rfsm "github.com/ethan/rfsm/src"
)

type demoContext struct {
	balance     int
	attempts    int
	validated   bool
	processed   bool
	logs        []string
}

func (ctx *demoContext) log(msg string) {
	ctx.logs = append(ctx.logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05.000"), msg))
}

func Actions() {
	fmt.Println("=== Actions, Hooks and Guards Demo ===")
	fmt.Println()

	ctx := &demoContext{
		balance: 100,
		attempts: 0,
	}

	def := defineActionsDemo(ctx)
	m := rfsm.NewMachine(def, ctx, 10)
	logger := &actionLogger{ctx: ctx}
	m.Subscribe(logger)

	if err := m.Start(); err != nil {
		fmt.Printf("Failed to start machine: %v\n", err)
		return
	}
	fmt.Printf("Machine started at: %s\n", m.Current())
	fmt.Println()

	events := []struct {
		name string
		desc string
		args []any
	}{
		{"start", "Start processing", nil},
		{"validate", "Validate with insufficient balance (should fail)", []any{150}},
		{"validate", "Validate with sufficient balance", []any{50}},
		{"process", "Process transaction", nil},
		{"complete", "Complete transaction", nil},
	}

	for i, evt := range events {
		fmt.Printf("Step %d: %s\n", i+1, evt.desc)
		fmt.Printf("  Current state: %s\n", m.Current())
		fmt.Printf("  Context: balance=%d, attempts=%d, validated=%v, processed=%v\n",
			ctx.balance, ctx.attempts, ctx.validated, ctx.processed)

		event := rfsm.Event{Name: evt.name, Args: evt.args}
		if err := m.Dispatch(event); err != nil {
			fmt.Printf("  ❌ Error: %v\n", err)
		} else {
			fmt.Printf("  ✓ Event dispatched successfully\n")
		}

		fmt.Printf("  New state: %s\n", m.Current())
		fmt.Println()

		if st, ok := def.States[m.Current()]; ok && st.Final {
			fmt.Printf("✅ Reached final state: %s\n", m.Current())
			break
		}
	}

	if err := m.Stop(); err != nil {
		fmt.Printf("Failed to stop machine: %v\n", err)
	}

	fmt.Println()
	fmt.Println("=== Execution Log ===")
	for _, log := range ctx.logs {
		fmt.Println(log)
	}
}

type actionLogger struct {
	ctx *demoContext
}

func (l *actionLogger) OnTransition(from rfsm.StateID, to rfsm.StateID, e rfsm.Event, err error) {
	if err != nil {
		l.ctx.log(fmt.Sprintf("Transition failed: %s -> %s (event: %s, error: %v)", from, to, e.Name, err))
	} else {
		l.ctx.log(fmt.Sprintf("Transition: %s -> %s (event: %s)", from, to, e.Name))
	}
}

func defineActionsDemo(ctx *demoContext) *rfsm.Definition {
	def, _ := rfsm.NewDef("ActionsDemo").
		State("INIT", rfsm.WithInitial(), rfsm.WithEntry(entryHook("INIT", ctx)), rfsm.WithExit(exitHook("INIT", ctx))).
		State("PROCESSING", rfsm.WithEntry(entryHook("PROCESSING", ctx)), rfsm.WithExit(exitHook("PROCESSING", ctx))).
		State("VALIDATING", rfsm.WithEntry(entryHook("VALIDATING", ctx)), rfsm.WithExit(exitHook("VALIDATING", ctx))).
		State("SUCCESS", rfsm.WithFinal(), rfsm.WithEntry(entryHook("SUCCESS", ctx))).
		State("FAILED", rfsm.WithFinal(), rfsm.WithEntry(entryHook("FAILED", ctx))).
		Current("INIT").

		On("start", rfsm.WithFrom("INIT"), rfsm.WithTo("PROCESSING"),
			rfsm.WithAction(processAction(ctx))).

		On("validate", rfsm.WithFrom("PROCESSING"), rfsm.WithTo("VALIDATING"),
			rfsm.WithGuard(validateGuard(ctx)),
			rfsm.WithAction(validateAction(ctx))).

		On("validate", rfsm.WithFrom("VALIDATING"), rfsm.WithTo("VALIDATING"),
			rfsm.WithGuard(validateGuard(ctx)),
			rfsm.WithAction(validateAction(ctx))).

		On("process", rfsm.WithFrom("VALIDATING"), rfsm.WithTo("PROCESSING"),
			rfsm.WithAction(processAction(ctx))).

		On("complete", rfsm.WithFrom("PROCESSING"), rfsm.WithTo("SUCCESS"),
			rfsm.WithGuard(successGuard(ctx)),
			rfsm.WithAction(completeAction(ctx))).

		On("fail", rfsm.WithFrom("PROCESSING"), rfsm.WithTo("FAILED"),
			rfsm.WithAction(failAction(ctx))).

		Build()

	return def
}

func entryHook(stateName string, ctx *demoContext) rfsm.HookFunc {
	return func(e rfsm.Event, _ any) error {
		ctx.log(fmt.Sprintf("HOOK [Entry] %s: Entering state", stateName))
		return nil
	}
}

func exitHook(stateName string, ctx *demoContext) rfsm.HookFunc {
	return func(e rfsm.Event, _ any) error {
		ctx.log(fmt.Sprintf("HOOK [Exit] %s: Exiting state", stateName))
		return nil
	}
}

func processAction(ctx *demoContext) rfsm.ActionFunc {
	return func(e rfsm.Event, _ any) error {
		ctx.attempts++
		ctx.processed = true
		ctx.log(fmt.Sprintf("ACTION: Processing transaction (attempt %d)", ctx.attempts))
		return nil
	}
}

func validateGuard(ctx *demoContext) rfsm.GuardFunc {
	return func(e rfsm.Event, _ any) bool {
		if len(e.Args) == 0 {
			ctx.log("GUARD: Validation failed - no amount provided")
			return false
		}
		amount, ok := e.Args[0].(int)
		if !ok {
			ctx.log("GUARD: Validation failed - invalid amount type")
			return false
		}
		result := ctx.balance >= amount
		if result {
			ctx.log(fmt.Sprintf("GUARD: Balance check passed (balance=%d >= amount=%d)", ctx.balance, amount))
		} else {
			ctx.log(fmt.Sprintf("GUARD: Balance check failed (balance=%d < amount=%d)", ctx.balance, amount))
		}
		return result
	}
}

func validateAction(ctx *demoContext) rfsm.ActionFunc {
	return func(e rfsm.Event, _ any) error {
		ctx.validated = true
		amount := 0
		if len(e.Args) > 0 {
			if amt, ok := e.Args[0].(int); ok {
				amount = amt
			}
		}
		ctx.log(fmt.Sprintf("ACTION: Validated transaction for amount %d", amount))
		return nil
	}
}

func successGuard(ctx *demoContext) rfsm.GuardFunc {
	return func(e rfsm.Event, _ any) bool {
		result := ctx.validated && ctx.processed
		if result {
			ctx.log("GUARD: Success check passed (validated and processed)")
		} else {
			ctx.log(fmt.Sprintf("GUARD: Success check failed (validated=%v, processed=%v)", ctx.validated, ctx.processed))
		}
		return result
	}
}

func completeAction(ctx *demoContext) rfsm.ActionFunc {
	return func(e rfsm.Event, _ any) error {
		ctx.log("ACTION: Transaction completed successfully")
		return nil
	}
}

func failAction(ctx *demoContext) rfsm.ActionFunc {
	return func(e rfsm.Event, _ any) error {
		ctx.log("ACTION: Transaction failed")
		return nil
	}
}
