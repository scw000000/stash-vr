package internal

import (
	"strings"

	"stash-vr/internal/util"
)

// Projection describes how a VR scene should be rendered. Empty Geometry
// means no VR detected — the renderer falls back to a flat virtual screen.
type Projection struct {
	Geometry string  // "equirectangular" | "fisheye" | ""
	FOV      int     // 180, 190, 200, 360 (or 0 if Geometry is "")
	Stereo   string  // "sbs" | "tb" | ""  ("" = mono)
	Cant     float64 // RF52 canted-fisheye angle in degrees (0 for non-RF52)
}

// TagInput is the minimum tag shape Detect needs. Callers convert from
// gql.TagPartsArrayTagsTag (or any other tag type) to []TagInput so this
// module stays free of GraphQL types and is easy to unit-test.
type TagInput struct {
	Name    string
	Aliases []string
}

// Detect resolves a Projection from scene tags and the file basename.
// Tag pass runs first using util.StrSliceEquals (alias-aware,
// case-insensitive). Filename-keyword fallback runs only when the tag
// pass produced no Geometry. Tags always win when both passes would
// disagree.
func Detect(tags []TagInput, basename string) Projection {
	p := Projection{}

	// Tag pass. Walk tags once, accumulating matches. Resolve at the end
	// so most-specific Geometry wins regardless of tag order.
	hasMKX200, hasRF52, hasFisheye, hasSphere, hasDome := false, false, false, false, false
	hasSBS, hasTB := false, false
	// hasVR catches generic VR-suggesting tag names ("VR", "VR Scene",
	// "vr_180", etc.) so we still render in VR mode when no specific
	// projection tag is present. M2's detection used the same heuristic
	// and assumed DOME+SBS — this preserves that fallback so libraries
	// that don't tag every scene with a specific projection still work.
	hasVR := false

	for _, t := range tags {
		if strings.Contains(strings.ToUpper(t.Name), "VR") {
			hasVR = true
		}
		switch {
		case util.StrSliceEquals(t.Name, t.Aliases, TagVR_MKX200):
			hasMKX200 = true
		case util.StrSliceEquals(t.Name, t.Aliases, TagVR_RF52):
			hasRF52 = true
		case util.StrSliceEquals(t.Name, t.Aliases, TagVR_FISHEYE):
			hasFisheye = true
		case util.StrSliceEquals(t.Name, t.Aliases, TagVR_SPHERE):
			hasSphere = true
		case util.StrSliceEquals(t.Name, t.Aliases, TagVR_DOME):
			hasDome = true
		case util.StrSliceEquals(t.Name, t.Aliases, TagVR_SBS):
			hasSBS = true
		case util.StrSliceEquals(t.Name, t.Aliases, TagVR_TB):
			hasTB = true
		}
	}

	switch {
	case hasMKX200:
		p.Geometry, p.FOV = "fisheye", 200
	case hasRF52:
		p.Geometry, p.FOV, p.Cant = "fisheye", 180, 5.0
	case hasFisheye:
		p.Geometry, p.FOV = "fisheye", 180
	case hasSphere:
		p.Geometry, p.FOV = "equirectangular", 360
	case hasDome:
		p.Geometry, p.FOV = "equirectangular", 180
	}

	if p.Geometry != "" {
		switch {
		case hasSBS:
			p.Stereo = "sbs"
		case hasTB:
			p.Stereo = "tb"
		}
		return p
	}

	// M2-era fallback: a generic VR tag (e.g. "VR") with no specific
	// projection tag → assume DOME+SBS. Without this, scenes that M2
	// rendered as 180 SBS regress to flat virtual screen under M3a's
	// stricter detection. Honor SBS/TB tags if explicitly present.
	if hasVR {
		fallback := Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}
		if hasTB {
			fallback.Stereo = "tb"
		}
		return fallback
	}

	// Filename-keyword fallback. Only runs if no projection signal at all.
	return detectFromFilename(basename)
}

// detectFromFilename scans a basename for SKYBOX-style format keywords.
// First match per category wins. Ordered most-specific to least-specific.
func detectFromFilename(basename string) Projection {
	if basename == "" {
		return Projection{}
	}
	lc := strings.ToLower(basename)
	p := Projection{}

	// Geometry + FOV. Most-specific keywords first.
	switch {
	case strings.Contains(lc, "mkx200"):
		p.Geometry, p.FOV = "fisheye", 200
	case strings.Contains(lc, "fisheye200") || strings.Contains(lc, "_200_fisheye"):
		p.Geometry, p.FOV = "fisheye", 200
	case strings.Contains(lc, "fisheye190") || strings.Contains(lc, "_190_fisheye"):
		p.Geometry, p.FOV = "fisheye", 190
	case strings.Contains(lc, "fisheye180") || strings.Contains(lc, "_180_fisheye") || strings.Contains(lc, "fisheye"):
		p.Geometry, p.FOV = "fisheye", 180
	case strings.Contains(lc, "rf52"):
		p.Geometry, p.FOV, p.Cant = "fisheye", 180, 5.0
	case strings.Contains(lc, "_360") || strings.Contains(lc, "vr360"):
		p.Geometry, p.FOV = "equirectangular", 360
	case strings.Contains(lc, "_180") || strings.Contains(lc, "vr180"):
		p.Geometry, p.FOV = "equirectangular", 180
	}

	if p.Geometry == "" {
		return Projection{}
	}

	// Stereo. Detect independently — SBS keywords first, TB second, 2D
	// keywords explicitly force mono.
	// hasStereoToken checks for _TOKEN_ (surrounded by word separators) or
	// TOKEN_ prefix or _TOKEN suffix (before extension dot).
	hasStereoToken := func(token string) bool {
		return strings.Contains(lc, "_"+token+"_") ||
			strings.HasPrefix(lc, token+"_") ||
			strings.Contains(lc, "_"+token+".")
	}
	switch {
	case strings.Contains(lc, "_2d_") || strings.HasPrefix(lc, "2d_"):
		// Explicit mono. Leave p.Stereo empty.
	case hasStereoToken("lr"):
		p.Stereo = "sbs"
	case hasStereoToken("tb"):
		p.Stereo = "tb"
	}

	return p
}

// TagsForProjection returns the VR_* projection tag names that represent
// the given Projection. Empty Projection (no VR / Cinema) returns nil.
// Used by the projection-override handler to decide which tags to add
// to a scene after the user picks a format in the in-VR menu.
func TagsForProjection(p Projection) []string {
	if p.Geometry == "" {
		return nil
	}
	var tags []string
	switch {
	case p.Geometry == "fisheye" && p.FOV == 200:
		tags = append(tags, TagVR_MKX200)
	case p.Geometry == "fisheye":
		tags = append(tags, TagVR_FISHEYE)
	case p.Geometry == "equirectangular" && p.FOV == 360:
		tags = append(tags, TagVR_SPHERE)
	case p.Geometry == "equirectangular":
		tags = append(tags, TagVR_DOME)
	}
	switch p.Stereo {
	case "sbs":
		tags = append(tags, TagVR_SBS)
	case "tb":
		tags = append(tags, TagVR_TB)
	}
	return tags
}
