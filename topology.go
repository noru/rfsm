package rfsm

import (
	"errors"
)

var ErrCycleDetected = errors.New("state graph contains a cycle (not a DAG)")

type GraphTopology struct {
	Order []StateID
	Index map[StateID]int
}

// ComputeTopology builds a topological ordering from transitions.
// Nodes are all states in the definition (even isolated ones). Edges are transitions From->To.
// Returns ErrCycleDetected if a cycle exists.
func (d *Definition) ComputeTopology() (*GraphTopology, error) {
	// Build adjacency and indegree
	adj := make(map[StateID][]StateID, len(d.States))
	indeg := make(map[StateID]int, len(d.States))
	for id := range d.States {
		indeg[id] = 0
	}
	for i := range d.Transitions {
		t := d.Transitions[i]
		adj[t.From] = append(adj[t.From], t.To)
		indeg[t.To]++
	}
	// Kahn
	q := make([]StateID, 0, len(indeg))
	for id, deg := range indeg {
		if deg == 0 {
			q = append(q, id)
		}
	}
	order := make([]StateID, 0, len(indeg))
	// simple queue using slice; not stable by name
	for len(q) > 0 {
		v := q[len(q)-1]
		q = q[:len(q)-1]
		order = append(order, v)
		for _, w := range adj[v] {
			indeg[w]--
			if indeg[w] == 0 {
				q = append(q, w)
			}
		}
	}
	if len(order) != len(indeg) {
		return nil, ErrCycleDetected
	}
	index := make(map[StateID]int, len(order))
	for i, id := range order {
		index[id] = i
	}
	return &GraphTopology{Order: order, Index: index}, nil
}

func (gt *GraphTopology) IsBefore(a, b StateID) bool {
	ia, oka := gt.Index[a]
	ib, okb := gt.Index[b]
	if !oka || !okb {
		return false
	}
	return ia < ib
}

func (gt *GraphTopology) IsAfter(a, b StateID) bool {
	ia, oka := gt.Index[a]
	ib, okb := gt.Index[b]
	if !oka || !okb {
		return false
	}
	return ia > ib
}

// ensureTopology computes and caches the topology on the definition
func (d *Definition) ensureTopology() (*GraphTopology, error) {
	if d.topology != nil {
		return d.topology, nil
	}
	topo, err := d.ComputeTopology()
	if err != nil {
		return nil, err
	}
	d.topology = topo
	return topo, nil
}
