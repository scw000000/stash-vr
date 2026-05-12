package browse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Khan/genqlient/graphql"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
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

type loadFilterIndexSidebarFunc func(context.Context, graphql.Client, string, string) (SidebarData, error)
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

func loadFilterIndexData(ctx context.Context, client graphql.Client) (SidebarData, *gql.FindScenesForFacetIndexResponse, error) {
	return loadFilterIndexDataWithFns(ctx, client, LoadSidebar, gql.FindScenesForFacetIndex)
}

func loadFilterIndexDataWithFns(ctx context.Context, client graphql.Client, sidebarLoader loadFilterIndexSidebarFunc, scenesLoader loadFilterIndexScenesFunc) (SidebarData, *gql.FindScenesForFacetIndexResponse, error) {
	var (
		sidebar SidebarData
		resp    *gql.FindScenesForFacetIndexResponse
	)

	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		data, err := sidebarLoader(groupCtx, client, "", "")
		if err != nil {
			return &filterIndexLoadError{source: "sidebar", err: err}
		}
		sidebar = data
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
		return SidebarData{}, nil, err
	}
	return sidebar, resp, nil
}

func (h *httpHandler) filterIndexHandler(w http.ResponseWriter, r *http.Request) {
	sidebar, resp, err := loadFilterIndexData(r.Context(), h.libraryService.StashClient)
	if err != nil {
		var loadErr *filterIndexLoadError
		if errors.As(err, &loadErr) {
			switch loadErr.source {
			case "sidebar":
				log.Ctx(r.Context()).Err(loadErr.err).Msg("browse: filter-index load sidebar")
				http.Error(w, "load sidebar failed", http.StatusInternalServerError)
				return
			case "scenes":
				log.Ctx(r.Context()).Err(loadErr.err).Msg("browse: filter-index fetch scenes")
				http.Error(w, "fetch scenes failed", http.StatusInternalServerError)
				return
			}
		}
		log.Ctx(r.Context()).Err(err).Msg("browse: filter-index load data")
		http.Error(w, "load filter index failed", http.StatusInternalServerError)
		return
	}

	selectableTagIDs := make(map[string]struct{}, len(sidebar.Tags))
	for _, tag := range sidebar.Tags {
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
	if err := json.NewEncoder(w).Encode(buildFilterIndexPayload(sidebar.Performers, sidebar.Studios, sidebar.Tags, scenes, selectableTagIDs)); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: encode filter-index")
	}
}
