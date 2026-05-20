# M4c Intersecting Filter Facets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the in-VR Performer / Studio / Tag columns react to the active filter intersection immediately after one upfront index fetch, while keeping `/browse/grid` as the source of truth for the tile list.

**Architecture:** Add one backend JSON endpoint, `GET /browse/filter-index`, that returns a compact scene-to-entity membership index plus the existing sorted catalogs. In the browser, load that index once when the filters panel opens, compute the current matching scene set locally for performer/studio/tag/stars/o-count, and narrow the three entity columns from the cached catalogs. If the index fetch fails, fall back to the existing `/browse/filter-options/{kind}` behavior.

**Tech Stack:** Go 1.24, chi router, genqlient GraphQL bindings, embedded Go templates with inline browser JavaScript, `go test`, `go build`

---

## File Structure

- `internal/stash/gql/documents/query.graphql`
  - Add the one-shot scene-membership query used to build the facet index.
- `internal/stash/gql/generated.go`
  - Regenerated genqlient bindings for the new query.
- `internal/api/browse/data.go`
  - Add the JSON response types for the facet index payload.
- `internal/api/browse/filter_index.go`
  - New backend handler plus small projection helpers that turn the GraphQL response into the compact payload.
- `internal/api/browse/filter_index_test.go`
  - New Go tests for the backend payload projection helpers.
- `internal/api/browse/router.go`
  - Mount `GET /browse/filter-index`.
- `internal/static/browse_scene.gohtml`
  - Replace the current global-only option loading path with:
    - one-time `filter-index` fetch state
    - loading / failed fallback states
    - local matching-set computation
    - column rerender hooks for performer/studio/tag/stars/o-count changes

### Boundary notes

- Keep the new backend logic in `internal/api/browse/` with the other browse JSON handlers instead of teaching `library.Service` about a new cached concept. This is a view-specific payload, not a domain service.
- Keep the client logic inline in `browse_scene.gohtml`. The repository already keeps the VR browse logic there, so this change should extend that pattern rather than introducing a new JS build/test toolchain just for one follow-up.
- Automated tests in this plan are Go-side. The repository has no JS test harness, so browser behavior is verified through `go build` plus explicit manual VR/desktop checks.

---

### Task 1: Add the compact backend facet index

**Files:**
- Modify: `internal/stash/gql/documents/query.graphql`
- Modify: `internal/stash/gql/generated.go`
- Modify: `internal/api/browse/data.go`
- Create: `internal/api/browse/filter_index.go`
- Create: `internal/api/browse/filter_index_test.go`
- Modify: `internal/api/browse/router.go`

- [ ] **Step 1: Write the failing backend tests**

Create `internal/api/browse/filter_index_test.go` with table-driven coverage for the projection helper that builds the JSON payload from sorted catalogs plus compact scene membership rows.

```go
package browse

import (
	"reflect"
	"testing"
)

func TestBuildFilterIndexPayloadPreservesCatalogOrderAndSelectableTags(t *testing.T) {
	performers := []Entity{
		{ID: "p2", Name: "Bob", SceneCount: 2},
		{ID: "p1", Name: "Alice", SceneCount: 5},
	}
	studios := []Entity{
		{ID: "s1", Name: "Studio One", SceneCount: 3},
	}
	tags := []Entity{
		{ID: "t-visible", Name: "Outdoor", SceneCount: 7},
	}
	scenes := []facetSceneSeed{
		{
			ID:           "scene-1",
			PerformerIDs: []string{"p1", "p2"},
			StudioIDs:    []string{"s1"},
			TagIDs:       []string{"t-visible", "t-hidden", "t-ancestor"},
			Rating100:    80,
			OCount:       2,
		},
	}
	selectableTagIDs := map[string]struct{}{
		"t-visible": {},
	}

	got := buildFilterIndexPayload(performers, studios, tags, scenes, selectableTagIDs)
	want := FilterIndexResponse{
		Performers: []FilterOption{
			{ID: "p2", Name: "Bob"},
			{ID: "p1", Name: "Alice"},
		},
		Studios: []FilterOption{
			{ID: "s1", Name: "Studio One"},
		},
		Tags: []FilterOption{
			{ID: "t-visible", Name: "Outdoor"},
		},
		Scenes: []FilterIndexScene{
			{
				ID:           "scene-1",
				PerformerIDs: []string{"p1", "p2"},
				StudioIDs:    []string{"s1"},
				TagIDs:       []string{"t-visible"},
				Rating100:    80,
				OCount:       2,
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildFilterIndexPayload() = %#v, want %#v", got, want)
	}
}

func TestBuildFilterIndexPayloadKeepsScenesWithoutStudioAndEmptySelectableTags(t *testing.T) {
	got := buildFilterIndexPayload(
		nil,
		nil,
		nil,
		[]facetSceneSeed{{
			ID:           "scene-2",
			PerformerIDs: []string{"p9"},
			StudioIDs:    nil,
			TagIDs:       []string{"t-hidden"},
			Rating100:    100,
			OCount:       0,
		}},
		map[string]struct{}{},
	)

	want := FilterIndexResponse{
		Scenes: []FilterIndexScene{{
			ID:           "scene-2",
			PerformerIDs: []string{"p9"},
			StudioIDs:    nil,
			TagIDs:       nil,
			Rating100:    100,
			OCount:       0,
		}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildFilterIndexPayload() = %#v, want %#v", got, want)
	}
}
```

- [ ] **Step 2: Run the backend test to confirm the red state**

Run:

```bash
go test ./internal/api/browse -run TestBuildFilterIndexPayload -count=1
```

Expected: FAIL with undefined identifiers such as `FilterIndexResponse`, `FilterIndexScene`, `facetSceneSeed`, and `buildFilterIndexPayload`.

- [ ] **Step 3: Add the query, response types, and handler**

Add the compact query to `internal/stash/gql/documents/query.graphql`:

```graphql
query FindScenesForFacetIndex {
    findScenes(filter: {per_page: -1}) {
        scenes {
            id
            rating100
            o_counter
            studio {
                id
            }
            performers {
                id
            }
            tags {
                id
                sort_name
            }
        }
    }
}
```

Add the response types to `internal/api/browse/data.go`:

```go
type FilterIndexScene struct {
	ID           string   `json:"id"`
	PerformerIDs []string `json:"performerIds"`
	StudioIDs    []string `json:"studioIds,omitempty"`
	TagIDs       []string `json:"tagIds,omitempty"`
	Rating100    int      `json:"rating100"`
	OCount       int      `json:"oCount"`
}

type FilterIndexResponse struct {
	Performers []FilterOption    `json:"performers"`
	Studios    []FilterOption    `json:"studios"`
	Tags       []FilterOption    `json:"tags"`
	Scenes     []FilterIndexScene `json:"scenes"`
}
```

Create `internal/api/browse/filter_index.go` with a compact seed struct, the payload projection helper under test, and the HTTP handler:

```go
package browse

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"stash-vr/internal/stash/gql"
)

type facetSceneSeed struct {
	ID           string
	PerformerIDs []string
	StudioIDs    []string
	TagIDs       []string
	Rating100    int
	OCount       int
}

func buildFilterIndexPayload(
	performers, studios, tags []Entity,
	scenes []facetSceneSeed,
	selectableTagIDs map[string]struct{},
) FilterIndexResponse {
	out := FilterIndexResponse{
		Performers: make([]FilterOption, 0, len(performers)),
		Studios:    make([]FilterOption, 0, len(studios)),
		Tags:       make([]FilterOption, 0, len(tags)),
		Scenes:     make([]FilterIndexScene, 0, len(scenes)),
	}
	for _, e := range performers {
		out.Performers = append(out.Performers, FilterOption{ID: e.ID, Name: e.Name})
	}
	for _, e := range studios {
		out.Studios = append(out.Studios, FilterOption{ID: e.ID, Name: e.Name})
	}
	for _, e := range tags {
		out.Tags = append(out.Tags, FilterOption{ID: e.ID, Name: e.Name})
	}
	for _, s := range scenes {
		tagIDs := make([]string, 0, len(s.TagIDs))
		for _, id := range s.TagIDs {
			if _, ok := selectableTagIDs[id]; ok {
				tagIDs = append(tagIDs, id)
			}
		}
		out.Scenes = append(out.Scenes, FilterIndexScene{
			ID:           s.ID,
			PerformerIDs: s.PerformerIDs,
			StudioIDs:    s.StudioIDs,
			TagIDs:       tagIDs,
			Rating100:    s.Rating100,
			OCount:       s.OCount,
		})
	}
	return out
}

func (h *httpHandler) filterIndexHandler(w http.ResponseWriter, r *http.Request) {
	performers, err := fetchPerformers(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: load facet performers")
		http.Error(w, "load performers failed", http.StatusInternalServerError)
		return
	}
	studios, err := fetchStudios(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: load facet studios")
		http.Error(w, "load studios failed", http.StatusInternalServerError)
		return
	}
	tags, err := fetchTags(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: load facet tags")
		http.Error(w, "load tags failed", http.StatusInternalServerError)
		return
	}
	resp, err := gql.FindScenesForFacetIndex(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: load facet scenes")
		http.Error(w, "load scenes failed", http.StatusInternalServerError)
		return
	}

	selectableTagIDs := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		selectableTagIDs[t.ID] = struct{}{}
	}
	seeds := make([]facetSceneSeed, 0, len(resp.FindScenes.Scenes))
	for _, scene := range resp.FindScenes.Scenes {
		if scene == nil {
			continue
		}
		seed := facetSceneSeed{
			ID:        scene.Id,
			Rating100: scene.Rating100,
			OCount:    scene.O_counter,
		}
		for _, p := range scene.Performers {
			if p != nil {
				seed.PerformerIDs = append(seed.PerformerIDs, p.Id)
			}
		}
		if scene.Studio != nil {
			seed.StudioIDs = append(seed.StudioIDs, scene.Studio.Id)
		}
		for _, t := range scene.Tags {
			if t != nil {
				seed.TagIDs = append(seed.TagIDs, t.Id)
			}
		}
		seeds = append(seeds, seed)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(buildFilterIndexPayload(performers, studios, tags, seeds, selectableTagIDs)); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: encode filter index")
	}
}
```

Mount the route in `internal/api/browse/router.go`:

```go
r.Get("/filter-index", h.filterIndexHandler)
```

- [ ] **Step 4: Regenerate bindings and verify the backend passes**

Run:

```bash
go generate ./cmd/stash-vr
go test ./internal/api/browse -run TestBuildFilterIndexPayload -count=1
go build ./...
```

Expected:

- `go generate` updates `internal/stash/gql/generated.go`
- `go test` PASS
- `go build` PASS

- [ ] **Step 5: Commit the backend slice**

Run:

```bash
git add internal/stash/gql/documents/query.graphql internal/stash/gql/generated.go internal/api/browse/data.go internal/api/browse/filter_index.go internal/api/browse/filter_index_test.go internal/api/browse/router.go
git commit -m "Add VR filter facet index endpoint"
```

---

### Task 2: Load the facet index once and keep the legacy fallback

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

- [ ] **Step 1: Add facet-index state and one-time loading**

Extend the existing `browseState` object so the browser can distinguish
`idle`, `loading`, `ready`, and `failed` states without refetching:

```js
const browseState = {
  q: '',
  filters: { performer: [], studio: [], tag: [], favorite: '', stars: 0, ocount: 0 },
  filterNames: { performer: [], studio: [], tag: [] },
  cursor: '1',
  tiles: [],
  hasMore: true,
  loading: false,
  cachedOptions: {},
  facetIndex: null,
  facetIndexStatus: 'idle',
  facetIndexPromise: null,
  pickerQuery: { performer: '', studio: '', tag: '' },
  listScrollY: { performer: 0, studio: 0, tag: 0 },
  lastScrollFocus: 'grid',
  overlayTarget: 'grid'
};

function ensureFacetIndex() {
  if (browseState.facetIndexStatus === 'ready') {
    return Promise.resolve(browseState.facetIndex);
  }
  if (browseState.facetIndexPromise) {
    return browseState.facetIndexPromise;
  }
  browseState.facetIndexStatus = 'loading';
  browseState.facetIndexPromise = fetch('/browse/filter-index', {
    headers: { 'Accept': 'application/json' }
  })
    .then(r => {
      if (!r.ok) throw new Error('filter-index ' + r.status);
      return r.json();
    })
    .then(json => {
      browseState.facetIndex = json || null;
      browseState.facetIndexStatus = 'ready';
      return browseState.facetIndex;
    })
    .catch(err => {
      browseState.facetIndex = null;
      browseState.facetIndexStatus = 'failed';
      console.warn('stash-vr: filter index fetch failed', err);
      return null;
    });
  return browseState.facetIndexPromise;
}
```

- [ ] **Step 2: Render an explicit loading state instead of flashing the global lists**

Still in `internal/static/browse_scene.gohtml`, add a small helper that uses the
existing row geometry but strips click behavior:

```js
function renderColumnMessage(root, colTheta, colLen, text) {
  while (root.firstChild) root.removeChild(root.firstChild);
  const row = document.createElement('a-cylinder');
  row.setAttribute('radius', '2.97');
  row.setAttribute('height', LIST_ROW_GEOM_H.toString());
  row.setAttribute('open-ended', 'true');
  row.setAttribute('theta-start', colTheta.toString());
  row.setAttribute('theta-length', colLen.toString());
  row.setAttribute('position', '0 ' + LIST_TOP_Y.toFixed(3) + ' 0');
  row.setAttribute('material', 'shader: flat; color: #333; opacity: 1.0; side: double');
  attachRowLabel(row, text, colTheta, colLen);
  root.appendChild(row);
}
```

Then branch at the top of `renderColumnList(kind)`:

```js
if (browseState.facetIndexStatus === 'loading') {
  renderColumnMessage(root, colTheta, colLen, 'Loading…');
  return;
}
```

- [ ] **Step 3: Keep the current `/browse/filter-options/{kind}` code path as the failed-state fallback**

Preserve `ensureCachedOptions(kind)`, but only use it when the new fetch has
failed:

```js
if (browseState.facetIndexStatus === 'failed') {
  ensureCachedOptions(kind).then(opts => {
    if (!root.parentNode) return;
    const q = (browseState.pickerQuery[kind] || '').trim().toLowerCase();
    const filtered = q ? opts.filter(o => (o.name || '').toLowerCase().includes(q)) : opts;
    const selectedIds = browseState.filters[kind] || [];
    const selectedNames = browseState.filterNames[kind] || [];
    // Reuse the current content-container + selected-sub-list block
    // unchanged so the failed state behaves exactly like today's picker.
  });
  return;
}
```

- [ ] **Step 4: Trigger the one-time load when the browse panel opens**

Change `onBrowsePanelOpen()` so the first column paint is either the loading
state or the narrowed state from the ready cache:

```js
function onBrowsePanelOpen() {
  setupBrowseClipping();
  document.querySelectorAll('#vrBrowseTiles .vr-tile-cover').forEach(function(c) {
    const mesh = c.getObject3D && c.getObject3D('mesh');
    if (mesh && mesh.material) {
      mesh.material.clippingPlanes = browseClipPlanes;
      mesh.material.clipShadows = false;
      mesh.material.needsUpdate = true;
    }
  });
  fetchGrid(true);
  ensureFacetIndex().finally(function() {
    ['performer', 'tag', 'studio'].forEach(renderColumnList);
  });
  ['performer', 'tag', 'studio'].forEach(renderColumnList);
}
```

- [ ] **Step 5: Build-check the template and commit the loader slice**

Run:

```bash
go build ./...
```

Expected: PASS

Then commit:

```bash
git add internal/static/browse_scene.gohtml
git commit -m "Load VR filter facet index once"
```

---

### Task 3: Narrow all three columns from the current matching scene set

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

- [ ] **Step 1: Add pure client helpers for the local matching-set computation**

In `internal/static/browse_scene.gohtml`, add helpers just above
`renderColumnList(kind)`:

```js
function rerenderAllFacetColumns() {
  ['performer', 'tag', 'studio'].forEach(function(kind) {
    browseState.listScrollY[kind] = 0;
    renderColumnList(kind);
  });
}

function sceneMatchesActiveFacetFilters(scene) {
  const f = browseState.filters;
  if ((f.performer || []).some(id => (scene.performerIds || []).indexOf(id) === -1)) return false;
  if ((f.studio || []).some(id => (scene.studioIds || []).indexOf(id) === -1)) return false;
  if ((f.tag || []).some(id => (scene.tagIds || []).indexOf(id) === -1)) return false;
  if (f.stars > 0 && scene.rating100 <= (f.stars * 20 - 1)) return false;
  if (f.ocount > 0 && scene.oCount < f.ocount) return false;
  return true;
}

function visibleFacetIDSet(kind) {
  const out = new Set();
  const scenes = (browseState.facetIndex && browseState.facetIndex.scenes) || [];
  scenes.forEach(function(scene) {
    if (!sceneMatchesActiveFacetFilters(scene)) return;
    const ids = scene[kind + 'Ids'] || [];
    ids.forEach(function(id) { out.add(String(id)); });
  });
  return out;
}

function narrowedCatalog(kind) {
  const catalogs = {
    performer: (browseState.facetIndex && browseState.facetIndex.performers) || [],
    studio:    (browseState.facetIndex && browseState.facetIndex.studios) || [],
    tag:       (browseState.facetIndex && browseState.facetIndex.tags) || []
  };
  const allowed = visibleFacetIDSet(kind);
  return catalogs[kind].filter(function(opt) {
    return allowed.has(String(opt.id));
  });
}
```

- [ ] **Step 2: Make `renderColumnList(kind)` use the narrowed catalogs when the index is ready**

Replace the current `ensureCachedOptions(kind)`-first branch with a ready-state
path that:

- starts from `narrowedCatalog(kind)`
- applies the per-column text query
- excludes selected IDs from the regular list
- still renders the selected blue sub-list in selection order

Use this structure:

```js
if (browseState.facetIndexStatus === 'ready') {
  const all = narrowedCatalog(kind);
  const q = (browseState.pickerQuery[kind] || '').trim().toLowerCase();
  const selectedIds = (browseState.filters[kind] || []).map(String);
  const selectedNames = browseState.filterNames[kind] || [];
  const filtered = (q ? all.filter(o => (o.name || '').toLowerCase().includes(q)) : all)
    .filter(o => selectedIds.indexOf(String(o.id)) === -1);

  const hash = JSON.stringify([
    'ready',
    filtered.map(o => String(o.id)),
    selectedIds,
    selectedNames
  ]);
  if (_columnStateHash[kind] === hash) {
    updateColumnScroll(kind);
    return;
  }
  _columnStateHash[kind] = hash;

  while (root.firstChild) root.removeChild(root.firstChild);
  const initialScrollY = browseState.listScrollY[kind] || 0;
  const half = LIST_ROW_H / 2;
  const visTop = LIST_TOP_Y + half;
  const visBottom = LIST_REGULAR_BOTTOM_Y - half;
  const content = document.createElement('a-entity');
  content.id = 'vrFiltersListContent-' + kind;
  content.setAttribute('position', '0 ' + (LIST_TOP_Y + initialScrollY) + ' 0');
  filtered.forEach(function(opt, i) {
    const row = makeFilterRow(kind, opt, -i * LIST_ROW_H, colTheta, colLen, false);
    row.dataset.rowIdx = String(i);
    const worldY = LIST_TOP_Y + initialScrollY - i * LIST_ROW_H;
    if (worldY > visTop || worldY < visBottom) row.setAttribute('visible', false);
    content.appendChild(row);
  });
  root.appendChild(content);

  if (selectedIds.length) {
    const sel = document.createElement('a-entity');
    sel.id = 'vrFiltersSelected-' + kind;
    sel.setAttribute('position', '0 0 0');
    selectedIds.forEach(function(id, i) {
      const name = selectedNames[i] || '';
      const y = LIST_SELECTED_TOP_Y - i * LIST_ROW_H;
      sel.appendChild(makeFilterRow(kind, { id: id, name: name }, y, colTheta, colLen, true));
    });
    root.appendChild(sel);
  }
  updateRaycasterTargets();
  return;
}
```

- [ ] **Step 3: Make all filter changes rerender all three columns, not just the touched column**

Update `applyFilterPick(kind, id, name)` so entity picks and scalar picks both
refresh the whole facet surface:

```js
function applyFilterPick(kind, id, name) {
  if (kind === 'performer' || kind === 'studio' || kind === 'tag') {
    const ids = browseState.filters[kind];
    const names = browseState.filterNames[kind];
    const key = String(id);
    const idx = ids.map(String).indexOf(key);
    if (idx >= 0) {
      ids.splice(idx, 1);
      names.splice(idx, 1);
    } else {
      ids.push(key);
      names.push(name);
    }
  } else if (kind === 'favorite') {
    browseState.filters.favorite = browseState.filters.favorite === id ? '' : id;
    renderValuesRow();
  } else if (kind === 'stars') {
    const n = parseInt(id || '0', 10);
    browseState.filters.stars = browseState.filters.stars === n ? 0 : n;
    renderValuesRow();
  } else if (kind === 'ocount') {
    const n = parseInt(id || '0', 10);
    browseState.filters.ocount = browseState.filters.ocount === n ? 0 : n;
    renderValuesRow();
  }
  rerenderAllFacetColumns();
  renderActiveChips();
  fetchGrid(true);
}
```

Also update the scalar-clear and full-clear paths so they refresh all three
columns:

```js
function clearChip(kind) {
  if (kind === 'favorite') browseState.filters.favorite = '';
  else if (kind === 'stars') browseState.filters.stars = 0;
  else if (kind === 'ocount') browseState.filters.ocount = 0;
  rerenderAllFacetColumns();
  renderActiveChips();
  fetchGrid(true);
}
```

```js
// Keep the existing search-field / label reset code above this block.
} else if (action === 'filters-clear') {
  browseState.q = '';
  browseState.filters     = { performer: [], studio: [], tag: [], favorite: '', stars: 0, ocount: 0 };
  browseState.filterNames = { performer: [], studio: [], tag: [] };
  browseState.pickerQuery = { performer: '', studio: '', tag: '' };
  rerenderAllFacetColumns();
  renderValuesRow();
  fetchGrid(true);
}
```

- [ ] **Step 4: Keep scroll math aligned with the narrowed regular-list size**

Update `maxListScrollY(kind)` so the scroll range is computed from the same
regular-list source the column is currently rendering:

```js
function maxListScrollY(kind) {
  let opts = [];
  if (browseState.facetIndexStatus === 'ready') {
    const selectedIds = (browseState.filters[kind] || []).map(String);
    opts = narrowedCatalog(kind).filter(function(opt) {
      return selectedIds.indexOf(String(opt.id)) === -1;
    });
  } else {
    opts = browseState.cachedOptions[kind] || [];
  }
  const q = (browseState.pickerQuery[kind] || '').trim().toLowerCase();
  const filtered = q ? opts.filter(o => (o.name || '').toLowerCase().includes(q)) : opts;
  const overflow = filtered.length - LIST_VISIBLE_REGULAR;
  return Math.max(0, overflow * LIST_ROW_H);
}
```

- [ ] **Step 5: Build-check, run the targeted Go tests again, and commit**

Run:

```bash
go test ./internal/api/browse -run TestBuildFilterIndexPayload -count=1
go build ./...
```

Expected: PASS

Then commit:

```bash
git add internal/static/browse_scene.gohtml
git commit -m "Intersect VR filter facets locally"
```

---

### Task 4: Verify end-to-end behavior before calling it done

**Files:**
- Modify: none unless a verification issue is found

- [ ] **Step 1: Regenerate + run the full local checks**

Run:

```bash
go generate ./cmd/stash-vr
go test ./internal/api/browse -count=1
go build ./...
```

Expected: PASS / PASS / PASS

- [ ] **Step 2: Manual desktop verification of the JSON payload**

Run the app locally, then open `/browse/filter-index` on the same host/port you
use for your local stash-vr instance.

Check:

- response has `performers`, `studios`, `tags`, `scenes`
- scene rows carry `performerIds`, `studioIds`, `tagIds`, `rating100`, `oCount`
- hidden / ancestor tags are absent from `tagIds`

- [ ] **Step 3: Manual VR/desktop behavior verification**

Open any real scene detail page from your local browse grid in Meta Browser
(or desktop Chrome for the column logic), enter VR if needed, and verify:

1. Open Browse → Filters.
2. First open shows `Loading…` in the entity columns, then the real rows.
3. Select one performer. Confirm all three entity columns shrink to that performer's scene neighborhood.
4. Add one tag. Confirm the columns shrink again using AND semantics.
5. Remove the tag. Confirm the columns expand back to the performer-only neighborhood.
6. Toggle `★4` or `10+`. Confirm the entity columns react as well as the grid.
7. Toggle `Favorites: Only` or `Favorites: Not`. Confirm the grid changes but the entity columns do not drift or crash.

- [ ] **Step 4: Manual failed-fetch fallback verification**

In desktop Chrome DevTools, block the URL pattern:

```text
*/browse/filter-index
```

Reload the scene page, reopen Browse → Filters, and verify:

- no crash
- the entity columns populate from the old global lists
- selecting performer/tag/studio still refetches the grid through `/browse/grid`

- [ ] **Step 5: Final commit only if verification forced a change**

If any verification bug required code edits, commit that delta with a focused
message, for example:

```bash
git add internal/static/browse_scene.gohtml internal/api/browse/filter_index.go internal/api/browse/filter_index_test.go internal/api/browse/data.go internal/api/browse/router.go internal/stash/gql/documents/query.graphql internal/stash/gql/generated.go
git commit -m "Fix VR facet narrowing verification issues"
```
