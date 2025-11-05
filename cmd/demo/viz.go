package main

import (
	"fmt"

	rfsm "github.com/ethan/rfsm/src"
)

func Viz() {
	// Composite groups: FIAT / HEDGE / CRYPTO with single internal state
	def := DefineFiatCryptoFlow()
	// Print Mermaid with labels
	fmt.Println("=== State Machine Definition ===")
	fmt.Println(def.ToMermaidOpts(rfsm.VisualOptions{ShowGuards: true, ShowActions: true}))
	fmt.Println()
}
