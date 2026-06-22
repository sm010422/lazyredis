package ui

import (
	"sort"
	"strings"
)

const treeDelimiter = ":"

type nodeKind int

const (
	nodeDir  nodeKind = iota
	nodeLeaf          // actual Redis key
)

type treeNode struct {
	kind    nodeKind
	name    string // display segment only (no path prefix)
	fullKey string // full Redis key (leaves only)
	prefix  string // full prefix including trailing delimiter (dirs only)
	count   int    // descendant leaf count (dirs only)
}

// buildNodes returns visible nodes (dirs first, then leaves) at the given
// breadcrumb path.
func buildNodes(allKeys []string, pathSegs []string) []treeNode {
	prefix := ""
	if len(pathSegs) > 0 {
		prefix = strings.Join(pathSegs, treeDelimiter) + treeDelimiter
	}

	dirCounts := make(map[string]int)
	var leaves []string

	for _, k := range allKeys {
		if prefix != "" && !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := k[len(prefix):]
		if idx := strings.Index(rest, treeDelimiter); idx != -1 {
			dirCounts[rest[:idx]]++
		} else {
			leaves = append(leaves, k)
		}
	}

	dirNames := make([]string, 0, len(dirCounts))
	for d := range dirCounts {
		dirNames = append(dirNames, d)
	}
	sort.Strings(dirNames)

	nodes := make([]treeNode, 0, len(dirCounts)+len(leaves))
	for _, d := range dirNames {
		nodes = append(nodes, treeNode{
			kind:   nodeDir,
			name:   d,
			prefix: prefix + d + treeDelimiter,
			count:  dirCounts[d],
		})
	}
	for _, k := range leaves {
		nodes = append(nodes, treeNode{
			kind:    nodeLeaf,
			name:    k[len(prefix):],
			fullKey: k,
		})
	}
	return nodes
}

// keysWithPrefix returns all keys that start with the given prefix string.
func keysWithPrefix(allKeys []string, prefix string) []string {
	var out []string
	for _, k := range allKeys {
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	return out
}

// parentPath returns pathSegs with the last segment removed.
func parentPath(segs []string) []string {
	if len(segs) == 0 {
		return nil
	}
	cp := make([]string, len(segs)-1)
	copy(cp, segs[:len(segs)-1])
	return cp
}

// breadcrumbString returns a human-readable path like "/user/1/".
func breadcrumbString(segs []string) string {
	if len(segs) == 0 {
		return "/"
	}
	return "/" + strings.Join(segs, "/") + "/"
}
