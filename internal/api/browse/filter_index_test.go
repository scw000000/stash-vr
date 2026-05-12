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
		name             string
		performers       []Entity
		studios          []Entity
		tags             []Entity
		scenes           []facetSceneSeed
		selectableTagIDs map[string]struct{}
		want             FilterIndexResponse
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFilterIndexPayload(tt.performers, tt.studios, tt.tags, tt.scenes, tt.selectableTagIDs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildFilterIndexPayload() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLoadFilterIndexDataRunsCatalogAndSceneFetchConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})

	sidebarLoader := func(context.Context, graphql.Client, string, string) (SidebarData, error) {
		started <- "sidebar"
		<-release
		return SidebarData{}, nil
	}
	sceneLoader := func(context.Context, graphql.Client) (*gql.FindScenesForFacetIndexResponse, error) {
		started <- "scenes"
		<-release
		return &gql.FindScenesForFacetIndexResponse{}, nil
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := loadFilterIndexDataWithFns(context.Background(), nil, sidebarLoader, sceneLoader)
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
