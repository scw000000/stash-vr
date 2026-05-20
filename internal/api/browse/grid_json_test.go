package browse

import (
	"testing"

	"stash-vr/internal/stash/gql"
)

func TestBuildGridFilterMultiSelectUsesIncludesAll(t *testing.T) {
	f := buildGridFilter([]string{"p1", "p2"}, []string{"s1", "s2"}, []string{"t1", "t2"}, 0, 0)
	if f == nil {
		t.Fatal("buildGridFilter returned nil for non-empty selections")
	}
	if f.Performers == nil || f.Performers.Modifier != gql.CriterionModifierIncludesAll {
		t.Errorf("performers modifier = %v, want INCLUDES_ALL (AND across IDs)", f.Performers)
	}
	if f.Studios == nil || f.Studios.Modifier != gql.CriterionModifierIncludesAll {
		t.Errorf("studios modifier = %v, want INCLUDES_ALL (AND across IDs)", f.Studios)
	}
	if f.Tags == nil || f.Tags.Modifier != gql.CriterionModifierIncludesAll {
		t.Errorf("tags modifier = %v, want INCLUDES_ALL (AND across IDs)", f.Tags)
	}
}

func TestBuildGridFilterEmptyReturnsNil(t *testing.T) {
	if got := buildGridFilter(nil, nil, nil, 0, 0); got != nil {
		t.Errorf("buildGridFilter(empty) = %v, want nil", got)
	}
}
