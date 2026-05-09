package browse

import (
	apiinternal "stash-vr/internal/api/internal"
)

// Entity is a sidebar row (performer / studio / tag).
type Entity struct {
	ID         string
	Name       string
	SceneCount int
}

// EntityRef is a clickable chip target on the scene detail page —
// performer, studio, or tag. ID drives the link href; Name is the
// display text. JSON tags keep the wire format lowercase so the
// browser-side AJAX layer reads {id, name}.
type EntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
	SearchQuery string
}

// SceneDetailData drives browse_scene.gohtml.
type SceneDetailData struct {
	ID           string
	Title        string
	ThumbnailURL string
	BackURL      string // from Referer, fallback "/browse"
	Performers   []EntityRef
	Studio       *EntityRef // nil if the scene has no studio
	Date         string // YYYY-MM-DD or empty
	Duration     string
	Rating1to5   int    // 0 = unrated; 1..5 set
	IsFavorite   bool
	Tags         []EntityRef // chip-renderable tags, excluding favorite tag and ancestor-injected tags
	AllTagNames  []string // for the <datalist> autocomplete
	OCounter        int
	Organized       bool
	DirectStreamURL string
	Projection      apiinternal.Projection
	ErrMessage      string

	// StarSlice is a 5-element placeholder used purely so the template can
	// {{range $i, $_ := .StarSlice}} 0..4 to render the five star buttons.
	StarSlice [5]struct{}
}
