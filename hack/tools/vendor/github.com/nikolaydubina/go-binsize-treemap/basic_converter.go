package main

import (
	"errors"
	"log"
	"strings"

	"github.com/nikolaydubina/go-binsize-treemap/symtab"
	"github.com/nikolaydubina/treemap"
)

const (
	rootNodeName = "some-secret-string-binsize"
)

// BasicSymtabConverter converts parsed symtab file into treemap.
// Has no heat.
// Size is bytes.
type BasicSymtabConverter struct {
	MaxDepth uint // number of levels from root, including, if 0 then no limit
}

func (s BasicSymtabConverter) SymtabFileToTreemap(sf symtab.SymtabFile) treemap.Tree {
	if len(sf.Entries) == 0 {
		return treemap.Tree{}
	}

	tree := treemap.Tree{
		Nodes: map[string]treemap.Node{},
		To:    map[string][]string{},
	}

	hasParent := map[string]bool{}

	for _, entry := range sf.Entries {
		// skip unrecognized. mostly this is is C/C++ or something else. TODO: what is this?
		if entry.Type == symtab.Undefined {
			continue
		}

		symbolName := symtab.ParseSymbolName(entry.SymbolName)

		var parts []string

		// strange symbols. non-go like.
		// append them to still to single root unknown
		// TODO: what is this? C/C++?
		if len(symbolName.PackageParts) == 0 {
			parts = append(parts, "unknown")
		} else {
			parts = append(parts, symbolName.PackageParts...)
		}

		parts = append(parts, symbolName.SymbolParts...)

		if s.MaxDepth > 0 && len(parts) > int(s.MaxDepth) {
			parts = parts[:s.MaxDepth]
		}

		nodeName := strings.Join(parts, "/")

		if node, ok := tree.Nodes[nodeName]; ok {
			// accumulate reported size if duplicate
			tree.Nodes[nodeName] = treemap.Node{
				Path:    node.Path,
				Name:    node.Name,
				Size:    node.Size + float64(entry.Size),
				Heat:    node.Heat,
				HasHeat: node.HasHeat,
			}
			continue
		}

		tree.Nodes[nodeName] = treemap.Node{
			Path: nodeName,
			Size: float64(entry.Size),
		}

		hasParent[parts[0]] = false

		for parent, i := parts[0], 1; i < len(parts); i++ {
			child := parent + "/" + parts[i]

			tree.Nodes[parent] = treemap.Node{
				Path: parent,
			}

			tree.To[parent] = append(tree.To[parent], child)
			hasParent[child] = true

			parent = child
		}
	}

	for node, v := range tree.To {
		tree.To[node] = unique(v)
	}

	var roots []string
	for node, has := range hasParent {
		if !has {
			roots = append(roots, node)
		}
	}

	switch {
	case len(roots) == 0:
		log.Fatalf(errors.New("no roots, possible cycle in graph").Error())
	case len(roots) > 1:
		tree.Root = rootNodeName
		tree.To[tree.Root] = roots
	default:
		tree.Root = roots[0]
	}

	return tree
}

func unique(a []string) []string {
	u := map[string]bool{}
	var b []string
	for _, q := range a {
		if _, ok := u[q]; !ok {
			u[q] = true
			b = append(b, q)
		}
	}
	return b
}
