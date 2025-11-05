package rfsm

import (
	"testing"
)

// Nested structure:
// Root
//
//	├─ A (composite) -> children: A1(initial), A2
//	└─ B (simple)
//
// Transitions:
//
//	A1 --go--> B (from child)
//	A  --back--> B (from parent bubbling)
func TestNested_InitialDrillAndBubble(t *testing.T) {
	sub, err := NewDef("sub").
		State("A1", WithInitial()).
		State("A2", WithFinal()).
		Current("A1").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	def, err := NewDef("nested").
		State("A", WithSubDef(sub), WithInitial()).
		State("B", WithFinal()).
		Current("A").
		On("go", WithFrom("A1"), WithTo("B")).
		On("back", WithFrom("A"), WithTo("B")).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := NewMachine(def, nil, 8)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	// initial path should be [A, A1]
	path := m.CurrentPath()
	if len(path) != 2 || path[0] != "A" || path[1] != "A1" {
		t.Fatalf("initial path want [A A1], got %v", path)
	}

	// bubble from leaf A1 with event 'go' -> B
	if err := m.Dispatch(Event{Name: "go"}); err != nil {
		t.Fatal(err)
	}
	if m.Current() != "B" {
		t.Fatalf("want B got %v", m.Current())
	}

	// restart to test parent-level transition
	_ = m.Stop()
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if err := m.Dispatch(Event{Name: "back"}); err != nil {
		t.Fatal(err)
	}
	if m.Current() != "B" {
		t.Fatalf("parent transition to B failed, got %v", m.Current())
	}
}
