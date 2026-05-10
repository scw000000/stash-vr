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

// gridJSONHandler serves GET /browse/grid?... — the in-VR browser's
// JSON tile feed. Filter params (performer/studio/tag/favorite/stars/
// ocount) compose into a SceneFilterType; q is fed to the FindFilter's
// full-text search; cursor is a 1-indexed page number.
func (h *httpHandler) gridJSONHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	searchQ := q.Get("q")
	// Multi-value: in-VR picker allows AND-selection across multiple
	// performers/studios/tags. The Stash GraphQL filters take string slices.
	performers := q["performer"]
	studios := q["studio"]
	tags := q["tag"]
	favorite := q.Get("favorite")
	starsMin, _ := strconv.Atoi(q.Get("stars"))
	ocountMin, _ := strconv.Atoi(q.Get("ocount"))
	page, _ := strconv.Atoi(q.Get("cursor"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage <= 0 {
		perPage = 20 // default in-VR batch size; smaller than 2D pageSize=30
	}
	if perPage > 60 {
		perPage = 60
	}
	_ = favorite // favorite filter deferred — see plan note.

	sceneFilter := buildGridFilter(performers, studios, tags, starsMin, ocountMin)
	ids, total, err := fetchSceneIDsWithSize(r.Context(), h.libraryService.StashClient, sceneFilter, searchQ, page, perPage)
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
		HasMore: page*perPage < total,
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
// kind is a slice (multi-select); empty slice means "no filter for that
// kind". When everything is empty/zero the function returns nil so
// fetchSceneIDs runs without a sceneFilter.
//
// Stars: Stash's rating100 uses 0-100; map starsMin (1..5) to a
// strict-greater-than threshold of starsMin*20 - 1 so a filter of
// "3+ stars" matches scenes with rating100 >= 60.
//
// O-count: same trick — strict-greater-than (ocountMin - 1) is
// equivalent to "at least ocountMin".
//
// MultiCriterionInput / HierarchicalMultiCriterionInput with multiple
// IDs in Value behave as AND across the IDs (Stash's "Includes" modifier
// requires all listed IDs on the scene), which matches the in-VR UX
// expectation of "scenes with Alice AND Bob" / "scenes tagged POV AND
// Outdoor".
func buildGridFilter(performers, studios, tags []string, starsMin, ocountMin int) *gql.SceneFilterType {
	if len(performers) == 0 && len(studios) == 0 && len(tags) == 0 && starsMin == 0 && ocountMin == 0 {
		return nil
	}
	f := &gql.SceneFilterType{}
	if len(performers) > 0 {
		f.Performers = &gql.MultiCriterionInput{
			Value:    performers,
			Modifier: gql.CriterionModifierIncludes,
		}
	}
	if len(studios) > 0 {
		f.Studios = &gql.HierarchicalMultiCriterionInput{
			Value:    studios,
			Modifier: gql.CriterionModifierIncludes,
			Depth:    util.Ptr(-1),
		}
	}
	if len(tags) > 0 {
		f.Tags = &gql.HierarchicalMultiCriterionInput{
			Value:    tags,
			Modifier: gql.CriterionModifierIncludes,
			Depth:    util.Ptr(-1),
		}
	}
	if starsMin > 0 {
		f.Rating100 = &gql.IntCriterionInput{
			Value:    starsMin*20 - 1,
			Modifier: gql.CriterionModifierGreaterThan,
		}
	}
	if ocountMin > 0 {
		f.O_counter = &gql.IntCriterionInput{
			Value:    ocountMin - 1,
			Modifier: gql.CriterionModifierGreaterThan,
		}
	}
	return f
}

// filterOptionsHandler serves GET /browse/filter-options/{kind} — a
// flat JSON list of {id, name} objects for the in-VR filter pickers.
// kind is one of "performer"/"perf", "studio", or "tag".
func (h *httpHandler) filterOptionsHandler(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	// Normalize "performer" -> the internal "perf" kind LoadSidebar uses
	// for activeTab. The activeTab argument doesn't change which lists
	// are populated, so this is mostly cosmetic, but keeps the hint
	// internally consistent.
	tabHint := kind
	if tabHint == "performer" {
		tabHint = "perf"
	}
	sb, err := LoadSidebar(r.Context(), h.libraryService.StashClient, tabHint, "")
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: load sidebar (filter-options)")
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
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: encode filter-options")
	}
}
