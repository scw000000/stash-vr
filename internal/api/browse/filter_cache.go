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
