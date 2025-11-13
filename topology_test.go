package rfsm

import "testing"

func TestTopology_DAG(t *testing.T) {
	def, err := NewDef("dag").
		State("A", WithInitial()).State("B").State("C").State("D", WithFinal()).
		Current("A").
		On("ab", "A", "B").
		On("ac", "A", "C").
		On("bd", "B", "D").
		On("cd", "C", "D").
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
		On("ab", "A", "B").
		On("ba", "B", "A").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := def.ComputeTopology(); err == nil {
		t.Fatalf("expected cycle error, got nil")
	}
}

func TestTopology_IsBeforeIsAfter(t *testing.T) {
	def, err := NewDef("test").
		State("A", WithInitial()).
		State("B").
		State("C").
		State("D", WithFinal()).
		Current("A").
		On("ab", "A", "B").
		On("bc", "B", "C").
		On("cd", "C", "D").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	topo, err := def.ComputeTopology()
	if err != nil {
		t.Fatal(err)
	}

	if !topo.IsBefore("A", "D") {
		t.Fatal("A should be before D")
	}
	if topo.IsBefore("D", "A") {
		t.Fatal("D should not be before A")
	}
	if topo.IsBefore("A", "A") {
		t.Fatal("A should not be before A")
	}

	if !topo.IsAfter("D", "A") {
		t.Fatal("D should be after A")
	}
	if topo.IsAfter("A", "D") {
		t.Fatal("A should not be after D")
	}
	if topo.IsAfter("A", "A") {
		t.Fatal("A should not be after A")
	}

	// Test states not in topology
	if topo.IsBefore("X", "A") {
		t.Fatal("X should not be before A (X not in topology)")
	}
	if topo.IsAfter("X", "A") {
		t.Fatal("X should not be after A (X not in topology)")
	}
}
