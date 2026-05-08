package internal

import (
	"reflect"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name     string
		tags     []TagInput
		basename string
		want     Projection
	}{
		// Tag pass — single tag drives Geometry.
		{name: "tag DOME", tags: []TagInput{{Name: "DOME"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "tag SPHERE", tags: []TagInput{{Name: "SPHERE"}}, want: Projection{Geometry: "equirectangular", FOV: 360}},
		{name: "tag FISHEYE", tags: []TagInput{{Name: "FISHEYE"}}, want: Projection{Geometry: "fisheye", FOV: 180}},
		{name: "tag MKX200", tags: []TagInput{{Name: "MKX200"}}, want: Projection{Geometry: "fisheye", FOV: 200}},
		{name: "tag RF52", tags: []TagInput{{Name: "RF52"}}, want: Projection{Geometry: "fisheye", FOV: 180}},

		// Tag pass — stereo tags compose with geometry.
		{name: "DOME+SBS", tags: []TagInput{{Name: "DOME"}, {Name: "SBS"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "DOME+TB", tags: []TagInput{{Name: "DOME"}, {Name: "TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "tb"}},
		{name: "SPHERE+SBS", tags: []TagInput{{Name: "SPHERE"}, {Name: "SBS"}}, want: Projection{Geometry: "equirectangular", FOV: 360, Stereo: "sbs"}},
		{name: "FISHEYE+TB", tags: []TagInput{{Name: "FISHEYE"}, {Name: "TB"}}, want: Projection{Geometry: "fisheye", FOV: 180, Stereo: "tb"}},
		{name: "MKX200+SBS", tags: []TagInput{{Name: "MKX200"}, {Name: "SBS"}}, want: Projection{Geometry: "fisheye", FOV: 200, Stereo: "sbs"}},

		// Tag pass — most-specific Geometry wins (MKX200 > FISHEYE > DOME).
		{name: "DOME+FISHEYE+SBS prefers FISHEYE", tags: []TagInput{{Name: "DOME"}, {Name: "FISHEYE"}, {Name: "SBS"}}, want: Projection{Geometry: "fisheye", FOV: 180, Stereo: "sbs"}},
		{name: "FISHEYE+MKX200+SBS prefers MKX200", tags: []TagInput{{Name: "FISHEYE"}, {Name: "MKX200"}, {Name: "SBS"}}, want: Projection{Geometry: "fisheye", FOV: 200, Stereo: "sbs"}},

		// Tag pass — SBS wins when both stereo tags present.
		{name: "DOME+SBS+TB prefers SBS", tags: []TagInput{{Name: "DOME"}, {Name: "SBS"}, {Name: "TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},

		// Tag pass — alias-aware matching (StrSliceEquals also checks Aliases).
		{name: "alias matches DOME", tags: []TagInput{{Name: "vr_180", Aliases: []string{"dome"}}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "case-insensitive name", tags: []TagInput{{Name: "dome"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},

		// Tag pass — mono (no SBS/TB tag) leaves Stereo empty.
		{name: "DOME mono", tags: []TagInput{{Name: "DOME"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "FISHEYE mono", tags: []TagInput{{Name: "FISHEYE"}}, want: Projection{Geometry: "fisheye", FOV: 180}},

		// Filename pass — only triggers when tag pass produced no Geometry.
		{name: "filename MKX200", basename: "abc_MKX200.mp4", want: Projection{Geometry: "fisheye", FOV: 200}},
		{name: "filename FISHEYE190", basename: "scene_FISHEYE190_LR.mp4", want: Projection{Geometry: "fisheye", FOV: 190, Stereo: "sbs"}},
		{name: "filename FISHEYE200", basename: "scene_FISHEYE200.mp4", want: Projection{Geometry: "fisheye", FOV: 200}},
		{name: "filename FISHEYE180", basename: "scene_FISHEYE180.mp4", want: Projection{Geometry: "fisheye", FOV: 180}},
		{name: "filename bare FISHEYE", basename: "scene_FISHEYE.mp4", want: Projection{Geometry: "fisheye", FOV: 180}},
		{name: "filename RF52", basename: "scene_RF52.mp4", want: Projection{Geometry: "fisheye", FOV: 180}},
		{name: "filename _360", basename: "scene_360_LR.mp4", want: Projection{Geometry: "equirectangular", FOV: 360, Stereo: "sbs"}},
		{name: "filename VR360", basename: "VR360_scene.mp4", want: Projection{Geometry: "equirectangular", FOV: 360}},
		{name: "filename _180", basename: "scene_180_LR.mp4", want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "filename LR_180", basename: "scene_LR_180.mp4", want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "filename TB_180", basename: "scene_TB_180.mp4", want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "tb"}},
		{name: "filename TB_360", basename: "scene_TB_360.mp4", want: Projection{Geometry: "equirectangular", FOV: 360, Stereo: "tb"}},
		{name: "filename 2D_180 forces mono", basename: "scene_2D_180.mp4", want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "filename case-insensitive", basename: "scene_mkx200.mp4", want: Projection{Geometry: "fisheye", FOV: 200}},

		// Conflict resolution: tag pass result wins; filename pass is skipped.
		{name: "tag DOME beats filename FISHEYE", tags: []TagInput{{Name: "DOME"}, {Name: "SBS"}}, basename: "scene_FISHEYE.mp4", want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},

		// No detection in either pass → empty Projection (flat-screen fallback).
		{name: "no tags no filename", tags: nil, basename: "", want: Projection{}},
		{name: "irrelevant tag, irrelevant filename", tags: []TagInput{{Name: "softcore"}}, basename: "scene.mp4", want: Projection{}},

		// Stereo-only tag (no Geometry) does not produce a Projection — Geometry is required.
		{name: "SBS alone produces empty", tags: []TagInput{{Name: "SBS"}}, want: Projection{}},
		{name: "TB alone produces empty", tags: []TagInput{{Name: "TB"}}, want: Projection{}},

		// Filename stereo without geometry → empty (Geometry is required).
		{name: "filename _LR alone produces empty", basename: "scene_LR.mp4", want: Projection{}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Detect(c.tags, c.basename)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Detect(%v, %q) = %+v, want %+v", c.tags, c.basename, got, c.want)
			}
		})
	}
}
