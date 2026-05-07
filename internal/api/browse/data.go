package browse

// Entity is a sidebar row (performer / studio / tag).
type Entity struct {
	ID         string
	Name       string
	SceneCount int
}

// SidebarData holds the three lists that populate the sidebar.
type SidebarData struct {
	Performers []Entity
	Studios    []Entity
	Tags       []Entity
	// ActiveTab is one of "perf", "studio", "tag" — used to render the right
	// tab as initially visible. Defaults to "perf" on the index page.
	ActiveTab string
	// ActiveID is the entity id currently selected (for the highlight),
	// empty if no entity is selected.
	ActiveID string
}
