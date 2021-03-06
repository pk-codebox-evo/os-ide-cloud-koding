package index

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Node represents a file tree.
//
// A single node represents a file or a directory.
//
// Nodes marked with EntryPromiseDel flag are marked
// as deleted, and are not going to be reachable via
// Lookup, Count methods. Deleting such nodes is a nop.
// This is how Node implements shallow delete.
type Node struct {
	Sub   map[string]*Node `json:"d,omitempty"`
	Entry *Entry           `json:"e,omitempty"`
}

func newNode() *Node {
	return &Node{
		Sub:   make(map[string]*Node),
		Entry: newEntry(),
	}
}

func newEntry() *Entry {
	t := time.Now().UTC().UnixNano()

	return &Entry{
		CTime: t,
		MTime: t,
		Mode:  0700 | os.ModeDir,
		Size:  10,
	}
}

// Add adds the given entry under the given path.
//
// Any deleted node, encountered on the tree path that, is going to
// be undeleted (having the EntryPromiseDel flag removed).
func (nd *Node) Add(path string, entry *Entry) {
	if path == "/" || path == "" {
		nd.Entry = entry
		return
	}

	var node string

	for {
		if nd.Deleted() {
			nd.undelete()
		}

		node, path = split(path)

		sub, ok := nd.Sub[node]
		if !ok {
			sub = newNode()
			nd.Sub[node] = sub
		}

		if path == "" {
			sub.Entry = entry
			return
		}

		nd = sub
	}
}

// Del disconnected a whole subtree rooted at a node given by the path.
//
// Del will ignore and do not disconnect nodes which are marked as deleted.
func (nd *Node) Del(path string) {
	var node string

	for {
		if nd.Deleted() {
			return
		}

		node, path = split(path)

		if path == "" {
			delete(nd.Sub, node)
			return
		}

		sub, ok := nd.Sub[node]
		if !ok {
			return
		}

		nd = sub
	}
}

// PromiseAdd adds a node under the given path marked as newly added.
//
// If the node already exists, it'd be only marked with EntryPromiseAdd flag.
//
// If the node is already marked as newly added, the method is a no-op.
//
// If entry.Mode is non-zero, the effictive node's entry is overwritten
// with this value.
//
// If entry.Aux is non-zero, the effictive node's Aux is overwritten
// with this value.
//
// If the given entry was previously marked as deleted,
// the flag will be removed.
//
// Rest of entry's fields are currently ignored.
func (nd *Node) PromiseAdd(path string, entry *Entry) {
	var newE *Entry

	if nd, ok := nd.lookup(path, true); ok {
		newE = nd.Entry
	} else {
		newE = newEntry()
	}

	if entry.Mode != 0 {
		newE.Mode = entry.Mode
	}

	if entry.Aux != 0 {
		newE.Aux = entry.Aux
	}

	newE.Meta = (newE.Meta | EntryPromiseAdd) &^ EntryPromiseDel

	nd.Add(path, newE)
}

// PromiseDel marks a node under the given path as deleted.
//
// If the node does not exist or is already marked as deleted, the
// method is no-op.
//
// If the given entry was previously marked as added,
// the flag will be removed.
func (nd *Node) PromiseDel(path string) {
	nd, ok := nd.Lookup(path)
	if !ok {
		return
	}

	nd.Entry.Meta = (nd.Entry.Meta | EntryPromiseDel) &^ EntryPromiseAdd
}

// Count counts nodes which Entry.Size is at most maxsize.
//
// If maxsize is 0, the method is a no-op.
// If maxsize is < 0, the method counts all nodes.
//
// Count ignored nodes marked as deleted.
func (nd *Node) Count(maxsize int64) (count int) {
	if maxsize == 0 {
		return 0 // no-op
	}

	cur, stack := (*Node)(nil), []*Node{nd}

	for len(stack) != 0 {
		cur, stack = stack[0], stack[1:]

		if cur.Deleted() {
			continue
		}

		if cur.Entry != nil && (maxsize < 0 || cur.Entry.Size <= maxsize) && cur != nd {
			count++
		}

		for _, nd := range cur.Sub {
			stack = append(stack, nd)
		}
	}

	return count
}

// DiskSize sums all Entry.Size of the nodes, given the condition the size
// is at most maxsize.
//
// If maxsize is 0, the method is a no-op.
// If maxsize is <0, all the nodes are sumed up.
//
// DiskSize ignores nodes marked as deleted.
func (nd *Node) DiskSize(maxsize int64) (size int64) {
	if maxsize == 0 {
		return 0 // no-op
	}

	stack := []*Node{nd}

	for len(stack) != 0 {
		nd, stack = stack[0], stack[1:]

		if nd.Deleted() {
			continue
		}

		if nd.Entry != nil && (maxsize < 0 || nd.Entry.Size <= maxsize) {
			size += nd.Entry.Size
		}

		for _, nd := range nd.Sub {
			stack = append(stack, nd)
		}
	}

	return size
}

// ForEach traverses the tree and calls fn on every node's entry.
//
// It ignored nodes marked as deleted.
func (nd *Node) ForEach(fn func(string, *Entry)) {
	type node struct {
		path string
		node *Node
	}

	n, stack := node{}, make([]node, 0, len(nd.Sub))

	for path, nd := range nd.Sub {
		stack = append(stack, node{
			path: path,
			node: nd,
		})
	}

	for len(stack) != 0 {
		n, stack = stack[0], stack[1:]

		if n.node.Deleted() {
			continue
		}

		for path, nd := range n.node.Sub {
			stack = append(stack, node{
				path: filepath.Join(n.path, path),
				node: nd,
			})
		}

		fn(n.path, n.node.Entry)
	}
}

// Lookup looks up a node given by the path ignoring any of the node
// that is marked as deleted.
func (nd *Node) Lookup(path string) (*Node, bool) {
	return nd.lookup(path, false)
}

func (nd *Node) lookup(path string, deleted bool) (*Node, bool) {
	if path == "/" || path == "" {
		return nd.shallowCopy(), true
	}

	var node string

	for {
		if nd.Deleted() && !deleted {
			return nil, false
		}

		node, path = split(path)

		sub, ok := nd.Sub[node]
		if !ok {
			return nil, false
		}

		if path == "" {
			return sub.shallowCopy(), true
		}

		nd = sub
	}
}

// IsDir tells whether a node is a directory.
func (nd *Node) IsDir() bool {
	return nd.Entry.Mode&os.ModeDir != 0
}

// Deleted tells whether node is marked as deleted.
func (nd *Node) Deleted() bool {
	return nd.Entry.Meta&EntryPromiseDel != 0
}

func (nd *Node) undelete() {
	nd.Entry.Meta &^= EntryPromiseDel
}

func (nd *Node) shallowCopy() *Node {
	if len(nd.Sub) != 0 {
		sub := make(map[string]*Node, len(nd.Sub))

		for k, v := range nd.Sub {
			sub[k] = v
		}

		nd.Sub = sub
	}

	return nd
}

func split(path string) (string, string) {
	if path[0] == '/' {
		path = path[1:]
	}

	if i := strings.IndexRune(path, '/'); i != -1 {
		return path[:i], path[i+1:]
	}

	return path, ""
}
