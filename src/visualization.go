package rfsm

import (
	"bytes"
	"sort"
)

type VisualOptions struct {
	ShowGuards  bool
	ShowActions bool
}

// ToMermaid renders the definition as a Mermaid stateDiagram-v2 DSL.
// Default: only event label on edges. Use ToMermaidOpts to include guard/action markers.
func (d *Definition) ToMermaid() string { return d.ToMermaidOpts(VisualOptions{}) }

// ToMermaidOpts renders Mermaid with options.
// Edge label format: "event [guard] / action" (markers included when enabled and present)
func (d *Definition) ToMermaidOpts(opts VisualOptions) string {
	var buf bytes.Buffer
	buf.WriteString("stateDiagram-v2\n")

	// root initial pointer
	if d.Initial != "" {
		buf.WriteString("[*] --> ")
		buf.WriteString(string(d.Initial))
		buf.WriteByte('\n')
	}

	// build parent -> children map and root list
	childrenOf := make(map[StateID][]StateID)
	roots := make([]StateID, 0, len(d.States))
	for id, st := range d.States {
		if st.Parent == "" {
			roots = append(roots, id)
		} else {
			childrenOf[st.Parent] = append(childrenOf[st.Parent], id)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i] < roots[j] })
	for parent := range childrenOf {
		kids := childrenOf[parent]
		sort.Slice(kids, func(i, j int) bool { return kids[i] < kids[j] })
		childrenOf[parent] = kids
	}

	// recursive render for composite states
	var renderComposite func(id StateID, indent string)
	renderComposite = func(id StateID, indent string) {
		st := d.States[id]
		buf.WriteString(indent)
		buf.WriteString("state ")
		buf.WriteString(string(id))
		buf.WriteString(" {\n")
		// initial pointer inside composite
		if st.InitialChild != "" {
			buf.WriteString(indent)
			buf.WriteByte('\t')
			buf.WriteString("[*] --> ")
			buf.WriteString(string(st.InitialChild))
			buf.WriteByte('\n')
		}
		// render children
		for _, c := range childrenOf[id] {
			if len(d.States[c].Children) > 0 {
				renderComposite(c, indent+"\t")
			} else {
				// declare leaf inside composite to ensure visibility
				buf.WriteString(indent)
				buf.WriteByte('\t')
				buf.WriteString("state ")
				buf.WriteString(string(c))
				buf.WriteByte('\n')
			}
		}
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// render roots
	for _, r := range roots {
		if len(childrenOf[r]) > 0 { // composite root
			renderComposite(r, "")
		} else {
			// declare leaf root to ensure visibility if it has no transitions
			buf.WriteString("state ")
			buf.WriteString(string(r))
			buf.WriteByte('\n')
		}
	}

	// render transitions
	for i := range d.Transitions {
		t := d.Transitions[i]
		buf.WriteString(string(t.From))
		buf.WriteString(" --> ")
		buf.WriteString(string(t.To))
		// label
		var needLabel bool
		if t.Event != "" { needLabel = true }
		if opts.ShowGuards && t.Guard != nil { needLabel = true }
		if opts.ShowActions && t.Action != nil { needLabel = true }
		if needLabel {
			buf.WriteString(" : ")
			first := true
			if t.Event != "" {
				buf.WriteString(t.Event)
				first = false
			}
			if opts.ShowGuards && t.Guard != nil {
				if !first { buf.WriteString(" ") }
				buf.WriteString("[guard]")
				first = false
			}
			if opts.ShowActions && t.Action != nil {
				if !first { buf.WriteString(" ") }
				buf.WriteString("/ action")
			}
		}
		buf.WriteByte('\n')
	}

	return buf.String()
}

// ToDOT renders a Graphviz DOT directed graph, with clusters for composite states and point-shaped initial nodes.
func (d *Definition) ToDOT() string { return d.ToDOTOpts(VisualOptions{}) }

func (d *Definition) ToDOTOpts(opts VisualOptions) string {
	var buf bytes.Buffer
	buf.WriteString("digraph fsm {\n")
	buf.WriteString("  rankdir=LR;\n")
	buf.WriteString("  node [shape=rectangle];\n")
	// point node for root initial
	if d.Initial != "" {
		buf.WriteString("  __init_root [shape=point,label=\"\"];\n")
		buf.WriteString("  __init_root -> \"")
		buf.WriteString(string(d.Initial))
		buf.WriteString("\";\n")
	}

	// grouping children
	childrenOf := make(map[StateID][]StateID)
	roots := make([]StateID, 0, len(d.States))
	for id, st := range d.States {
		if st.Parent == "" {
			roots = append(roots, id)
		} else {
			childrenOf[st.Parent] = append(childrenOf[st.Parent], id)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i] < roots[j] })
	for parent := range childrenOf {
		kids := childrenOf[parent]
		sort.Slice(kids, func(i, j int) bool { return kids[i] < kids[j] })
		childrenOf[parent] = kids
	}

	// recursive clusters
	var renderCluster func(id StateID, indent string)
	renderCluster = func(id StateID, indent string) {
		st := d.States[id]
		buf.WriteString(indent)
		buf.WriteString("subgraph cluster_")
		buf.WriteString(string(id))
		buf.WriteString(" {\n")
		buf.WriteString(indent)
		buf.WriteString("  label=\"")
		buf.WriteString(string(id))
		buf.WriteString("\";\n")
		// initial pointer inside cluster
		if st.InitialChild != "" {
			buf.WriteString(indent)
			buf.WriteString("  __init_")
			buf.WriteString(string(id))
			buf.WriteString(" [shape=point,label=\"\"];\n")
			buf.WriteString(indent)
			buf.WriteString("  __init_")
			buf.WriteString(string(id))
			buf.WriteString(" -> \"")
			buf.WriteString(string(st.InitialChild))
			buf.WriteString("\";\n")
		}
		for _, c := range childrenOf[id] {
			if len(d.States[c].Children) > 0 {
				renderCluster(c, indent+"  ")
			} else {
				buf.WriteString(indent)
				buf.WriteString("  \"")
				buf.WriteString(string(c))
				buf.WriteString("\";\n")
			}
		}
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// render roots
	for _, r := range roots {
		if len(childrenOf[r]) > 0 {
			renderCluster(r, "  ")
		} else {
			buf.WriteString("  \"")
			buf.WriteString(string(r))
			buf.WriteString("\";\n")
		}
	}

	// transitions
	for i := range d.Transitions {
		t := d.Transitions[i]
		buf.WriteString("  \"")
		buf.WriteString(string(t.From))
		buf.WriteString("\" -> \"")
		buf.WriteString(string(t.To))
		buf.WriteString("\"")
		// label
		var need bool
		if t.Event != "" { need = true }
		if opts.ShowGuards && t.Guard != nil { need = true }
		if opts.ShowActions && t.Action != nil { need = true }
		if need {
			buf.WriteString(" [label=\"")
			first := true
			if t.Event != "" {
				buf.WriteString(t.Event)
				first = false
			}
			if opts.ShowGuards && t.Guard != nil {
				if !first { buf.WriteString(" ") }
				buf.WriteString("[guard]")
				first = false
			}
			if opts.ShowActions && t.Action != nil {
				if !first { buf.WriteString(" ") }
				buf.WriteString("/ action")
			}
			buf.WriteString("\"]")
		}
		buf.WriteString(";\n")
	}

	buf.WriteString("}\n")
	return buf.String()
}


