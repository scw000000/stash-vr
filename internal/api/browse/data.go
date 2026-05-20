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

type CaptionRef struct {
	LanguageCode string `json:"languageCode"`
	CaptionType  string `json:"captionType"`
}

type SceneMarker struct {
	Seconds float64 `json:"seconds"`
	Title   string  `json:"title"`
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
	Sidebar     SidebarData
	Header      string // e.g. "All scenes — newest first"
	SubHead     string // e.g. "Page 3 / 27" or "23 scenes"
	BackURL     string // empty on /browse, "/browse" on entity-filtered routes
	Cards       []Card
	PrevURL     string
	NextURL     string
	PageNum     int
	PageMax     int
	ErrMessage  string
	SearchQuery string
}

// SceneDetailData drives browse_scene.gohtml.
type SceneDetailData struct {
	ID              string
	Title           string
	ThumbnailURL    string
	BackURL         string // from Referer, fallback "/browse"
	Performers      []EntityRef
	Studio          *EntityRef // nil if the scene has no studio
	Date            string     // YYYY-MM-DD or empty
	Duration        string
	Rating1to5      int // 0 = unrated; 1..5 set
	IsFavorite      bool
	Tags            []EntityRef // chip-renderable tags, excluding favorite tag and ancestor-injected tags
	Captions        []CaptionRef
	SceneMarkers    []SceneMarker
	DurationSec     float64  // duration in seconds; 0 if unknown
	AllTagNames     []string // for the <datalist> autocomplete
	OCounter        int
	Organized       bool
	DirectStreamURL string
	Projection      apiinternal.Projection

	// StarSlice is a 5-element placeholder used purely so the template can
	// {{range $i, $_ := .StarSlice}} 0..4 to render the five star buttons.
	StarSlice [5]struct{}
}

// GridTile is one tile in the JSON grid response used by the in-VR
// browser. Mirrors Card but exposes the projection metadata so the
// client can pick an appropriate render path or icon.
type GridTile struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	ThumbnailURL string                 `json:"thumbnailURL"`
	Projection   apiinternal.Projection `json:"projection"`
}

// GridResponse is the JSON envelope returned by GET /browse/grid.
type GridResponse struct {
	Tiles      []GridTile `json:"tiles"`
	NextCursor string     `json:"nextCursor,omitempty"`
	HasMore    bool       `json:"hasMore"`
}

// FilterOption is one entry in the JSON returned by
// GET /browse/filter-options/{kind}.
type FilterOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FilterIndexScene struct {
	ID           string   `json:"id"`
	PerformerIDs []string `json:"performerIds"`
	StudioIDs    []string `json:"studioIds,omitempty"`
	TagIDs       []string `json:"tagIds,omitempty"`
	Rating100    int      `json:"rating100"`
	OCount       int      `json:"oCount"`
}

type FilterIndexResponse struct {
	Performers []FilterOption     `json:"performers"`
	Studios    []FilterOption     `json:"studios"`
	Tags       []FilterOption     `json:"tags"`
	Scenes     []FilterIndexScene `json:"scenes"`
}

// FilterCatalogResponse is the JSON shape returned by /browse/filter-catalog.
// It carries only the sidebar entity lists; the client uses it to render
// columns immediately while the matrix payload is still in flight.
type FilterCatalogResponse struct {
	Performers []FilterOption `json:"performers"`
	Studios    []FilterOption `json:"studios"`
	Tags       []FilterOption `json:"tags"`
}

// FilterMatrixResponse is the JSON shape returned by /browse/filter-matrix.
// It carries the per-scene facet ID memberships used by the client-side
// intersection logic. Catalog parent maps are pre-applied so studio + tag
// IDs are already ancestor-expanded.
type FilterMatrixResponse struct {
	Scenes []FilterIndexScene `json:"scenes"`
}

// SceneState is the JSON returned by every mutation POST. The client
// uses it to update the DOM in place.
type SceneState struct {
	Rating1to5 int         `json:"rating1to5"`
	IsFavorite bool        `json:"isFavorite"`
	OCounter   int         `json:"oCounter"`
	Organized  bool        `json:"organized"`
	Tags       []EntityRef `json:"tags"`
	Err        string      `json:"err,omitempty"`
}
