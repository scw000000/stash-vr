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

// Card is one scene tile in the grid.
type Card struct {
	ID           string
	Title        string
	ThumbnailURL string
	Duration     string // already-formatted "HH:MM:SS" or "MM:SS"
	Performers   string // comma-joined names
	Studio       string
	DeoVRPlayURL string // direct play URL for the quick-play overlay
	DetailURL    string // /browse/scene/{id}
}

// PageData is what browse.gohtml expects.
type PageData struct {
	Sidebar SidebarData
	Header  string // e.g. "All scenes — newest first"
	SubHead string // e.g. "Page 3 / 27" or "23 scenes"
	BackURL string // empty on /browse, "/browse" on entity-filtered routes
	Cards   []Card
	PrevURL string
	NextURL string
	PageNum int
	PageMax int
	ErrMessage string
}
