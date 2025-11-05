package rfsm

import "testing"

func TestTopology_DAG(t *testing.T) {
	def, err := NewDef("dag").
		State("A", WithInitial()).State("B").State("C").State("D", WithFinal()).
		Current("A").
		On(TransitionKey{From: "A", To: "B"}, WithName("ab")).
		On(TransitionKey{From: "A", To: "C"}, WithName("ac")).
		On(TransitionKey{From: "B", To: "D"}, WithName("bd")).
		On(TransitionKey{From: "C", To: "D"}, WithName("cd")).
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
		State("A", WithInitial()).State("B", WithFinal()).
		Current("A").
		On(TransitionKey{From: "A", To: "B"}, WithName("ab")).
		On(TransitionKey{From: "B", To: "A"}, WithName("ba")).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := def.ComputeTopology(); err == nil {
		t.Fatalf("expected cycle error, got nil")
	}
}
