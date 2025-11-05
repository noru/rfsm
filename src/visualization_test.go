package rfsm

import "testing"

func TestToMermaid_NestedAndTransitions(t *testing.T) {
	sub, err := NewDef("sub").
		State("A1", WithInitial()).
		State("A2", WithFinal()).
		Current("A1").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	def, err := NewDef("root").
		State("A", WithSubDef(sub), WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", WithFrom("A1"), WithTo("B")).
		On("back", WithFrom("A"), WithTo("B")).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s := def.ToMermaid()
	// basic expectations
	if want := "stateDiagram-v2"; !contains(s, want) {
		t.Fatalf("missing %q", want)
	}
	if want := "[*] --> A"; !contains(s, want) {
		t.Fatalf("missing %q", want)
	}
	if want := "state A {"; !contains(s, want) {
		t.Fatalf("missing %q", want)
	}
	if want := "[*] --> A1"; !contains(s, want) {
		t.Fatalf("missing %q", want)
	}
	if want := "A1 --> B : go"; !contains(s, want) {
		t.Fatalf("missing %q", want)
	}
	if want := "A --> B : back"; !contains(s, want) {
		t.Fatalf("missing %q", want)
	}
}

func TestToMermaid_WithMarkers_And_DOT(t *testing.T) {
	def, err := NewDef("root").
		State("A", WithInitial()).State("B", WithFinal()).
		Current("A").
		On("go", WithFrom("A"), WithTo("B"), WithGuard(func(e Event, ctx any) bool { return true }), WithAction(func(e Event, ctx any) error { return nil })).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ms := def.ToMermaidOpts(VisualOptions{ShowGuards: true, ShowActions: true})
	if want := "go [guard] / action"; !contains(ms, want) {
		t.Fatalf("mermaid missing markers: %q in %q", want, ms)
	}

	ds := def.ToDOTOpts(VisualOptions{ShowGuards: true, ShowActions: true})
	if want := "digraph fsm"; !contains(ds, want) {
		t.Fatalf("dot missing header")
	}
	if want := "\"A\" -> \"B\" [label=\"go [guard] / action\"]"; !contains(ds, want) {
		t.Fatalf("dot missing labeled edge: %q", want)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool { return (len(find(s, sub)) > 0) })()
}

func find(s, sub string) string {
	// simple wrapper around built-in search to avoid importing strings in multiple places
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return sub
		}
	}
	return ""
}
