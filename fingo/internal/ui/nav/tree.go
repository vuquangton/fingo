package nav

// Node is one row in the rail: a section, or an entry inside one.
//
// It is a nav type and not a toolkit type on purpose. What the rail *is* — five
// sections, these entries, in this order — is a product decision that is
// asserted in tests. How it is drawn is the shell's business, and the shell
// supplies a thin adapter for whatever tree interface its toolkit wants.
type Node struct {
	text     string
	parent   *Node
	children []*Node

	entry    Entry
	hasEntry bool
}

// Text is what the rail shows.
func (n *Node) Text() string { return n.text }

// Parent is the section this node belongs to, or nil for a section itself.
func (n *Node) Parent() *Node { return n.parent }

// ChildCount is how many entries a section holds.
func (n *Node) ChildCount() int { return len(n.children) }

// ChildAt is one of them.
func (n *Node) ChildAt(i int) *Node { return n.children[i] }

// Entry is the destination this node opens. It is the zero Entry for a section.
func (n *Node) Entry() Entry { return n.entry }

// HasEntry reports whether this node is a destination rather than a heading.
func (n *Node) HasEntry() bool { return n.hasEntry }

// BuildTree arranges entries under their sections, in the order they appear.
//
// A section is created the first time an entry names it, so the section order is
// the rail's order and a section with nothing in it is never shown — a heading
// with nothing under it is a promise the rail does not keep.
func BuildTree(entries []Entry) []*Node {
	var roots []*Node

	bySection := make(map[string]*Node, len(entries))

	for _, e := range entries {
		root, ok := bySection[e.Section]
		if !ok {
			root = &Node{text: e.Section}
			bySection[e.Section] = root
			roots = append(roots, root)
		}

		root.children = append(root.children, &Node{
			text:     e.Label,
			parent:   root,
			entry:    e,
			hasEntry: true,
		})
	}

	return roots
}
