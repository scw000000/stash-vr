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
		{name: "tag VR_DOME", tags: []TagInput{{Name: "VR_DOME"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "tag VR_SPHERE", tags: []TagInput{{Name: "VR_SPHERE"}}, want: Projection{Geometry: "equirectangular", FOV: 360}},
		{name: "tag VR_FISHEYE", tags: []TagInput{{Name: "VR_FISHEYE"}}, want: Projection{Geometry: "fisheye", FOV: 180}},
		{name: "tag VR_MKX200", tags: []TagInput{{Name: "VR_MKX200"}}, want: Projection{Geometry: "fisheye", FOV: 200}},
		{name: "tag VR_RF52", tags: []TagInput{{Name: "VR_RF52"}}, want: Projection{Geometry: "fisheye", FOV: 180}},

		// Tag pass — stereo tags compose with geometry.
		{name: "VR_DOME+VR_SBS", tags: []TagInput{{Name: "VR_DOME"}, {Name: "VR_SBS"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "VR_DOME+VR_TB", tags: []TagInput{{Name: "VR_DOME"}, {Name: "VR_TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "tb"}},
		{name: "VR_SPHERE+VR_SBS", tags: []TagInput{{Name: "VR_SPHERE"}, {Name: "VR_SBS"}}, want: Projection{Geometry: "equirectangular", FOV: 360, Stereo: "sbs"}},
		{name: "VR_FISHEYE+VR_TB", tags: []TagInput{{Name: "VR_FISHEYE"}, {Name: "VR_TB"}}, want: Projection{Geometry: "fisheye", FOV: 180, Stereo: "tb"}},
		{name: "VR_MKX200+VR_SBS", tags: []TagInput{{Name: "VR_MKX200"}, {Name: "VR_SBS"}}, want: Projection{Geometry: "fisheye", FOV: 200, Stereo: "sbs"}},

		// Tag pass — most-specific Geometry wins (MKX200 > FISHEYE > DOME).
		{name: "VR_DOME+VR_FISHEYE+VR_SBS prefers FISHEYE", tags: []TagInput{{Name: "VR_DOME"}, {Name: "VR_FISHEYE"}, {Name: "VR_SBS"}}, want: Projection{Geometry: "fisheye", FOV: 180, Stereo: "sbs"}},
		{name: "VR_FISHEYE+VR_MKX200+VR_SBS prefers MKX200", tags: []TagInput{{Name: "VR_FISHEYE"}, {Name: "VR_MKX200"}, {Name: "VR_SBS"}}, want: Projection{Geometry: "fisheye", FOV: 200, Stereo: "sbs"}},

		// Tag pass — SBS wins when both stereo tags present.
		{name: "VR_DOME+VR_SBS+VR_TB prefers SBS", tags: []TagInput{{Name: "VR_DOME"}, {Name: "VR_SBS"}, {Name: "VR_TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},

		// Tag pass — alias-aware matching (StrSliceEquals also checks Aliases).
		{name: "alias matches VR_DOME", tags: []TagInput{{Name: "MyVRTag", Aliases: []string{"vr_dome"}}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "case-insensitive name", tags: []TagInput{{Name: "vr_dome"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},

		// Tag pass — mono (no SBS/TB tag) leaves Stereo empty.
		{name: "VR_DOME mono", tags: []TagInput{{Name: "VR_DOME"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "VR_FISHEYE mono", tags: []TagInput{{Name: "VR_FISHEYE"}}, want: Projection{Geometry: "fisheye", FOV: 180}},

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
		{name: "tag VR_DOME beats filename FISHEYE", tags: []TagInput{{Name: "VR_DOME"}, {Name: "VR_SBS"}}, basename: "scene_FISHEYE.mp4", want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},

		// M2-era fallback: a generic VR tag with no specific projection → DOME+SBS.
		{name: "generic VR tag", tags: []TagInput{{Name: "VR"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "VR Scene tag", tags: []TagInput{{Name: "VR Scene"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "vr_180 tag", tags: []TagInput{{Name: "vr_180"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "generic VR + VR_TB tags → DOME+TB", tags: []TagInput{{Name: "VR"}, {Name: "VR_TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "tb"}},
		{name: "generic VR + filename FISHEYE → tag wins (DOME+SBS)", tags: []TagInput{{Name: "VR"}}, basename: "scene_FISHEYE.mp4", want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},

		// No detection in either pass → empty Projection (flat-screen fallback).
		{name: "no tags no filename", tags: nil, basename: "", want: Projection{}},
		{name: "irrelevant tag, irrelevant filename", tags: []TagInput{{Name: "softcore"}}, basename: "scene.mp4", want: Projection{}},

		// Stereo-only tag (no Geometry) does not produce a Projection — Geometry is required.
		// Note: VR_SBS contains "VR" so it triggers the generic-VR fallback.
		{name: "VR_SBS alone triggers fallback (DOME+SBS)", tags: []TagInput{{Name: "VR_SBS"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "VR_TB alone triggers fallback (DOME+TB)", tags: []TagInput{{Name: "VR_TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "tb"}},
		{name: "bare SBS alone produces empty (no VR substring)", tags: []TagInput{{Name: "SBS"}}, want: Projection{}},

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
