package browse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"stash-vr/internal/config"
	"stash-vr/internal/prefix"
	"stash-vr/internal/stash/gql"
)

type facetSceneSeed struct {
	ID           string
	PerformerIDs []string
	StudioID     string
	TagIDs       []string
	Rating100    int
	OCount       int
}

type filterIndexCatalog struct {
	sidebar             SidebarData
	selectableStudioIDs map[string]struct{}
	selectableTagIDs    map[string]struct{}
	studioParentsByID   map[string]string
	tagParentsByID      map[string][]string
}

type loadFilterIndexCatalogFunc func(context.Context, graphql.Client) (filterIndexCatalog, error)
type loadFilterIndexScenesFunc func(context.Context, graphql.Client) (*gql.FindScenesForFacetIndexResponse, error)

type filterIndexLoadError struct {
	source string
	err    error
}

func (e *filterIndexLoadError) Error() string {
	return e.err.Error()
}

func (e *filterIndexLoadError) Unwrap() error {
	return e.err
}

func buildFilterIndexPayload(
	performers, studios, tags []Entity,
	scenes []facetSceneSeed,
	selectableStudioIDs, selectableTagIDs map[string]struct{},
	studioParentsByID map[string]string,
	tagParentsByID map[string][]string,
) FilterIndexResponse {
	out := FilterIndexResponse{
		Performers: make([]FilterOption, 0, len(performers)),
		Studios:    make([]FilterOption, 0, len(studios)),
		Tags:       make([]FilterOption, 0, len(tags)),
		Scenes:     make([]FilterIndexScene, 0, len(scenes)),
	}

	for _, performer := range performers {
		out.Performers = append(out.Performers, FilterOption{ID: performer.ID, Name: performer.Name})
	}
	for _, studio := range studios {
		out.Studios = append(out.Studios, FilterOption{ID: studio.ID, Name: studio.Name})
	}
	for _, tag := range tags {
		out.Tags = append(out.Tags, FilterOption{ID: tag.ID, Name: tag.Name})
	}

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

func appendUniqueIDs(dst []string, seen map[string]struct{}, ids ...string) []string {
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		dst = append(dst, id)
	}
	return dst
}

func expandedStudioIDs(sceneStudioID string, selectableStudioIDs map[string]struct{}, studioParentsByID map[string]string) []string {
	if sceneStudioID == "" {
		return nil
	}
	out := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	for id := sceneStudioID; id != ""; id = studioParentsByID[id] {
		if _, loop := seen[id]; loop {
			break
		}
		if _, ok := selectableStudioIDs[id]; ok {
			out = appendUniqueIDs(out, seen, id)
			continue
		}
		seen[id] = struct{}{}
	}
	return out
}

func expandedTagIDs(sceneTagIDs []string, selectableTagIDs map[string]struct{}, tagParentsByID map[string][]string) []string {
	out := make([]string, 0, len(sceneTagIDs))
	seen := make(map[string]struct{}, len(sceneTagIDs))
	queue := append([]string(nil), sceneTagIDs...)
	for len(queue) > 0 {
		tagID := queue[0]
		queue = queue[1:]
		if tagID == "" {
			continue
		}
		if _, ok := seen[tagID]; ok {
			continue
		}
		seen[tagID] = struct{}{}
		if _, ok := selectableTagIDs[tagID]; ok {
			out = append(out, tagID)
		}
		queue = append(queue, tagParentsByID[tagID]...)
	}
	return out
}

func buildSelectableIDSet(entries []Entity) map[string]struct{} {
	out := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		out[entry.ID] = struct{}{}
	}
	return out
}

func loadFilterIndexCatalog(ctx context.Context, client graphql.Client) (filterIndexCatalog, error) {
	var (
		performers        []Entity
		studios           []Entity
		tags              []Entity
		studioParentsByID = map[string]string{}
		tagParentsByID    = map[string][]string{}
	)

	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		data, err := fetchPerformers(groupCtx, client)
		if err != nil {
			return err
		}
		performers = data
		return nil
	})
	g.Go(func() error {
		data, err := fetchStudiosDetailed(groupCtx, client)
		if err != nil {
			return err
		}
		out := make([]Entity, 0, len(data))
		for _, studio := range data {
			if studio.ParentID != "" {
				studioParentsByID[studio.ID] = studio.ParentID
			}
			out = append(out, studio.Entity)
		}
		studios = out
		return nil
	})
	g.Go(func() error {
		data, err := fetchTagsDetailed(groupCtx, client)
		if err != nil {
			return err
		}
		out := make([]Entity, 0, len(data))
		for _, tag := range data {
			if tag.SortName == config.Application().ExcludeSortName {
				continue
			}
			if strings.HasPrefix(tag.SortName, prefix.SvrAncestor) {
				continue
			}
			out = append(out, tag.Entity)
		}
		tags = out
		return nil
	})
	g.Go(func() error {
		resp, err := gql.FindAllTags(groupCtx, client)
		if err != nil {
			return fmt.Errorf("FindAllTags: %w", err)
		}
		for _, tag := range resp.FindTags.Tags {
			if tag == nil || len(tag.Parents) == 0 {
				continue
			}
			parentIDs := make([]string, 0, len(tag.Parents))
			for _, parent := range tag.Parents {
				if parent == nil {
					continue
				}
				parentIDs = append(parentIDs, parent.Id)
			}
			if len(parentIDs) > 0 {
				tagParentsByID[tag.Id] = parentIDs
			}
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return filterIndexCatalog{}, err
	}

	sidebar := SidebarData{
		Performers: performers,
		Studios:    studios,
		Tags:       tags,
		ActiveTab:  "perf",
	}
	return filterIndexCatalog{
		sidebar:             sidebar,
		selectableStudioIDs: buildSelectableIDSet(studios),
		selectableTagIDs:    buildSelectableIDSet(tags),
		studioParentsByID:   studioParentsByID,
		tagParentsByID:      tagParentsByID,
	}, nil
}

func loadFilterIndexData(ctx context.Context, client graphql.Client) (filterIndexCatalog, *gql.FindScenesForFacetIndexResponse, error) {
	return loadFilterIndexDataWithFns(ctx, client, loadFilterIndexCatalog, gql.FindScenesForFacetIndex)
}

func loadFilterIndexDataWithFns(ctx context.Context, client graphql.Client, catalogLoader loadFilterIndexCatalogFunc, scenesLoader loadFilterIndexScenesFunc) (filterIndexCatalog, *gql.FindScenesForFacetIndexResponse, error) {
	var (
		catalog filterIndexCatalog
		resp    *gql.FindScenesForFacetIndexResponse
	)

	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		data, err := catalogLoader(groupCtx, client)
		if err != nil {
			return &filterIndexLoadError{source: "catalog", err: err}
		}
		catalog = data
		return nil
	})
	g.Go(func() error {
		data, err := scenesLoader(groupCtx, client)
		if err != nil {
			return &filterIndexLoadError{source: "scenes", err: err}
		}
		resp = data
		return nil
	})

	if err := g.Wait(); err != nil {
		return filterIndexCatalog{}, nil, err
	}
	return catalog, resp, nil
}

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
