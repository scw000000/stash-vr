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
	StudioID     string
	TagIDs       []string
	Rating100    int
	OCount       int
}

func buildFilterIndexPayload(performers, studios, tags []Entity, scenes []facetSceneSeed, selectableTagIDs map[string]struct{}) FilterIndexResponse {
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
			item.StudioIDs = []string{scene.StudioID}
		}
		if len(scene.TagIDs) > 0 {
			item.TagIDs = make([]string, 0, len(scene.TagIDs))
			for _, tagID := range scene.TagIDs {
				if _, ok := selectableTagIDs[tagID]; !ok {
					continue
				}
				item.TagIDs = append(item.TagIDs, tagID)
			}
			if len(item.TagIDs) == 0 {
				item.TagIDs = nil
			}
		}
		out.Scenes = append(out.Scenes, item)
	}

	return out
}

func (h *httpHandler) filterIndexHandler(w http.ResponseWriter, r *http.Request) {
	performers, err := fetchPerformers(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-index fetch performers")
		http.Error(w, "fetch performers failed", http.StatusInternalServerError)
		return
	}
	studios, err := fetchStudios(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-index fetch studios")
		http.Error(w, "fetch studios failed", http.StatusInternalServerError)
		return
	}
	tags, err := fetchTags(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-index fetch tags")
		http.Error(w, "fetch tags failed", http.StatusInternalServerError)
		return
	}

	resp, err := gql.FindScenesForFacetIndex(r.Context(), h.libraryService.StashClient)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-index fetch scenes")
		http.Error(w, "fetch scenes failed", http.StatusInternalServerError)
		return
	}

	selectableTagIDs := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		selectableTagIDs[tag.ID] = struct{}{}
	}

	scenes := make([]facetSceneSeed, 0, len(resp.FindScenes.Scenes))
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
		scenes = append(scenes, item)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(buildFilterIndexPayload(performers, studios, tags, scenes, selectableTagIDs)); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: encode filter-index")
	}
}
