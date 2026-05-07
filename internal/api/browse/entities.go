package browse

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"golang.org/x/sync/singleflight"

	"stash-vr/internal/config"
	"stash-vr/internal/prefix"
	"stash-vr/internal/stash/gql"
)

// entitiesGroup coalesces concurrent fetches of the same entity list.
// Keys: "browse:performers", "browse:studios", "browse:tags".
var entitiesGroup singleflight.Group

// fetchPerformers returns all performers with at least one scene.
func fetchPerformers(ctx context.Context, client graphql.Client) ([]Entity, error) {
	v, err, _ := entitiesGroup.Do("browse:performers", func() (interface{}, error) {
		// The FindPerformersWithSceneCount query uses modifier GREATER_THAN
		// on min_scene_count, so passing 0 -> Stash filters scene_count > 0,
		// i.e. performers with >=1 scene. (Caller passes min_scene_count - 1.)
		resp, ferr := gql.FindPerformersWithSceneCount(ctx, client, 0, -1)
		if ferr != nil {
			return nil, fmt.Errorf("FindPerformersWithSceneCount: %w", ferr)
		}
		out := make([]Entity, 0, len(resp.FindPerformers.Performers))
		for _, p := range resp.FindPerformers.Performers {
			if p == nil {
				continue
			}
			out = append(out, Entity{
				ID:         p.Id,
				Name:       p.Name,
				SceneCount: p.Scene_count,
			})
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]Entity), nil
}

// fetchStudios returns studios that have at least one scene.
func fetchStudios(ctx context.Context, client graphql.Client) ([]Entity, error) {
	v, err, _ := entitiesGroup.Do("browse:studios", func() (interface{}, error) {
		resp, ferr := gql.FindAllStudiosWithCount(ctx, client)
		if ferr != nil {
			return nil, fmt.Errorf("FindAllStudiosWithCount: %w", ferr)
		}
		out := make([]Entity, 0, len(resp.FindStudios.Studios))
		for _, s := range resp.FindStudios.Studios {
			if s == nil {
				continue
			}
			if s.Scene_count == 0 {
				continue
			}
			out = append(out, Entity{
				ID:         s.Id,
				Name:       s.Name,
				SceneCount: s.Scene_count,
			})
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]Entity), nil
}

// fetchTags returns tags with scene_count > 0, sorted descending by scene
// count, excluding tags whose Sort_name matches EXCLUDE_SORT_NAME or starts
// with the SvrAncestor prefix marker.
func fetchTags(ctx context.Context, client graphql.Client) ([]Entity, error) {
	v, err, _ := entitiesGroup.Do("browse:tags", func() (interface{}, error) {
		tagFilter := &gql.TagFilterType{
			Scene_count: &gql.IntCriterionInput{
				Value:    0,
				Modifier: gql.CriterionModifierGreaterThan,
			},
		}
		sortStr := "scenes_count"
		dir := gql.SortDirectionEnumDesc
		resp, ferr := gql.FindTags(ctx, client, tagFilter, &sortStr, &dir)
		if ferr != nil {
			return nil, fmt.Errorf("FindTags: %w", ferr)
		}
		excludeSort := config.Application().ExcludeSortName
		out := make([]Entity, 0, len(resp.FindTags.Tags))
		for _, t := range resp.FindTags.Tags {
			if t == nil {
				continue
			}
			sortName := t.TagParts.Sort_name
			if excludeSort != "" && sortName == excludeSort {
				continue
			}
			if strings.HasPrefix(sortName, prefix.SvrAncestor) {
				continue
			}
			out = append(out, Entity{
				ID:         t.TagParts.Id,
				Name:       t.TagParts.Name,
				SceneCount: t.Scene_count,
			})
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]Entity), nil
}

// LoadSidebar runs the three entity fetches concurrently and assembles a
// SidebarData. activeTab defaults to "perf" if empty. Any fetch error is
// returned (the first one encountered).
func LoadSidebar(ctx context.Context, client graphql.Client, activeTab, activeID string) (SidebarData, error) {
	if activeTab == "" {
		activeTab = "perf"
	}

	type result struct {
		kind    string
		entries []Entity
		err     error
	}
	ch := make(chan result, 3)

	go func() {
		p, err := fetchPerformers(ctx, client)
		ch <- result{kind: "perf", entries: p, err: err}
	}()
	go func() {
		s, err := fetchStudios(ctx, client)
		ch <- result{kind: "studio", entries: s, err: err}
	}()
	go func() {
		t, err := fetchTags(ctx, client)
		ch <- result{kind: "tag", entries: t, err: err}
	}()

	var data SidebarData
	data.ActiveTab = activeTab
	data.ActiveID = activeID

	for i := 0; i < 3; i++ {
		r := <-ch
		if r.err != nil {
			return SidebarData{}, r.err
		}
		switch r.kind {
		case "perf":
			data.Performers = r.entries
		case "studio":
			data.Studios = r.entries
		case "tag":
			data.Tags = r.entries
		}
	}

	return data, nil
}
