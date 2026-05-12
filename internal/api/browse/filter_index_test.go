package browse

import (
	"reflect"
	"testing"
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
