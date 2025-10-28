package rfsm

import "testing"

func TestTopology_DAG(t *testing.T) {
	def, err := NewDef("dag").
		State("A").State("B").State("C").State("D").
		Initial("A").
		On("ab", WithFrom("A"), WithTo("B")).
		On("ac", WithFrom("A"), WithTo("C")).
		On("bd", WithFrom("B"), WithTo("D")).
		On("cd", WithFrom("C"), WithTo("D")).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	topo, err := def.ComputeTopology()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !topo.IsBefore("A", "D") {
		t.Fatalf("A should be before D")
	}
	if topo.IsBefore("D", "A") {
		t.Fatalf("D should not be before A")
	}
}

func TestTopology_Cycle(t *testing.T) {
	def, err := NewDef("cyc").
		State("A").State("B").
		Initial("A").
		On("ab", WithFrom("A"), WithTo("B")).
		On("ba", WithFrom("B"), WithTo("A")).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := def.ComputeTopology(); err == nil {
		t.Fatalf("expected cycle error, got nil")
	}
}
