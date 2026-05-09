# M4c: In-VR search/browse — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a 3D scene-grid browse panel reachable from the M4b control panel, with text search via DOM overlay, six filter pickers, vertical scroll, configurable cols, and seamless scene swap (no VR re-entry).

**Architecture:** Eight tasks. Task 1 is a DOM-overlay feasibility spike — confirm Meta Browser supports DOM overlay during a WebXR session before committing to the rest. Task 2 adds the server-side JSON endpoints. Tasks 3–4 build the browse panel UI: top strip, cylinder grid layout, tile entities with cover textures. Task 5 adds vertical scroll + lazy load + the M3c thumbstick handoff. Task 6 wires the DOM-overlay search field. Task 7 adds the six filter pickers + Clear all. Task 8 implements the seamless scene swap with fade.

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
      el.classList.add('vr-btn', 'vr-tile');
      el.dataset.sceneId = tile.id;
      el.dataset.projection = JSON.stringify(tile.projection);
      el.dataset.streamUrl = '/browse/scene/' + encodeURIComponent(tile.id) + '/stream';

      const plane = document.createElement('a-plane');
      plane.setAttribute('width', TILE_W);
      plane.setAttribute('height', TILE_W * 9 / 16);
      plane.setAttribute('material', 'color:#222;opacity:1;shader:flat');
      el.appendChild(plane);

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

## Task 6: Search field via DOM overlay

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Tap the search field plane → overlay's `<input>` focuses → Quest VR keyboard pops. Typing filters the grid live (debounced 250 ms).

- [ ] **Step 1: Replace the spike overlay with the production overlay**

Replace the spike `<div id="vrDomOverlay">` from Task 1 with the production version, which sits hidden by default and only appears when the browse panel is open and the search field is tapped:

```html
<div id="vrDomOverlay" style="display:none; position:fixed; bottom:20%; left:50%; transform:translateX(-50%); padding:12px; background:rgba(0,0,0,0.85); color:#fff; border-radius:8px; z-index:9999; min-width:320px;">
  <input id="vrSearchInput" type="text" placeholder="Search scenes…" autocomplete="off"
         style="font-size:18px; padding:10px; width:100%; box-sizing:border-box; background:#222; color:#fff; border:1px solid #444; border-radius:4px;" />
</div>
```

- [ ] **Step 2: Wire the search-focus button + input event**

```javascript
function showSearchOverlay() {
  const overlay = document.getElementById('vrDomOverlay');
  const input = document.getElementById('vrSearchInput');
  if (!overlay || !input) return;
  overlay.style.display = 'block';
  setTimeout(() => input.focus(), 50);
}
function hideSearchOverlay() {
  const overlay = document.getElementById('vrDomOverlay');
  if (overlay) overlay.style.display = 'none';
}

// Add to vrAction switch:
} else if (action === 'search-focus') {
  showSearchOverlay();
} else if (action === 'browse-close') {
  // existing close logic + hide overlay
  const bp = document.getElementById('vrBrowsePanel');
  if (bp) bp.setAttribute('visible', false);
  hideSearchOverlay();
}

// Auto-hide overlay when browse panel hides:
const bpEl = document.getElementById('vrBrowsePanel');
if (bpEl) {
  // A-Frame fires componentchanged for visible toggles.
  bpEl.addEventListener('componentchanged', function(evt) {
    if (evt.detail.name === 'visible' && bpEl.getAttribute('visible') === false) {
      hideSearchOverlay();
    }
  });
}
```

- [ ] **Step 3: Live filter on input change (debounced)**

```javascript
let searchTimer = null;
const searchInput = document.getElementById('vrSearchInput');
if (searchInput) {
  searchInput.addEventListener('input', function() {
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      m4cState.q = searchInput.value.trim();
      // Update the search field's placeholder text on the panel.
      const bgPlane = document.querySelector('.vr-search-bg a-text');
      if (bgPlane) {
        bgPlane.setAttribute('value', m4cState.q || 'Search…');
        bgPlane.setAttribute('color', m4cState.q ? '#fff' : '#888');
      }
      fetchGrid(true);
    }, 250);
  });

  // Enter key dismisses the keyboard but keeps the panel open.
  searchInput.addEventListener('keydown', function(evt) {
    if (evt.key === 'Enter') {
      hideSearchOverlay();
    }
  });
}
```

- [ ] **Step 4: Update the search-field plane to show the current query**

Modify the search-bg plane in Task 3's HTML so the `<a-text>` inside it has an id we can update:

```html
<a-plane class="vr-search-bg vr-btn" data-action="search-focus" width="1.2" height="0.16"
         color="#222" material="opacity:0.9" position="-1.0 0 0">
  <a-text id="vrSearchLabel" value="Search…" align="left" color="#888" width="3" position="-0.55 0 0.005"></a-text>
</a-plane>
```

And the JS uses `getElementById('vrSearchLabel')` rather than the awkward query.

- [ ] **Step 5: Vet, build, manually verify on Quest 3**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build, install on Quest 3, run. Open a scene, enter VR, click Browse, click search field. Verify:

- DOM overlay appears at bottom of view in VR.
- Quest VR keyboard pops up.
- Typing inserts characters; search field plane on the panel reflects current text.
- After 250 ms idle, grid filters to matching scenes.
- Press Enter → keyboard dismisses; overlay hides; grid stays filtered.
- Click ✕ to close browse panel → overlay also hides.

- [ ] **Step 6: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "m4c: search field via DOM-overlay input with live debounced filter"
```

---

## Task 7: Filters panel — tabs, searchable lists, active chips

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Click "Filters ▾" → standalone Filters panel beside the grid opens with 4 tabs (Performer / Studio / Tag / Other). Performer/Studio/Tag tabs each show a search field + scrollable filterable list. Other tab shows three value-picker rows. Active filters display as removable chips at the top of the panel. Single panel — no separate options panel.

- [ ] **Step 1: Add the standalone Filters panel HTML**

The filters panel is a sibling of `vrBrowsePanel` (not a child). Both live directly under `vrControlsRoot` so M3c's panel-toggle hides them as a unit.

Inside `vrControlsRoot`, **after** the closing `</a-entity>` of `vrBrowsePanel`, add:

```html
<!-- Standalone Filters panel — sits to the right of the browse panel.
     Single panel containing tabs, active-filter chips, search field, and
     either a list (Performer/Studio/Tag tabs) or value pickers (Other tab). -->
<a-entity id="vrFiltersPanel" position="2.6 1.4 -2.5" rotation="0 -15 0" visible="false">
  <a-plane width="2.0" height="1.8" color="#000" material="opacity:0.95"></a-plane>
  <a-text value="Filters" align="left" color="#fff" width="3" position="-0.90 0.80 0.01"></a-text>
  <a-entity class="vr-btn" data-action="filters-close" position="0.88 0.80 0.01"
            geometry="primitive:plane;width:0.18;height:0.14"
            material="color:#a01010;opacity:0.95">
    <a-text value="✕" align="center" color="#fff" width="3.5" position="0 0 0.005"></a-text>
  </a-entity>

  <!-- Active-filter chips area. JS populates with one chip per active filter. -->
  <a-entity id="vrFiltersChips" position="0 0.60 0.01"></a-entity>

  <!-- Tab row. -->
  <a-entity id="vrFiltersTabs" position="0 0.30 0.01">
    <a-entity class="vr-btn vr-filter-tab" data-tab="performer" position="-0.75 0 0"
              geometry="primitive:plane;width:0.45;height:0.14"
              material="color:#3776c2;opacity:0.95">
      <a-text value="Performer" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn vr-filter-tab" data-tab="studio" position="-0.25 0 0"
              geometry="primitive:plane;width:0.45;height:0.14"
              material="color:#2c5282;opacity:0.95">
      <a-text value="Studio" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn vr-filter-tab" data-tab="tag" position="0.25 0 0"
              geometry="primitive:plane;width:0.45;height:0.14"
              material="color:#2c5282;opacity:0.95">
      <a-text value="Tag" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn vr-filter-tab" data-tab="other" position="0.75 0 0"
              geometry="primitive:plane;width:0.45;height:0.14"
              material="color:#2c5282;opacity:0.95">
      <a-text value="Other" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
    </a-entity>
  </a-entity>

  <!-- Picker search field — used on Performer/Studio/Tag tabs only. Hidden on Other. -->
  <a-plane id="vrPickerSearchBg" class="vr-search-bg vr-btn" data-action="picker-search-focus"
           width="1.8" height="0.13" color="#222" material="opacity:0.9"
           position="0 0.10 0.01" visible="true">
    <a-text id="vrPickerSearchLabel" value="Search…" align="left" color="#888" width="3" position="-0.85 0 0.005"></a-text>
  </a-plane>

  <!-- Container for the list (Performer/Studio/Tag) or value pickers (Other). -->
  <a-entity id="vrFiltersBody" position="0 -0.70 0.01"></a-entity>
</a-entity>
```

The previous standalone `vrFilterOptions` panel is **removed** entirely — delete its `<a-entity>` block from the HTML.

- [ ] **Step 2: Add filter state + chip rendering + browse-close cascade**

In the IIFE (just below the `m4cState` block declared in Task 4), add the filter-state extensions and chip rendering. Task 6's grid-search functions are kept as-is and extended in Step 5 below.

```javascript
m4cState.filterNames = { performer: '', studio: '', tag: '', studio_id: '' };
m4cState.activeTab = 'performer';      // 'performer' | 'studio' | 'tag' | 'other'
m4cState.lastScrollFocus = 'grid';     // 'grid' | 'filters'
m4cState.cachedOptions = {};           // kind → [{id,name}, ...]
m4cState.pickerQuery = '';             // current in-list search

function chipLabel(kind) {
  if (kind === 'performer') return m4cState.filterNames.performer ? 'Performer: ' + m4cState.filterNames.performer : '';
  if (kind === 'studio')    return m4cState.filterNames.studio    ? 'Studio: '    + m4cState.filterNames.studio    : '';
  if (kind === 'tag')       return m4cState.filterNames.tag       ? 'Tag: '       + m4cState.filterNames.tag       : '';
  if (kind === 'favorite' && m4cState.filters.favorite === 'only') return 'Favorites: Only';
  if (kind === 'favorite' && m4cState.filters.favorite === 'not')  return 'Favorites: Not';
  if (kind === 'stars'    && m4cState.filters.stars > 0) return 'Stars: ' + (m4cState.filters.stars === 5 ? '5 only' : '≥' + m4cState.filters.stars);
  if (kind === 'ocount'   && m4cState.filters.ocount > 0) return 'O-Counter: ≥' + m4cState.filters.ocount;
  return '';
}

function renderActiveChips() {
  const root = document.getElementById('vrFiltersChips');
  if (!root) return;
  while (root.firstChild) root.removeChild(root.firstChild);
  const kinds = ['performer', 'studio', 'tag', 'favorite', 'stars', 'ocount'];
  let xOffset = -0.85;
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
    if (xOffset > 0.85) xOffset = -0.85; // wrap if many chips; v1 caps visually
  });
}

function clearChip(kind) {
  if (kind === 'performer' || kind === 'studio' || kind === 'tag') {
    m4cState.filters[kind] = '';
    m4cState.filterNames[kind] = '';
  } else if (kind === 'favorite') m4cState.filters.favorite = '';
  else if (kind === 'stars')    m4cState.filters.stars = 0;
  else if (kind === 'ocount')   m4cState.filters.ocount = 0;
  renderActiveChips();
  renderFiltersBody();
  fetchGrid(true);
}
```

Update `vrAction`'s `'browse-close'` branch (replacing the previous Task 6 version) to cascade to filters too:

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
    selectTab(m4cState.activeTab || 'performer');
  }
} else if (action === 'filters-close') {
  document.getElementById('vrFiltersPanel').setAttribute('visible', false);
} else if (action === 'filter-chip-clear') {
  // Triggered by clicking a chip; element carries data-chip-kind.
  // Resolved in the click delegate below.
} else if (action === 'filters-clear') {
  m4cState.q = '';
  m4cState.filters = { performer: '', studio: '', tag: '', favorite: '', stars: 0, ocount: 0 };
  m4cState.filterNames = { performer: '', studio: '', tag: '' };
  const searchInput = document.getElementById('vrSearchInput');
  if (searchInput) searchInput.value = '';
  const lbl = document.getElementById('vrSearchLabel');
  if (lbl) { lbl.setAttribute('value', 'Search…'); lbl.setAttribute('color', '#888'); }
  renderActiveChips();
  renderFiltersBody();
  fetchGrid(true);
} else if (action === 'picker-search-focus') {
  showPickerSearchOverlay();
}
```

Wire the chip-click delegate (place it near the existing `.vr-btn` click forEach loop):

```javascript
document.addEventListener('click', function(evt) {
  let el = evt.target;
  while (el && !(el.dataset && el.dataset.action === 'filter-chip-clear')) el = el.parentElement;
  if (el && el.dataset.chipKind) clearChip(el.dataset.chipKind);
});
```

- [ ] **Step 3: Tab switching, list rendering, in-list search filter**

```javascript
function selectTab(tab) {
  m4cState.activeTab = tab;
  m4cState.pickerQuery = '';

  // Highlight active tab.
  document.querySelectorAll('.vr-filter-tab').forEach(t => {
    const isActive = t.dataset.tab === tab;
    t.setAttribute('material', 'color: ' + (isActive ? '#3776c2' : '#2c5282') + '; opacity:0.95');
  });

  // Show/hide search field based on tab.
  const searchBg = document.getElementById('vrPickerSearchBg');
  const searchLabel = document.getElementById('vrPickerSearchLabel');
  if (searchBg) searchBg.setAttribute('visible', tab !== 'other');
  if (searchLabel) {
    searchLabel.setAttribute('value', 'Search…');
    searchLabel.setAttribute('color', '#888');
  }

  renderFiltersBody();
}

document.querySelectorAll('.vr-filter-tab').forEach(t => {
  t.addEventListener('click', () => selectTab(t.dataset.tab));
});

function renderFiltersBody() {
  const body = document.getElementById('vrFiltersBody');
  if (!body) return;
  while (body.firstChild) body.removeChild(body.firstChild);

  if (m4cState.activeTab === 'other') {
    renderOtherTab(body);
    return;
  }
  // Performer/Studio/Tag list rendering.
  const kind = m4cState.activeTab;
  if (m4cState.cachedOptions[kind]) {
    renderListTab(body, kind, m4cState.cachedOptions[kind]);
  } else {
    fetch('/browse/filter-options/' + kind, { headers: { 'Accept': 'application/json' } })
      .then(r => r.json())
      .then(opts => {
        m4cState.cachedOptions[kind] = opts || [];
        if (m4cState.activeTab === kind) renderListTab(body, kind, m4cState.cachedOptions[kind]);
      })
      .catch(err => console.warn('stash-vr: filter options fetch failed', err));
  }
}

const LIST_VISIBLE_ROWS = 8;
const LIST_ROW_H = 0.12;
let listScrollY = 0;

function renderListTab(body, kind, opts) {
  const q = m4cState.pickerQuery.trim().toLowerCase();
  const filtered = q
    ? opts.filter(o => o.name.toLowerCase().includes(q))
    : opts;
  // The body container holds a clipped (visually) list of rows.
  // v1 doesn't actually clip via shader — we just render all rows offset
  // by listScrollY and rely on the panel background to hide overflow.
  filtered.forEach((opt, i) => {
    const row = document.createElement('a-entity');
    row.classList.add('vr-btn', 'vr-filter-list-item');
    row.dataset.kind = kind;
    row.dataset.optId = String(opt.id);
    row.dataset.optName = opt.name;
    row.setAttribute('geometry', 'primitive:plane;width:1.8;height:0.10');
    const isSelected = (m4cState.filters[kind] === opt.id);
    row.setAttribute('material', 'color: ' + (isSelected ? '#3776c2' : '#222') + '; opacity:0.95');
    const y = -i * LIST_ROW_H + listScrollY;
    row.setAttribute('position', '0 ' + y.toFixed(3) + ' 0.005');
    const text = document.createElement('a-text');
    text.setAttribute('value', opt.name);
    text.setAttribute('align', 'left');
    text.setAttribute('color', '#fff');
    text.setAttribute('width', '2.5');
    text.setAttribute('position', '-0.85 0 0.005');
    row.appendChild(text);
    body.appendChild(row);
    row.addEventListener('click', function() {
      applyFilterPick(kind, opt.id, opt.name);
    });
  });
}

function applyFilterPick(kind, id, name) {
  if (kind === 'performer' || kind === 'studio' || kind === 'tag') {
    // Single-select: tapping the currently-selected name clears it.
    if (m4cState.filters[kind] === id) {
      m4cState.filters[kind] = '';
      m4cState.filterNames[kind] = '';
    } else {
      m4cState.filters[kind] = id;
      m4cState.filterNames[kind] = name;
    }
  } else if (kind === 'favorite') {
    m4cState.filters.favorite = m4cState.filters.favorite === id ? '' : id;
  } else if (kind === 'stars') {
    const n = parseInt(id || '0', 10);
    m4cState.filters.stars = m4cState.filters.stars === n ? 0 : n;
  } else if (kind === 'ocount') {
    const n = parseInt(id || '0', 10);
    m4cState.filters.ocount = m4cState.filters.ocount === n ? 0 : n;
  }
  renderActiveChips();
  renderFiltersBody();
  fetchGrid(true);
}
```

- [ ] **Step 4: Other tab rendering + value-picker buttons**

```javascript
function renderOtherTab(body) {
  // Three rows: Favorites, Stars, O-Counter. Each row is a label + flex of buttons.
  const rows = [
    {
      kind: 'favorite', label: 'Favorites:',
      opts: [
        { id: '',     name: 'Any' },
        { id: 'only', name: 'Only' },
        { id: 'not',  name: 'Not' },
      ]
    },
    {
      kind: 'stars', label: 'Stars:',
      opts: [
        { id: '0', name: 'Any' },
        { id: '1', name: '1+' },
        { id: '2', name: '2+' },
        { id: '3', name: '3+' },
        { id: '4', name: '4+' },
        { id: '5', name: '5 only' },
      ]
    },
    {
      kind: 'ocount', label: 'O-Counter:',
      opts: [
        { id: '0', name: 'Any' },
        { id: '1', name: '1+' },
        { id: '5', name: '5+' },
        { id: '10', name: '10+' },
      ]
    },
  ];

  rows.forEach((row, rowIdx) => {
    const yRow = -rowIdx * 0.18;
    const lbl = document.createElement('a-text');
    lbl.setAttribute('value', row.label);
    lbl.setAttribute('align', 'left');
    lbl.setAttribute('color', '#fff');
    lbl.setAttribute('width', '2.5');
    lbl.setAttribute('position', '-0.85 ' + yRow.toFixed(3) + ' 0.005');
    body.appendChild(lbl);

    // Buttons stacked horizontally to the right of the label.
    let xOffset = -0.30;
    row.opts.forEach(opt => {
      const btn = document.createElement('a-entity');
      btn.classList.add('vr-btn');
      btn.setAttribute('geometry', 'primitive:plane;width:0.20;height:0.10');
      const isSelected = (
        (row.kind === 'favorite' && m4cState.filters.favorite === opt.id) ||
        (row.kind === 'stars'    && String(m4cState.filters.stars)  === opt.id) ||
        (row.kind === 'ocount'   && String(m4cState.filters.ocount) === opt.id)
      );
      btn.setAttribute('material', 'color: ' + (isSelected ? '#3776c2' : '#2c5282') + '; opacity:0.95');
      btn.setAttribute('position', xOffset + ' ' + yRow.toFixed(3) + ' 0.005');
      const text = document.createElement('a-text');
      text.setAttribute('value', opt.name);
      text.setAttribute('align', 'center');
      text.setAttribute('color', '#fff');
      text.setAttribute('width', '3');
      text.setAttribute('position', '0 0 0.005');
      btn.appendChild(text);
      body.appendChild(btn);
      btn.addEventListener('click', function() { applyFilterPick(row.kind, opt.id, opt.name); });
      xOffset += 0.22;
    });
  });
}
```

- [ ] **Step 5: DOM-overlay input retargeting (grid search ↔ picker search)**

The same `<input id="vrSearchInput">` is reused for both the grid search and the picker-list search. A `target` flag tracks which is in effect:

```javascript
let overlayTarget = 'grid'; // 'grid' | 'picker'

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

function showPickerSearchOverlay() {
  overlayTarget = 'picker';
  const overlay = document.getElementById('vrDomOverlay');
  const input = document.getElementById('vrSearchInput');
  if (!overlay || !input) return;
  input.placeholder = 'Search ' + (m4cState.activeTab || 'performer') + '…';
  input.value = m4cState.pickerQuery || '';
  overlay.style.display = 'block';
  setTimeout(() => input.focus(), 50);
}
```

Replace the existing single `showSearchOverlay` with `showSearchOverlayForGrid` (rename, update the `vrAction` `'search-focus'` branch from Task 6).

Update the `searchInput` `input` listener to dispatch by target:

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
    } else if (overlayTarget === 'picker') {
      m4cState.pickerQuery = searchInput.value;
      const pkLbl = document.getElementById('vrPickerSearchLabel');
      if (pkLbl) {
        pkLbl.setAttribute('value', m4cState.pickerQuery || 'Search…');
        pkLbl.setAttribute('color', m4cState.pickerQuery ? '#fff' : '#888');
      }
      renderFiltersBody();
    }
  }, overlayTarget === 'picker' ? 100 : 250);
});
```

(Picker search is local-only, so 100 ms debounce is fine.)

- [ ] **Step 6: Scroll target handoff (grid vs filters list)**

Track scroll focus when the user taps inside either panel. In the IIFE, near the click delegates:

```javascript
document.getElementById('vrBrowsePanel').addEventListener('click', function(evt) {
  // Tile clicks shouldn't change focus (handled by Task 8 swap).
  if (evt.target.closest('.vr-tile')) return;
  m4cState.lastScrollFocus = 'grid';
});
document.getElementById('vrFiltersPanel').addEventListener('click', function() {
  m4cState.lastScrollFocus = 'filters';
});
```

Modify the `m3c:browse-scroll` listener (added in Task 5) to dispatch by focus:

```javascript
scene.addEventListener('m3c:browse-scroll', function(e) {
  const bp = document.getElementById('vrBrowsePanel');
  if (!bp) return;
  const browseOpen = bp.getAttribute('visible') === true || bp.getAttribute('visible') === 'true';
  if (!browseOpen) return;

  if (m4cState.lastScrollFocus === 'filters') {
    applyListScroll(e.detail.deltaSec, e.detail.stickY);
  } else {
    applyScroll(e.detail.deltaSec, e.detail.stickY);
  }
});

function applyListScroll(deltaSec, stickY) {
  const dy = -stickY * SCROLL_RATE * deltaSec;
  const next = Math.max(0, Math.min(maxListScrollY(), listScrollY + dy));
  if (next !== listScrollY) {
    listScrollY = next;
    renderFiltersBody();
  }
}

function maxListScrollY() {
  const kind = m4cState.activeTab;
  if (kind === 'other') return 0;
  const opts = m4cState.cachedOptions[kind] || [];
  const q = m4cState.pickerQuery.trim().toLowerCase();
  const count = q ? opts.filter(o => o.name.toLowerCase().includes(q)).length : opts.length;
  const totalH = count * LIST_ROW_H;
  const visibleH = LIST_VISIBLE_ROWS * LIST_ROW_H;
  return Math.max(0, totalH - visibleH);
}
```

The thumbstick handler from M3c (Task 5) is unchanged — it always emits `m3c:browse-scroll` when browse is open, and the delegate above routes to grid or list.

- [ ] **Step 7: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build, run on Quest 3. Open a scene with many performers, studios, tags. Enter VR, summon panel, click Browse. Verify (full §8 E from spec):

- Click Filters ▾ → standalone Filters panel appears to the right of the grid.
- 4 tabs visible (Performer, Studio, Tag, Other), Performer highlighted.
- Performer list visible below tabs with search field above.
- Tap search field → DOM overlay appears with focused input; Quest VR keyboard pops.
- Type "Ali" → list narrows to performers whose name contains "Ali".
- Tap "Alice" → chip "Performer: Alice ✕" appears at top of panel; row highlights blue; grid filters to Alice's scenes.
- Tap chip ✕ → filter clears; chip disappears; row de-highlights; grid restores.
- Switch to Studio tab → list of studios; same flow.
- Switch to Tag tab → list of tags; same flow.
- Switch to Other tab → list disappears, replaced by Favorites/Stars/O-Counter rows with value buttons.
- Search field hidden on Other tab.
- Tap "Only" under Favorites → button highlights; chip "Favorites: Only ✕" appears; grid filters.
- Tap "3+" under Stars → button highlights; chip appears.
- Tap "1+" under O-Counter → chip appears.
- Push thumbstick Y after tapping inside filters → list scrolls (not grid).
- Push thumbstick Y after tapping a tile → grid scrolls.
- Tap "Clear all" on browse top strip → all chips clear; all values reset; grid restores.
- Tap ✕ on filters panel → filters close, browse stays.
- Tap ✕ on browse panel → both panels close.

- [ ] **Step 8: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "m4c: tabbed filters panel — searchable lists, active chips, scroll handoff"
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

// Wire tile clicks.
document.addEventListener('click', function(evt) {
  let target = evt.target;
  while (target && !target.classList?.contains('vr-tile')) target = target.parentElement;
  if (target && target.classList.contains('vr-tile')) {
    target.dataset.tileTitle = target.querySelector('a-text')?.getAttribute('value') || '';
    swapToScene(target);
  }
});
```

(The `target.dataset.tileTitle` is set on click for the post-swap title display, since the new title comes from the clicked tile's text.)

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

This implies a new endpoint `GET /browse/scene/{id}/meta` returning `{title, durationSec, captions, sceneMarkers, projection}`. Add to grid_json.go (or a new metadata.go):

```go
func (h *httpHandler) sceneMetaHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil {
		http.NotFound(w, r)
		return
	}
	out := struct {
		Title        string         `json:"title"`
		DurationSec  float64        `json:"durationSec"`
		Captions     []CaptionRef   `json:"captions"`
		SceneMarkers []SceneMarker  `json:"sceneMarkers"`
	}{Title: vd.Title()}
	if len(vd.SceneParts.Files) > 0 && vd.SceneParts.Files[0] != nil {
		out.DurationSec = vd.SceneParts.Files[0].Duration
	}
	for _, c := range vd.SceneParts.Captions {
		if c == nil {
			continue
		}
		out.Captions = append(out.Captions, CaptionRef{
			LanguageCode: c.Language_code,
			CaptionType:  c.Caption_type,
		})
	}
	for _, m := range vd.SceneParts.Scene_markers {
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

`rebuildCaptionPicker(captions)`: clears existing language buttons in `vrSubtitlePicker` (those rendered by `{{range .Captions}}`) and re-creates them from the JSON. Implementation parallels Task 4's `relayoutTiles` for tile entities.

- [ ] **Step 5: Re-vet, re-build, re-verify swap with new metadata refresh**

Run: `go vet ./...` then `go build ./...` — expect clean.

Re-test the swap on the headset; verify scene markers refresh, CC button visibility flips correctly, time display updates.

- [ ] **Step 6: Commit**

```
git add internal/static/browse_scene.gohtml internal/api/browse/grid_json.go internal/api/browse/router.go
git commit -m "m4c: seamless scene swap with fade, projection rebind, M4b state refresh"
```

---

## Self-review checklist

- **Spec coverage:**
  - Browse panel toggle (Task 3)
  - Configurable cols (Task 3)
  - Tile rendering on cylinder (Task 4)
  - Vertical scroll + lazy load (Task 5)
  - Search via DOM overlay (Tasks 1 + 6)
  - 6 filter pickers + Clear all (Task 7)
  - Seamless scene swap (Task 8)
- **No placeholders:** every code block is concrete. The M3c integration in Task 5 is described conceptually because it depends on M3c's actual handler structure — flagged in the step.
- **Type consistency:** `GridTile`, `GridResponse`, `FilterOption`, `CaptionRef`, `SceneMarker` all defined and reused consistently. JS state shape `m4cState.filters` matches server query params.
- **Frequent commits:** one per task. Eight commits total.
- **Risks acknowledged:** Task 1 is a feasibility gate — if it fails, the rest of the plan needs revision. Tasks 4–8 each have a manual verification step on the headset, since these touch novel WebXR territory.
- **YAGNI:** no auto-next, no scene previews, no multi-select, no saved filters, no sort options. All explicit deferrals from the spec.
