# M4c: In-VR search/browse — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a 3D scene-grid browse panel reachable from the M4b control panel, with text search via DOM overlay, six filter pickers, vertical scroll, configurable cols, and seamless scene swap (no VR re-entry).

**Architecture:** Ten tasks. Task 1 is a DOM-overlay feasibility spike — confirm Meta Browser supports DOM overlay during a WebXR session before committing to the rest. Task 2 adds the server-side JSON endpoints. Tasks 3–4 build the browse panel UI: top strip, cylinder grid layout, tile entities with cover textures + ⓘ detail badge. Task 5 adds vertical scroll + lazy load + the M3c thumbstick handoff. Task 6 wires the DOM-overlay search field. Task 7 adds the 3-column filters panel with searchable lists. Task 8 implements the seamless scene swap with fade and the rich `/scene/{id}/meta` endpoint. Task 9 adds the standalone detail panel with chip-click filters. **Task 10 is RF52 canting math** — a rendering-correctness fix bundled in at the user's direction (orthogonal to in-VR search but ships in this release window).

**Tech Stack:** Go 1.24, A-Frame 1.7, Three.js, vanilla JS, WebXR DOM Overlay Module.

**Spec:** [docs/superpowers/specs/2026-05-09-m4c-in-vr-search.md](../specs/2026-05-09-m4c-in-vr-search.md)

**No tests in this project.** Verification is `go vet ./...`, `go build ./...`, and the manual browser steps in §8 of the spec at the end of each task.

**Prerequisite:** M4a + M4b should be merged. M4c assumes the M4b panel exists and can host a "Browse" button.

---

## Task 1: DOM-overlay spike

**Files:**
- Modify: `internal/static/browse_scene.gohtml` (temporarily — spike code may be removed if it works as expected)

**Goal:** Confirm Meta Browser supports `dom-overlay` as an optional WebXR feature. The entire M4c search UX hinges on this. If it fails, the rest of the plan needs revision (custom in-VR keyboard).

- [ ] **Step 1: Add DOM overlay request to enterVR**

In [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml), find:

```javascript
scene.enterVR().catch(err => {
  console.warn('stash-vr: enterVR failed', err);
  show2D();
});
```

A-Frame's `scene.enterVR()` doesn't directly accept a `domOverlay` config. Instead, we configure the optional features via A-Frame's `xr-mode-ui` / `webxr` system component. Update the `<a-scene>` opening tag to add the WebXR config:

```html
<a-scene id="vrScene" embedded
         webxr="optionalFeatures: dom-overlay; overlayElement: #vrDomOverlay"
         vr-mode-ui="enabled: true"
         loading-screen="enabled: false"
         background="color: #111"
         data-stereo="{{.Projection.Stereo}}" data-geometry="{{.Projection.Geometry}}" data-fov="{{.Projection.FOV}}">
```

- [ ] **Step 2: Add the overlay div**

Just before `<a-scene>`, add:

```html
<div id="vrDomOverlay" style="display:none; position:fixed; top:50%; left:50%; transform:translate(-50%,-50%); padding:16px; background:rgba(0,0,0,0.8); color:#fff; border-radius:8px; z-index:9999;">
  <div style="margin-bottom:8px">M4c spike: DOM overlay is alive.</div>
  <input id="vrSearchInput" type="text" placeholder="Search test..." style="font-size:18px; padding:8px; width:300px;" />
</div>
```

- [ ] **Step 3: Show the overlay during the spike**

In the IIFE, after `scene.enterVR()` succeeds, set the overlay visible:

```javascript
btn.addEventListener('click', () => {
  hide2D();
  const playPromise = video.play();
  if (playPromise && playPromise.catch) {
    playPromise.catch(err => console.warn('stash-vr: video play failed', err));
  }
  if (typeof scene.enterVR !== 'function') {
    console.warn('stash-vr: a-scene not ready');
    show2D();
    return;
  }
  scene.enterVR().then(() => {
    const overlay = document.getElementById('vrDomOverlay');
    if (overlay) overlay.style.display = 'block';
    setTimeout(() => {
      const input = document.getElementById('vrSearchInput');
      if (input) input.focus();
    }, 100);
  }).catch(err => {
    console.warn('stash-vr: enterVR failed', err);
    show2D();
  });
});
```

And on exit, hide the overlay:

```javascript
scene.addEventListener('exit-vr', function() {
  const overlay = document.getElementById('vrDomOverlay');
  if (overlay) overlay.style.display = 'none';
  show2D();
});
```

- [ ] **Step 4: Build, install on Quest 3, and test**

Build via `scripts\build-windows.bat`. Run.

On the Quest 3 in Meta Browser, open `/browse/scene/{id}`. Click "Enter VR".

**Pass criteria:**
- The overlay appears in front of the user's view in VR.
- Tapping the search input via raycast/laser focuses it (cursor blinks, blue border).
- Quest's system VR keyboard pops up.
- Typing a key inserts a character into the input.
- Backspace works.

**Fail criteria:**
- The overlay doesn't appear at all.
- The overlay appears but tapping doesn't focus the input.
- Focus succeeds but the keyboard doesn't pop.
- Keyboard pops but typing doesn't reach the input.

- [ ] **Step 5: Decide**

If **PASS:** the rest of M4c proceeds as drafted. Continue to Task 2.

If **FAIL:** stop. Open a new spec for "M4c custom in-VR keyboard" — adds a 2D-style on-screen keyboard widget with raycast keys. Re-spec with the user.

- [ ] **Step 6: Commit (only if PASS)**

The spike code (overlay div, post-enterVR show) stays as-is — Task 6 will rewire the input's behavior to drive the search filter. If FAIL, revert.

```
git add internal/static/browse_scene.gohtml
git commit -m "m4c: spike DOM overlay during WebXR session"
```

---

## Task 2: Server JSON endpoints

**Files:**
- Create: `internal/api/browse/grid_json.go`
- Modify: `internal/api/browse/data.go`
- Modify: `internal/api/browse/router.go`

**Goal:** Add `GET /browse/grid?...` (returns `GridResponse` JSON) and `GET /browse/filter-options/{kind}` (returns `[]FilterOption`).

- [ ] **Step 1: Add types to data.go**

Append to [internal/api/browse/data.go](../../../internal/api/browse/data.go):

```go
type GridTile struct {
	ID           string                  `json:"id"`
	Title        string                  `json:"title"`
	ThumbnailURL string                  `json:"thumbnailURL"`
	Projection   apiinternal.Projection  `json:"projection"`
}

type GridResponse struct {
	Tiles      []GridTile `json:"tiles"`
	NextCursor string     `json:"nextCursor,omitempty"`
	HasMore    bool       `json:"hasMore"`
}

type FilterOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
```

- [ ] **Step 2: Create grid_json.go**

Create [internal/api/browse/grid_json.go](../../../internal/api/browse/grid_json.go):

```go
package browse

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/api/heatmap"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/stash/gql"
	"stash-vr/internal/util"
)

func (h *httpHandler) gridJSONHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	searchQ := q.Get("q")
	performer := q.Get("performer")
	studio := q.Get("studio")
	tag := q.Get("tag")
	favorite := q.Get("favorite")
	starsMin, _ := strconv.Atoi(q.Get("stars"))
	ocountMin, _ := strconv.Atoi(q.Get("ocount"))
	page, _ := strconv.Atoi(q.Get("cursor"))
	if page < 1 {
		page = 1
	}

	sceneFilter := buildGridFilter(performer, studio, tag, favorite, starsMin, ocountMin)
	ids, total, err := fetchSceneIDs(r.Context(), h.libraryService.StashClient, sceneFilter, searchQ, page)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: grid fetchSceneIDs")
		http.Error(w, "fetch failed", http.StatusInternalServerError)
		return
	}

	baseURL := apiinternal.GetBaseUrl(r)
	tiles := make([]GridTile, 0, len(ids))
	for _, id := range ids {
		vd, err := h.libraryService.GetScene(r.Context(), id, false)
		if err != nil || vd == nil || vd.SceneParts == nil {
			continue
		}
		thumb := ""
		if vd.SceneParts.Paths != nil && vd.SceneParts.Paths.Screenshot != nil {
			thumb = heatmap.GetCoverUrl(baseURL, id)
		}
		basename := ""
		if len(vd.SceneParts.Files) > 0 && vd.SceneParts.Files[0] != nil {
			basename = vd.SceneParts.Files[0].Basename
		}
		tagInputs := make([]apiinternal.TagInput, 0, len(vd.SceneParts.Tags))
		for _, t := range vd.SceneParts.Tags {
			if t == nil {
				continue
			}
			tagInputs = append(tagInputs, apiinternal.TagInput{Name: t.TagParts.Name, Aliases: t.Aliases})
		}
		tiles = append(tiles, GridTile{
			ID:           id,
			Title:        vd.Title(),
			ThumbnailURL: thumb,
			Projection:   apiinternal.Detect(tagInputs, basename),
		})
	}

	resp := GridResponse{
		Tiles:   tiles,
		HasMore: page*pageSize < total,
	}
	if resp.HasMore {
		resp.NextCursor = strconv.Itoa(page + 1)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: encode grid")
	}
}

// buildGridFilter composes a SceneFilterType from URL params. Each
// individual filter is optional; absent means no constraint.
func buildGridFilter(performer, studio, tag, favorite string, starsMin, ocountMin int) *gql.SceneFilterType {
	if performer == "" && studio == "" && tag == "" && favorite == "" && starsMin == 0 && ocountMin == 0 {
		return nil
	}
	f := &gql.SceneFilterType{}
	if performer != "" {
		f.Performers = &gql.MultiCriterionInput{
			Value:    []string{performer},
			Modifier: gql.CriterionModifierIncludes,
		}
	}
	if studio != "" {
		f.Studios = &gql.HierarchicalMultiCriterionInput{
			Value:    []string{studio},
			Modifier: gql.CriterionModifierIncludes,
			Depth:    util.Ptr(-1),
		}
	}
	if tag != "" {
		f.Tags = &gql.HierarchicalMultiCriterionInput{
			Value:    []string{tag},
			Modifier: gql.CriterionModifierIncludes,
			Depth:    util.Ptr(-1),
		}
	}
	if starsMin > 0 {
		// Stash's rating100 uses 0-100 scale; map stars 1-5 to *20.
		ratingMin := starsMin * 20
		f.Rating100 = &gql.IntCriterionInput{
			Value:    ratingMin,
			Modifier: gql.CriterionModifierGreaterThan, // GREATER_THAN_OR_EQUAL absent in schema; caller passes ratingMin-1
		}
		f.Rating100.Value = ratingMin - 1
	}
	if ocountMin > 0 {
		f.O_counter = &gql.IntCriterionInput{
			Value:    ocountMin - 1,
			Modifier: gql.CriterionModifierGreaterThan,
		}
	}
	if favorite == "only" || favorite == "not" {
		// Implemented via tag filter: include or exclude the FAVORITE_TAG.
		// Done in the handler before calling fetchSceneIDs because it
		// requires the favorite-tag's id, which we don't have here.
		// Caller handles this case separately. (See note below.)
	}
	return f
}

func (h *httpHandler) filterOptionsHandler(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	sb, err := LoadSidebar(r.Context(), h.libraryService.StashClient, kind, "")
	if err != nil {
		http.Error(w, "load sidebar failed", http.StatusInternalServerError)
		return
	}
	var list []Entity
	switch kind {
	case "performer", "perf":
		list = sb.Performers
	case "studio":
		list = sb.Studios
	case "tag":
		list = sb.Tags
	default:
		http.NotFound(w, r)
		return
	}
	out := make([]FilterOption, 0, len(list))
	for _, e := range list {
		out = append(out, FilterOption{ID: e.ID, Name: e.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
```

(Note: the `favorite` filter is left as a follow-up if the simple param-based version turns out tricky — for v1, handler can resolve `FAVORITE_TAG` to a tag id and treat `favorite=only` as equivalent to `tag={favTagID}`.)

- [ ] **Step 3: Mount the routes**

In [internal/api/browse/router.go](../../../internal/api/browse/router.go), add:

```go
	r.Get("/grid", h.gridJSONHandler)
	r.Get("/filter-options/{kind}", h.filterOptionsHandler)
```

- [ ] **Step 4: Vet, build**

Run: `go vet ./...` then `go build ./...`

Expected: clean.

- [ ] **Step 5: Manual verify**

Build, run. In a terminal:

```
curl -i "https://stash-vr.duckdns.org/browse/grid?cursor=1"
```

Expected: 200, JSON with `tiles` array, each tile having `id`, `title`, `thumbnailURL`, `projection`.

```
curl -i "https://stash-vr.duckdns.org/browse/grid?q=POV"
```

Expected: filtered to scenes matching "POV".

```
curl -i "https://stash-vr.duckdns.org/browse/filter-options/performer"
```

Expected: 200, JSON array of `{id, name}`.

- [ ] **Step 6: Commit**

```
git add internal/api/browse/data.go internal/api/browse/grid_json.go internal/api/browse/router.go
git commit -m "browse: JSON grid + filter-options endpoints for in-VR search"
```

---

## Task 3: Browse panel skeleton + Browse button on M4b panel

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Add the browse panel container, its top strip (search field placeholder, Filters button, Clear all, Cols cycle, Close), and a Browse button on M4b's panel that toggles its visibility. No tile rendering yet.

- [ ] **Step 1: Add a Browse button to the M4b panel**

In `vrControls`'s row 3, M4b currently has 8 buttons. Insert a 9th: "Browse." Either shrink button widths to 0.30 m or shift positions. The simpler path: shrink to 0.28 m and tighten gaps:

Find the row 3 buttons in [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml). The current positions (M4b Task 2) are spaced 0.32 m apart starting at -1.20.

Replace with positions spaced 0.28 m apart starting at -1.12, plus a new Browse button between Loop and Format:

| x | Button |
|---|---|
| -1.12 | Mute |
| -0.84 | CC |
| -0.56 | Play/Pause |
| -0.28 | Speed |
|  0.00 | Loop |
|  0.28 | Browse |
|  0.56 | Format |
|  0.84 | Help |
|  1.12 | Exit |

Update each button's `position="x ..."` and `geometry="primitive:plane;width:0.28;height:0.20"`.

Add the Browse button:

```html
<a-entity class="vr-btn" data-action="browse" position="0.28 -0.20 0.01"
          geometry="primitive:plane;width:0.28;height:0.20"
          material="color:#2c5282;opacity:0.95">
  <a-text value="Browse" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
</a-entity>
```

- [ ] **Step 2: Add the browse panel container**

Inside `vrControlsRoot`, after `vrHelpPanel`, add:

```html
<a-entity id="vrBrowsePanel" position="0 1.4 -2.5" rotation="-15 0 0" visible="false">
  <a-plane class="vr-grid-bg vr-btn" data-action="grid-bg" width="3.6" height="2.4"
           color="#000" material="opacity:0.85"></a-plane>

  <!-- Top strip (Tasks 6 + 7 fill these). Skeleton placeholders. -->
  <a-entity id="vrBrowseTopStrip" position="0 1.0 0.01">
    <a-plane class="vr-search-bg vr-btn" data-action="search-focus" width="1.2" height="0.16"
             color="#222" material="opacity:0.9" position="-1.0 0 0">
      <a-text value="Search…" align="left" color="#888" width="3" position="-0.55 0 0.005"></a-text>
    </a-plane>
    <a-entity class="vr-btn" data-action="filters" position="-0.10 0 0"
              geometry="primitive:plane;width:0.45;height:0.16"
              material="color:#2c5282;opacity:0.95">
      <a-text value="Filters ▾" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="filters-clear" position="0.40 0 0"
              geometry="primitive:plane;width:0.45;height:0.16"
              material="color:#2c5282;opacity:0.95">
      <a-text value="Clear all" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="cols-cycle" id="vrColsBtn" position="0.95 0 0"
              geometry="primitive:plane;width:0.40;height:0.16"
              material="color:#2c5282;opacity:0.95">
      <a-text id="vrColsLabel" value="Cols: 4" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="browse-close" position="1.45 0 0"
              geometry="primitive:plane;width:0.20;height:0.16"
              material="color:#a01010;opacity:0.95">
      <a-text value="✕" align="center" color="#fff" width="3.5" position="0 0 0.005"></a-text>
    </a-entity>
  </a-entity>

  <!-- Tile grid root (Task 4 populates). -->
  <a-entity id="vrBrowseTiles"></a-entity>

  <!-- "Loading more..." sentinel (Task 5 toggles). -->
  <a-text id="vrBrowseLoadMore" value="" align="center" color="#888" width="3"
          position="0 -1.05 0.01" visible="false"></a-text>
</a-entity>
```

- [ ] **Step 3: Wire `data-action="browse"` and panel toggle**

In `vrAction`, add the browse and browse-close branches:

```javascript
} else if (action === 'browse') {
  const bp = document.getElementById('vrBrowsePanel');
  if (bp) {
    const visible = bp.getAttribute('visible');
    bp.setAttribute('visible', !visible);
    if (!visible) onBrowsePanelOpen(); // Task 4 will define
  }
} else if (action === 'browse-close') {
  const bp = document.getElementById('vrBrowsePanel');
  if (bp) bp.setAttribute('visible', false);
}
```

Stub `onBrowsePanelOpen` for now:

```javascript
function onBrowsePanelOpen() {
  // Tile fetch + render fills this in Task 4.
}
```

- [ ] **Step 4: Wire cols cycle button (visual only — relayout in Task 4)**

```javascript
let m4cCols = parseInt(localStorage.getItem('m4c.cols') || '4', 10);
if (m4cCols < 3 || m4cCols > 6) m4cCols = 4;

function updateColsLabel() {
  const el = document.getElementById('vrColsLabel');
  if (el) el.setAttribute('value', 'Cols: ' + m4cCols);
}
updateColsLabel();

// Add to vrAction switch:
} else if (action === 'cols-cycle') {
  m4cCols = (m4cCols >= 6) ? 3 : (m4cCols + 1);
  localStorage.setItem('m4c.cols', String(m4cCols));
  updateColsLabel();
  if (typeof relayoutTiles === 'function') relayoutTiles(); // Task 4 defines
}
```

- [ ] **Step 5: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build, run, open a scene. Click Enter VR, summon panel. Verify:

- 9 buttons in row 3, including "Browse" between Loop and Format.
- Click Browse → empty browse panel appears in front of user (3.6 × 2.4 m).
- Top strip shows: search placeholder, Filters ▾, Clear all, Cols: 4, ✕.
- Click Cols cycle → label cycles `Cols: 5 → Cols: 6 → Cols: 3 → Cols: 4`. Persists across reloads.
- Click ✕ → panel hides.
- M4b regressions intact.

- [ ] **Step 6: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "m4c: browse panel skeleton + Browse button on M4b panel + cols cycle"
```

---

## Task 4: Tile rendering + cylinder layout

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Fetch the grid JSON when the browse panel opens, render tiles on a cylinder curve with cover textures, support cols relayout without re-fetch.

- [ ] **Step 1: Add tile fetch + state**

In the IIFE:

```javascript
const m4cState = {
  q: '',
  filters: { performer: '', studio: '', tag: '', favorite: '', stars: 0, ocount: 0 },
  cursor: '1',
  tiles: [],   // accumulated GridTile[]
  hasMore: true,
  loading: false
};

function buildGridParams() {
  const p = new URLSearchParams();
  if (m4cState.q) p.set('q', m4cState.q);
  if (m4cState.filters.performer) p.set('performer', m4cState.filters.performer);
  if (m4cState.filters.studio)    p.set('studio',    m4cState.filters.studio);
  if (m4cState.filters.tag)       p.set('tag',       m4cState.filters.tag);
  if (m4cState.filters.favorite)  p.set('favorite',  m4cState.filters.favorite);
  if (m4cState.filters.stars > 0) p.set('stars',     String(m4cState.filters.stars));
  if (m4cState.filters.ocount > 0) p.set('ocount',   String(m4cState.filters.ocount));
  if (m4cState.cursor !== '1')    p.set('cursor',    m4cState.cursor);
  return p;
}

function fetchGrid(reset) {
  if (m4cState.loading) return;
  if (reset) {
    m4cState.cursor = '1';
    m4cState.tiles = [];
    m4cState.hasMore = true;
  }
  if (!m4cState.hasMore) return;
  m4cState.loading = true;
  const sentinel = document.getElementById('vrBrowseLoadMore');
  if (sentinel) {
    sentinel.setAttribute('value', 'Loading…');
    sentinel.setAttribute('visible', 'true');
  }
  fetch('/browse/grid?' + buildGridParams().toString(), {
    headers: { 'Accept': 'application/json' }
  })
    .then(r => r.json())
    .then(json => {
      m4cState.tiles = m4cState.tiles.concat(json.tiles || []);
      m4cState.hasMore = !!json.hasMore;
      m4cState.cursor = json.nextCursor || '1';
      m4cState.loading = false;
      relayoutTiles();
      if (sentinel) {
        sentinel.setAttribute('value', m4cState.hasMore ? '' : (m4cState.tiles.length ? 'No more scenes' : 'No scenes found'));
        sentinel.setAttribute('visible', !!sentinel.getAttribute('value'));
      }
    })
    .catch(err => {
      m4cState.loading = false;
      if (sentinel) {
        sentinel.setAttribute('value', 'fetch failed');
        sentinel.setAttribute('visible', 'true');
      }
      console.warn('stash-vr: grid fetch failed', err);
    });
}

function onBrowsePanelOpen() {
  fetchGrid(true);
}
```

- [ ] **Step 2: Tile rendering with cylinder layout**

Add the tile rendering logic:

```javascript
const TILE_W = 0.6;
const TILE_H = 0.34;
const TILE_GAP = 0.06;
const ARC_RADIUS = 3.0;
const ARC_HALF = Math.PI / 3; // ±60°
let scrollY = 0; // updated by Task 5

function tileCellPositions() {
  // Returns [{x, y, z, rotY}] for each tile slot, given cols and current scroll.
  const cols = m4cCols;
  const arcStep = (2 * ARC_HALF) / Math.max(cols, 1);
  const positions = [];
  const rows = Math.ceil(m4cState.tiles.length / cols);
  for (let row = 0; row < rows; row++) {
    for (let col = 0; col < cols; col++) {
      const arcOffset = (col - (cols - 1) / 2) * arcStep;
      const x = ARC_RADIUS * Math.sin(arcOffset);
      const z = -ARC_RADIUS * Math.cos(arcOffset) + 1.0; // shift toward user
      const y = 0.6 - row * (TILE_H + TILE_GAP) - scrollY;
      positions.push({ x, y, z, rotY: -arcOffset * 180 / Math.PI });
    }
  }
  return positions;
}

const tileTextures = {}; // cache: sceneId → THREE.Texture

function getTileTexture(url) {
  if (!url) return null;
  if (tileTextures[url]) return tileTextures[url];
  const loader = new AFRAME.THREE.TextureLoader();
  const tex = loader.load(url);
  tileTextures[url] = tex;
  return tex;
}

function relayoutTiles() {
  const root = document.getElementById('vrBrowseTiles');
  if (!root) return;

  // Remove tile entities for tiles no longer in state.tiles.
  const expected = new Set(m4cState.tiles.map(t => t.id));
  Array.from(root.children).forEach(child => {
    if (!expected.has(child.dataset.sceneId)) {
      root.removeChild(child);
    }
  });

  // Add new tile entities for any missing.
  const existing = new Set(Array.from(root.children).map(c => c.dataset.sceneId));
  const positions = tileCellPositions();
  m4cState.tiles.forEach((tile, i) => {
    const pos = positions[i];
    let el = root.querySelector('a-entity[data-scene-id="' + CSS.escape(tile.id) + '"]');
    if (!el) {
      el = document.createElement('a-entity');
      el.classList.add('vr-tile');
      el.dataset.sceneId = tile.id;
      el.dataset.projection = JSON.stringify(tile.projection);
      el.dataset.streamUrl = '/browse/scene/' + encodeURIComponent(tile.id) + '/stream';

      // Cover plane — tap → seamless scene swap (Play). The class
      // .vr-tile-cover is the click target; .vr-btn allows raycaster.
      const plane = document.createElement('a-plane');
      plane.classList.add('vr-btn', 'vr-tile-cover');
      plane.setAttribute('width', TILE_W);
      plane.setAttribute('height', TILE_W * 9 / 16);
      plane.setAttribute('material', 'color:#222;opacity:1;shader:flat');
      el.appendChild(plane);

      // ⓘ detail badge — small circle in the top-right corner of the
      // tile. Tap → opens the detail panel (Task 9).
      const detailBadge = document.createElement('a-entity');
      detailBadge.classList.add('vr-btn', 'vr-tile-detail');
      detailBadge.setAttribute('geometry', 'primitive:circle;radius:0.04');
      detailBadge.setAttribute('material', 'color:#000;opacity:0.85;shader:flat');
      // Top-right corner: half tile width minus radius, half tile height minus radius.
      const badgeX = (TILE_W / 2) - 0.05;
      const badgeY = ((TILE_W * 9 / 16) / 2) - 0.05;
      detailBadge.setAttribute('position', badgeX.toFixed(3) + ' ' + badgeY.toFixed(3) + ' 0.01');
      const badgeText = document.createElement('a-text');
      badgeText.setAttribute('value', 'ⓘ');
      badgeText.setAttribute('align', 'center');
      badgeText.setAttribute('color', '#fff');
      badgeText.setAttribute('width', '1.5');
      badgeText.setAttribute('position', '0 0 0.005');
      detailBadge.appendChild(badgeText);
      el.appendChild(detailBadge);

      const titleEl = document.createElement('a-text');
      titleEl.setAttribute('value', tile.title);
      titleEl.setAttribute('align', 'center');
      titleEl.setAttribute('color', '#fff');
      titleEl.setAttribute('width', '2.5');
      titleEl.setAttribute('wrap-count', '22');
      titleEl.setAttribute('position', '0 -' + ((TILE_W * 9 / 16) / 2 + 0.06) + ' 0.005');
      el.appendChild(titleEl);

      // Texture loads async; bind once available.
      const tex = getTileTexture(tile.thumbnailURL);
      if (tex) {
        plane.addEventListener('loaded', function() {
          const mesh = plane.getObject3D('mesh');
          if (mesh) {
            mesh.material = new AFRAME.THREE.MeshBasicMaterial({ map: tex });
          }
        });
      }

      root.appendChild(el);
    }
    el.setAttribute('position', { x: pos.x, y: pos.y, z: pos.z });
    el.setAttribute('rotation', { x: 0, y: pos.rotY, z: 0 });
  });
}
```

- [ ] **Step 3: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build, run, open a scene. Click Enter VR, summon panel, click Browse. Verify:

- Browse panel opens; "Loading…" sentinel briefly visible.
- Tiles populate (~24 tiles by default page size). Cover textures load and render on planes.
- Each tile shows title text below.
- Tiles are arranged on a slight curve, facing the user.
- Click Cols cycle → tiles relayout to new col count without re-fetching.
- Tiles past the visible window's bottom are positioned but invisible (panel background clips them — Task 5's scroll fixes this).

- [ ] **Step 4: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "m4c: tile rendering with cylinder layout, cover textures, cols relayout"
```

---

## Task 5: Vertical scroll + lazy load + thumbstick handoff

**Files:**
- Modify: `internal/static/browse_scene.gohtml`
- Modify: `internal/static/m3c-controls.js`

**Goal:** Thumbstick Y scrolls the grid when the browse panel is open. Scroll near bottom triggers lazy load. M3c's geometry-scale binding is suppressed while browse is active.

- [ ] **Step 1: Add scroll handler in browse_scene.gohtml**

```javascript
const SCROLL_RATE = 0.6; // m/sec at full stick magnitude
const SCROLL_TRIGGER_MARGIN = 0.5; // start lazy-load when last row within this distance of viewport bottom

function maxScrollY() {
  const cols = m4cCols;
  const rows = Math.ceil(m4cState.tiles.length / cols);
  const totalH = rows * (TILE_H + TILE_GAP);
  const visibleH = 1.6; // vrBrowsePanel height minus top strip
  return Math.max(0, totalH - visibleH);
}

function applyScroll(deltaSec, stickY) {
  // Push UP (negative stickY in WebXR convention) → scroll content up
  // (scrollY increases; tiles shift up). Match the natural feel.
  const dy = -stickY * SCROLL_RATE * deltaSec;
  const next = Math.max(0, Math.min(maxScrollY(), scrollY + dy));
  if (next !== scrollY) {
    scrollY = next;
    relayoutTiles();
    maybeLoadMore();
  }
}

function maybeLoadMore() {
  if (!m4cState.hasMore || m4cState.loading) return;
  if (scrollY > maxScrollY() - SCROLL_TRIGGER_MARGIN) {
    fetchGrid(false);
  }
}

scene.addEventListener('m3c:browse-scroll', function(e) {
  const bp = document.getElementById('vrBrowsePanel');
  if (!bp || bp.getAttribute('visible') !== true && bp.getAttribute('visible') !== 'true') return;
  applyScroll(e.detail.deltaSec, e.detail.stickY);
});
```

- [ ] **Step 2: Modify M3c controller component to emit `m3c:browse-scroll`**

In [internal/static/m3c-controls.js](../../../internal/static/m3c-controls.js), find the thumbstick Y handler that currently emits `m3c:scale`. Add a check: if `vrBrowsePanel` is visible, emit `m3c:browse-scroll` instead.

Conceptual change (the implementer reads the actual file to find exact handler):

```javascript
// Inside the per-tick thumbstick handler:
const browsePanel = document.getElementById('vrBrowsePanel');
const browseOpen = browsePanel && (browsePanel.getAttribute('visible') === true || browsePanel.getAttribute('visible') === 'true');

if (browseOpen && Math.abs(yMag) > 0.3) {
  this.el.sceneEl.emit('m3c:browse-scroll', {
    deltaSec: dtSec,
    stickY: yNorm
  });
  return; // skip the m3c:scale emission for this tick
}
// ... existing scale emission unchanged
```

- [ ] **Step 3: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build, run, open a scene with many results. Enter VR, summon panel, click Browse. Verify:

- Push thumbstick Y down/up → grid scrolls vertically.
- Hold past the visible window → tiles continue scrolling.
- Reach the bottom → "Loading…" sentinel; new batch loads; sentinel clears.
- Reach the very end → "No more scenes" appears.
- Close browse panel → thumbstick Y resumes M3c scale on the active geometry (sphere/plane gets bigger/smaller).
- Re-open browse → scroll resets to 0 if state was reset, or persists if state preserved (v1 spec says re-entering VR resets, but mid-session re-open keeps state — Task 4's `fetchGrid(true)` resets on every open).

- [ ] **Step 4: Commit**

```
git add internal/static/browse_scene.gohtml internal/static/m3c-controls.js
git commit -m "m4c: vertical scroll + lazy load + thumbstick handoff with M3c"
```

---

## Task 6: Search field via invisible-input + 3D echo

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Tap the curved `vr-search-bg` button → focus a hidden `<input>` → Quest VR system keyboard pops. Typing filters the grid live (debounced 250 ms). The typed text is echoed onto the existing curved label of `vr-search-bg` (no DOM overlay).

**Spike outcome (resolved 2026-05-09):** Meta Browser refuses the WebXR DOM Overlay Module (`session.domOverlayState === null`), so a visible HTML `<input>` over the XR content is not viable. **However**, focusing a hidden DOM `<input>` from JS still pops Quest's system VR keyboard, and `input` events still fire with the typed value. Task 6 uses the hidden-input pattern: the `<input>` is just a buffer endpoint that pulls the keyboard up; all visuals (the search field, the typed text echo) are A-Frame entities on the panel.

The `vr-search-bg` cylinder, its `data-action="search-focus"` and per-character curved label already exist. Task 6 adds: (a) a hidden `<input>` element, (b) a `search-focus` handler that focuses it, (c) an `input` listener that updates `m4cState.q` + refetches + relabels the cylinder, (d) auto-blur on browse-panel close.

- [ ] **Step 1: Add the hidden `<input>` buffer**

Insert immediately before `<a-scene>` (so it's a sibling, not a child of the scene — A-Frame treats unknown DOM children oddly):

```html
<input id="vrSearchInput" type="text" autocomplete="off" inputmode="search"
       style="position:fixed; top:-1000px; left:-1000px; opacity:0; width:1px; height:1px;
              pointer-events:none;" />
```

- [ ] **Step 2: Wire the `search-focus` action**

In the IIFE, find the `vrAction(action)` switch (currently has `playpause`, `mute`, …, `cols-cycle`, …). Add a new branch alongside `cols-cycle`:

```javascript
      } else if (action === 'search-focus') {
        // Focus the hidden <input>. Quest's Meta Browser auto-pops the
        // system VR keyboard on input focus, even though it refuses the
        // WebXR DOM Overlay Module. The input itself is invisible — we
        // mirror its value onto the curved cylinder label.
        const si = document.getElementById('vrSearchInput');
        if (si) si.focus();
```

- [ ] **Step 3: Wire the input listener with debounce + label update**

Just after `updateColsLabel();` (so `m4cState`, `fetchGrid`, and `curveLabelInto` are all in scope), add:

```javascript
    // M4c Task 6: live search via hidden <input>. Quest's keyboard
    // delivers chars to the input even though the input itself isn't
    // visible during the WebXR session (DOM overlay is refused). We
    // mirror the value onto the existing curved search-bar label.
    (function wireSearchInput() {
      const si = document.getElementById('vrSearchInput');
      if (!si) return;
      const bg = document.querySelector('.vr-search-bg');
      let timer = null;

      function updateSearchLabel() {
        if (!bg) return;
        const v = si.value;
        bg.setAttribute('data-label', v || 'Search…');
        bg.setAttribute('data-label-color', v ? '#fff' : '#888');
        curveLabelInto(bg);
      }

      si.addEventListener('input', function() {
        updateSearchLabel();
        if (timer) clearTimeout(timer);
        timer = setTimeout(function() {
          const q = si.value.trim();
          if (q === m4cState.q) return;
          m4cState.q = q;
          fetchGrid(true);
        }, 250);
      });

      // Enter dismisses Quest's keyboard but keeps panel open.
      si.addEventListener('keydown', function(ev) {
        if (ev.key === 'Enter') si.blur();
      });

      // When the browse panel hides, drop focus so the keyboard goes away.
      const bp = document.getElementById('vrBrowsePanel');
      if (bp) {
        bp.addEventListener('componentchanged', function(evt) {
          if (evt.detail && evt.detail.name === 'visible') {
            const v = bp.getAttribute('visible');
            if (v === false || v === 'false') si.blur();
          }
        });
      }
    })();
```

- [ ] **Step 4: Vet, build, manually verify on Quest 3**

```
go vet ./...
go build ./...
```

On Quest 3:
- Open a scene, Enter VR, summon control panel, click Browse.
- Tap the curved "Search…" cylinder → Quest's system VR keyboard pops.
- Type a few characters → cylinder label updates char-by-char (gray "Search…" replaced with white typed text).
- After 250 ms idle, grid refetches with `?q=...`.
- Press Enter → keyboard hides; label keeps the typed text; grid stays filtered.
- Tap "Clear all" or backspace to empty → label reverts to gray "Search…", grid refetches without `q`.
- Close browse panel (✕) → keyboard hides on the next tap if it was open.

- [ ] **Step 5: Commit**

```
git add internal/static/browse_scene.gohtml docs/superpowers/plans/2026-05-09-m4c-in-vr-search.md docs/superpowers/specs/2026-05-09-m4c-in-vr-search.md
git commit -m "m4c: in-VR search via hidden <input> + curved-label echo (Quest VR keyboard)"
```

---

## Task 7: Filters panel — 3-column layout, searchable lists, value-picker row, active chips

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Click "Filters ▾" → standalone Filters panel beside the grid opens with three side-by-side columns (Performer / Studio / Tag), each with its own header, search field, and scrollable list. A bottom row holds value-pickers for Favorites / Stars / O-Counter. Active filters display as chips at the top. No tabs. No "Any" buttons — toggling clears.

- [ ] **Step 1: Add the standalone Filters panel HTML (3 columns + bottom row)**

The filters panel is a sibling of `vrBrowsePanel` (not a child). Both live directly under `vrControlsRoot`.

Inside `vrControlsRoot`, **after** the closing `</a-entity>` of `vrBrowsePanel`, add:

```html
<!-- Standalone Filters panel — sits to the right of the browse panel.
     Three side-by-side columns (Performer / Studio / Tag) plus a bottom
     row of value-pickers (Favorites / Stars / O-Counter). All sections
     visible simultaneously; no tabs. -->
<a-entity id="vrFiltersPanel" position="3.6 1.4 -2.5" rotation="0 -25 0" visible="false">
  <a-plane width="3.0" height="1.8" color="#000" material="opacity:0.95"></a-plane>
  <a-text value="Filters" align="left" color="#fff" width="3" position="-1.40 0.80 0.01"></a-text>
  <a-entity class="vr-btn" data-action="filters-close" position="1.38 0.80 0.01"
            geometry="primitive:plane;width:0.18;height:0.14"
            material="color:#a01010;opacity:0.95">
    <a-text value="✕" align="center" color="#fff" width="3.5" position="0 0 0.005"></a-text>
  </a-entity>

  <!-- Active-filter chips area. JS populates with one chip per active filter. -->
  <a-entity id="vrFiltersChips" position="0 0.62 0.01"></a-entity>

  <!-- Three columns side-by-side. Each column owns its header label,
       search field, and scrollable list. Column centers at x = -1.0, 0, 1.0
       (column width 0.95m each). Header at panel-y 0.40, search at 0.27,
       list starts at 0.16. -->
  {{range $kind, $center := (dict "performer" -1.0 "studio" 0.0 "tag" 1.0)}}
  <!-- (kind = performer | studio | tag) -->
  {{end}}

  <!-- Performer column -->
  <a-text value="Performer" align="center" color="#fff" width="2.5" position="-1.0 0.40 0.01"></a-text>
  <a-plane id="vrPickerSearchBg-performer" class="vr-search-bg vr-btn" data-action="picker-search-focus" data-picker-kind="performer"
           width="0.92" height="0.12" color="#222" material="opacity:0.9" position="-1.0 0.27 0.01">
    <a-text id="vrPickerSearchLabel-performer" value="Search…" align="left" color="#888" width="3" position="-0.42 0 0.005"></a-text>
  </a-plane>
  <a-entity id="vrFilterList-performer" data-kind="performer" position="-1.0 0.16 0.01"></a-entity>

  <!-- Studio column -->
  <a-text value="Studio" align="center" color="#fff" width="2.5" position="0.0 0.40 0.01"></a-text>
  <a-plane id="vrPickerSearchBg-studio" class="vr-search-bg vr-btn" data-action="picker-search-focus" data-picker-kind="studio"
           width="0.92" height="0.12" color="#222" material="opacity:0.9" position="0.0 0.27 0.01">
    <a-text id="vrPickerSearchLabel-studio" value="Search…" align="left" color="#888" width="3" position="-0.42 0 0.005"></a-text>
  </a-plane>
  <a-entity id="vrFilterList-studio" data-kind="studio" position="0.0 0.16 0.01"></a-entity>

  <!-- Tag column -->
  <a-text value="Tag" align="center" color="#fff" width="2.5" position="1.0 0.40 0.01"></a-text>
  <a-plane id="vrPickerSearchBg-tag" class="vr-search-bg vr-btn" data-action="picker-search-focus" data-picker-kind="tag"
           width="0.92" height="0.12" color="#222" material="opacity:0.9" position="1.0 0.27 0.01">
    <a-text id="vrPickerSearchLabel-tag" value="Search…" align="left" color="#888" width="3" position="-0.42 0 0.005"></a-text>
  </a-plane>
  <a-entity id="vrFilterList-tag" data-kind="tag" position="1.0 0.16 0.01"></a-entity>

  <!-- Bottom row: value-pickers (Favorites / Stars / O-Counter). Single horizontal
       row at panel-y -0.75. JS renders the buttons. -->
  <a-entity id="vrFilterValuesRow" position="0 -0.75 0.01"></a-entity>
</a-entity>
```

(Remove the `{{range}}` template-stub block above — it's just a comment showing the column-center mapping; only the explicit per-column HTML matters.)

The previous standalone `vrFilterOptions` panel from earlier revisions of this plan is **removed** entirely — delete its `<a-entity>` block from the HTML if present.

- [ ] **Step 2: Add filter state + chip rendering + browse-close cascade**

In the IIFE (just below the `m4cState` block declared in Task 4), add the filter-state extensions and chip rendering. Task 6's grid-search functions are kept as-is and extended in Step 5 below.

```javascript
m4cState.filterNames    = { performer: '', studio: '', tag: '' };
m4cState.lastScrollFocus = 'grid';     // 'grid' | 'list-performer' | 'list-studio' | 'list-tag' | 'none'
m4cState.cachedOptions  = {};          // kind → [{id,name}, ...]
m4cState.pickerQuery    = { performer: '', studio: '', tag: '' };
m4cState.listScrollY    = { performer: 0, studio: 0, tag: 0 };

const LIST_VISIBLE_ROWS = 5;
const LIST_ROW_H = 0.10;

function chipLabel(kind) {
  if (kind === 'performer') return m4cState.filterNames.performer ? 'Performer: ' + m4cState.filterNames.performer : '';
  if (kind === 'studio')    return m4cState.filterNames.studio    ? 'Studio: '    + m4cState.filterNames.studio    : '';
  if (kind === 'tag')       return m4cState.filterNames.tag       ? 'Tag: '       + m4cState.filterNames.tag       : '';
  if (kind === 'favorite' && m4cState.filters.favorite === 'only') return 'Favorites: Only';
  if (kind === 'favorite' && m4cState.filters.favorite === 'not')  return 'Favorites: Not';
  if (kind === 'stars'    && m4cState.filters.stars > 0) return 'Stars: ' + (m4cState.filters.stars === 5 ? '5 only' : m4cState.filters.stars + '+');
  if (kind === 'ocount'   && m4cState.filters.ocount > 0) return 'O-Counter: ' + m4cState.filters.ocount + '+';
  return '';
}

function renderActiveChips() {
  const root = document.getElementById('vrFiltersChips');
  if (!root) return;
  while (root.firstChild) root.removeChild(root.firstChild);
  const kinds = ['performer', 'studio', 'tag', 'favorite', 'stars', 'ocount'];
  let xOffset = -1.30;
  kinds.forEach(kind => {
    const lbl = chipLabel(kind);
    if (!lbl) return;
    const chip = document.createElement('a-entity');
    chip.classList.add('vr-btn');
    chip.dataset.action = 'filter-chip-clear';
    chip.dataset.chipKind = kind;
    chip.setAttribute('geometry', 'primitive:plane;width:0.85;height:0.10');
    chip.setAttribute('material', 'color:#3776c2;opacity:0.95');
    chip.setAttribute('position', xOffset + ' 0 0');
    const text = document.createElement('a-text');
    text.setAttribute('value', lbl + '  ✕');
    text.setAttribute('align', 'center');
    text.setAttribute('color', '#fff');
    text.setAttribute('width', '2.5');
    text.setAttribute('position', '0 0 0.005');
    chip.appendChild(text);
    root.appendChild(chip);
    xOffset += 0.95;
    // Wrap to next row if too many; v1 caps at one row's worth.
    if (xOffset > 1.30) xOffset = -1.30;
  });
}

function clearChip(kind) {
  if (kind === 'performer' || kind === 'studio' || kind === 'tag') {
    m4cState.filters[kind] = '';
    m4cState.filterNames[kind] = '';
    renderColumnList(kind);
  } else if (kind === 'favorite') {
    m4cState.filters.favorite = '';
    renderValuesRow();
  } else if (kind === 'stars') {
    m4cState.filters.stars = 0;
    renderValuesRow();
  } else if (kind === 'ocount') {
    m4cState.filters.ocount = 0;
    renderValuesRow();
  }
  renderActiveChips();
  fetchGrid(true);
}
```

Update `vrAction`'s `'browse-close'` branch (replacing the previous Task 6 version) to cascade to filters:

```javascript
} else if (action === 'browse-close') {
  document.getElementById('vrBrowsePanel').setAttribute('visible', false);
  document.getElementById('vrFiltersPanel').setAttribute('visible', false);
  hideSearchOverlay();
} else if (action === 'filters') {
  const fp = document.getElementById('vrFiltersPanel');
  if (fp) {
    fp.setAttribute('visible', true);
    renderActiveChips();
    ['performer', 'studio', 'tag'].forEach(renderColumnList);
    renderValuesRow();
  }
} else if (action === 'filters-close') {
  document.getElementById('vrFiltersPanel').setAttribute('visible', false);
} else if (action === 'filters-clear') {
  m4cState.q = '';
  m4cState.filters = { performer: '', studio: '', tag: '', favorite: '', stars: 0, ocount: 0 };
  m4cState.filterNames = { performer: '', studio: '', tag: '' };
  m4cState.pickerQuery = { performer: '', studio: '', tag: '' };
  const searchInput = document.getElementById('vrSearchInput');
  if (searchInput) searchInput.value = '';
  const lbl = document.getElementById('vrSearchLabel');
  if (lbl) { lbl.setAttribute('value', 'Search…'); lbl.setAttribute('color', '#888'); }
  ['performer', 'studio', 'tag'].forEach(k => {
    const pkLbl = document.getElementById('vrPickerSearchLabel-' + k);
    if (pkLbl) { pkLbl.setAttribute('value', 'Search…'); pkLbl.setAttribute('color', '#888'); }
  });
  renderActiveChips();
  ['performer', 'studio', 'tag'].forEach(renderColumnList);
  renderValuesRow();
  fetchGrid(true);
} else if (action === 'picker-search-focus') {
  // The clicked element carries data-picker-kind; resolved by the click delegate below.
}
```

Wire two delegates near the existing `.vr-btn` click forEach loop — chip-clear and picker-search-focus need `data-*` lookup:

```javascript
document.addEventListener('click', function(evt) {
  // Chip clear.
  let chip = evt.target;
  while (chip && !(chip.dataset && chip.dataset.action === 'filter-chip-clear')) chip = chip.parentElement;
  if (chip && chip.dataset.chipKind) {
    clearChip(chip.dataset.chipKind);
    return;
  }
  // Picker-search-focus: identify which column was tapped.
  let bg = evt.target;
  while (bg && !(bg.dataset && bg.dataset.action === 'picker-search-focus')) bg = bg.parentElement;
  if (bg && bg.dataset.pickerKind) {
    showPickerSearchOverlay(bg.dataset.pickerKind);
  }
});
```

- [ ] **Step 3: Per-column list rendering with caching**

```javascript
function ensureCachedOptions(kind) {
  return new Promise(resolve => {
    if (m4cState.cachedOptions[kind]) { resolve(m4cState.cachedOptions[kind]); return; }
    fetch('/browse/filter-options/' + kind, { headers: { 'Accept': 'application/json' } })
      .then(r => r.json())
      .then(opts => { m4cState.cachedOptions[kind] = opts || []; resolve(m4cState.cachedOptions[kind]); })
      .catch(err => { console.warn('stash-vr: filter options fetch failed', err); resolve([]); });
  });
}

function renderColumnList(kind) {
  const body = document.getElementById('vrFilterList-' + kind);
  if (!body) return;
  while (body.firstChild) body.removeChild(body.firstChild);

  ensureCachedOptions(kind).then(opts => {
    if (!body.parentNode) return; // panel closed
    const q = (m4cState.pickerQuery[kind] || '').trim().toLowerCase();
    const filtered = q ? opts.filter(o => o.name.toLowerCase().includes(q)) : opts;
    const scrollY = m4cState.listScrollY[kind] || 0;
    while (body.firstChild) body.removeChild(body.firstChild);
    filtered.forEach((opt, i) => {
      const row = document.createElement('a-entity');
      row.classList.add('vr-btn');
      row.dataset.kind = kind;
      row.dataset.optId = String(opt.id);
      row.setAttribute('geometry', 'primitive:plane;width:0.92;height:' + (LIST_ROW_H - 0.01).toFixed(2));
      const isSelected = (m4cState.filters[kind] === opt.id);
      row.setAttribute('material', 'color: ' + (isSelected ? '#3776c2' : '#222') + '; opacity:0.95');
      const y = -i * LIST_ROW_H + scrollY;
      row.setAttribute('position', '0 ' + y.toFixed(3) + ' 0.005');
      const text = document.createElement('a-text');
      text.setAttribute('value', opt.name);
      text.setAttribute('align', 'left');
      text.setAttribute('color', '#fff');
      text.setAttribute('width', '2.5');
      text.setAttribute('position', '-0.42 0 0.005');
      row.appendChild(text);
      body.appendChild(row);
      row.addEventListener('click', function() {
        m4cState.lastScrollFocus = 'list-' + kind;
        applyFilterPick(kind, opt.id, opt.name);
      });
    });
  });
}

function applyFilterPick(kind, id, name) {
  if (kind === 'performer' || kind === 'studio' || kind === 'tag') {
    if (m4cState.filters[kind] === id) {
      m4cState.filters[kind] = '';
      m4cState.filterNames[kind] = '';
    } else {
      m4cState.filters[kind] = id;
      m4cState.filterNames[kind] = name;
    }
    renderColumnList(kind);
  } else if (kind === 'favorite') {
    m4cState.filters.favorite = m4cState.filters.favorite === id ? '' : id;
    renderValuesRow();
  } else if (kind === 'stars') {
    const n = parseInt(id || '0', 10);
    m4cState.filters.stars = m4cState.filters.stars === n ? 0 : n;
    renderValuesRow();
  } else if (kind === 'ocount') {
    const n = parseInt(id || '0', 10);
    m4cState.filters.ocount = m4cState.filters.ocount === n ? 0 : n;
    renderValuesRow();
  }
  renderActiveChips();
  fetchGrid(true);
}
```

- [ ] **Step 4: Bottom-row value pickers (no "Any")**

```javascript
function renderValuesRow() {
  const body = document.getElementById('vrFilterValuesRow');
  if (!body) return;
  while (body.firstChild) body.removeChild(body.firstChild);

  // Single horizontal row: Favorites buttons (left), Stars buttons (middle),
  // O-Counter buttons (right). No "Any" button — absence-of-selection means
  // "any". Toggling the active button off clears the filter.
  const groups = [
    { kind: 'favorite', label: 'Favorites:', startX: -1.40, opts: [
      { id: 'only', name: 'Only' },
      { id: 'not',  name: 'Not'  },
    ]},
    { kind: 'stars', label: 'Stars:', startX: -0.55, opts: [
      { id: '1', name: '1+' },
      { id: '2', name: '2+' },
      { id: '3', name: '3+' },
      { id: '4', name: '4+' },
      { id: '5', name: '5 only' },
    ]},
    { kind: 'ocount', label: 'O-Count:', startX: 0.78, opts: [
      { id: '1',  name: '1+'  },
      { id: '5',  name: '5+'  },
      { id: '10', name: '10+' },
    ]},
  ];

  groups.forEach(g => {
    const lbl = document.createElement('a-text');
    lbl.setAttribute('value', g.label);
    lbl.setAttribute('align', 'left');
    lbl.setAttribute('color', '#fff');
    lbl.setAttribute('width', '2.5');
    lbl.setAttribute('position', g.startX.toFixed(2) + ' 0.10 0.005');
    body.appendChild(lbl);

    let xOffset = g.startX;
    g.opts.forEach(opt => {
      const btn = document.createElement('a-entity');
      btn.classList.add('vr-btn');
      btn.setAttribute('geometry', 'primitive:plane;width:0.18;height:0.10');
      const isSelected = (
        (g.kind === 'favorite' && m4cState.filters.favorite === opt.id) ||
        (g.kind === 'stars'    && String(m4cState.filters.stars)  === opt.id) ||
        (g.kind === 'ocount'   && String(m4cState.filters.ocount) === opt.id)
      );
      btn.setAttribute('material', 'color: ' + (isSelected ? '#3776c2' : '#2c5282') + '; opacity:0.95');
      btn.setAttribute('position', xOffset.toFixed(2) + ' -0.04 0.005');
      const text = document.createElement('a-text');
      text.setAttribute('value', opt.name);
      text.setAttribute('align', 'center');
      text.setAttribute('color', '#fff');
      text.setAttribute('width', '3');
      text.setAttribute('position', '0 0 0.005');
      btn.appendChild(text);
      body.appendChild(btn);
      btn.addEventListener('click', function() {
        m4cState.lastScrollFocus = 'none';
        applyFilterPick(g.kind, opt.id, opt.name);
      });
      xOffset += 0.20;
    });
  });
}
```

- [ ] **Step 5: DOM-overlay input retargeting (grid + 3 picker columns)**

The same `<input id="vrSearchInput">` is reused for the grid search and each of the three picker-list searches. `overlayTarget` extends to one of: `'grid'`, `'picker-performer'`, `'picker-studio'`, `'picker-tag'`.

```javascript
let overlayTarget = 'grid';

function showSearchOverlayForGrid() {
  overlayTarget = 'grid';
  const overlay = document.getElementById('vrDomOverlay');
  const input = document.getElementById('vrSearchInput');
  if (!overlay || !input) return;
  input.placeholder = 'Search scenes…';
  input.value = m4cState.q || '';
  overlay.style.display = 'block';
  setTimeout(() => input.focus(), 50);
}

function showPickerSearchOverlay(kind) {
  overlayTarget = 'picker-' + kind;
  m4cState.lastScrollFocus = 'list-' + kind;
  const overlay = document.getElementById('vrDomOverlay');
  const input = document.getElementById('vrSearchInput');
  if (!overlay || !input) return;
  input.placeholder = 'Search ' + kind + '…';
  input.value = m4cState.pickerQuery[kind] || '';
  overlay.style.display = 'block';
  setTimeout(() => input.focus(), 50);
}
```

Replace the existing single `showSearchOverlay` (Task 6) with `showSearchOverlayForGrid`. Update `vrAction`'s `'search-focus'` branch:

```javascript
} else if (action === 'search-focus') {
  showSearchOverlayForGrid();
}
```

Replace the `searchInput.addEventListener('input', ...)` body from Task 6 with target-aware dispatch:

```javascript
searchInput.addEventListener('input', function() {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    if (overlayTarget === 'grid') {
      m4cState.q = searchInput.value.trim();
      const lbl = document.getElementById('vrSearchLabel');
      if (lbl) {
        lbl.setAttribute('value', m4cState.q || 'Search…');
        lbl.setAttribute('color', m4cState.q ? '#fff' : '#888');
      }
      fetchGrid(true);
    } else if (overlayTarget && overlayTarget.startsWith('picker-')) {
      const kind = overlayTarget.slice(7); // 'performer' | 'studio' | 'tag'
      m4cState.pickerQuery[kind] = searchInput.value;
      const pkLbl = document.getElementById('vrPickerSearchLabel-' + kind);
      if (pkLbl) {
        pkLbl.setAttribute('value', m4cState.pickerQuery[kind] || 'Search…');
        pkLbl.setAttribute('color', m4cState.pickerQuery[kind] ? '#fff' : '#888');
      }
      m4cState.listScrollY[kind] = 0;
      renderColumnList(kind);
    }
  }, overlayTarget === 'grid' ? 250 : 100);
});
```

(Picker search is local-only, so 100 ms debounce is fine.)

- [ ] **Step 6: Scroll target handoff (grid vs each list)**

Track which column the user last interacted with. The applicable foci are: `grid`, `list-performer`, `list-studio`, `list-tag`, `none`. The `m3c:browse-scroll` event routes to whichever is current.

In the IIFE, near the click delegates:

```javascript
// Tapping a tile sets focus to grid (so subsequent thumbstick scrolls the grid).
// applyFilterPick + renderColumnList already set 'list-<kind>'; bottom-row clicks
// set 'none' (no scroll).
document.getElementById('vrBrowsePanel').addEventListener('click', function(evt) {
  if (evt.target.closest('.vr-tile')) return; // tile click → handled by Task 8 swap
  m4cState.lastScrollFocus = 'grid';
});
```

Modify the `m3c:browse-scroll` listener (added in Task 5) to dispatch by focus:

```javascript
scene.addEventListener('m3c:browse-scroll', function(e) {
  const bp = document.getElementById('vrBrowsePanel');
  if (!bp) return;
  const browseOpen = bp.getAttribute('visible') === true || bp.getAttribute('visible') === 'true';
  if (!browseOpen) return;

  const focus = m4cState.lastScrollFocus;
  if (focus === 'grid') {
    applyScroll(e.detail.deltaSec, e.detail.stickY);
  } else if (focus === 'list-performer' || focus === 'list-studio' || focus === 'list-tag') {
    const kind = focus.slice(5); // strip 'list-'
    applyListScroll(kind, e.detail.deltaSec, e.detail.stickY);
  }
  // focus === 'none' → no scroll
});

function applyListScroll(kind, deltaSec, stickY) {
  const dy = -stickY * SCROLL_RATE * deltaSec;
  const cur = m4cState.listScrollY[kind] || 0;
  const next = Math.max(0, Math.min(maxListScrollY(kind), cur + dy));
  if (next !== cur) {
    m4cState.listScrollY[kind] = next;
    renderColumnList(kind);
  }
}

function maxListScrollY(kind) {
  const opts = m4cState.cachedOptions[kind] || [];
  const q = (m4cState.pickerQuery[kind] || '').trim().toLowerCase();
  const count = q ? opts.filter(o => o.name.toLowerCase().includes(q)).length : opts.length;
  const totalH = count * LIST_ROW_H;
  const visibleH = LIST_VISIBLE_ROWS * LIST_ROW_H;
  return Math.max(0, totalH - visibleH);
}
```

The thumbstick handler from M3c (Task 5) is unchanged — it always emits `m3c:browse-scroll` when browse is open; the delegate above routes by focus.

- [ ] **Step 7: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build, run on Quest 3. Open a scene. Enter VR, summon panel, click Browse, click Filters ▾. Verify (full §8 E from spec):

- Filters panel appears to the right of the grid (angled toward user). All sections visible at once: 3 columns + bottom value-row + chips area at top.
- Each column shows a header label, a search field, and a list of names.
- Bottom row shows Favorites buttons [Only][Not], Stars buttons [1+][2+][3+][4+][5 only], O-Counter buttons [1+][5+][10+]. No "Any" buttons.
- Tap Performer column's search field → DOM overlay appears with focused input; Quest VR keyboard pops with placeholder "Search performer…".
- Type "Ali" → only the Performer list narrows; Studio and Tag lists unchanged.
- Tap "Alice" → row highlights blue; chip "Performer: Alice ✕" appears at top; grid filters.
- Tap "Alice" again → row de-highlights; chip disappears. Grid restores.
- Repeat search/select on Studio column independently.
- Repeat on Tag column.
- Tap "Only" under Favorites → button highlights; chip "Favorites: Only ✕" appears.
- Tap "Only" again → de-highlights; chip clears.
- Tap "3+" under Stars → button highlights; chip appears.
- Tap "1+" under O-Counter → chip appears.
- Tap inside Performer column then push thumbstick Y → only Performer list scrolls.
- Tap inside Studio column then push thumbstick Y → only Studio list scrolls.
- Tap inside Tag column then push thumbstick Y → only Tag list scrolls.
- Tap a value-picker button then push thumbstick Y → nothing scrolls (focus = 'none').
- Tap a grid tile area (not a tile itself) then push thumbstick Y → grid scrolls.
- Tap "Clear all" on browse top strip → all chips clear; all column highlights de-activate; all value buttons de-highlight; grid restores.
- Tap ✕ on filters panel → filters close; browse stays.
- Tap ✕ on browse panel → both panels close.

- [ ] **Step 8: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "m4c: filters panel — 3 columns + value row, searchable lists, active chips"
```

---

## Task 8: Seamless scene swap

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Click a tile → fade audio + visual to black → swap `<video>` src + projection rebind → fade back. WebXR session stays alive throughout.

- [ ] **Step 1: Add the fade overlay plane**

Inside `<a-scene>`, add a black overlay plane parented to camera:

```html
<a-entity id="vrFadeOverlay" position="0 1.6 0">
  <a-plane id="vrFadePlane" width="100" height="100" color="#000"
           material="opacity:0;transparent:true;shader:flat" position="0 0 -0.5"></a-plane>
</a-entity>
```

(Position 100×100 plane just in front of camera; material starts transparent. Plane is parented to scene; we'll reposition it as a child of the camera at runtime.)

- [ ] **Step 2: Add swap logic**

```javascript
let swapInFlight = false;

function fadeAudio(target, durMs) {
  return new Promise(resolve => {
    const start = video.volume;
    const t0 = performance.now();
    function step() {
      const elapsed = performance.now() - t0;
      const k = Math.min(1, elapsed / durMs);
      video.volume = start + (target - start) * k;
      if (k < 1) requestAnimationFrame(step);
      else resolve();
    }
    step();
  });
}

function fadeVisual(target, durMs) {
  return new Promise(resolve => {
    const plane = document.getElementById('vrFadePlane');
    if (!plane) { resolve(); return; }
    const startMaterial = plane.getAttribute('material') || {};
    const startOp = parseFloat(startMaterial.opacity || 0);
    const t0 = performance.now();
    function step() {
      const elapsed = performance.now() - t0;
      const k = Math.min(1, elapsed / durMs);
      const op = startOp + (target - startOp) * k;
      plane.setAttribute('material', 'opacity:' + op + ';transparent:true;shader:flat;color:#000');
      if (k < 1) requestAnimationFrame(step);
      else resolve();
    }
    step();
  });
}

function reparentFadeToCamera() {
  const fade = document.getElementById('vrFadeOverlay');
  if (!fade || !fade.object3D || !scene.camera) return;
  if (fade.object3D.parent !== scene.camera) {
    if (fade.object3D.parent) fade.object3D.parent.remove(fade.object3D);
    scene.camera.add(fade.object3D);
    fade.object3D.position.set(0, 0, 0);
  }
}

async function swapToScene(tile) {
  if (swapInFlight) return;
  swapInFlight = true;
  reparentFadeToCamera();

  // Fade out.
  await Promise.all([fadeAudio(0, 200), fadeVisual(1, 200)]);

  // Swap video source.
  video.pause();
  video.src = tile.dataset.streamUrl;
  video.load();

  // Update projection if different.
  const newProjection = JSON.parse(tile.dataset.projection || '{}');
  scene.dataset.geometry = newProjection.Geometry || '';
  scene.dataset.fov = String(newProjection.FOV || '');
  scene.dataset.stereo = (newProjection.Stereo || '').toLowerCase();

  // Decide active geometry id from projection.
  let activeId = 'vrFlat';
  if (newProjection.Geometry === 'fisheye') {
    activeId = 'vrFisheye';
    const fovEl = document.getElementById('vrFisheye');
    if (fovEl) fovEl.dataset.fov = String(newProjection.FOV || 180);
  } else if (newProjection.Geometry === 'equirectangular' && newProjection.FOV === 360) {
    activeId = 'vrSphere360';
  } else if (newProjection.Geometry === 'equirectangular') {
    activeId = 'vrSphere180';
  }
  ['vrSphere180', 'vrSphere360', 'vrFisheye', 'vrFlat'].forEach(id => {
    const el = document.getElementById(id);
    if (!el) return;
    el.setAttribute('visible', id === activeId);
    const mesh = el.getObject3D('mesh');
    if (mesh) mesh.userData.boundVR = false;
  });
  // Force material rebind via existing applyAll.
  applyAll();

  // Reset M3c geometry pose.
  resetGeometry();

  // Wait for video to be ready.
  await new Promise(resolve => {
    const onCanPlay = () => { video.removeEventListener('canplay', onCanPlay); resolve(); };
    video.addEventListener('canplay', onCanPlay);
    // Safety timeout.
    setTimeout(resolve, 5000);
  });

  // Resume playback.
  try { await video.play(); } catch (e) { console.warn('stash-vr: post-swap play failed', e); }

  // Fade back in.
  await Promise.all([fadeAudio(1, 200), fadeVisual(0, 200)]);

  // Hide browse panel.
  const bp = document.getElementById('vrBrowsePanel');
  if (bp) bp.setAttribute('visible', false);
  hideSearchOverlay();

  // Update title text.
  const titleEl = document.getElementById('vrTitle');
  if (titleEl) titleEl.setAttribute('value', tile.dataset.tileTitle || '');

  swapInFlight = false;
}

// Wire tile clicks. Distinguish cover-tap (Play) from ⓘ-tap (Detail).
document.addEventListener('click', function(evt) {
  // Walk up to find the nearest classed ancestor.
  let cover = evt.target;
  while (cover && !(cover.classList && cover.classList.contains('vr-tile-cover'))) cover = cover.parentElement;
  let detail = evt.target;
  while (detail && !(detail.classList && detail.classList.contains('vr-tile-detail'))) detail = detail.parentElement;

  if (detail) {
    // ⓘ tapped — Task 9 wires the detail panel.
    const tileEl = detail.closest('.vr-tile');
    if (tileEl && typeof openDetailPanel === 'function') openDetailPanel(tileEl.dataset.sceneId);
    return;
  }
  if (cover) {
    // Cover tapped — Play.
    const tileEl = cover.closest('.vr-tile');
    if (!tileEl) return;
    tileEl.dataset.tileTitle = tileEl.querySelector('a-text')?.getAttribute('value') || '';
    swapToScene(tileEl);
  }
});
```

(The `tileEl.dataset.tileTitle` is set on click for the post-swap title display, since the new title comes from the clicked tile's text.)

- [ ] **Step 3: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build, run, open a scene. Enter VR. Open Browse. Verify:

- Click a tile of a different projection (e.g., currently DOME 180° SBS, click a fisheye tile) → black fade out, video swaps, projection rebinds, fade in. New scene plays from start.
- Click a tile of the same projection → fade out, src swap only, fade in.
- Browse panel auto-closes after swap.
- M4b state preserved: mute, speed, loop survive.
- Subtitles reset (CC button hides if new scene has no captions).
- Time display updates to new scene's duration.
- Scrub bar reflects new scene.
- Scene markers refresh from new scene.
- M3c geometry pose at default for new projection.
- Click swap while previous swap in-flight → ignored (debounced via `swapInFlight`).

- [ ] **Step 4: Refresh M4b panel state on swap**

The post-swap title update is in step 2, but other M4b state (CC button visibility, scene markers, duration) needs refresh. Since the new scene's metadata is on the tile (only `id`, `title`, `thumbnailURL`, `projection`), captions and scene markers aren't there.

Two paths:
(a) Add `captions` and `sceneMarkers` to `GridTile` JSON (server change).
(b) After swap, fetch full scene metadata from a small JSON endpoint (`/browse/scene/{id}.json`) and update M4b state from it.

For v1, go with (b). Add a tiny fetch after the swap completes:

```javascript
// At end of swapToScene, before hiding browse panel:
const meta = await fetch('/browse/scene/' + encodeURIComponent(tile.dataset.sceneId) + '/meta', {
  headers: { 'Accept': 'application/json' }
}).then(r => r.json()).catch(() => null);
if (meta) {
  // Update sceneMarkers, captions for M4b's UI.
  sceneMarkers.length = 0;
  Array.prototype.push.apply(sceneMarkers, meta.sceneMarkers || []);
  buildSceneMarkerDots();
  // Reset captions UI.
  currentCues = []; currentLang = '';
  const ccBtn = document.getElementById('vrCCBtn');
  if (ccBtn) ccBtn.setAttribute('visible', !!(meta.captions && meta.captions.length));
  // Rebuild caption picker buttons (Task 5 of M4b template-rendered them; for swap we need a JS rebuild).
  rebuildCaptionPicker(meta.captions || []);
}
```

This implies a new endpoint `GET /browse/scene/{id}/meta` returning a rich payload — used both here (post-swap M4b refresh) AND by Task 9's detail panel. Single source of truth.

Add to grid_json.go (or a new metadata.go):

```go
type SceneMeta struct {
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	DurationSec  float64       `json:"durationSec"`
	Date         string        `json:"date"`
	Rating1to5   int           `json:"rating1to5"`
	Performers   []EntityRef   `json:"performers"`
	Studio       *EntityRef    `json:"studio,omitempty"`
	Tags         []EntityRef   `json:"tags"`
	Captions     []CaptionRef  `json:"captions"`
	SceneMarkers []SceneMarker `json:"sceneMarkers"`
}

func (h *httpHandler) sceneMetaHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil {
		http.NotFound(w, r)
		return
	}
	sp := vd.SceneParts
	out := SceneMeta{Title: vd.Title()}
	if sp.Details != nil {
		out.Description = *sp.Details
	}
	if sp.Date != nil {
		out.Date = *sp.Date
	}
	if sp.Rating100 != nil {
		out.Rating1to5 = *sp.Rating100 / 20
	}
	if len(sp.Files) > 0 && sp.Files[0] != nil {
		out.DurationSec = sp.Files[0].Duration
	}
	for _, p := range sp.Performers {
		if p == nil {
			continue
		}
		out.Performers = append(out.Performers, EntityRef{ID: p.Id, Name: p.Name})
	}
	if sp.Studio != nil {
		out.Studio = &EntityRef{ID: sp.Studio.Id, Name: sp.Studio.Name}
	}
	favTag := config.Application().FavoriteTag
	for _, t := range sp.Tags {
		if t == nil {
			continue
		}
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		if favTag != "" && t.TagParts.Name == favTag {
			continue
		}
		out.Tags = append(out.Tags, EntityRef{ID: t.TagParts.Id, Name: t.TagParts.Name})
	}
	for _, c := range sp.Captions {
		if c == nil {
			continue
		}
		out.Captions = append(out.Captions, CaptionRef{
			LanguageCode: c.Language_code,
			CaptionType:  c.Caption_type,
		})
	}
	for _, m := range sp.Scene_markers {
		if m == nil {
			continue
		}
		out.SceneMarkers = append(out.SceneMarkers, SceneMarker{
			Seconds: m.Seconds,
			Title:   m.Title,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
```

Mount: `r.Get("/scene/{id}/meta", h.sceneMetaHandler)` in router.go.

(Verify that `SceneParts.Details` exists in the GraphQL fragment — it's the description field. If not, add `details` to the `SceneParts` fragment in `query.graphql` and regen.)

`rebuildCaptionPicker(captions)`: clears existing language buttons in `vrSubtitlePicker` (those rendered by M4b's `renderSubtitlePicker`) and re-creates them from the JSON. Implementation: just re-call `renderSubtitlePicker()` after assigning the new caption list to its source variable.

- [ ] **Step 5: Re-vet, re-build, re-verify swap with new metadata refresh**

Run: `go vet ./...` then `go build ./...` — expect clean.

Re-test the swap on the headset; verify scene markers refresh, CC button visibility flips correctly, time display updates.

- [ ] **Step 6: Commit**

```
git add internal/static/browse_scene.gohtml internal/api/browse/grid_json.go internal/api/browse/router.go
git commit -m "m4c: seamless scene swap with fade, projection rebind, M4b state refresh"
```

---

## Task 9: Detail panel

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Tap ⓘ on a tile → standalone detail panel opens in front of the grid showing title, description, performer/studio/tag chips, date, duration, rating, and a "Play this scene" button. Clicking a chip closes the panel and applies the matching filter. The panel uses the rich `/scene/{id}/meta` endpoint (extended in Task 8).

- [ ] **Step 1: Add the detail panel HTML**

Inside `vrControlsRoot`, after the closing `</a-entity>` of `vrFiltersPanel` (Task 7), add:

```html
<a-entity id="vrDetailPanel" position="0 1.4 -1.5" rotation="0 0 0" visible="false">
  <a-plane width="2.4" height="1.6" color="#000" material="opacity:0.95"></a-plane>
  <a-text id="vrDetailTitle" value="" align="left" color="#fff" width="3" position="-1.10 0.65 0.01" wrap-count="42"></a-text>
  <a-entity class="vr-btn" data-action="detail-close" position="1.08 0.68 0.01"
            geometry="primitive:plane;width:0.18;height:0.14"
            material="color:#a01010;opacity:0.95">
    <a-text value="✕" align="center" color="#fff" width="3.5" position="0 0 0.005"></a-text>
  </a-entity>

  <!-- Loading sentinel — visible while fetch is in flight. -->
  <a-text id="vrDetailLoading" value="Loading…" align="center" color="#888" width="3"
          position="0 0 0.01" visible="false"></a-text>

  <!-- Meta row: performers, studio, date, duration, rating. JS populates. -->
  <a-entity id="vrDetailMeta" position="0 0.40 0.01"></a-entity>

  <!-- Description body. Multi-line, scrollable when overflowing. -->
  <a-entity id="vrDetailBody" position="-1.10 0.05 0.01">
    <a-text id="vrDetailDescription" value="" align="left" color="#ddd" width="3.5"
            wrap-count="68"></a-text>
  </a-entity>

  <!-- Tag chips row at bottom of body. JS populates. -->
  <a-entity id="vrDetailTags" position="0 -0.40 0.01"></a-entity>

  <!-- Primary action: Play this scene. -->
  <a-entity class="vr-btn" data-action="detail-play" id="vrDetailPlayBtn"
            position="0 -0.65 0.01"
            geometry="primitive:plane;width:1.2;height:0.16"
            material="color:#3776c2;opacity:0.95">
    <a-text value="▶ Play this scene" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
  </a-entity>
</a-entity>
```

- [ ] **Step 2: Add detail-panel state + open/close + chip handlers**

In the IIFE, add detail panel state and the open/close functions:

```javascript
m4cState.detailSceneId = '';
m4cState.detailMeta = null;
m4cState.detailScrollY = 0;
const DETAIL_DESC_VISIBLE_H = 0.30; // visible height in panel space

function openDetailPanel(sceneId) {
  if (!sceneId) return;
  m4cState.detailSceneId = sceneId;
  m4cState.detailMeta = null;
  m4cState.detailScrollY = 0;
  m4cState.lastScrollFocus = 'detail';

  const panel = document.getElementById('vrDetailPanel');
  const loading = document.getElementById('vrDetailLoading');
  const title = document.getElementById('vrDetailTitle');
  const desc = document.getElementById('vrDetailDescription');
  const meta = document.getElementById('vrDetailMeta');
  const tags = document.getElementById('vrDetailTags');
  if (!panel) return;
  panel.setAttribute('visible', true);
  if (title) title.setAttribute('value', '');
  if (desc) desc.setAttribute('value', '');
  if (meta) while (meta.firstChild) meta.removeChild(meta.firstChild);
  if (tags) while (tags.firstChild) tags.removeChild(tags.firstChild);
  if (loading) loading.setAttribute('visible', true);

  fetch('/browse/scene/' + encodeURIComponent(sceneId) + '/meta', {
    headers: { 'Accept': 'application/json' }
  })
    .then(r => r.json())
    .then(data => {
      if (m4cState.detailSceneId !== sceneId) return; // user switched
      m4cState.detailMeta = data;
      renderDetailPanel(data);
    })
    .catch(err => {
      console.warn('stash-vr: detail meta fetch failed', err);
      if (loading) loading.setAttribute('value', 'fetch failed');
    });
}

function closeDetailPanel() {
  m4cState.detailSceneId = '';
  m4cState.detailMeta = null;
  document.getElementById('vrDetailPanel').setAttribute('visible', false);
}

function formatDurationSec(s) {
  if (!isFinite(s) || s <= 0) return '';
  const total = Math.floor(s);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const sec = total % 60;
  if (h > 0) return h + ':' + String(m).padStart(2, '0') + ':' + String(sec).padStart(2, '0');
  return m + ':' + String(sec).padStart(2, '0');
}

function renderDetailPanel(data) {
  const loading = document.getElementById('vrDetailLoading');
  const title = document.getElementById('vrDetailTitle');
  const desc = document.getElementById('vrDetailDescription');
  const meta = document.getElementById('vrDetailMeta');
  const tags = document.getElementById('vrDetailTags');

  if (loading) loading.setAttribute('visible', false);
  if (title) title.setAttribute('value', data.title || '');
  if (desc) desc.setAttribute('value', data.description || '');

  // Meta row: performer chips + studio chip + date/duration/rating text.
  let xOffset = -1.10;
  (data.performers || []).forEach(p => {
    const chip = document.createElement('a-entity');
    chip.classList.add('vr-btn');
    chip.dataset.action = 'detail-chip-perf';
    chip.dataset.entityId = p.id;
    chip.dataset.entityName = p.name;
    chip.setAttribute('geometry', 'primitive:plane;width:0.55;height:0.10');
    chip.setAttribute('material', 'color:#2a2a2a;opacity:0.95');
    chip.setAttribute('position', xOffset.toFixed(2) + ' 0.05 0');
    const text = document.createElement('a-text');
    text.setAttribute('value', p.name);
    text.setAttribute('align', 'center');
    text.setAttribute('color', '#fff');
    text.setAttribute('width', '2.5');
    text.setAttribute('position', '0 0 0.005');
    chip.appendChild(text);
    meta.appendChild(chip);
    xOffset += 0.60;
    if (xOffset > 1.10) { xOffset = -1.10; }
  });
  if (data.studio) {
    const chip = document.createElement('a-entity');
    chip.classList.add('vr-btn');
    chip.dataset.action = 'detail-chip-studio';
    chip.dataset.entityId = data.studio.id;
    chip.dataset.entityName = data.studio.name;
    chip.setAttribute('geometry', 'primitive:plane;width:0.55;height:0.10');
    chip.setAttribute('material', 'color:#2a2a3a;opacity:0.95');
    chip.setAttribute('position', xOffset.toFixed(2) + ' 0.05 0');
    const text = document.createElement('a-text');
    text.setAttribute('value', data.studio.name);
    text.setAttribute('align', 'center');
    text.setAttribute('color', '#fff');
    text.setAttribute('width', '2.5');
    text.setAttribute('position', '0 0 0.005');
    chip.appendChild(text);
    meta.appendChild(chip);
  }

  // Date / duration / rating text below the chips.
  const stats = [];
  if (data.date) stats.push(data.date);
  const d = formatDurationSec(data.durationSec || 0);
  if (d) stats.push(d);
  if (data.rating1to5 > 0) stats.push('★'.repeat(data.rating1to5) + '☆'.repeat(5 - data.rating1to5));
  if (stats.length) {
    const statsText = document.createElement('a-text');
    statsText.setAttribute('value', stats.join('   ·   '));
    statsText.setAttribute('align', 'left');
    statsText.setAttribute('color', '#aaa');
    statsText.setAttribute('width', '3');
    statsText.setAttribute('position', '-1.10 -0.10 0');
    meta.appendChild(statsText);
  }

  // Tag chips.
  let tagX = -1.10;
  (data.tags || []).slice(0, 8).forEach(t => {
    const chip = document.createElement('a-entity');
    chip.classList.add('vr-btn');
    chip.dataset.action = 'detail-chip-tag';
    chip.dataset.entityId = t.id;
    chip.dataset.entityName = t.name;
    chip.setAttribute('geometry', 'primitive:plane;width:0.40;height:0.09');
    chip.setAttribute('material', 'color:#2a2a2a;opacity:0.95');
    chip.setAttribute('position', tagX.toFixed(2) + ' 0 0');
    const text = document.createElement('a-text');
    text.setAttribute('value', t.name);
    text.setAttribute('align', 'center');
    text.setAttribute('color', '#fff');
    text.setAttribute('width', '2.5');
    text.setAttribute('position', '0 0 0.005');
    chip.appendChild(text);
    tags.appendChild(chip);
    tagX += 0.45;
    if (tagX > 1.10) tagX = -1.10;
  });
}
```

Wire actions in `vrAction`:

```javascript
} else if (action === 'detail-close') {
  closeDetailPanel();
} else if (action === 'detail-play') {
  // Reuse swapToScene with a synthetic tile element.
  const meta = m4cState.detailMeta;
  if (!meta || !m4cState.detailSceneId) return;
  // Find the original tile to read projection metadata.
  const tileEl = document.querySelector('.vr-tile[data-scene-id="' + CSS.escape(m4cState.detailSceneId) + '"]');
  if (!tileEl) return;
  tileEl.dataset.tileTitle = meta.title || '';
  closeDetailPanel();
  swapToScene(tileEl);
}
```

Wire chip-click delegate (extend the existing chip handler near `clearChip`):

```javascript
document.addEventListener('click', function(evt) {
  let el = evt.target;
  while (el && !(el.dataset && (el.dataset.action === 'detail-chip-perf' || el.dataset.action === 'detail-chip-studio' || el.dataset.action === 'detail-chip-tag'))) el = el.parentElement;
  if (!el) return;
  const id = el.dataset.entityId;
  const name = el.dataset.entityName;
  if (!id) return;
  if (el.dataset.action === 'detail-chip-perf') {
    m4cState.filters.performer = id;
    m4cState.filterNames.performer = name;
    renderColumnList('performer');
  } else if (el.dataset.action === 'detail-chip-studio') {
    m4cState.filters.studio = id;
    m4cState.filterNames.studio = name;
    renderColumnList('studio');
  } else if (el.dataset.action === 'detail-chip-tag') {
    m4cState.filters.tag = id;
    m4cState.filterNames.tag = name;
    renderColumnList('tag');
  }
  closeDetailPanel();
  renderActiveChips();
  fetchGrid(true);
});
```

- [ ] **Step 3: Description scrolling via thumbstick Y**

Extend the `m3c:browse-scroll` listener (added in Task 5, modified in Task 7) to also handle `lastScrollFocus === 'detail'`:

```javascript
scene.addEventListener('m3c:browse-scroll', function(e) {
  const bp = document.getElementById('vrBrowsePanel');
  if (!bp) return;
  const browseOpen = bp.getAttribute('visible') === true || bp.getAttribute('visible') === 'true';
  if (!browseOpen) return;

  const focus = m4cState.lastScrollFocus;
  if (focus === 'grid') {
    applyScroll(e.detail.deltaSec, e.detail.stickY);
  } else if (focus === 'list-performer' || focus === 'list-studio' || focus === 'list-tag') {
    applyListScroll(focus.slice(5), e.detail.deltaSec, e.detail.stickY);
  } else if (focus === 'detail') {
    applyDetailScroll(e.detail.deltaSec, e.detail.stickY);
  }
});

function applyDetailScroll(deltaSec, stickY) {
  const dy = -stickY * SCROLL_RATE * deltaSec;
  m4cState.detailScrollY = Math.max(0, m4cState.detailScrollY + dy);
  const body = document.getElementById('vrDetailBody');
  if (body) body.setAttribute('position', '-1.10 ' + (0.05 + m4cState.detailScrollY).toFixed(3) + ' 0.01');
}
```

Set `lastScrollFocus = 'detail'` whenever user taps inside the detail panel:

```javascript
document.getElementById('vrDetailPanel').addEventListener('click', function() {
  m4cState.lastScrollFocus = 'detail';
});
```

- [ ] **Step 4: Cascade close when browse panel closes**

Update `vrAction`'s `'browse-close'` branch (originally set in Task 6, extended in Task 7) to also close the detail panel:

```javascript
} else if (action === 'browse-close') {
  document.getElementById('vrBrowsePanel').setAttribute('visible', false);
  document.getElementById('vrFiltersPanel').setAttribute('visible', false);
  closeDetailPanel();
  hideSearchOverlay();
}
```

- [ ] **Step 5: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build, run on Quest 3. Open a scene with a description, performers, studio, tags. Enter VR, summon panel, click Browse. Verify:

- Each tile shows ⓘ badge in the top-right corner of the cover.
- Tap the cover (NOT ⓘ) → seamless scene swap (Play). Detail panel does not open.
- Tap ⓘ → detail panel opens with "Loading…" briefly, then displays:
  - Title at top.
  - Performer chip(s) + studio chip in the meta row.
  - Date · duration · rating below the chips.
  - Description text in the body.
  - Tag chips at the bottom.
  - "▶ Play this scene" button.
- Tap a performer chip → detail panel closes; chip "Performer: Alice ✕" appears in filters panel; grid filters.
- Tap a tag chip → detail panel closes; tag filter applies; grid filters.
- Tap a studio chip → studio filter applies; grid filters.
- Tap "▶ Play this scene" → same fade-out / swap / fade-in flow as cover tap.
- Tap ✕ on detail panel → panel closes; grid stays.
- For a long description: push thumbstick Y after tapping inside detail panel → description scrolls vertically.
- After tapping a grid tile then a list, scroll target switches accordingly.
- Tap ✕ on browse panel → all panels (browse + filters + detail) close together.

- [ ] **Step 6: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "m4c: tile detail badge + standalone detail panel with chip-click filters"
```

---

## Task 10: RF52 canting math (rendering correctness)

**Files:**
- Modify: `internal/api/internal/projection.go`
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Pre-rotate the fisheye sampling direction by ±cant per eye for RF52 sources, fixing the small stereo error from M3a's punted canting.

This task is a rendering-correctness fix orthogonal to the in-VR search work; bundled in M4c at the user's direction so it ships in the same release window. Touches the fisheye shader and the `Projection.Detect` mapping; no new UI.

- [ ] **Step 1: Add `Cant` field to `Projection`**

In [internal/api/internal/projection.go](../../../internal/api/internal/projection.go), find the `Projection` struct:

```go
type Projection struct {
	Geometry string
	FOV      int
	Stereo   string
}
```

Add a `Cant` field (degrees, signed; rotation around Y per eye, positive = right eye outward):

```go
type Projection struct {
	Geometry string
	FOV      int
	Stereo   string
	Cant     float64 // RF52 canted-fisheye angle in degrees (0 for non-RF52)
}
```

In `Detect()`, find the RF52 branch (likely `case hasRF52:` or similar). Set `Cant = 5.0`:

```go
case hasRF52:
	return Projection{Geometry: "fisheye", FOV: 180, Stereo: stereo, Cant: 5.0}
```

Other branches leave `Cant` as zero-value (0.0). Filename-detection RF52 path also sets `Cant = 5.0` — find both occurrences.

- [ ] **Step 2: Emit cant on the fisheye entity in the template**

In [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml), find the `vrFisheye` entity (it currently carries `data-fov`):

```html
<a-entity id="vrFisheye"
          visible="..."
          data-fov="..."
          geometry="..."></a-entity>
```

Add `data-cant`:

```html
<a-entity id="vrFisheye"
          visible="..."
          data-fov="..."
          data-cant="{{.Projection.Cant}}"
          geometry="..."></a-entity>
```

- [ ] **Step 3: Add `uCant` uniform + per-eye rotation in the fisheye shader**

In `applyFisheye`, locate the existing `material` definition with its `uniforms` map. Add a `uCant` uniform:

```javascript
uniforms: {
  uMap:       { value: tex },
  uFOV:       { value: fov },
  uCant:      { value: 0.0 },     // signed radians; set per-eye in onBeforeRender
  uEyeOffset: { value: new AFRAME.THREE.Vector2(0, 0) },
  uEyeRepeat: { value: new AFRAME.THREE.Vector2(1, 1) }
},
```

Update the fragment shader to rotate `d` around Y by `uCant` before computing `theta` and `phi`. Find the existing fragment shader inside `applyFisheye`:

```glsl
void main() {
  vec3 d = normalize(vDir);
  float theta = acos(-d.z);
  float maxTheta = radians(uFOV * 0.5);
  if (theta > maxTheta) discard;
  float r = (theta / maxTheta) * 0.5;
  float phi = atan(d.y, d.x);
  vec2 uv = vec2(0.5 + r * cos(phi), 0.5 + r * sin(phi));
  uv = uv * uEyeRepeat + uEyeOffset;
  gl_FragColor = texture2D(uMap, uv);
}
```

Replace with:

```glsl
uniform float uCant;
void main() {
  vec3 d = normalize(vDir);
  // Pre-rotate d around Y by uCant (signed). For RF52 the call site
  // sets uCant = -cant for left eye, +cant for right eye, in radians.
  float c = cos(uCant);
  float s = sin(uCant);
  vec3 dr = vec3(c * d.x + s * d.z, d.y, -s * d.x + c * d.z);
  float theta = acos(-dr.z);
  float maxTheta = radians(uFOV * 0.5);
  if (theta > maxTheta) discard;
  float r = (theta / maxTheta) * 0.5;
  float phi = atan(dr.y, dr.x);
  vec2 uv = vec2(0.5 + r * cos(phi), 0.5 + r * sin(phi));
  uv = uv * uEyeRepeat + uEyeOffset;
  gl_FragColor = texture2D(uMap, uv);
}
```

(The `uniform float uCant;` declaration goes at the top alongside the existing `uniform` declarations, not inside `main()`.)

- [ ] **Step 4: Set `uCant` per eye in `onBeforeRender`**

In `applyFisheye`, the existing `mesh.onBeforeRender` reads stereo + eye and sets `uEyeOffset` / `uEyeRepeat`. Read the cant from the entity's `data-cant` attribute and apply per eye:

Find the existing handler:

```javascript
mesh.onBeforeRender = function(renderer, sceneObj, cam) {
  const xr = renderer.xr;
  const stereo = scene.dataset.stereo || '';
  const u = material.uniforms;
  if (!xr || !xr.isPresenting || !stereo) {
    u.uEyeOffset.value.set(0, 0);
    u.uEyeRepeat.value.set(1, 1);
    return;
  }
  const xrCam = xr.getCamera();
  if (!xrCam || !xrCam.cameras || xrCam.cameras.length < 2) return;
  const isLeft = cam === xrCam.cameras[0];
  const isRight = cam === xrCam.cameras[1];
  if (!isLeft && !isRight) return;
  if (stereo === 'sbs') { ... }
  else if (stereo === 'tb') { ... }
  else { ... }
};
```

Just before the stereo branches, set `uCant`:

```javascript
const cantDeg = parseFloat(el.dataset.cant || '0');
const cantRad = cantDeg * Math.PI / 180;
u.uCant.value = isLeft ? -cantRad : cantRad;
```

(Outside-of-XR / mono branches set `u.uCant.value = 0`.)

- [ ] **Step 5: Drop cant when user manually picks via Format picker**

The M3b picker re-applies projection state when the user picks a Type/Degree. The picker doesn't preserve cant — when the user picks "FishEye + 180° + SBS," it should be plain fisheye, not RF52-canted. Find `applyPickerState` in the IIFE. After the existing `data-fov` mutation:

```javascript
if (fovEl) fovEl.dataset.fov = (pickerState.degree === '200' ? '200' : '180');
```

Add (right after, in the same block):

```javascript
if (fovEl) fovEl.dataset.cant = '0'; // user-picked fisheye is plain fisheye
```

(Auto-detect's initial render still sees the server-rendered `data-cant` from §4.10a.)

- [ ] **Step 6: Vet, build**

Run: `go vet ./...` then `go build ./...`

Expected: clean.

- [ ] **Step 7: Manual verify on Quest 3**

Open a scene tagged `VR_RF52` (or with `RF52` in its filename). Click Enter VR. Observe stereo separation — should feel more natural than M3a's plain-fisheye fallback. Open a plain `VR_FISHEYE 180°` scene → no behavior change vs M3a (cant is 0).

Spec §8 K covers the full validation matrix:
- RF52 scene → cant active, stereo feels correct.
- Plain FISHEYE 180° → no behavior change.
- RF52 → manually pick FishEye + 180° + SBS in Format picker → cant resets to 0; reload → auto-detect restores cant.

- [ ] **Step 8: Commit**

```
git add internal/api/internal/projection.go internal/static/browse_scene.gohtml
git commit -m "m4c: RF52 canting math — per-eye outward rotation in fisheye shader"
```

---

## Self-review checklist

- **Spec coverage:**
  - Browse panel toggle (Task 3)
  - Configurable cols (Task 3)
  - Tile rendering on cylinder + ⓘ detail badge (Task 4)
  - Vertical scroll + lazy load (Task 5)
  - Search via DOM overlay (Tasks 1 + 6)
  - 3-column filters panel with searchable lists (Task 7)
  - Seamless scene swap + rich /scene/{id}/meta endpoint (Task 8)
  - Standalone detail panel with chip-click filters (Task 9)
  - RF52 canting math (Task 10)
- **No placeholders:** every code block is concrete. The M3c integration in Task 5 is described conceptually because it depends on M3c's actual handler structure — flagged in the step.
- **Type consistency:** `GridTile`, `GridResponse`, `FilterOption`, `CaptionRef`, `SceneMarker`, `SceneMeta`, `EntityRef` all defined and reused consistently. JS state shape `m4cState.filters` matches server query params. The new `Projection.Cant` field flows through `Detect()` → template → `data-cant` → `uCant` uniform.
- **Frequent commits:** one per task. Ten commits total.
- **Risks acknowledged:** Task 1 is a feasibility gate — if it fails, the rest of the plan needs revision. Tasks 4–9 each have a manual verification step on the headset, since these touch novel WebXR territory. Task 10's cant sign convention may need flipping after on-headset validation — fix is one negation in step 4.
- **YAGNI:** no auto-next, no scene previews, no multi-select, no saved filters, no sort options. All explicit deferrals from the spec.
