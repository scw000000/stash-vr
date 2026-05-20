# VR grid browser: streaming stubs, tile pool, filter-index split — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the in-VR `/browse` panel open feel snappy on cold cache by batching the grid's per-tile GraphQL loop, rendering empty stubs before hydration, bounding tile entities with a pool, and splitting the heavy filter-index into a cached catalog + matrix served with weak ETags.

**Architecture:**
- **Server:** one GraphQL batch call replaces N sequential calls in the grid handler. A new `filterCache` in the browse package holds two independently-versioned snapshots (catalog + matrix) with TTL + content-hash invalidation, served with weak ETags. Two new endpoints (`/browse/filter-catalog`, `/browse/filter-matrix`) replace the monolithic `/browse/filter-index`; the legacy endpoint becomes a thin alias.
- **Client:** `relayoutTiles` does a two-pass render — stub pass (cylinder + bg color only, one frame) followed by hydration scheduled across `requestAnimationFrame` ticks (texture, MSDF title, ⓘ badge, hover handlers). Tile entities live in a fixed pool sized to `cols × (visibleRows + bufferRows*2)`, rebound to logical rows on scroll. Facet fetch splits into `ensureFacetCatalog()` + `ensureFacetMatrix()`; column rendering degrades gracefully when matrix isn't yet loaded.

**Tech Stack:** Go 1.24, genqlient GraphQL client, `golang.org/x/sync/singleflight`, chi router, A-Frame WebXR, vanilla JS (no bundler), zerolog.

**Spec:** [docs/superpowers/specs/2026-05-19-vr-grid-streaming-stubs-design.md](../specs/2026-05-19-vr-grid-streaming-stubs-design.md)

---

## File map

**New files:**
- `internal/api/browse/filter_cache.go` — snapshot store with version/ETag/TTL/singleflight.
- `internal/api/browse/filter_cache_test.go` — unit tests for snapshot lifecycle.

**Modified files:**
- `internal/api/browse/grid_json.go` — replace per-tile loop with batch call; add timing logs.
- `internal/api/browse/filter_index.go` — refactor handler to compose from cache snapshots.
- `internal/api/browse/data.go` — add response DTOs for catalog-only and matrix-only shapes if not already shaped correctly.
- `internal/api/browse/router.go` — register the two new endpoints.
- `internal/static/browse_scene.gohtml` — split facet state machine; rewrite `relayoutTiles` for two-pass + pool; add `performance.mark` calls.

**No file moves; no library-package changes** — snapshot lives in the browse package, keeping the diff contained.

---

## Task 1: Server — batch the grid's per-tile fetch

**Files:**
- Modify: [internal/api/browse/grid_json.go:53-80](../../../internal/api/browse/grid_json.go#L53-L80)
- Test: extend existing tests if any cover this path; otherwise this task is verified by `go vet` + `go build` + manual smoke.

### Step 1: Read the current loop

Read [internal/api/browse/grid_json.go:53-80](../../../internal/api/browse/grid_json.go#L53-L80). Confirm the loop calls `h.libraryService.GetScene(r.Context(), id, false)` once per id, then builds a `GridTile` for each. The batch helper `GetScenesByIds` exists at [internal/library/scenes.go:72-115](../../../internal/library/scenes.go#L72-L115); it returns `[]*VideoData` aligned with the input ids slice (nils for missing).

- [ ] **Step 2: Replace the loop with a single batch call**

Edit `internal/api/browse/grid_json.go`. Replace lines 53-80 (the `tiles := make(...)` block through the closing `}` of the loop) with:

```go
	baseURL := apiinternal.GetBaseUrl(r)
	vds, err := h.libraryService.GetScenesByIds(r.Context(), ids)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: grid GetScenesByIds")
		http.Error(w, "fetch failed", http.StatusInternalServerError)
		return
	}
	tiles := make([]GridTile, 0, len(vds))
	for i, vd := range vds {
		if vd == nil || vd.SceneParts == nil {
			continue
		}
		thumb := ""
		if vd.SceneParts.Paths != nil && vd.SceneParts.Paths.Screenshot != nil {
			thumb = heatmap.GetCoverUrl(baseURL, ids[i])
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
			ID:           ids[i],
			Title:        vd.Title(),
			ThumbnailURL: thumb,
			Projection:   apiinternal.Detect(tagInputs, basename),
		})
	}
```

This collapses up to N sequential GraphQL round-trips into one batch. The output `tiles` slice is unchanged in shape (same JSON wire format).

- [ ] **Step 3: Verify build**

Run: `go vet ./... && go build ./...`
Expected: clean compile.

- [ ] **Step 4: Smoke-test manually**

Run: `scripts\build-windows.bat` (per memory `reference_build_script.md`), launch stash-vr, open `/browse` in a browser. Confirm the 2D grid still renders. Then open the in-VR panel and confirm tiles populate.

- [ ] **Step 5: Commit**

```bash
git add internal/api/browse/grid_json.go
git commit -m "browse: batch grid per-tile fetch via GetScenesByIds"
```

---

## Task 2: Library snapshot infrastructure for filter catalog + matrix

**Files:**
- Create: `internal/api/browse/filter_cache.go`
- Create: `internal/api/browse/filter_cache_test.go`

This task introduces the cache with versioning + content hash + TTL + singleflight. No HTTP handlers wired up yet — those come in tasks 3 and 4.

### Step 1: Write the failing test

Create `internal/api/browse/filter_cache_test.go` with:

```go
package browse

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
)

// stubBuilder lets a test substitute the heavy GraphQL work so we can
// assert version/hash/TTL behavior without touching Stash.
type stubBuilder struct {
	catalogCalls int32
	matrixCalls  int32
	catalog      filterIndexCatalog
	matrixResp   filterMatrixSeeds
}

func (s *stubBuilder) BuildCatalog(_ context.Context, _ graphql.Client) (filterIndexCatalog, error) {
	atomic.AddInt32(&s.catalogCalls, 1)
	return s.catalog, nil
}

func (s *stubBuilder) BuildMatrix(_ context.Context, _ graphql.Client) (filterMatrixSeeds, error) {
	atomic.AddInt32(&s.matrixCalls, 1)
	return s.matrixResp, nil
}

func newTestCatalog() filterIndexCatalog {
	return filterIndexCatalog{
		sidebar: SidebarData{
			Performers: []Entity{{ID: "p1", Name: "Alpha"}},
			Studios:    []Entity{{ID: "s1", Name: "S"}},
			Tags:       []Entity{{ID: "t1", Name: "T"}},
		},
		selectableStudioIDs: map[string]struct{}{"s1": {}},
		selectableTagIDs:    map[string]struct{}{"t1": {}},
	}
}

func newTestMatrix() filterMatrixSeeds {
	return filterMatrixSeeds{
		Scenes: []facetSceneSeed{
			{ID: "sc1", PerformerIDs: []string{"p1"}, StudioID: "s1", TagIDs: []string{"t1"}, Rating100: 80, OCount: 2},
		},
	}
}

func TestFilterCache_CatalogBuildsOnceWithinTTL(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	cache := newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 1*time.Hour)

	_, etag1, err := cache.Catalog(context.Background(), nil)
	if err != nil {
		t.Fatalf("Catalog(1) err = %v", err)
	}
	_, etag2, err := cache.Catalog(context.Background(), nil)
	if err != nil {
		t.Fatalf("Catalog(2) err = %v", err)
	}
	if etag1 != etag2 {
		t.Fatalf("etag changed within TTL: %s vs %s", etag1, etag2)
	}
	if got := atomic.LoadInt32(&stub.catalogCalls); got != 1 {
		t.Fatalf("catalog builder calls = %d, want 1", got)
	}
}

func TestFilterCache_CatalogRebuildsAfterTTL(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	cache := newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 10*time.Millisecond)

	_, etag1, _ := cache.Catalog(context.Background(), nil)
	time.Sleep(20 * time.Millisecond)
	_, etag2, _ := cache.Catalog(context.Background(), nil)
	// Content identical → version stays the same → ETag stable.
	if etag1 != etag2 {
		t.Fatalf("etag changed despite identical content: %s vs %s", etag1, etag2)
	}
	if got := atomic.LoadInt32(&stub.catalogCalls); got != 2 {
		t.Fatalf("catalog builder calls = %d, want 2 (rebuild after TTL)", got)
	}
}

func TestFilterCache_VersionBumpsOnContentChange(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	cache := newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 10*time.Millisecond)

	_, etag1, _ := cache.Catalog(context.Background(), nil)
	// Mutate the stub's returned catalog so the next build sees new content.
	stub.catalog.sidebar.Performers = append(stub.catalog.sidebar.Performers, Entity{ID: "p2", Name: "Bravo"})
	time.Sleep(20 * time.Millisecond)
	_, etag2, _ := cache.Catalog(context.Background(), nil)
	if etag1 == etag2 {
		t.Fatalf("etag did not change despite content change: %s", etag1)
	}
}

func TestFilterCache_MatrixUsesCachedCatalogParents(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	cache := newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 1*time.Hour)

	if _, _, err := cache.Matrix(context.Background(), nil); err != nil {
		t.Fatalf("Matrix err = %v", err)
	}
	// Matrix must implicitly produce or reuse a catalog (for parent expansion).
	if atomic.LoadInt32(&stub.catalogCalls) < 1 {
		t.Fatalf("matrix did not trigger catalog build; calls=%d", atomic.LoadInt32(&stub.catalogCalls))
	}
	if atomic.LoadInt32(&stub.matrixCalls) != 1 {
		t.Fatalf("matrix calls = %d, want 1", atomic.LoadInt32(&stub.matrixCalls))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/browse/... -run TestFilterCache -v`
Expected: FAIL (the symbols `newFilterCache`, `filterMatrixSeeds`, and the methods don't exist yet).

- [ ] **Step 3: Implement the cache**

Create `internal/api/browse/filter_cache.go`:

```go
package browse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Khan/genqlient/graphql"
	"golang.org/x/sync/singleflight"
)

// filterMatrixSeeds is the raw scene-seed list returned by the matrix
// builder. The wire payload (matrixWireResponse) is computed from this
// plus the catalog's parent maps so ancestor IDs are expanded into each
// scene's studio/tag membership lists.
type filterMatrixSeeds struct {
	Scenes []facetSceneSeed
}

type catalogBuilderFn func(context.Context, graphql.Client) (filterIndexCatalog, error)
type matrixBuilderFn func(context.Context, graphql.Client) (filterMatrixSeeds, error)

// filterSnapshot holds one cached payload plus its identifier. Payload is
// JSON bytes ready to ship; etag is the weak-ETag value (already quoted).
// version increments only when contentHash changes between rebuilds, so
// idempotent rebuilds keep clients revalidating to 304.
type filterSnapshot struct {
	payload     []byte
	etag        string
	version     int
	contentHash [sha256.Size]byte
	builtAt     time.Time
}

type filterCache struct {
	buildCatalog catalogBuilderFn
	buildMatrix  matrixBuilderFn
	ttl          time.Duration
	now          func() time.Time

	mu          sync.RWMutex
	catalogSnap *filterSnapshot
	matrixSnap  *filterSnapshot
	// catalogData caches the raw filterIndexCatalog so the matrix
	// builder can expand parent IDs without re-running the catalog
	// GraphQL queries.
	catalogData *filterIndexCatalog

	sf singleflight.Group
}

func newFilterCache(buildCatalog catalogBuilderFn, buildMatrix matrixBuilderFn, ttl time.Duration) *filterCache {
	return &filterCache{
		buildCatalog: buildCatalog,
		buildMatrix:  buildMatrix,
		ttl:          ttl,
		now:          time.Now,
	}
}

// Catalog returns the catalog wire payload + weak ETag. Rebuilds when
// snapshot is nil or older than ttl. If the rebuilt content hash equals
// the previous snapshot's, the version is preserved so revalidating
// clients still get 304.
func (c *filterCache) Catalog(ctx context.Context, client graphql.Client) ([]byte, string, error) {
	if snap := c.freshCatalog(); snap != nil {
		return snap.payload, snap.etag, nil
	}
	v, err, _ := c.sf.Do("catalog", func() (any, error) {
		return c.rebuildCatalog(ctx, client)
	})
	if err != nil {
		return nil, "", err
	}
	snap := v.(*filterSnapshot)
	return snap.payload, snap.etag, nil
}

// Matrix returns the matrix wire payload + weak ETag. Requires catalog
// parent maps for ancestor expansion, so it ensures catalog is built
// first (via the same cache, deduped through singleflight).
func (c *filterCache) Matrix(ctx context.Context, client graphql.Client) ([]byte, string, error) {
	if snap := c.freshMatrix(); snap != nil {
		return snap.payload, snap.etag, nil
	}
	v, err, _ := c.sf.Do("matrix", func() (any, error) {
		return c.rebuildMatrix(ctx, client)
	})
	if err != nil {
		return nil, "", err
	}
	snap := v.(*filterSnapshot)
	return snap.payload, snap.etag, nil
}

func (c *filterCache) freshCatalog() *filterSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.catalogSnap == nil {
		return nil
	}
	if c.now().Sub(c.catalogSnap.builtAt) > c.ttl {
		return nil
	}
	return c.catalogSnap
}

func (c *filterCache) freshMatrix() *filterSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.matrixSnap == nil {
		return nil
	}
	if c.now().Sub(c.matrixSnap.builtAt) > c.ttl {
		return nil
	}
	return c.matrixSnap
}

func (c *filterCache) rebuildCatalog(ctx context.Context, client graphql.Client) (*filterSnapshot, error) {
	data, err := c.buildCatalog(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("buildCatalog: %w", err)
	}
	payload, err := json.Marshal(catalogWirePayload(data))
	if err != nil {
		return nil, fmt.Errorf("marshal catalog: %w", err)
	}
	hash := sha256.Sum256(payload)

	c.mu.Lock()
	defer c.mu.Unlock()
	prev := c.catalogSnap
	version := 1
	if prev != nil {
		version = prev.version
		if hash != prev.contentHash {
			version = prev.version + 1
		}
	}
	snap := &filterSnapshot{
		payload:     payload,
		etag:        weakETag(version),
		version:     version,
		contentHash: hash,
		builtAt:     c.now(),
	}
	c.catalogSnap = snap
	c.catalogData = &data
	return snap, nil
}

func (c *filterCache) rebuildMatrix(ctx context.Context, client graphql.Client) (*filterSnapshot, error) {
	// Ensure catalog data is available for parent expansion. Use the
	// cached data path directly so we don't double-encode the wire JSON.
	if _, _, err := c.Catalog(ctx, client); err != nil {
		return nil, fmt.Errorf("matrix requires catalog: %w", err)
	}
	c.mu.RLock()
	catalogData := c.catalogData
	c.mu.RUnlock()
	if catalogData == nil {
		return nil, fmt.Errorf("matrix build: catalog data unavailable")
	}

	seeds, err := c.buildMatrix(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("buildMatrix: %w", err)
	}
	wire := matrixWirePayload(seeds.Scenes,
		catalogData.selectableStudioIDs,
		catalogData.selectableTagIDs,
		catalogData.studioParentsByID,
		catalogData.tagParentsByID,
	)
	payload, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("marshal matrix: %w", err)
	}
	hash := sha256.Sum256(payload)

	c.mu.Lock()
	defer c.mu.Unlock()
	prev := c.matrixSnap
	version := 1
	if prev != nil {
		version = prev.version
		if hash != prev.contentHash {
			version = prev.version + 1
		}
	}
	snap := &filterSnapshot{
		payload:     payload,
		etag:        weakETag(version),
		version:     version,
		contentHash: hash,
		builtAt:     c.now(),
	}
	c.matrixSnap = snap
	return snap, nil
}

func weakETag(version int) string {
	return fmt.Sprintf(`W/"%d"`, version)
}

// hashForLogs returns a short prefix for log lines; never used for caching.
func hashForLogs(h [sha256.Size]byte) string {
	return hex.EncodeToString(h[:4])
}

// catalogWirePayload narrows filterIndexCatalog to just the entity lists
// the client renders into sidebar columns.
func catalogWirePayload(data filterIndexCatalog) FilterCatalogResponse {
	resp := FilterCatalogResponse{
		Performers: make([]FilterOption, 0, len(data.sidebar.Performers)),
		Studios:    make([]FilterOption, 0, len(data.sidebar.Studios)),
		Tags:       make([]FilterOption, 0, len(data.sidebar.Tags)),
	}
	for _, p := range data.sidebar.Performers {
		resp.Performers = append(resp.Performers, FilterOption{ID: p.ID, Name: p.Name})
	}
	for _, s := range data.sidebar.Studios {
		resp.Studios = append(resp.Studios, FilterOption{ID: s.ID, Name: s.Name})
	}
	for _, t := range data.sidebar.Tags {
		resp.Tags = append(resp.Tags, FilterOption{ID: t.ID, Name: t.Name})
	}
	return resp
}

// matrixWirePayload returns the per-scene facet ID matrix used by the
// client-side intersection logic. Studios + tags are ancestor-expanded
// against the catalog's selectable ID sets so the wire payload contains
// only IDs that have a corresponding sidebar entry.
func matrixWirePayload(
	scenes []facetSceneSeed,
	selectableStudioIDs, selectableTagIDs map[string]struct{},
	studioParentsByID map[string]string,
	tagParentsByID map[string][]string,
) FilterMatrixResponse {
	out := FilterMatrixResponse{Scenes: make([]FilterIndexScene, 0, len(scenes))}
	for _, scene := range scenes {
		item := FilterIndexScene{
			ID:           scene.ID,
			PerformerIDs: append([]string(nil), scene.PerformerIDs...),
			Rating100:    scene.Rating100,
			OCount:       scene.OCount,
		}
		if scene.StudioID != "" {
			item.StudioIDs = expandedStudioIDs(scene.StudioID, selectableStudioIDs, studioParentsByID)
		}
		if len(scene.TagIDs) > 0 {
			item.TagIDs = expandedTagIDs(scene.TagIDs, selectableTagIDs, tagParentsByID)
			if len(item.TagIDs) == 0 {
				item.TagIDs = nil
			}
		}
		out.Scenes = append(out.Scenes, item)
	}
	return out
}
```

Also add the two new wire-format DTOs to `internal/api/browse/data.go` (next to the existing `FilterIndexResponse` at line ~132):

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/api/browse/... -run TestFilterCache -v`
Expected: PASS for all four tests.

- [ ] **Step 5: Run all existing browse tests to confirm no regression**

Run: `go test ./internal/api/browse/... -v`
Expected: PASS for `TestBuildFilterIndexPayload` (3 subtests) + `TestLoadFilterIndexDataRunsCatalogAndSceneFetchConcurrently` + the 4 new tests.

- [ ] **Step 6: Verify build**

Run: `go vet ./... && go build ./...`
Expected: clean compile.

- [ ] **Step 7: Commit**

```bash
git add internal/api/browse/filter_cache.go internal/api/browse/filter_cache_test.go internal/api/browse/data.go
git commit -m "browse: add filter cache snapshot with ETag versioning"
```

---

## Task 3: Server — `/browse/filter-catalog` endpoint

**Files:**
- Modify: `internal/api/browse/router.go` — register handler.
- Modify: `internal/api/browse/filter_index.go` — add handler + builder adapters.
- Test: `internal/api/browse/filter_cache_test.go` — add HTTP-level test.

### Step 1: Add the builder adapter inside filter_index.go

The existing `loadFilterIndexCatalog` returns the right type already. We just need a `FindScenesForFacetIndex` builder for matrix.

Read [internal/api/browse/filter_index.go:161-251](../../../internal/api/browse/filter_index.go#L161-L251) for the existing catalog loader signature. It matches `catalogBuilderFn` exactly. Wire-up is direct.

- [ ] **Step 2: Add a constructor and matrix adapter**

In `internal/api/browse/filter_index.go`, just below `loadFilterIndexDataWithFns` (~ line 285), add:

```go
// buildMatrixSeeds adapts gql.FindScenesForFacetIndex into the
// filterMatrixSeeds shape consumed by filterCache.rebuildMatrix.
func buildMatrixSeeds(ctx context.Context, client graphql.Client) (filterMatrixSeeds, error) {
	resp, err := gql.FindScenesForFacetIndex(ctx, client)
	if err != nil {
		return filterMatrixSeeds{}, fmt.Errorf("FindScenesForFacetIndex: %w", err)
	}
	out := filterMatrixSeeds{Scenes: make([]facetSceneSeed, 0, len(resp.FindScenes.Scenes))}
	for _, scene := range resp.FindScenes.Scenes {
		if scene == nil {
			continue
		}
		item := facetSceneSeed{
			ID:           scene.Id,
			PerformerIDs: make([]string, 0, len(scene.Performers)),
			TagIDs:       make([]string, 0, len(scene.Tags)),
		}
		if scene.Rating100 != nil {
			item.Rating100 = *scene.Rating100
		}
		if scene.O_counter != nil {
			item.OCount = *scene.O_counter
		}
		if scene.Studio != nil {
			item.StudioID = scene.Studio.Id
		}
		for _, performer := range scene.Performers {
			if performer == nil {
				continue
			}
			item.PerformerIDs = append(item.PerformerIDs, performer.Id)
		}
		for _, tag := range scene.Tags {
			if tag == nil {
				continue
			}
			item.TagIDs = append(item.TagIDs, tag.Id)
		}
		if len(item.TagIDs) == 0 {
			item.TagIDs = nil
		}
		out.Scenes = append(out.Scenes, item)
	}
	return out, nil
}
```

- [ ] **Step 3: Add the cache to httpHandler**

Edit [internal/api/browse/router.go:10-12](../../../internal/api/browse/router.go#L10-L12). Change `httpHandler` to include the cache, and have `Router` instantiate it:

```go
type httpHandler struct {
	libraryService *library.Service
	filterCache    *filterCache
}

func Router(libraryService *library.Service) http.Handler {
	h := httpHandler{
		libraryService: libraryService,
		filterCache:    newFilterCache(loadFilterIndexCatalog, buildMatrixSeeds, 5*time.Minute),
	}
	r := chi.NewRouter()
```

Add the `time` import to the router file's imports.

- [ ] **Step 4: Add the catalog handler**

In `internal/api/browse/filter_index.go`, after the existing `filterIndexHandler` (~ line 359), add:

```go
// filterCatalogHandler serves GET /browse/filter-catalog — sidebar entity
// lists only. ETag-revalidated against an in-memory snapshot so reopens
// without library changes return 304.
func (h *httpHandler) filterCatalogHandler(w http.ResponseWriter, r *http.Request) {
	payload, etag, err := h.filterCache.Catalog(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-catalog build")
		http.Error(w, "load filter catalog failed", http.StatusInternalServerError)
		return
	}
	if match := r.Header.Get("If-None-Match"); match != "" && etagMatches(match, etag) {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(payload); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-catalog write")
	}
}

// etagMatches returns true if any If-None-Match list entry matches the
// snapshot's weak ETag. Strips surrounding whitespace and the optional
// W/ prefix on each list element. Single "*" matches everything.
func etagMatches(headerVal, etag string) bool {
	if headerVal == "*" {
		return true
	}
	target := strings.TrimPrefix(etag, "W/")
	for _, part := range strings.Split(headerVal, ",") {
		p := strings.TrimSpace(part)
		p = strings.TrimPrefix(p, "W/")
		if p == target {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Register the route**

Edit `internal/api/browse/router.go`. Add below the existing `r.Get("/filter-index", h.filterIndexHandler)`:

```go
	r.Get("/filter-catalog", h.filterCatalogHandler)
```

- [ ] **Step 6: Add an HTTP-level test**

Append to `internal/api/browse/filter_cache_test.go`:

```go
import (
	// keep existing imports
	"net/http"
	"net/http/httptest"
)

func TestFilterCatalogHandler304(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	h := &httpHandler{
		filterCache: newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 1*time.Hour),
	}

	// First request: expect 200 with ETag header.
	req1 := httptest.NewRequest(http.MethodGet, "/browse/filter-catalog", nil)
	rec1 := httptest.NewRecorder()
	h.filterCatalogHandler(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec1.Code)
	}
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first response missing ETag")
	}

	// Second request with If-None-Match: expect 304 and empty body.
	req2 := httptest.NewRequest(http.MethodGet, "/browse/filter-catalog", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.filterCatalogHandler(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("revalidated request status = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("304 response body should be empty, got %d bytes", rec2.Body.Len())
	}
}
```

Make sure the `import` block adds `"net/http"`, `"net/http/httptest"`, and `"strings"` (the last only in filter_index.go where `etagMatches` is defined).

- [ ] **Step 7: Run tests**

Run: `go test ./internal/api/browse/... -v`
Expected: PASS including the new `TestFilterCatalogHandler304`.

- [ ] **Step 8: Verify build**

Run: `go vet ./... && go build ./...`
Expected: clean.

- [ ] **Step 9: Commit**

```bash
git add internal/api/browse/router.go internal/api/browse/filter_index.go internal/api/browse/filter_cache_test.go
git commit -m "browse: add /browse/filter-catalog endpoint with weak ETag"
```

---

## Task 4: Server — `/browse/filter-matrix` endpoint

**Files:**
- Modify: `internal/api/browse/router.go` — register handler.
- Modify: `internal/api/browse/filter_index.go` — add `filterMatrixHandler`.
- Test: `internal/api/browse/filter_cache_test.go` — add HTTP-level test.

### Step 1: Add the matrix handler

In `internal/api/browse/filter_index.go`, below `filterCatalogHandler`, add:

```go
// filterMatrixHandler serves GET /browse/filter-matrix — the per-scene
// facet ID matrix used by client-side intersection. Same ETag protocol
// as filter-catalog. Heavier payload; the snapshot pays for itself most
// on warm reopens (304).
func (h *httpHandler) filterMatrixHandler(w http.ResponseWriter, r *http.Request) {
	payload, etag, err := h.filterCache.Matrix(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-matrix build")
		http.Error(w, "load filter matrix failed", http.StatusInternalServerError)
		return
	}
	if match := r.Header.Get("If-None-Match"); match != "" && etagMatches(match, etag) {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(payload); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-matrix write")
	}
}
```

- [ ] **Step 2: Register the route**

Edit `internal/api/browse/router.go`. Add below the catalog line:

```go
	r.Get("/filter-matrix", h.filterMatrixHandler)
```

- [ ] **Step 3: Add HTTP-level test**

Append to `internal/api/browse/filter_cache_test.go`:

```go
func TestFilterMatrixHandler304(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	h := &httpHandler{
		filterCache: newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 1*time.Hour),
	}

	req1 := httptest.NewRequest(http.MethodGet, "/browse/filter-matrix", nil)
	rec1 := httptest.NewRecorder()
	h.filterMatrixHandler(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec1.Code)
	}
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first response missing ETag")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/browse/filter-matrix", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.filterMatrixHandler(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("revalidated request status = %d, want 304", rec2.Code)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/api/browse/... -v`
Expected: PASS including `TestFilterMatrixHandler304`.

- [ ] **Step 5: Verify build**

Run: `go vet ./... && go build ./...`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add internal/api/browse/router.go internal/api/browse/filter_index.go internal/api/browse/filter_cache_test.go
git commit -m "browse: add /browse/filter-matrix endpoint with weak ETag"
```

---

## Task 5: Server — refactor `/browse/filter-index` to compose from cache

**Files:**
- Modify: `internal/api/browse/filter_index.go` — `filterIndexHandler`.
- Test: `internal/api/browse/filter_cache_test.go` — add composition test.

The legacy endpoint stays online for one cycle as a thin alias that composes catalog + matrix from the cache. Old clients keep working; nobody pays the GraphQL cost twice.

### Step 1: Rewrite filterIndexHandler

Replace the existing [filterIndexHandler at filter_index.go:287-359](../../../internal/api/browse/filter_index.go#L287-L359) with:

```go
// filterIndexHandler serves the legacy GET /browse/filter-index endpoint
// as a thin alias that composes the catalog + matrix snapshots into the
// pre-split wire shape (FilterIndexResponse). Kept online for one cycle
// so clients that haven't been updated still work; remove once all
// callers use /browse/filter-catalog + /browse/filter-matrix.
func (h *httpHandler) filterIndexHandler(w http.ResponseWriter, r *http.Request) {
	catalogPayload, _, err := h.filterCache.Catalog(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-index (legacy) catalog")
		http.Error(w, "load filter catalog failed", http.StatusInternalServerError)
		return
	}
	matrixPayload, _, err := h.filterCache.Matrix(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-index (legacy) matrix")
		http.Error(w, "load filter matrix failed", http.StatusInternalServerError)
		return
	}

	var catalog FilterCatalogResponse
	if err := json.Unmarshal(catalogPayload, &catalog); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-index (legacy) decode catalog")
		http.Error(w, "load filter index failed", http.StatusInternalServerError)
		return
	}
	var matrix FilterMatrixResponse
	if err := json.Unmarshal(matrixPayload, &matrix); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-index (legacy) decode matrix")
		http.Error(w, "load filter index failed", http.StatusInternalServerError)
		return
	}

	resp := FilterIndexResponse{
		Performers: catalog.Performers,
		Studios:    catalog.Studios,
		Tags:       catalog.Tags,
		Scenes:     matrix.Scenes,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-index (legacy) encode")
	}
}
```

The old helper functions `loadFilterIndexData` and `loadFilterIndexDataWithFns` are no longer referenced by the handler. Keep them — they're covered by the existing concurrency test (`TestLoadFilterIndexDataRunsCatalogAndSceneFetchConcurrently`) and removing them would force a separate cleanup commit. They'll go away when the legacy endpoint is removed in a follow-up.

- [ ] **Step 2: Add a composition test**

Append to `internal/api/browse/filter_cache_test.go`:

```go
func TestFilterIndexHandlerComposesCachedSnapshots(t *testing.T) {
	stub := &stubBuilder{catalog: newTestCatalog(), matrixResp: newTestMatrix()}
	h := &httpHandler{
		filterCache: newFilterCache(stub.BuildCatalog, stub.BuildMatrix, 1*time.Hour),
	}

	req := httptest.NewRequest(http.MethodGet, "/browse/filter-index", nil)
	rec := httptest.NewRecorder()
	h.filterIndexHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp FilterIndexResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Performers) != 1 || resp.Performers[0].ID != "p1" {
		t.Fatalf("performers = %#v, want [{p1,Alpha}]", resp.Performers)
	}
	if len(resp.Scenes) != 1 || resp.Scenes[0].ID != "sc1" {
		t.Fatalf("scenes = %#v, want [{sc1,...}]", resp.Scenes)
	}
	// Second call must hit the cache (builders ran exactly once each).
	rec2 := httptest.NewRecorder()
	h.filterIndexHandler(rec2, httptest.NewRequest(http.MethodGet, "/browse/filter-index", nil))
	if got := atomic.LoadInt32(&stub.catalogCalls); got != 1 {
		t.Fatalf("catalog calls = %d after 2nd request, want 1", got)
	}
	if got := atomic.LoadInt32(&stub.matrixCalls); got != 1 {
		t.Fatalf("matrix calls = %d after 2nd request, want 1", got)
	}
}
```

The `import` block needs `"encoding/json"` if not already present.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/api/browse/... -v`
Expected: PASS. The legacy `TestLoadFilterIndexDataRunsCatalogAndSceneFetchConcurrently` still passes (helper untouched).

- [ ] **Step 4: Verify build**

Run: `go vet ./... && go build ./...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add internal/api/browse/filter_index.go internal/api/browse/filter_cache_test.go
git commit -m "browse: route legacy /filter-index through cache snapshot"
```

---

## Task 6: Client — split facet fetch into catalog + matrix

**Files:**
- Modify: [internal/static/browse_scene.gohtml:1726-1950](../../../internal/static/browse_scene.gohtml#L1726-L1950) — state vars + `ensureFacetIndex`.
- Modify: [internal/static/browse_scene.gohtml:2062-2129](../../../internal/static/browse_scene.gohtml#L2062-L2129) — `visibleFacetIDSet` + `narrowedCatalog`.
- Modify: [internal/static/browse_scene.gohtml:2957-2978](../../../internal/static/browse_scene.gohtml#L2957-L2978) — `onBrowsePanelOpen`.

No automated tests; verify via build + manual run.

### Step 1: Update `browseState` shape

Read [internal/static/browse_scene.gohtml:1726-1733](../../../internal/static/browse_scene.gohtml#L1726-L1733). Replace the three `facetIndex*` fields with four (catalog + matrix have independent status):

Find:
```js
      facetIndex: null,
      facetIndexStatus: 'idle',                                      // 'idle' | 'loading' | 'ready' | 'failed'
      facetIndexPromise: null,
```

Replace with:
```js
      facetCatalog: null,                                            // { performers, studios, tags }
      facetCatalogStatus: 'idle',                                    // 'idle' | 'loading' | 'ready' | 'failed'
      facetCatalogPromise: null,
      facetCatalogEtag: '',                                          // weak ETag from /browse/filter-catalog
      facetMatrix: null,                                             // { scenes: [...] }
      facetMatrixStatus: 'idle',                                     // same enum as catalog
      facetMatrixPromise: null,
      facetMatrixEtag: '',
```

- [ ] **Step 2: Replace `ensureFacetIndex` with two new functions**

Find the existing `ensureFacetIndex` block at lines 1918-1951. Replace it with two helpers (preserve the surrounding `function ensureCachedOptions` above and whatever follows below):

```js
    function ensureFacetCatalog() {
      if (browseState.facetCatalogStatus === 'ready') {
        return Promise.resolve(browseState.facetCatalog);
      }
      if (browseState.facetCatalogStatus === 'failed') {
        return Promise.resolve(null);
      }
      if (browseState.facetCatalogPromise) {
        return browseState.facetCatalogPromise;
      }
      browseState.facetCatalogStatus = 'loading';
      const headers = { 'Accept': 'application/json' };
      if (browseState.facetCatalogEtag) {
        headers['If-None-Match'] = browseState.facetCatalogEtag;
      }
      browseState.facetCatalogPromise = fetch('/browse/filter-catalog', { headers: headers })
        .then(function(r) {
          if (r.status === 304) {
            // Snapshot unchanged — keep the existing cached payload.
            return browseState.facetCatalog;
          }
          if (!r.ok) throw new Error('filter-catalog ' + r.status);
          browseState.facetCatalogEtag = r.headers.get('ETag') || '';
          return r.json();
        })
        .then(function(json) {
          browseState.facetCatalog = json || null;
          browseState.facetCatalogStatus = 'ready';
          return browseState.facetCatalog;
        })
        .catch(function(err) {
          console.warn('stash-vr: filter-catalog fetch failed', err);
          browseState.facetCatalog = null;
          browseState.facetCatalogStatus = 'failed';
          return null;
        })
        .finally(function() {
          browseState.facetCatalogPromise = null;
        });
      return browseState.facetCatalogPromise;
    }

    function ensureFacetMatrix() {
      if (browseState.facetMatrixStatus === 'ready') {
        return Promise.resolve(browseState.facetMatrix);
      }
      if (browseState.facetMatrixStatus === 'failed') {
        return Promise.resolve(null);
      }
      if (browseState.facetMatrixPromise) {
        return browseState.facetMatrixPromise;
      }
      browseState.facetMatrixStatus = 'loading';
      const headers = { 'Accept': 'application/json' };
      if (browseState.facetMatrixEtag) {
        headers['If-None-Match'] = browseState.facetMatrixEtag;
      }
      browseState.facetMatrixPromise = fetch('/browse/filter-matrix', { headers: headers })
        .then(function(r) {
          if (r.status === 304) {
            return browseState.facetMatrix;
          }
          if (!r.ok) throw new Error('filter-matrix ' + r.status);
          browseState.facetMatrixEtag = r.headers.get('ETag') || '';
          return r.json();
        })
        .then(function(json) {
          browseState.facetMatrix = json || null;
          browseState.facetMatrixStatus = 'ready';
          return browseState.facetMatrix;
        })
        .catch(function(err) {
          console.warn('stash-vr: filter-matrix fetch failed', err);
          browseState.facetMatrix = null;
          browseState.facetMatrixStatus = 'failed';
          return null;
        })
        .finally(function() {
          browseState.facetMatrixPromise = null;
        });
      return browseState.facetMatrixPromise;
    }
```

- [ ] **Step 3: Update `facetIndexOptionsForKind` and friends**

Replace the existing `facetIndexOptionsForKind`, `currentColumnOptions`, `visibleFacetIDSet`, `narrowedCatalog` block at lines 2062-2129 with:

```js
    function facetCatalogOptionsForKind(kind) {
      if (!browseState.facetCatalog) return [];
      if (kind === 'performer') return browseState.facetCatalog.performers || [];
      if (kind === 'studio') return browseState.facetCatalog.studios || [];
      if (kind === 'tag') return browseState.facetCatalog.tags || [];
      return [];
    }

    function currentColumnOptions(kind) {
      if (browseState.facetCatalogStatus === 'ready') return facetCatalogOptionsForKind(kind);
      if (browseState.facetCatalogStatus === 'failed') return browseState.cachedOptions[kind] || [];
      return [];
    }

    function sceneMatchesActiveFacetFilters(scene) {
      scene = scene || {};
      const performerIDs = (scene.performerIds || scene.performerIDs || []).map(String);
      const studioIDs = (scene.studioIds || scene.studioIDs || []).map(String);
      const tagIDs = (scene.tagIds || scene.tagIDs || []).map(String);
      const selectedPerformers = (browseState.filters.performer || []).map(String);
      const selectedStudios = (browseState.filters.studio || []).map(String);
      const selectedTags = (browseState.filters.tag || []).map(String);

      if (!selectedPerformers.every(id => performerIDs.indexOf(id) >= 0)) return false;
      if (!selectedStudios.every(id => studioIDs.indexOf(id) >= 0)) return false;
      if (!selectedTags.every(id => tagIDs.indexOf(id) >= 0)) return false;

      const starsMin = parseInt(browseState.filters.stars || '0', 10) || 0;
      const ocountMin = parseInt(browseState.filters.ocount || '0', 10) || 0;
      const rating100 = parseInt(scene.rating100 || 0, 10) || 0;
      const oCount = parseInt(scene.oCount || 0, 10) || 0;
      if (starsMin > 0 && rating100 <= starsMin * 20 - 1) return false;
      if (ocountMin > 0 && oCount <= ocountMin - 1) return false;
      return true;
    }

    // Returns a Set of facet IDs that are still reachable given current
    // filters, OR null when the matrix hasn't loaded yet (caller should
    // treat null as "no narrowing — show every option").
    function visibleFacetIDSet(kind) {
      if (!browseState.facetMatrix || !Array.isArray(browseState.facetMatrix.scenes)) {
        return null;
      }
      const key = kind === 'performer' ? 'performerIds'
        : kind === 'studio' ? 'studioIds'
        : kind === 'tag' ? 'tagIds'
        : null;
      const out = new Set();
      if (!key) return out;
      browseState.facetMatrix.scenes.forEach(scene => {
        if (!sceneMatchesActiveFacetFilters(scene)) return;
        const ids = scene[key] || [];
        ids.forEach(id => out.add(String(id)));
      });
      return out;
    }

    function narrowedCatalog(kind) {
      const visible = visibleFacetIDSet(kind);
      if (visible === null) {
        // Matrix not loaded yet — show all catalog options (no dimming).
        return facetCatalogOptionsForKind(kind);
      }
      return facetCatalogOptionsForKind(kind).filter(opt => visible.has(String(opt.id)));
    }
```

- [ ] **Step 4: Migrate remaining `facetIndex*` references in `renderColumnList` and `maxListScrollY`**

Two more sites need updating. **Rule:** "ready to show row text" depends on **catalog**; "ready to apply intersection dimming" depends on **matrix**. Since `narrowedCatalog` already returns the full catalog when matrix isn't loaded, both helpers should gate on catalog status.

In `renderColumnList` (existing at lines 2234-2251 in the file as it stands), replace:

```js
      if (browseState.facetIndexStatus === 'loading') {
        _columnStateHash[kind] = null;
        renderColumnMessage(root, colTheta, colLen, 'Loading…');
        return;
      }
      if (browseState.facetIndexStatus === 'failed') {
        ensureCachedOptions(kind).then(opts => {
          if (!root.parentNode) return; // panel torn down
          renderColumnOptions(root, kind, colTheta, colLen, opts || []);
        });
        return;
      }
      if (browseState.facetIndexStatus !== 'ready') {
        _columnStateHash[kind] = null;
        renderColumnMessage(root, colTheta, colLen, 'Loading…');
        return;
      }
      renderColumnOptions(root, kind, colTheta, colLen, narrowedCatalog(kind));
```

with:

```js
      if (browseState.facetCatalogStatus === 'loading' || browseState.facetCatalogStatus === 'idle') {
        _columnStateHash[kind] = null;
        renderColumnMessage(root, colTheta, colLen, 'Loading…');
        return;
      }
      if (browseState.facetCatalogStatus === 'failed') {
        ensureCachedOptions(kind).then(opts => {
          if (!root.parentNode) return; // panel torn down
          renderColumnOptions(root, kind, colTheta, colLen, opts || []);
        });
        return;
      }
      // Catalog ready: render rows immediately. narrowedCatalog returns the
      // full list while matrix is still loading (no intersection dimming),
      // then automatically narrows once the matrix arrives and the column
      // is re-rendered.
      renderColumnOptions(root, kind, colTheta, colLen, narrowedCatalog(kind));
```

In `maxListScrollY` (existing at lines 2279-2287), replace:

```js
      if (browseState.facetIndexStatus === 'ready') {
        opts = narrowedCatalog(kind);
      } else if (browseState.facetIndexStatus === 'failed') {
        opts = browseState.cachedOptions[kind] || [];
      }
```

with:

```js
      if (browseState.facetCatalogStatus === 'ready') {
        opts = narrowedCatalog(kind);
      } else if (browseState.facetCatalogStatus === 'failed') {
        opts = browseState.cachedOptions[kind] || [];
      }
```

After these two edits, `facetIndex*` should no longer be referenced anywhere in the file. Confirm via grep:

```bash
grep -n "facetIndex" internal/static/browse_scene.gohtml
```

Expected: no matches.

- [ ] **Step 5: Update `onBrowsePanelOpen`**

Find the existing block at lines 2973-2977. Replace:

```js
      fetchGrid(true);
      ensureFacetIndex().finally(function() {
        ['performer', 'tag', 'studio'].forEach(renderColumnList);
      });
      ['performer', 'tag', 'studio'].forEach(renderColumnList);
```

with:

```js
      fetchGrid(true);
      ensureFacetCatalog().finally(function() {
        ['performer', 'tag', 'studio'].forEach(renderColumnList);
      });
      ensureFacetMatrix().finally(function() {
        ['performer', 'tag', 'studio'].forEach(renderColumnList);
      });
      ['performer', 'tag', 'studio'].forEach(renderColumnList);
```

The catalog promise resolves first → sidebar entity rows appear. The matrix promise resolves later → intersection-aware dimming kicks in via the same render path.

- [ ] **Step 6: Verify build**

Run: `go vet ./... && go build ./...`
Expected: clean (no Go changes here; this confirms nothing accidentally broke).

- [ ] **Step 7: Manual smoke-test in browser/headset**

Build (`scripts\build-windows.bat`), run the server, open the in-VR browse panel. Confirm:
1. Performer / studio / tag columns populate visibly before the heavier matrix data lands.
2. Network panel shows two separate requests: `/browse/filter-catalog` and `/browse/filter-matrix`.
3. After closing and reopening the panel within the TTL window, both should return `304 Not Modified`.

- [ ] **Step 8: Commit**

```bash
git add internal/static/browse_scene.gohtml
git commit -m "browse: split client facet fetch into catalog+matrix with ETag"
```

---

## Task 7: Client — two-pass tile render (stubs + hydration)

**Files:**
- Modify: [internal/static/browse_scene.gohtml:2735-2954](../../../internal/static/browse_scene.gohtml#L2735-L2954) — `relayoutTiles`.

Goal: When a new tile slot needs an entity, build a cheap stub (cylinder + bg color, no texture / no title / no badge / no handlers) synchronously; queue the full hydration step on a `requestAnimationFrame` so each frame has bounded work.

### Step 1: Add a hydration queue at the top of the script

Just below the `browseState` declaration (~ line 1733), add:

```js
    const tileHydrationQueue = [];   // [{el, tile, geom}, ...]
    let tileHydrationScheduled = false;

    function scheduleTileHydration() {
      if (tileHydrationScheduled) return;
      tileHydrationScheduled = true;
      requestAnimationFrame(processTileHydrationFrame);
    }

    function processTileHydrationFrame() {
      tileHydrationScheduled = false;
      // Bound work-per-frame so 60Hz holds even under heavy hydration load.
      const HYDRATION_BUDGET_PER_FRAME = 4;
      let budget = HYDRATION_BUDGET_PER_FRAME;
      while (budget > 0 && tileHydrationQueue.length > 0) {
        const job = tileHydrationQueue.shift();
        try {
          hydrateTile(job.el, job.tile, job.geom);
        } catch (e) {
          console.warn('stash-vr: hydrateTile failed', e);
        }
        budget--;
      }
      if (tileHydrationQueue.length > 0) {
        scheduleTileHydration();
      }
    }
```

- [ ] **Step 2: Extract a `buildTileStub` helper**

In `relayoutTiles` the "tile not found, build new" branch (currently at line 2786 onward) does all the work. Split it. Just before `function relayoutTiles()` at ~ line 2735, add helpers:

```js
    // Build the cheapest possible tile entity — cylinder cover with a
    // solid background color and the right theta range. No texture load,
    // no title chars, no badge, no hover handlers. Returns the entity
    // ready to append; the full content arrives via hydrateTile().
    function buildTileStub(tile, geom) {
      const el = document.createElement('a-entity');
      el.classList.add('vr-tile');
      el.dataset.sceneId = tile.id;
      el.dataset.projection = JSON.stringify(tile.projection);
      el.dataset.streamUrl = '/browse/scene/' + encodeURIComponent(tile.id) + '/stream';
      el.dataset.previewUrl = '/browse/scene/' + encodeURIComponent(tile.id) + '/preview';
      el.dataset.thumbnailUrl = tile.thumbnailURL || '';
      el.dataset.tileTitle = tile.title || '';
      el.dataset.hydrationState = 'stub';

      const plane = document.createElement('a-cylinder');
      plane.classList.add('vr-btn', 'vr-tile-cover');
      plane.setAttribute('radius', ARC_R);
      plane.setAttribute('height', geom.tileCoverH);
      plane.setAttribute('open-ended', 'true');
      // Stub material: flat dark grey, no texture. Hydration swaps in the
      // real cover URL via material src once we get to this tile's slot
      // in the queue.
      plane.setAttribute('material',
        'shader: flat; side: double; color: #222');
      plane.setAttribute('theta-start',  geom.thetaStartDeg);
      plane.setAttribute('theta-length', geom.thetaLengthDeg);
      el.appendChild(plane);
      return el;
    }

    // Hydrate a stub: swap in the real cover texture, build the curved
    // title characters, add the ⓘ detail badge, wire hover/preview
    // handlers. Skipped if the element has already been hydrated for the
    // bound tile id.
    function hydrateTile(el, tile, geom) {
      if (!el || !el.parentNode) return;
      if (el.dataset.hydrationState === 'full' && el.dataset.sceneId === tile.id) return;

      // Rebind in case the pool slot now points to a different tile id.
      el.dataset.sceneId = tile.id;
      el.dataset.projection = JSON.stringify(tile.projection);
      el.dataset.streamUrl = '/browse/scene/' + encodeURIComponent(tile.id) + '/stream';
      el.dataset.previewUrl = '/browse/scene/' + encodeURIComponent(tile.id) + '/preview';
      el.dataset.thumbnailUrl = tile.thumbnailURL || '';
      el.dataset.tileTitle = tile.title || '';

      // Real cover texture.
      const plane = el.querySelector('a-cylinder.vr-tile-cover');
      if (plane && tile.thumbnailURL) {
        plane.setAttribute('material',
          'shader: flat; side: double; color: #fff; src: url(' + tile.thumbnailURL + ')');
        plane.addEventListener('materialtextureloaded', function onLoaded() {
          const mesh = plane.getObject3D('mesh');
          if (!mesh || !mesh.material) return;
          applyCoverMaterial(mesh.material, mesh.material.map || null);
        });
        getTileTexture(tile.thumbnailURL);
      }

      // Detail badge (build if absent; reposition if present from a prior
      // bind on a different tile but the pool slot's geometry is reused).
      let badge = el.querySelector(':scope > .vr-tile-detail');
      if (!badge) {
        badge = document.createElement('a-entity');
        badge.classList.add('vr-btn', 'vr-tile-detail');
        badge.setAttribute('geometry', 'primitive:circle;radius:' + geom.badgeR.toFixed(3));
        badge.setAttribute('material', 'color:#000;opacity:0.85;shader:flat');
        badge.setAttribute('position',
          geom.badgeLocalX.toFixed(3) + ' ' + geom.badgeLocalY.toFixed(3) + ' ' + geom.badgeLocalZ.toFixed(3));
        badge.setAttribute('rotation', '0 ' + geom.badgeYawDeg.toFixed(3) + ' 0');
        const badgeText = document.createElement('a-text');
        badgeText.setAttribute('value', 'ⓘ');
        badgeText.setAttribute('align', 'center');
        badgeText.setAttribute('color', '#fff');
        badgeText.setAttribute('width', (geom.badgeR * 8).toFixed(3));
        badgeText.setAttribute('position', '0 0 0.005');
        badge.appendChild(badgeText);
        el.appendChild(badge);
      }

      // Curved title characters.
      let titleWrap = el.querySelector(':scope > .vr-tile-title');
      if (!titleWrap) {
        titleWrap = document.createElement('a-entity');
        titleWrap.classList.add('vr-tile-title');
        el.appendChild(titleWrap);
      } else {
        while (titleWrap.firstChild) titleWrap.removeChild(titleWrap.firstChild);
      }
      placeCurvedString(titleWrap, tile.title, {
        R: ARC_R,
        thetaStartDeg:  geom.thetaStartDeg,
        thetaLengthDeg: geom.thetaLengthDeg,
        color: '#fff',
        y: geom.titleY,
      }, 'curved-title-char');

      // Hover handlers — attach exactly once per element.
      if (!el._previewHandlersAttached) {
        attachPreviewHandlers(el);
        el._previewHandlersAttached = true;
      }

      el.dataset.hydrationState = 'full';
      el.dataset.geomSig = geom.thetaStartDeg.toFixed(2) + '|' +
                           geom.thetaLengthDeg.toFixed(2) + '|' +
                           geom.tileCoverH.toFixed(3);
    }
```

- [ ] **Step 3: Rewire `relayoutTiles` to use stubs + queue hydration**

In `relayoutTiles` (line 2735+), find the "new tile" branch (starts around line 2786 with `if (!el) {`). Replace the entire `if (!el) { ... }` block with the call to `buildTileStub` + enqueue:

```js
        const geom = {
          thetaStartDeg: pos.thetaStartDeg,
          thetaLengthDeg: pos.thetaLengthDeg,
          tileCoverH: tileCoverH,
          badgeR: badgeR,
          badgeLocalX: badgeLocalX,
          badgeLocalY: badgeLocalY,
          badgeLocalZ: badgeLocalZ,
          badgeYawDeg: badgeYawDeg,
          titleY: titleY,
        };
        let el = root.querySelector('a-entity[data-scene-id="' + CSS.escape(tile.id) + '"]');
        if (!el) {
          el = buildTileStub(tile, geom);
          root.appendChild(el);
          tileHydrationQueue.push({ el: el, tile: tile, geom: geom });
          scheduleTileHydration();
        } else if (el.dataset.hydrationState !== 'full') {
          // Stub already exists (created earlier this layout); ensure it's
          // still on the queue.
          tileHydrationQueue.push({ el: el, tile: tile, geom: geom });
          scheduleTileHydration();
        } else {
          // Existing fully-hydrated tile: cols may have changed → resize cover, badge, title.
          // (keep the original "geomChanged" branch from lines ~2880-2920 verbatim here)
        }
```

Lift the original "Existing tile" branch from lines ~2880-2920 verbatim under the `else` to preserve the geom-resize logic. The position/rotation/visibility block immediately below (lines ~2921-2954) stays unchanged and applies to every iteration.

The variables `el`, `geom`, and `badgeLocalX`/`badgeLocalY`/`badgeLocalZ`/`badgeYawDeg`/`titleY` are declared above (in the outer `forEach`'s body around lines 2761-2783) — the geom object now packages them so `hydrateTile` doesn't need a long parameter list.

- [ ] **Step 4: Verify build**

Run: `go vet ./... && go build ./...`
Expected: clean.

- [ ] **Step 5: Manual smoke-test**

Run the server, open the in-VR browse panel. Watch for:
1. Tiles appear immediately with a solid grey background.
2. Cover textures fade in over the next ~ second as the queue drains.
3. Titles + ⓘ badge appear as each tile hydrates.
4. Scrolling still works; no broken tiles.

- [ ] **Step 6: Commit**

```bash
git add internal/static/browse_scene.gohtml
git commit -m "browse: render tile stubs immediately, hydrate across frames"
```

---

## Task 8: Client — tile pool virtualization

**Files:**
- Modify: [internal/static/browse_scene.gohtml:2735-2954](../../../internal/static/browse_scene.gohtml#L2735-L2954) — `relayoutTiles`.

Goal: cap entity count regardless of how many tiles have been loaded by the infinite-scroll. Allocate a fixed pool sized `cols × (visibleRows + bufferRows*2)` and rebind pool slots to tile rows based on scroll position.

### Step 1: Add pool config near the tile geometry constants

Find the `LIST_ROW_H` constants block (~ line 1890) — the grid-tile constants are nearby (look for `TILE_GAP_Y`, `PANEL_USABLE_W`, `ARC_R` etc.). Add nearby:

```js
    const TILE_BUFFER_ROWS = 1;  // rows above + below the visible band kept in the pool
```

- [ ] **Step 2: Compute the visible row window in `relayoutTiles`**

At the top of `relayoutTiles` (just after the `tileH` / `badgeR` computations around line 2743), add:

```js
      // Visible band height in row units. Used to size the pool window
      // and to skip tiles that would never be on-screen.
      const visibleH = TILE_AREA_TOP_Y - TILE_AREA_BOTTOM_Y;
      const visibleRows = Math.max(1, Math.ceil(visibleH / (tileH + TILE_GAP_Y)));
      const poolRowSpan = visibleRows + TILE_BUFFER_ROWS * 2;
      // First fully-or-partially-visible logical row given current scroll.
      // tickBrowseScroll keeps `browseState.scrollY` in [-browseMaxScroll, 0]
      // where 0 = top of content; higher rows scroll up out of band.
      const scrollY = (browseState && typeof browseState.scrollY === 'number') ? browseState.scrollY : 0;
      // Row pitch = tileH + TILE_GAP_Y; row 0 sits at TILE_AREA_TOP_Y - tileH/2.
      const rowPitch = tileH + TILE_GAP_Y;
      const firstVisibleRow = Math.max(0, Math.floor((-scrollY) / rowPitch) - TILE_BUFFER_ROWS);
      const lastVisibleRow = firstVisibleRow + poolRowSpan - 1;
      const firstVisibleIdx = firstVisibleRow * cols;
      const lastVisibleIdx = Math.min(browseState.tiles.length - 1, (lastVisibleRow + 1) * cols - 1);
```

Note: if the existing code uses a different scroll variable name (e.g. `browseScrollY`), substitute accordingly. Inspect [internal/static/browse_scene.gohtml:3293-3370](../../../internal/static/browse_scene.gohtml#L3293-L3370) (`tickBrowseScroll`) to confirm the variable name in this codebase, then use that.

- [ ] **Step 3: Skip tiles outside the pool window**

Wrap the `browseState.tiles.forEach((tile, i) => { ... })` loop body (line 2762 onward) so iterations outside `[firstVisibleIdx, lastVisibleIdx]` release any pool entity bound to that tile id and skip the rest:

```js
      browseState.tiles.forEach((tile, i) => {
        if (i < firstVisibleIdx || i > lastVisibleIdx) {
          // Outside the pool window — release any entity bound to this tile.
          const orphan = root.querySelector('a-entity[data-scene-id="' + CSS.escape(tile.id) + '"]');
          if (orphan && orphan.parentNode) {
            // Cancel any pending hydration job that still references it.
            for (let q = 0; q < tileHydrationQueue.length; q++) {
              if (tileHydrationQueue[q].el === orphan) {
                tileHydrationQueue.splice(q, 1);
                q--;
              }
            }
            orphan.parentNode.removeChild(orphan);
          }
          return; // skip layout/build for off-pool tiles
        }
        // ... existing per-tile geometry calc + stub/hydrate/relayout body ...
      });
```

Indent the existing forEach body under the new conditional. The "Remove tile entities for tiles no longer in state.tiles" pre-loop at lines 2747-2751 stays unchanged; this new conditional handles the in-state-but-off-pool case.

- [ ] **Step 4: Tighten the "remove entities" pre-loop**

The pre-loop at lines 2747-2751 currently removes entities whose `data-scene-id` isn't in `browseState.tiles`. With the pool change, it should also remove entities whose tile *is* in state but is now off-pool. The Step 3 conditional handles the *off-pool entity removal* on the fly, but the pre-loop becomes redundant for that case — leave it as-is; it still correctly catches the "tiles wiped on `fetchGrid(true)`" path.

- [ ] **Step 5: Verify build**

Run: `go vet ./... && go build ./...`
Expected: clean.

- [ ] **Step 6: Manual stress test**

Run the server. Open the in-VR browse panel and trigger infinite-scroll by scrolling past the bottom sentinel. Repeat 5+ times so `browseState.tiles.length > 100`. Verify:
1. `document.querySelectorAll('#vrBrowseTiles a-entity').length` stays at roughly `cols × (visibleRows + 2)` — open the WebXR inspector or eval in console.
2. Scrolling back to the top re-creates entities for the now-visible top tiles.
3. No frame drops, no MSDF text glyphs stuck in mid-air after they scroll out.

- [ ] **Step 7: Commit**

```bash
git add internal/static/browse_scene.gohtml
git commit -m "browse: virtualize tile entities with fixed pool"
```

---

## Task 9: Instrumentation — server timing + client perf marks

**Files:**
- Modify: `internal/api/browse/grid_json.go` — server-side timing logs.
- Modify: `internal/api/browse/filter_index.go` — catalog/matrix handler timing logs.
- Modify: `internal/static/browse_scene.gohtml` — `performance.mark`/`measure` calls.

### Step 1: Server-side timing on grid handler

Edit `gridJSONHandler` in `internal/api/browse/grid_json.go`. Wrap the existing logic with start/end times for each stage:

```go
func (h *httpHandler) gridJSONHandler(w http.ResponseWriter, r *http.Request) {
	startAll := time.Now()
	// ... existing parameter parsing ...

	startIds := time.Now()
	ids, total, err := fetchSceneIDsWithSize(r.Context(), h.libraryService.StashClient, sceneFilter, searchQ, page, perPage)
	findIdsMs := time.Since(startIds).Milliseconds()
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: grid fetchSceneIDs")
		http.Error(w, "fetch failed", http.StatusInternalServerError)
		return
	}

	startBatch := time.Now()
	baseURL := apiinternal.GetBaseUrl(r)
	vds, err := h.libraryService.GetScenesByIds(r.Context(), ids)
	batchFetchMs := time.Since(startBatch).Milliseconds()
	// ... existing tile-building loop ...

	startEncode := time.Now()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: encode grid")
	}
	encodeMs := time.Since(startEncode).Milliseconds()

	log.Ctx(r.Context()).Trace().
		Int64("findIdsMs", findIdsMs).
		Int64("batchFetchMs", batchFetchMs).
		Int64("encodeMs", encodeMs).
		Int64("totalMs", time.Since(startAll).Milliseconds()).
		Int("tiles", len(tiles)).
		Msg("browse: grid timing")
}
```

Add the `time` import to grid_json.go if not already present.

- [ ] **Step 2: Server-side timing on catalog and matrix handlers**

In `internal/api/browse/filter_index.go`, edit `filterCatalogHandler` and `filterMatrixHandler` to log timings:

```go
func (h *httpHandler) filterCatalogHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	payload, etag, err := h.filterCache.Catalog(r.Context(), h.libraryService.StashClient)
	gqlMs := time.Since(start).Milliseconds()
	// ... existing error + 304 handling ...

	startWrite := time.Now()
	// ... existing 200 response writing ...
	encodeMs := time.Since(startWrite).Milliseconds()

	log.Ctx(r.Context()).Trace().
		Int64("gqlMs", gqlMs).
		Int64("encodeMs", encodeMs).
		Int("bytes", len(payload)).
		Bool("cacheHit", gqlMs < 5).   // <5ms = served from snapshot
		Msg("browse: filter-catalog timing")
}
```

Same shape for `filterMatrixHandler` (use `snapshotAgeMs := time.Since(snap.builtAt).Milliseconds()` if you can expose `builtAt` — otherwise just log `gqlMs`/`encodeMs`/`bytes`).

- [ ] **Step 3: Client-side performance marks**

In `internal/static/browse_scene.gohtml`, near the top of the `<script>` block (just under `browseState`), add:

```js
    function perfMark(name) {
      try { performance.mark('vrbrowse.' + name); } catch (e) {}
    }
    function perfReport(label, fromMark, toMark) {
      try {
        const from = performance.getEntriesByName('vrbrowse.' + fromMark).pop();
        const to = performance.getEntriesByName('vrbrowse.' + toMark).pop();
        if (!from || !to) return;
        console.log('vrbrowse: ' + label + ' = ' + Math.round(to.startTime - from.startTime) + 'ms');
      } catch (e) {}
    }
```

Add `perfMark` calls at:

- `onBrowsePanelOpen` entry: `perfMark('open')`.
- `fetchGrid` JSON resolve: `perfMark('grid.json.received')`.
- After the stub pass completes in `relayoutTiles` (right before the `forEach` loop's final position/visibility block): `perfMark('grid.stubs.rendered')`.
- After the hydration queue drains in `processTileHydrationFrame` (when both `budget > 0` exits AND the queue is empty): `perfMark('grid.hydration.complete')`.
- In `ensureFacetCatalog`'s `.then(json)` after `facetCatalogStatus = 'ready'`: `perfMark('facets.catalog.received')`.
- In `ensureFacetMatrix`'s `.then(json)` after `facetMatrixStatus = 'ready'`: `perfMark('facets.matrix.received')`.

Also add a single summary log in `onBrowsePanelOpen` to be called from a `setTimeout(.., 5000)` so all marks are captured:

```js
      setTimeout(function() {
        perfReport('json wait', 'open', 'grid.json.received');
        perfReport('stub render', 'grid.json.received', 'grid.stubs.rendered');
        perfReport('hydration', 'grid.stubs.rendered', 'grid.hydration.complete');
        perfReport('catalog', 'open', 'facets.catalog.received');
        perfReport('matrix', 'open', 'facets.matrix.received');
      }, 5000);
```

- [ ] **Step 4: Verify build**

Run: `go vet ./... && go build ./...`
Expected: clean.

- [ ] **Step 5: Run all browse tests once more**

Run: `go test ./internal/api/browse/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/browse/grid_json.go internal/api/browse/filter_index.go internal/static/browse_scene.gohtml
git commit -m "browse: add server timing logs + client perf marks for open"
```

---

## Task 10: Manual acceptance + build verification

**Files:** none modified — verification step.

### Step 1: Clean build

Run: `go vet ./... && go build ./...`
Expected: clean compile, no warnings.

- [ ] **Step 2: Full test pass**

Run: `go test ./... -v`
Expected: PASS for `internal/api/browse/...` (existing + new tests); other packages have no test suite.

- [ ] **Step 3: Cold open scenario**

Restart stash-vr (server starts fresh). Open the in-VR browse panel. Confirm:
1. Grid shows stub tiles (grey rectangles) within ~one frame of the JSON response.
2. Cover textures fade in over the next ~1-2 seconds as hydration drains.
3. Sidebar performer/studio/tag rows appear before intersection-aware dimming kicks in.

Server logs should show `findIdsMs`, `batchFetchMs`, and the catalog/matrix `gqlMs` fields. Client console should print the perf summary after 5 seconds.

- [ ] **Step 4: Warm reopen scenario**

Close the panel, reopen it within 5 minutes. Confirm:
1. Catalog and matrix requests return `304 Not Modified` (check Network tab in browser devtools / headset's remote-debug session).
2. Grid JSON returns quickly (warm `vdCache`).
3. Sidebar columns appear without re-rendering visibly.

- [ ] **Step 5: Infinite-scroll stress scenario**

Open the panel, scroll past the load-more sentinel 5+ times so `browseState.tiles.length > 100`. Confirm:
1. `document.querySelectorAll('#vrBrowseTiles a-entity').length` stays at roughly `cols × (visibleRows + 2)`.
2. Frame rate stays at 72fps / 90fps (whichever the headset is configured for) during scroll.
3. Scrolling back to the top re-hydrates the now-visible top tiles correctly.

- [ ] **Step 6: Verify no regressions in adjacent surfaces**

1. 2D browse page (`http://server:9999/browse`) still works.
2. HereSphere endpoint (`/heresphere`) still serves video data.
3. Scene detail page (`/browse/scene/{id}`) still renders.

- [ ] **Step 7: Final commit (if any cleanup needed) or push**

Push the branch:
```bash
git push -u origin <branch>
```

Or merge into master per the existing workflow (per CLAUDE.md, the user's preference is a single PR-sized batch; defer to user choice via `finishing-a-development-branch` skill).

---

## Spec coverage map

| Spec section | Implementing task(s) |
|---|---|
| §1 Grid: server batch | Task 1 |
| §1 Grid: stub + hydration two-pass | Task 7 |
| §1 Grid: tile pool | Task 8 |
| §2 Filter-index: catalog endpoint | Task 3 |
| §2 Filter-index: matrix endpoint | Task 4 |
| §2 Filter-index: server snapshot, version, content hash, TTL | Task 2 |
| §2 Filter-index: ETag + `If-None-Match` | Tasks 3, 4 |
| §2 Filter-index: legacy `/filter-index` as alias | Task 5 |
| §2 Filter-index: client split fetch + graceful degradation | Task 6 |
| §3 Server timing instrumentation | Task 9 |
| §3 Client performance marks | Task 9 |
| §3 Manual acceptance scenarios | Task 10 |

Every spec requirement maps to a concrete task.
