package shell

import (
	"github.com/lxn/walk"

	"fingo/internal/ui/nav"
)

// railModel adapts nav's tree to walk's TreeModel.
//
// This is the whole of the toolkit adapter for the rail. nav decides what the
// wiring is, walk decides how it is drawn, and the two meet in about thirty
// lines that contain no decisions at all — which is the point. A screen that
// made decisions here would be a screen that cannot be tested.
type railModel struct {
	walk.TreeModelBase
	roots []*nav.Node
}

func newRailModel(roots []*nav.Node) *railModel { return &railModel{roots: roots} }

func (m *railModel) LazyPopulation() bool { return false }

func (m *railModel) RootCount() int { return len(m.roots) }

func (m *railModel) RootAt(index int) walk.TreeItem { return railItem{m.roots[index]} }

// railItem is one nav node as walk sees it.
type railItem struct{ node *nav.Node }

func (i railItem) Text() string { return i.node.Text() }

func (i railItem) Parent() walk.TreeItem {
	if p := i.node.Parent(); p != nil {
		return railItem{p}
	}

	return nil
}

func (i railItem) ChildCount() int { return i.node.ChildCount() }

func (i railItem) ChildAt(index int) walk.TreeItem { return railItem{i.node.ChildAt(index)} }
