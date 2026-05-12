package browse

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"stash-vr/internal/stash/gql"
)

func TestBuildFilterIndexPayload(t *testing.T) {
	tests := []struct {
		name                string
		performers          []Entity
		studios             []Entity
		tags                []Entity
		scenes              []facetSceneSeed
		selectableStudioIDs map[string]struct{}
		selectableTagIDs    map[string]struct{}
		studioParentsByID   map[string]string
		tagParentsByID      map[string][]string
		want                FilterIndexResponse
	}{
		{
			name: "preserves catalog order and filters non-selectable scene tags",
			performers: []Entity{
				{ID: "perf-2", Name: "Bravo"},
				{ID: "perf-1", Name: "Alpha"},
			},
			studios: []Entity{
				{ID: "studio-2", Name: "Second"},
				{ID: "studio-1", Name: "First"},
			},
			tags: []Entity{
				{ID: "tag-2", Name: "Second Tag"},
				{ID: "tag-1", Name: "First Tag"},
			},
			scenes: []facetSceneSeed{
				{
					ID:           "scene-1",
					PerformerIDs: []string{"perf-1", "perf-2"},
					StudioID:     "studio-1",
					TagIDs:       []string{"tag-1", "tag-x", "tag-2"},
					Rating100:    80,
					OCount:       3,
				},
			},
			selectableStudioIDs: map[string]struct{}{
				"studio-1": {},
				"studio-2": {},
			},
			selectableTagIDs: map[string]struct{}{
				"tag-1": {},
				"tag-2": {},
			},
			want: FilterIndexResponse{
				Performers: []FilterOption{
					{ID: "perf-2", Name: "Bravo"},
					{ID: "perf-1", Name: "Alpha"},
				},
				Studios: []FilterOption{
					{ID: "studio-2", Name: "Second"},
					{ID: "studio-1", Name: "First"},
				},
				Tags: []FilterOption{
					{ID: "tag-2", Name: "Second Tag"},
					{ID: "tag-1", Name: "First Tag"},
				},
				Scenes: []FilterIndexScene{
					{
						ID:           "scene-1",
						PerformerIDs: []string{"perf-1", "perf-2"},
						StudioIDs:    []string{"studio-1"},
						TagIDs:       []string{"tag-1", "tag-2"},
						Rating100:    80,
						OCount:       3,
					},
				},
			},
		},
		{
			name: "preserves scenes without studios and scenes with zero selectable tags",
			performers: []Entity{
				{ID: "perf-1", Name: "Alpha"},
			},
			studios: []Entity{
				{ID: "studio-1", Name: "First"},
			},
			tags: []Entity{
				{ID: "tag-1", Name: "First Tag"},
			},
			scenes: []facetSceneSeed{
				{
					ID:           "scene-no-studio",
					PerformerIDs: []string{"perf-1"},
					TagIDs:       []string{"tag-x"},
					Rating100:    0,
					OCount:       0,
				},
				{
					ID:           "scene-no-tags",
					PerformerIDs: []string{"perf-1"},
					StudioID:     "studio-1",
					Rating100:    40,
					OCount:       1,
				},
			},
			selectableStudioIDs: map[string]struct{}{
				"studio-1": {},
			},
			selectableTagIDs: map[string]struct{}{
				"tag-1": {},
			},
			want: FilterIndexResponse{
				Performers: []FilterOption{
					{ID: "perf-1", Name: "Alpha"},
				},
				Studios: []FilterOption{
					{ID: "studio-1", Name: "First"},
				},
				Tags: []FilterOption{
					{ID: "tag-1", Name: "First Tag"},
				},
				Scenes: []FilterIndexScene{
					{
						ID:           "scene-no-studio",
						PerformerIDs: []string{"perf-1"},
						Rating100:    0,
						OCount:       0,
					},
					{
						ID:           "scene-no-tags",
						PerformerIDs: []string{"perf-1"},
						StudioIDs:    []string{"studio-1"},
						Rating100:    40,
						OCount:       1,
					},
				},
			},
		},
		{
			name: "expands selectable studio and tag ancestors into scene memberships",
			performers: []Entity{
				{ID: "perf-1", Name: "Alpha"},
			},
			studios: []Entity{
				{ID: "studio-grand", Name: "Grand Studio"},
				{ID: "studio-parent", Name: "Parent Studio"},
				{ID: "studio-child", Name: "Child Studio"},
			},
			tags: []Entity{
				{ID: "tag-grand", Name: "Grand Tag"},
				{ID: "tag-parent", Name: "Parent Tag"},
				{ID: "tag-child", Name: "Child Tag"},
			},
			scenes: []facetSceneSeed{
				{
					ID:           "scene-1",
					PerformerIDs: []string{"perf-1"},
					StudioID:     "studio-child",
					TagIDs:       []string{"tag-child", "tag-hidden"},
					Rating100:    60,
					OCount:       2,
				},
			},
			selectableStudioIDs: map[string]struct{}{
				"studio-grand":  {},
				"studio-parent": {},
				"studio-child":  {},
			},
			selectableTagIDs: map[string]struct{}{
				"tag-grand":  {},
				"tag-parent": {},
				"tag-child":  {},
			},
			studioParentsByID: map[string]string{
				"studio-child":  "studio-parent",
				"studio-parent": "studio-grand",
				"studio-grand":  "studio-hidden-root",
			},
			tagParentsByID: map[string][]string{
				"tag-child":  {"tag-parent", "tag-hidden-root"},
				"tag-parent": {"tag-grand"},
				"tag-hidden": {"tag-parent"},
			},
			want: FilterIndexResponse{
				Performers: []FilterOption{
					{ID: "perf-1", Name: "Alpha"},
				},
				Studios: []FilterOption{
					{ID: "studio-grand", Name: "Grand Studio"},
					{ID: "studio-parent", Name: "Parent Studio"},
					{ID: "studio-child", Name: "Child Studio"},
				},
				Tags: []FilterOption{
					{ID: "tag-grand", Name: "Grand Tag"},
					{ID: "tag-parent", Name: "Parent Tag"},
					{ID: "tag-child", Name: "Child Tag"},
				},
				Scenes: []FilterIndexScene{
					{
						ID:           "scene-1",
						PerformerIDs: []string{"perf-1"},
						StudioIDs:    []string{"studio-child", "studio-parent", "studio-grand"},
						TagIDs:       []string{"tag-child", "tag-parent", "tag-grand"},
						Rating100:    60,
						OCount:       2,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFilterIndexPayload(
				tt.performers,
				tt.studios,
				tt.tags,
				tt.scenes,
				tt.selectableStudioIDs,
				tt.selectableTagIDs,
				tt.studioParentsByID,
				tt.tagParentsByID,
			)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildFilterIndexPayload() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLoadFilterIndexDataRunsCatalogAndSceneFetchConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})

	catalogLoader := func(context.Context, graphql.Client) (filterIndexCatalog, error) {
		started <- "catalog"
		<-release
		return filterIndexCatalog{}, nil
	}
	sceneLoader := func(context.Context, graphql.Client) (*gql.FindScenesForFacetIndexResponse, error) {
		started <- "scenes"
		<-release
		return &gql.FindScenesForFacetIndexResponse{}, nil
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := loadFilterIndexDataWithFns(context.Background(), nil, catalogLoader, sceneLoader)
		done <- err
	}()

	seen := map[string]bool{}
	timeout := time.NewTimer(500 * time.Millisecond)
	defer timeout.Stop()

	for len(seen) < 2 {
		select {
		case name := <-started:
			seen[name] = true
		case <-timeout.C:
			t.Fatalf("timed out waiting for concurrent starts; saw %v", seen)
		}
	}

	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("loadFilterIndexDataWithFns() error = %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for concurrent loaders to finish")
	}
}
