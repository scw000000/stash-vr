# M3b In-VR Projection Picker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an in-VR projection-format picker that lets the user override the auto-detected projection per scene, immediately rebinding the WebXR renderer and writing the corresponding `VR_*` tags back to Stash. Migrates the seven projection tag constants from bare names to `VR_`-prefixed.

**Architecture:** Three-row picker (Type / Degree / Stereo + Auto) modeled on SKYBOX picture 2. The browse template now emits all four render entities (`vrSphere180`, `vrSphere360`, `vrFisheye`, `vrFlat`) at once with `visible` toggling so JS can switch between them at runtime. A shared `THREE.VideoTexture` is reused across all entities. Tap → JS state updates → `applyAll()` re-binds → POST to a new `/browse/scene/{id}/projection` endpoint that adjusts Stash tags.

**Spec:** [docs/superpowers/specs/2026-05-08-m3b-in-vr-projection-picker.md](../specs/2026-05-08-m3b-in-vr-projection-picker.md)

**Tech Stack:** Go 1.24 (`html/template`, no SPA framework, no JS bundler), A-Frame 1.7.0 (already vendored at `internal/static/vendor/aframe.min.js`), Three.js (bundled inside A-Frame), inline GLSL fragment + vertex shaders, inline JS in the gohtml template. Per [CLAUDE.md](../../../CLAUDE.md), `go vet ./...` and `go build ./...` are the standard checks; `go test ./internal/api/internal/...` runs the focused unit tests for the projection module.

---

## File Structure

| Path | Responsibility | Status |
|---|---|---|
| `internal/api/internal/legend.go` | Migrate the seven `TagVR_*` constants from bare names (`DOME`, `SBS`, etc.) to `VR_`-prefixed (`VR_DOME`, `VR_SBS`, etc.). | Task 1. |
| `internal/api/internal/projection_test.go` | Update existing test cases to use the prefixed names. | Task 1. |
| `internal/api/internal/projection.go` | Add `TagsForProjection(p Projection) []string` helper that maps a `Projection` back to the set of `VR_*` tag names that represent it. | Task 2. |
| `internal/api/internal/projection_test.go` | Add `TestTagsForProjection` covering the 13 valid mappings. | Task 2. |
| `internal/api/browse/scene_projection.go` | **New.** `sceneProjectionHandler` — POST handler that reads `{type, degree, stereo}` (or `{auto: true}`) from the request body, drops the seven `VR_*` projection tags from the scene's current tag set, adds the tags from `TagsForProjection`, and calls `library.UpdateTags`. | Task 3. |
| `internal/api/browse/router.go` | Register `r.Post("/scene/{id}/projection", h.sceneProjectionHandler)`. | Task 3. |
| `internal/static/browse_scene.gohtml` | Multi-entity emit (Task 4): always emit `vrSphere180`, `vrSphere360`, `vrFisheye`, `vrFlat`; JS shared `VideoTexture`, JS visibility toggling. Picker UI markup (Task 5): Format button + `<a-entity id="vrFormatPicker">` block with three rows + Auto, hidden by default. Picker JS (Task 6): tap handlers, state, applyAll re-binding, POST integration, active-highlight, invalid-combo disabling. | Tasks 4–6. |

No new vendored libraries. No new env vars. No genqlient regeneration. The `/deovr` and `/heresphere` handlers update automatically via the constant migration in Task 1.

## Pre-flight

- [ ] **Step 0a: Confirm working directory and tree state**

Run: `git status` and `git log -1 --oneline`

Expected: working dir `c:\dev\stash-vr`, tree clean. Last commit `c8092d9 docs: spec for M3b in-VR projection picker + tag write-back`. If newer commits touch `legend.go`, `projection.go`, `projection_test.go`, `browse/router.go`, `browse/scene_projection.go`, or `browse_scene.gohtml`, re-read the relevant file before applying edits.

- [ ] **Step 0b: Confirm Go is on PATH**

PowerShell: `$env:Path += ";C:\Program Files\Go\bin"`. Bash's PATH does not include Go in this environment.

---

## Task 1: Migrate `TagVR_*` constants from bare to `VR_`-prefixed names

**Files:**
- Modify: `internal/api/internal/legend.go`
- Modify: `internal/api/internal/projection_test.go`

The seven projection tag constants change names. The `/deovr` and `/heresphere` handlers reference these constants and pick up the new values automatically without code changes there. The `CUBEMAP` and `EAC` constants are unused (deferred to M4 or never) and stay bare to avoid churn.

- [ ] **Step 1: Migrate the seven projection constants in `legend.go`**

Edit `c:\dev\stash-vr\internal\api\internal\legend.go`. Replace the projection-tag block:

```
old_string:
var (
	TagVR_DOME    = "DOME"
	TagVR_SPHERE  = "SPHERE"
	TagVR_FISHEYE = "FISHEYE"
	TagVR_MKX200  = "MKX200"
	TagVR_RF52    = "RF52"
	TagVR_SBS     = "SBS"
	TagVR_TB      = "TB"

	TagVR_CUBEMAP = "CUBEMAP"
	TagVR_EAC     = "EAC"
)

new_string:
var (
	TagVR_DOME    = "VR_DOME"
	TagVR_SPHERE  = "VR_SPHERE"
	TagVR_FISHEYE = "VR_FISHEYE"
	TagVR_MKX200  = "VR_MKX200"
	TagVR_RF52    = "VR_RF52"
	TagVR_SBS     = "VR_SBS"
	TagVR_TB      = "VR_TB"

	TagVR_CUBEMAP = "CUBEMAP"
	TagVR_EAC     = "EAC"
)
```

- [ ] **Step 2: Update tag-pass test cases in `projection_test.go`**

Edit `c:\dev\stash-vr\internal\api\internal\projection_test.go`. Update each tag-pass test case to use the prefixed name. Use the Edit tool one block at a time.

**Edit 2a: Single-tag drives Geometry block.**

```
old_string:
		// Tag pass — single tag drives Geometry.
		{name: "tag DOME", tags: []TagInput{{Name: "DOME"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "tag SPHERE", tags: []TagInput{{Name: "SPHERE"}}, want: Projection{Geometry: "equirectangular", FOV: 360}},
		{name: "tag FISHEYE", tags: []TagInput{{Name: "FISHEYE"}}, want: Projection{Geometry: "fisheye", FOV: 180}},
		{name: "tag MKX200", tags: []TagInput{{Name: "MKX200"}}, want: Projection{Geometry: "fisheye", FOV: 200}},
		{name: "tag RF52", tags: []TagInput{{Name: "RF52"}}, want: Projection{Geometry: "fisheye", FOV: 180}},

new_string:
		// Tag pass — single tag drives Geometry.
		{name: "tag VR_DOME", tags: []TagInput{{Name: "VR_DOME"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "tag VR_SPHERE", tags: []TagInput{{Name: "VR_SPHERE"}}, want: Projection{Geometry: "equirectangular", FOV: 360}},
		{name: "tag VR_FISHEYE", tags: []TagInput{{Name: "VR_FISHEYE"}}, want: Projection{Geometry: "fisheye", FOV: 180}},
		{name: "tag VR_MKX200", tags: []TagInput{{Name: "VR_MKX200"}}, want: Projection{Geometry: "fisheye", FOV: 200}},
		{name: "tag VR_RF52", tags: []TagInput{{Name: "VR_RF52"}}, want: Projection{Geometry: "fisheye", FOV: 180}},
```

**Edit 2b: Stereo composes block.**

```
old_string:
		// Tag pass — stereo tags compose with geometry.
		{name: "DOME+SBS", tags: []TagInput{{Name: "DOME"}, {Name: "SBS"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "DOME+TB", tags: []TagInput{{Name: "DOME"}, {Name: "TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "tb"}},
		{name: "SPHERE+SBS", tags: []TagInput{{Name: "SPHERE"}, {Name: "SBS"}}, want: Projection{Geometry: "equirectangular", FOV: 360, Stereo: "sbs"}},
		{name: "FISHEYE+TB", tags: []TagInput{{Name: "FISHEYE"}, {Name: "TB"}}, want: Projection{Geometry: "fisheye", FOV: 180, Stereo: "tb"}},
		{name: "MKX200+SBS", tags: []TagInput{{Name: "MKX200"}, {Name: "SBS"}}, want: Projection{Geometry: "fisheye", FOV: 200, Stereo: "sbs"}},

new_string:
		// Tag pass — stereo tags compose with geometry.
		{name: "VR_DOME+VR_SBS", tags: []TagInput{{Name: "VR_DOME"}, {Name: "VR_SBS"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "VR_DOME+VR_TB", tags: []TagInput{{Name: "VR_DOME"}, {Name: "VR_TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "tb"}},
		{name: "VR_SPHERE+VR_SBS", tags: []TagInput{{Name: "VR_SPHERE"}, {Name: "VR_SBS"}}, want: Projection{Geometry: "equirectangular", FOV: 360, Stereo: "sbs"}},
		{name: "VR_FISHEYE+VR_TB", tags: []TagInput{{Name: "VR_FISHEYE"}, {Name: "VR_TB"}}, want: Projection{Geometry: "fisheye", FOV: 180, Stereo: "tb"}},
		{name: "VR_MKX200+VR_SBS", tags: []TagInput{{Name: "VR_MKX200"}, {Name: "VR_SBS"}}, want: Projection{Geometry: "fisheye", FOV: 200, Stereo: "sbs"}},
```

**Edit 2c: Most-specific Geometry wins block.**

```
old_string:
		// Tag pass — most-specific Geometry wins (MKX200 > FISHEYE > DOME).
		{name: "DOME+FISHEYE+SBS prefers FISHEYE", tags: []TagInput{{Name: "DOME"}, {Name: "FISHEYE"}, {Name: "SBS"}}, want: Projection{Geometry: "fisheye", FOV: 180, Stereo: "sbs"}},
		{name: "FISHEYE+MKX200+SBS prefers MKX200", tags: []TagInput{{Name: "FISHEYE"}, {Name: "MKX200"}, {Name: "SBS"}}, want: Projection{Geometry: "fisheye", FOV: 200, Stereo: "sbs"}},

new_string:
		// Tag pass — most-specific Geometry wins (MKX200 > FISHEYE > DOME).
		{name: "VR_DOME+VR_FISHEYE+VR_SBS prefers FISHEYE", tags: []TagInput{{Name: "VR_DOME"}, {Name: "VR_FISHEYE"}, {Name: "VR_SBS"}}, want: Projection{Geometry: "fisheye", FOV: 180, Stereo: "sbs"}},
		{name: "VR_FISHEYE+VR_MKX200+VR_SBS prefers MKX200", tags: []TagInput{{Name: "VR_FISHEYE"}, {Name: "VR_MKX200"}, {Name: "VR_SBS"}}, want: Projection{Geometry: "fisheye", FOV: 200, Stereo: "sbs"}},
```

**Edit 2d: SBS wins over TB block.**

```
old_string:
		// Tag pass — SBS wins when both stereo tags present.
		{name: "DOME+SBS+TB prefers SBS", tags: []TagInput{{Name: "DOME"}, {Name: "SBS"}, {Name: "TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},

new_string:
		// Tag pass — SBS wins when both stereo tags present.
		{name: "VR_DOME+VR_SBS+VR_TB prefers SBS", tags: []TagInput{{Name: "VR_DOME"}, {Name: "VR_SBS"}, {Name: "VR_TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
```

**Edit 2e: Alias-aware and case-insensitive block.**

```
old_string:
		// Tag pass — alias-aware matching (StrSliceEquals also checks Aliases).
		{name: "alias matches DOME", tags: []TagInput{{Name: "vr_180", Aliases: []string{"dome"}}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "case-insensitive name", tags: []TagInput{{Name: "dome"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},

		// Tag pass — mono (no SBS/TB tag) leaves Stereo empty.
		{name: "DOME mono", tags: []TagInput{{Name: "DOME"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "FISHEYE mono", tags: []TagInput{{Name: "FISHEYE"}}, want: Projection{Geometry: "fisheye", FOV: 180}},

new_string:
		// Tag pass — alias-aware matching (StrSliceEquals also checks Aliases).
		{name: "alias matches VR_DOME", tags: []TagInput{{Name: "MyVRTag", Aliases: []string{"vr_dome"}}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "case-insensitive name", tags: []TagInput{{Name: "vr_dome"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},

		// Tag pass — mono (no SBS/TB tag) leaves Stereo empty.
		{name: "VR_DOME mono", tags: []TagInput{{Name: "VR_DOME"}}, want: Projection{Geometry: "equirectangular", FOV: 180}},
		{name: "VR_FISHEYE mono", tags: []TagInput{{Name: "VR_FISHEYE"}}, want: Projection{Geometry: "fisheye", FOV: 180}},
```

Note on the alias test: the `MyVRTag` name contains "VR" which would also trigger the generic-VR fallback, but the alias pass for `vr_dome` matches first so the test still asserts the alias path specifically (DOME+180 with Stereo empty, NOT the fallback's DOME+180+SBS).

**Edit 2f: Conflict resolution block.**

```
old_string:
		// Conflict resolution: tag pass result wins; filename pass is skipped.
		{name: "tag DOME beats filename FISHEYE", tags: []TagInput{{Name: "DOME"}, {Name: "SBS"}}, basename: "scene_FISHEYE.mp4", want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},

new_string:
		// Conflict resolution: tag pass result wins; filename pass is skipped.
		{name: "tag VR_DOME beats filename FISHEYE", tags: []TagInput{{Name: "VR_DOME"}, {Name: "VR_SBS"}}, basename: "scene_FISHEYE.mp4", want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
```

**Edit 2g: Stereo-only without geometry block.**

```
old_string:
		// Stereo-only tag (no Geometry) does not produce a Projection — Geometry is required.
		{name: "SBS alone produces empty", tags: []TagInput{{Name: "SBS"}}, want: Projection{}},
		{name: "TB alone produces empty", tags: []TagInput{{Name: "TB"}}, want: Projection{}},

new_string:
		// Stereo-only tag (no Geometry) does not produce a Projection — Geometry is required.
		// Note: VR_SBS contains "VR" so it triggers the generic-VR fallback.
		{name: "VR_SBS alone triggers fallback (DOME+SBS)", tags: []TagInput{{Name: "VR_SBS"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}},
		{name: "VR_TB alone triggers fallback (DOME+TB)", tags: []TagInput{{Name: "VR_TB"}}, want: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "tb"}},
		{name: "bare SBS alone produces empty (no VR substring)", tags: []TagInput{{Name: "SBS"}}, want: Projection{}},
```

The stereo-only test case shifts because a `VR_SBS` tag now contains the "VR" substring → the M2-fallback fires, producing DOME+SBS, not empty. The bare `SBS` test stays as a no-VR-substring case for completeness. The bare `TB` case is removed (covered by the bare `SBS` case for the no-VR-substring path).

- [ ] **Step 3: Run tests to verify they pass**

PowerShell from `c:\dev\stash-vr`:

```
$env:Path += ";C:\Program Files\Go\bin"
go test ./internal/api/internal/... -v
```

Expected: every subtest passes. `ok stash-vr/internal/api/internal <duration>`. If a test fails, the constant value or test expectation is mismatched — fix the test to match the new constant value, NOT the constants.

- [ ] **Step 4: Run full build to confirm no regressions**

```
$env:Path += ";C:\Program Files\Go\bin"
go vet ./...
go build ./...
```

Both clean.

- [ ] **Step 5: Commit**

```
git add internal/api/internal/legend.go internal/api/internal/projection_test.go
git commit -m "browse: migrate projection tag constants to VR_ prefix

Seven projection-tag constants in internal/api/internal/legend.go move
from bare names (DOME, SBS, etc.) to VR_-prefixed (VR_DOME, VR_SBS,
etc.). The user confirmed their library does not rely on bare-form
projection tags, so no fallback for the bare forms is added. /deovr
and /heresphere handlers reference these constants and pick up the new
values automatically.

Updates projection_test.go test cases. The stereo-only-without-geometry
tests that previously asserted Projection{} for bare SBS/TB now use
VR_SBS/VR_TB which contain the VR substring and trigger the M2-era
fallback (DOME+SBS or DOME+TB)."
```

---

## Task 2: Add `TagsForProjection` helper + tests

**Files:**
- Modify: `internal/api/internal/projection.go`
- Modify: `internal/api/internal/projection_test.go`

Pure helper function: maps a `Projection` back to the `VR_*` tag names that represent it. The new `/browse/scene/{id}/projection` handler in Task 3 calls this. Self-contained so it's testable independently.

- [ ] **Step 1: Write the failing tests**

Append a new `TestTagsForProjection` function to `c:\dev\stash-vr\internal\api\internal\projection_test.go`. Use Edit to add it after the closing brace of `TestDetect`:

```
old_string:
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Detect(c.tags, c.basename)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Detect(%v, %q) = %+v, want %+v", c.tags, c.basename, got, c.want)
			}
		})
	}
}

new_string:
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Detect(c.tags, c.basename)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Detect(%v, %q) = %+v, want %+v", c.tags, c.basename, got, c.want)
			}
		})
	}
}

func TestTagsForProjection(t *testing.T) {
	cases := []struct {
		name string
		p    Projection
		want []string
	}{
		// Empty Projection → nil (no projection tags).
		{name: "empty", p: Projection{}, want: nil},
		{name: "geometry empty, fov set is still empty", p: Projection{FOV: 180, Stereo: "sbs"}, want: nil},

		// Equirectangular 180 (DOME) variants.
		{name: "DOME mono", p: Projection{Geometry: "equirectangular", FOV: 180}, want: []string{"VR_DOME"}},
		{name: "DOME SBS", p: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "sbs"}, want: []string{"VR_DOME", "VR_SBS"}},
		{name: "DOME TB", p: Projection{Geometry: "equirectangular", FOV: 180, Stereo: "tb"}, want: []string{"VR_DOME", "VR_TB"}},

		// Equirectangular 360 (SPHERE) variants.
		{name: "SPHERE mono", p: Projection{Geometry: "equirectangular", FOV: 360}, want: []string{"VR_SPHERE"}},
		{name: "SPHERE SBS", p: Projection{Geometry: "equirectangular", FOV: 360, Stereo: "sbs"}, want: []string{"VR_SPHERE", "VR_SBS"}},
		{name: "SPHERE TB", p: Projection{Geometry: "equirectangular", FOV: 360, Stereo: "tb"}, want: []string{"VR_SPHERE", "VR_TB"}},

		// Fisheye 180 variants → VR_FISHEYE.
		{name: "FISHEYE 180 mono", p: Projection{Geometry: "fisheye", FOV: 180}, want: []string{"VR_FISHEYE"}},
		{name: "FISHEYE 180 SBS", p: Projection{Geometry: "fisheye", FOV: 180, Stereo: "sbs"}, want: []string{"VR_FISHEYE", "VR_SBS"}},
		{name: "FISHEYE 180 TB", p: Projection{Geometry: "fisheye", FOV: 180, Stereo: "tb"}, want: []string{"VR_FISHEYE", "VR_TB"}},

		// Fisheye 200 variants → VR_MKX200 (more-specific lens).
		{name: "MKX200 mono", p: Projection{Geometry: "fisheye", FOV: 200}, want: []string{"VR_MKX200"}},
		{name: "MKX200 SBS", p: Projection{Geometry: "fisheye", FOV: 200, Stereo: "sbs"}, want: []string{"VR_MKX200", "VR_SBS"}},
		{name: "MKX200 TB", p: Projection{Geometry: "fisheye", FOV: 200, Stereo: "tb"}, want: []string{"VR_MKX200", "VR_TB"}},

		// Fisheye 190 falls through to VR_FISHEYE (no specific 190 tag).
		{name: "FISHEYE 190 SBS → VR_FISHEYE", p: Projection{Geometry: "fisheye", FOV: 190, Stereo: "sbs"}, want: []string{"VR_FISHEYE", "VR_SBS"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := TagsForProjection(c.p)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("TagsForProjection(%+v) = %v, want %v", c.p, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails (function not yet defined)**

```
$env:Path += ";C:\Program Files\Go\bin"
go test ./internal/api/internal/... -run TestTagsForProjection
```

Expected: compile error `undefined: TagsForProjection`.

- [ ] **Step 3: Add `TagsForProjection` to `projection.go`**

Append the helper to the end of `c:\dev\stash-vr\internal\api\internal\projection.go`. Use Edit to add it after `detectFromFilename`'s closing brace:

```
old_string:
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

new_string:
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
```

- [ ] **Step 4: Run tests to verify they pass**

```
$env:Path += ";C:\Program Files\Go\bin"
go test ./internal/api/internal/... -v
```

Expected: all `TestDetect` and `TestTagsForProjection` subtests pass.

- [ ] **Step 5: Build verify**

```
go vet ./...
go build ./...
```

Clean.

- [ ] **Step 6: Commit**

```
git add internal/api/internal/projection.go internal/api/internal/projection_test.go
git commit -m "browse: add TagsForProjection helper

Pure mapping function from Projection back to the VR_* tag names that
represent it. Used by the upcoming projection-override handler to
decide which tags to add to a scene after the user picks a format in
the in-VR menu. 14-case table-driven test covers every (Geometry, FOV,
Stereo) cell of the M3a matrix."
```

---

## Task 3: New POST `/browse/scene/{id}/projection` endpoint

**Files:**
- Create: `internal/api/browse/scene_projection.go`
- Modify: `internal/api/browse/router.go`

The handler accepts JSON `{type, degree, stereo}` (apply) or `{auto: true}` (clear all projection tags). It mirrors the existing tag-mutation handler pattern: GetScene with forceFetch=true, walk tags filtering ancestor-injected, drop the seven `VR_*` projection tags, add `TagsForProjection(newProjection)`, call `library.UpdateTags`, refresh the scene cache, return 204.

- [ ] **Step 1: Create `scene_projection.go`**

Create `c:\dev\stash-vr\internal\api\browse\scene_projection.go`:

```go
package browse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/prefix"
)

// projectionRequest is the JSON body shape of POST /browse/scene/{id}/projection.
// Either Auto is true (clears all projection tags), or Type/Degree/Stereo are
// set (writes the corresponding tags after dropping the existing ones).
type projectionRequest struct {
	Auto   bool   `json:"auto,omitempty"`
	Type   string `json:"type,omitempty"`   // "Normal" | "FishEye"
	Degree string `json:"degree,omitempty"` // "Cinema" | "180" | "200" | "360"
	Stereo string `json:"stereo,omitempty"` // "2D" | "SBS" | "TB"
}

// sceneProjectionHandler updates a scene's VR_* projection tags based on the
// in-VR picker selection. Drops the seven VR_* projection tags from the
// scene's current tag set, adds the tags returned by TagsForProjection for
// the new selection (or none on Auto), and persists via library.UpdateTags.
// Ancestor-injected tags are filtered out before computing the new set.
func (h *httpHandler) sceneProjectionHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	var req projectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var addTags []string
	if !req.Auto {
		proj, err := projectionFromRequest(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		addTags = apiinternal.TagsForProjection(proj)
	}

	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		log.Ctx(r.Context()).Warn().Err(err).Str("id", id).Msg("projection: scene not found")
		http.Error(w, "scene not found", http.StatusNotFound)
		return
	}

	projectionTags := map[string]bool{
		apiinternal.TagVR_DOME:    true,
		apiinternal.TagVR_SPHERE:  true,
		apiinternal.TagVR_FISHEYE: true,
		apiinternal.TagVR_MKX200:  true,
		apiinternal.TagVR_RF52:    true,
		apiinternal.TagVR_SBS:     true,
		apiinternal.TagVR_TB:      true,
	}
	newTags := make([]string, 0, len(vd.SceneParts.Tags)+len(addTags))
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		// Skip ancestor-injected tags (they're persisted as ancestors of
		// other tags, not as direct tags on the scene).
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		// Drop existing projection tags so we can replace cleanly.
		if projectionTags[t.TagParts.Name] {
			continue
		}
		newTags = append(newTags, t.TagParts.Name)
	}
	newTags = append(newTags, addTags...)

	if err := h.libraryService.UpdateTags(r.Context(), id, newTags); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("projection: UpdateTags failed")
		http.Error(w, "tag update failed", http.StatusInternalServerError)
		return
	}
	h.refreshSceneCache(r, id)
	w.WriteHeader(http.StatusNoContent)
}

// projectionFromRequest validates the three-field input and maps it to a
// Projection. Returns an error on invalid combinations (Normal+200,
// FishEye+Cinema, FishEye+360, or any unknown value).
func projectionFromRequest(req *projectionRequest) (apiinternal.Projection, error) {
	// Cinema means "no VR" — empty Projection regardless of Type/Stereo.
	if req.Degree == "Cinema" {
		return apiinternal.Projection{}, nil
	}

	proj := apiinternal.Projection{}
	switch req.Type {
	case "Normal":
		proj.Geometry = "equirectangular"
		switch req.Degree {
		case "180":
			proj.FOV = 180
		case "360":
			proj.FOV = 360
		default:
			return apiinternal.Projection{}, fmt.Errorf("invalid degree %q for type Normal", req.Degree)
		}
	case "FishEye":
		proj.Geometry = "fisheye"
		switch req.Degree {
		case "180":
			proj.FOV = 180
		case "200":
			proj.FOV = 200
		default:
			return apiinternal.Projection{}, fmt.Errorf("invalid degree %q for type FishEye", req.Degree)
		}
	default:
		return apiinternal.Projection{}, fmt.Errorf("invalid type %q", req.Type)
	}

	switch req.Stereo {
	case "2D", "":
		// Mono — leave proj.Stereo empty.
	case "SBS":
		proj.Stereo = "sbs"
	case "TB":
		proj.Stereo = "tb"
	default:
		return apiinternal.Projection{}, fmt.Errorf("invalid stereo %q", req.Stereo)
	}

	return proj, nil
}
```

- [ ] **Step 2: Wire the route in `router.go`**

Edit `c:\dev\stash-vr\internal\api\browse\router.go`:

```
old_string:
	r.Post("/scene/{id}/tags/add", h.sceneTagAddHandler)
	r.Post("/scene/{id}/tags/remove", h.sceneTagRemoveHandler)

new_string:
	r.Post("/scene/{id}/tags/add", h.sceneTagAddHandler)
	r.Post("/scene/{id}/tags/remove", h.sceneTagRemoveHandler)
	r.Post("/scene/{id}/projection", h.sceneProjectionHandler)
```

- [ ] **Step 3: Build verify**

```
$env:Path += ";C:\Program Files\Go\bin"
go vet ./...
go build ./...
go test ./internal/api/internal/...
```

All clean.

- [ ] **Step 4: Source-grep verify**

Use the Grep tool to verify the route and handler are wired:

- In `c:\dev\stash-vr\internal\api\browse\router.go`, pattern `/scene/{id}/projection` should match exactly once.
- In `c:\dev\stash-vr\internal\api\browse\scene_projection.go`, pattern `func \(h \*httpHandler\) sceneProjectionHandler` should match exactly once.
- In `c:\dev\stash-vr\internal\api\browse\scene_projection.go`, pattern `apiinternal\.TagsForProjection` should match exactly once.

- [ ] **Step 5: Commit**

```
git add internal/api/browse/scene_projection.go internal/api/browse/router.go
git commit -m "browse: add POST /scene/{id}/projection endpoint

New handler accepts JSON {type, degree, stereo} (apply) or {auto: true}
(clear). Drops the seven VR_* projection tags from the scene's current
direct tags (ancestor-injected tags are filtered out separately, same
guard the chip add/remove handlers use), adds tags from
TagsForProjection for the new selection, and persists via
library.UpdateTags. Refreshes the scene cache so subsequent reads see
the new tags.

Returns 204 on success; 4xx on invalid input (unknown type/degree/
stereo, invalid combinations like FishEye+360); 5xx on Stash failure."
```

---

## Task 4: Template emits all four render entities + shared `VideoTexture`

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

Current state (after M3a + the 1407b52 fallback fix): the template emits exactly *one* render entity per scene based on the server-detected Projection — `vrSphere` (for both DOME 180 and SPHERE 360 with different geometry strings), `vrFisheye`, or `vrFlat`. To support runtime override (Tasks 5–6), the template must emit *all four* render entities at once and toggle visibility from JS based on the active Projection.

After this task, the renderer behavior is unchanged for any given scene — only the active entity is visible, the others are `visible="false"`. The infrastructure is in place for the picker in Task 5+.

- [ ] **Step 1: Replace the geometry-branch chain with all-four entities + initial-visibility template logic**

Edit `c:\dev\stash-vr\internal\static\browse_scene.gohtml`. Find the existing geometry-branch chain (currently around lines 65-79):

```
old_string:
{{if and (eq .Projection.Geometry "equirectangular") (eq .Projection.FOV 180)}}
  <a-entity id="vrSphere"
            geometry="primitive:sphere;radius:100;phiStart:180;phiLength:180;thetaLength:180;segmentsWidth:64;segmentsHeight:64"></a-entity>
{{else if and (eq .Projection.Geometry "equirectangular") (eq .Projection.FOV 360)}}
  <a-entity id="vrSphere"
            geometry="primitive:sphere;radius:100;segmentsWidth:64;segmentsHeight:64"></a-entity>
{{else if eq .Projection.Geometry "fisheye"}}
  <a-entity id="vrFisheye"
            data-fov="{{.Projection.FOV}}"
            geometry="primitive:sphere;radius:100;segmentsWidth:64;segmentsHeight:64"></a-entity>
{{else}}
  <a-entity id="vrFlat"
            geometry="primitive:plane;width:4;height:2.25"
            position="0 1.6 -3"></a-entity>
{{end}}
```

Replace with four entities, all emitted, each with `visible` set per the active Projection:

```
new_string:
  <!--
    All four render entities are always emitted. JS toggles visibility
    based on the active Projection. The initial visibility below
    matches the server-side detected Projection so the page renders
    correctly before any JS runs.
  -->
  <a-entity id="vrSphere180"
            visible="{{if and (eq .Projection.Geometry "equirectangular") (eq .Projection.FOV 180)}}true{{else}}false{{end}}"
            geometry="primitive:sphere;radius:100;phiStart:180;phiLength:180;thetaLength:180;segmentsWidth:64;segmentsHeight:64"></a-entity>
  <a-entity id="vrSphere360"
            visible="{{if and (eq .Projection.Geometry "equirectangular") (eq .Projection.FOV 360)}}true{{else}}false{{end}}"
            geometry="primitive:sphere;radius:100;segmentsWidth:64;segmentsHeight:64"></a-entity>
  <a-entity id="vrFisheye"
            visible="{{if eq .Projection.Geometry "fisheye"}}true{{else}}false{{end}}"
            data-fov="{{if eq .Projection.Geometry "fisheye"}}{{.Projection.FOV}}{{else}}180{{end}}"
            geometry="primitive:sphere;radius:100;segmentsWidth:64;segmentsHeight:64"></a-entity>
  <a-entity id="vrFlat"
            visible="{{if not .Projection.Geometry}}true{{else}}false{{end}}"
            geometry="primitive:plane;width:4;height:2.25"
            position="0 1.6 -3"></a-entity>
```

The `vrFisheye` entity always carries `data-fov` (default `180`) so the JS `parseFloat(el.dataset.fov || '180')` keeps working.

- [ ] **Step 2: Update the JS to bind both vrSphere180 and vrSphere360, share a single `VideoTexture`, and update the `forEach` listener array**

Edit the inline JS block. Two sub-edits.

**Edit 2a:** Replace `applySphere` and the texture creation pattern. Currently `applySphere` creates a fresh `VideoTexture` each call; we change it to (a) handle both vrSphere180 and vrSphere360, (b) share a single `VideoTexture` across all four bind functions via a small helper.

Find this region:

```
old_string:
    function applySphere() {
      const el = document.getElementById('vrSphere');
      if (!el || !window.AFRAME || !AFRAME.THREE) return;
      const mesh = el.getObject3D('mesh');
      if (!mesh || mesh.userData.boundVR) return;
      const tex = new AFRAME.THREE.VideoTexture(video);
      if (AFRAME.THREE.SRGBColorSpace) tex.colorSpace = AFRAME.THREE.SRGBColorSpace;
      mesh.material = new AFRAME.THREE.MeshBasicMaterial({
        map: tex,
        side: AFRAME.THREE.BackSide
      });
      mesh.onBeforeRender = function(renderer, sceneObj, cam) {
        const xr = renderer.xr;
        const stereo = scene.dataset.stereo || '';
        // Out of XR or mono content: full texture, both eyes see the same thing.
        if (!xr || !xr.isPresenting || !stereo) {
          tex.offset.set(0, 0);
          tex.repeat.set(1, 1);
          return;
        }
        const xrCam = xr.getCamera();
        if (!xrCam || !xrCam.cameras || xrCam.cameras.length < 2) return;
        const isLeft = cam === xrCam.cameras[0];
        const isRight = cam === xrCam.cameras[1];
        if (!isLeft && !isRight) return;
        if (stereo === 'sbs') {
          tex.repeat.set(0.5, 1);
          tex.offset.set(isLeft ? 0 : 0.5, 0);
        } else if (stereo === 'tb') {
          tex.repeat.set(1, 0.5);
          tex.offset.set(0, isLeft ? 0 : 0.5);
        } else {
          // Unknown stereo string — render full texture defensively.
          tex.offset.set(0, 0);
          tex.repeat.set(1, 1);
        }
      };
      mesh.userData.boundVR = true;
    }
```

Replace with:

```
new_string:
    // Shared VideoTexture instance reused across all bind functions.
    // sphere materials mutate offset/repeat per eye; fisheye shader
    // ignores offset/repeat and applies its own UV math via uniforms,
    // so sharing is safe.
    let sharedTex = null;
    function getSharedTex() {
      if (!sharedTex) {
        sharedTex = new AFRAME.THREE.VideoTexture(video);
        if (AFRAME.THREE.SRGBColorSpace) sharedTex.colorSpace = AFRAME.THREE.SRGBColorSpace;
      }
      return sharedTex;
    }
    function bindSphereMesh(id) {
      const el = document.getElementById(id);
      if (!el || !window.AFRAME || !AFRAME.THREE) return;
      const mesh = el.getObject3D('mesh');
      if (!mesh || mesh.userData.boundVR) return;
      const tex = getSharedTex();
      mesh.material = new AFRAME.THREE.MeshBasicMaterial({
        map: tex,
        side: AFRAME.THREE.BackSide
      });
      mesh.onBeforeRender = function(renderer, sceneObj, cam) {
        const xr = renderer.xr;
        const stereo = scene.dataset.stereo || '';
        // Out of XR or mono content: full texture, both eyes see the same thing.
        if (!xr || !xr.isPresenting || !stereo) {
          tex.offset.set(0, 0);
          tex.repeat.set(1, 1);
          return;
        }
        const xrCam = xr.getCamera();
        if (!xrCam || !xrCam.cameras || xrCam.cameras.length < 2) return;
        const isLeft = cam === xrCam.cameras[0];
        const isRight = cam === xrCam.cameras[1];
        if (!isLeft && !isRight) return;
        if (stereo === 'sbs') {
          tex.repeat.set(0.5, 1);
          tex.offset.set(isLeft ? 0 : 0.5, 0);
        } else if (stereo === 'tb') {
          tex.repeat.set(1, 0.5);
          tex.offset.set(0, isLeft ? 0 : 0.5);
        } else {
          tex.offset.set(0, 0);
          tex.repeat.set(1, 1);
        }
      };
      mesh.userData.boundVR = true;
    }
    function applySphere() {
      bindSphereMesh('vrSphere180');
      bindSphereMesh('vrSphere360');
    }
```

**Edit 2b:** Update `applyFlat` and `applyFisheye` to use the shared texture, and update the `forEach` listener array to include the new ids.

Find `applyFlat`:

```
old_string:
    function applyFlat() {
      const el = document.getElementById('vrFlat');
      if (!el || !window.AFRAME || !AFRAME.THREE) return;
      const mesh = el.getObject3D('mesh');
      if (!mesh || mesh.userData.boundVR) return;
      const tex = new AFRAME.THREE.VideoTexture(video);
      if (AFRAME.THREE.SRGBColorSpace) tex.colorSpace = AFRAME.THREE.SRGBColorSpace;
      mesh.material = new AFRAME.THREE.MeshBasicMaterial({ map: tex });
      mesh.userData.boundVR = true;
    }
```

Replace with:

```
new_string:
    function applyFlat() {
      const el = document.getElementById('vrFlat');
      if (!el || !window.AFRAME || !AFRAME.THREE) return;
      const mesh = el.getObject3D('mesh');
      if (!mesh || mesh.userData.boundVR) return;
      mesh.material = new AFRAME.THREE.MeshBasicMaterial({ map: getSharedTex() });
      mesh.userData.boundVR = true;
    }
```

Find `applyFisheye` (the texture line near the top of the function):

```
old_string:
      const tex = new AFRAME.THREE.VideoTexture(video);
      if (AFRAME.THREE.SRGBColorSpace) tex.colorSpace = AFRAME.THREE.SRGBColorSpace;
      const fov = parseFloat(el.dataset.fov || '180');
      const material = new AFRAME.THREE.ShaderMaterial({
        side: AFRAME.THREE.BackSide,
        uniforms: {
          uMap:       { value: tex },

new_string:
      const tex = getSharedTex();
      const fov = parseFloat(el.dataset.fov || '180');
      const material = new AFRAME.THREE.ShaderMaterial({
        side: AFRAME.THREE.BackSide,
        uniforms: {
          uMap:       { value: tex },
```

Find the `forEach` listener array and add the new ids:

```
old_string:
    scene.addEventListener('loaded', applyAll);
    ['vrSphere', 'vrFlat', 'vrFisheye'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.addEventListener('object3dset', applyAll);
    });

new_string:
    scene.addEventListener('loaded', applyAll);
    ['vrSphere180', 'vrSphere360', 'vrFlat', 'vrFisheye'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.addEventListener('object3dset', applyAll);
    });
```

- [ ] **Step 3: Build verify**

```
$env:Path += ";C:\Program Files\Go\bin"
go vet ./...
go build ./...
```

Clean.

- [ ] **Step 4: Source-grep verify**

Use the Grep tool against `c:\dev\stash-vr\internal\static\browse_scene.gohtml`:

- Pattern `id="vrSphere180"` matches exactly once (template entity).
- Pattern `id="vrSphere360"` matches exactly once.
- Pattern `id="vrFisheye"` matches exactly once.
- Pattern `id="vrFlat"` matches exactly once.
- Pattern `id="vrSphere"` (without the `180`/`360` suffix) matches nowhere.
- Pattern `getSharedTex` matches multiple times (helper definition + 3 call sites in apply functions).
- Pattern `bindSphereMesh` matches exactly twice (helper definition + 2 calls inside applySphere).
- Pattern `'vrSphere180'` matches multiple times (in applySphere + forEach array).
- Pattern `'vrSphere360'` matches multiple times.

If any expected count is off, re-read the file and verify edits.

- [ ] **Step 5: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: emit all four render entities, share VideoTexture

Template now emits vrSphere180, vrSphere360, vrFisheye, and vrFlat at
all times; the active entity has visible=true, others visible=false,
based on the server-rendered Projection. This sets up the runtime
override path the upcoming projection picker (M3b) will use to switch
between renderers without a page reload.

A single shared THREE.VideoTexture is now reused across all four bind
functions instead of each creating its own. Sphere materials mutate
offset/repeat per eye; the fisheye shader ignores offset/repeat and
applies its own UV math via uniforms, so sharing is safe. Reduces
VRAM/decode duplication by ~4x in the override-active state.

applySphere now binds both vrSphere180 and vrSphere360 (each gated by
mesh.userData.boundVR for idempotency). The applyAll listener array
is updated to register object3dset listeners on the new ids."
```

---

## Task 5: Format button + Picker UI markup + open/close

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

Adds the picker DOM and visibility toggling. No state management or POST yet — Task 6 wires those.

- [ ] **Step 1: Add a "Format" button to the in-VR playback panel**

Edit `c:\dev\stash-vr\internal\static\browse_scene.gohtml`. Find the existing playback panel block (the `vrControls` entity with four `vr-btn` children) and add a fifth button before the Exit button.

The current playback panel is at roughly lines 75-100. Find this region:

```
old_string:
    <a-entity class="vr-btn" data-action="seek-fwd" position="0.18 0 0.01"
              geometry="primitive:plane;width:0.32;height:0.28"
              material="color:#2c5282;opacity:0.95">
      <a-text value="+10s" align="center" color="#fff" width="2" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="exit" position="0.55 0 0.01"
              geometry="primitive:plane;width:0.32;height:0.28"
              material="color:#a01010;opacity:0.95">
      <a-text value="Exit VR" align="center" color="#fff" width="2" position="0 0 0.005"></a-text>
    </a-entity>
  </a-entity>
```

To accommodate a 6th button, redistribute the 5 existing buttons + the new Format button across the 1.6m-wide panel. New positions: `-0.65, -0.39, -0.13, 0.13, 0.39, 0.65`. Replace the entire `vrControls` body:

```
old_string:
  <!-- In-VR playback controls. Positioned below eye-line, tilted toward user. -->
  <a-entity id="vrControls" position="0 0.4 -1.5" rotation="-30 0 0">
    <a-plane width="1.6" height="0.4" color="#000" material="opacity:0.65"></a-plane>
    <a-entity class="vr-btn" data-action="playpause" position="-0.55 0 0.01"
              geometry="primitive:plane;width:0.32;height:0.28"
              material="color:#2c5282;opacity:0.95">
      <a-text value="Play/Pause" align="center" color="#fff" width="2" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="seek-back" position="-0.18 0 0.01"
              geometry="primitive:plane;width:0.32;height:0.28"
              material="color:#2c5282;opacity:0.95">
      <a-text value="-10s" align="center" color="#fff" width="2" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="seek-fwd" position="0.18 0 0.01"
              geometry="primitive:plane;width:0.32;height:0.28"
              material="color:#2c5282;opacity:0.95">
      <a-text value="+10s" align="center" color="#fff" width="2" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="exit" position="0.55 0 0.01"
              geometry="primitive:plane;width:0.32;height:0.28"
              material="color:#a01010;opacity:0.95">
      <a-text value="Exit VR" align="center" color="#fff" width="2" position="0 0 0.005"></a-text>
    </a-entity>
  </a-entity>

new_string:
  <!-- In-VR playback controls. Positioned below eye-line, tilted toward user. -->
  <a-entity id="vrControls" position="0 0.4 -1.5" rotation="-30 0 0">
    <a-plane width="1.85" height="0.4" color="#000" material="opacity:0.65"></a-plane>
    <a-entity class="vr-btn" data-action="playpause" position="-0.75 0 0.01"
              geometry="primitive:plane;width:0.28;height:0.28"
              material="color:#2c5282;opacity:0.95">
      <a-text value="Play/Pause" align="center" color="#fff" width="2.2" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="seek-back" position="-0.45 0 0.01"
              geometry="primitive:plane;width:0.28;height:0.28"
              material="color:#2c5282;opacity:0.95">
      <a-text value="-10s" align="center" color="#fff" width="2" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="seek-fwd" position="-0.15 0 0.01"
              geometry="primitive:plane;width:0.28;height:0.28"
              material="color:#2c5282;opacity:0.95">
      <a-text value="+10s" align="center" color="#fff" width="2" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="format" position="0.15 0 0.01"
              geometry="primitive:plane;width:0.28;height:0.28"
              material="color:#2c5282;opacity:0.95">
      <a-text value="Format" align="center" color="#fff" width="2.2" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="exit" position="0.45 0 0.01"
              geometry="primitive:plane;width:0.28;height:0.28"
              material="color:#a01010;opacity:0.95">
      <a-text value="Exit VR" align="center" color="#fff" width="2.2" position="0 0 0.005"></a-text>
    </a-entity>
  </a-entity>
```

Note: only 5 buttons total (Play/Pause, -10s, +10s, Format, Exit) — the panel grew from 4 to 5 buttons; the +0.65/-0.65 positions are unused so the layout slots are -0.75/-0.45/-0.15/+0.15/+0.45 with the panel widened from 1.6m to 1.85m.

- [ ] **Step 2: Add the picker entity (markup only, no JS yet)**

Add the `vrFormatPicker` entity inside `<a-scene>`, after the `vrControls` entity. Find this region:

```
old_string:
    <a-entity class="vr-btn" data-action="exit" position="0.45 0 0.01"
              geometry="primitive:plane;width:0.28;height:0.28"
              material="color:#a01010;opacity:0.95">
      <a-text value="Exit VR" align="center" color="#fff" width="2.2" position="0 0 0.005"></a-text>
    </a-entity>
  </a-entity>

  <!-- Laser controllers for raycast-clicking the panel buttons. -->
```

Replace with the same content plus the new picker block before the laser-controls comment:

```
new_string:
    <a-entity class="vr-btn" data-action="exit" position="0.45 0 0.01"
              geometry="primitive:plane;width:0.28;height:0.28"
              material="color:#a01010;opacity:0.95">
      <a-text value="Exit VR" align="center" color="#fff" width="2.2" position="0 0 0.005"></a-text>
    </a-entity>
  </a-entity>

  <!--
    In-VR projection picker. Hidden by default; toggled by the Format
    button on the playback panel. Three rows (Type, Degree, Stereo)
    of mutually-exclusive buttons + an Auto button that re-detects.
    Each preset button has data-pick-row and data-pick-value attrs that
    the JS tap handler reads to update state.
  -->
  <a-entity id="vrFormatPicker" position="0 1.4 -1.5" rotation="-15 0 0" visible="false">
    <a-plane width="2.4" height="1.2" color="#000" material="opacity:0.7"></a-plane>
    <a-text value="Format" align="left" color="#fff" width="3.5" position="-1.1 0.5 0.01"></a-text>

    <!-- Row 1: Type -->
    <a-text value="Type"   align="right" color="#aaa" width="2" position="-0.85 0.28 0.01"></a-text>
    <a-entity class="vr-btn picker-btn" data-pick-row="type" data-pick-value="Normal"  position="-0.55 0.28 0.01"
              geometry="primitive:plane;width:0.45;height:0.18" material="color:#2c5282;opacity:0.95">
      <a-text value="Normal"  align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn picker-btn" data-pick-row="type" data-pick-value="FishEye" position="0.0 0.28 0.01"
              geometry="primitive:plane;width:0.45;height:0.18" material="color:#2c5282;opacity:0.95">
      <a-text value="FishEye" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
    </a-entity>

    <!-- Row 2: Degree -->
    <a-text value="Degree" align="right" color="#aaa" width="2" position="-0.85 0.04 0.01"></a-text>
    <a-entity class="vr-btn picker-btn" data-pick-row="degree" data-pick-value="Cinema" position="-0.55 0.04 0.01"
              geometry="primitive:plane;width:0.30;height:0.18" material="color:#2c5282;opacity:0.95">
      <a-text value="Cinema" align="center" color="#fff" width="3" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn picker-btn" data-pick-row="degree" data-pick-value="180"    position="-0.18 0.04 0.01"
              geometry="primitive:plane;width:0.30;height:0.18" material="color:#2c5282;opacity:0.95">
      <a-text value="180" align="center" color="#fff" width="3" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn picker-btn" data-pick-row="degree" data-pick-value="200"    position="0.19 0.04 0.01"
              geometry="primitive:plane;width:0.30;height:0.18" material="color:#2c5282;opacity:0.95">
      <a-text value="200" align="center" color="#fff" width="3" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn picker-btn" data-pick-row="degree" data-pick-value="360"    position="0.56 0.04 0.01"
              geometry="primitive:plane;width:0.30;height:0.18" material="color:#2c5282;opacity:0.95">
      <a-text value="360" align="center" color="#fff" width="3" position="0 0 0.005"></a-text>
    </a-entity>

    <!-- Row 3: Stereo -->
    <a-text value="Stereo" align="right" color="#aaa" width="2" position="-0.85 -0.20 0.01"></a-text>
    <a-entity class="vr-btn picker-btn" data-pick-row="stereo" data-pick-value="2D"  position="-0.55 -0.20 0.01"
              geometry="primitive:plane;width:0.30;height:0.18" material="color:#2c5282;opacity:0.95">
      <a-text value="2D" align="center" color="#fff" width="3" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn picker-btn" data-pick-row="stereo" data-pick-value="SBS" position="-0.18 -0.20 0.01"
              geometry="primitive:plane;width:0.30;height:0.18" material="color:#2c5282;opacity:0.95">
      <a-text value="SBS" align="center" color="#fff" width="3" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn picker-btn" data-pick-row="stereo" data-pick-value="TB"  position="0.19 -0.20 0.01"
              geometry="primitive:plane;width:0.30;height:0.18" material="color:#2c5282;opacity:0.95">
      <a-text value="TB" align="center" color="#fff" width="3" position="0 0 0.005"></a-text>
    </a-entity>

    <!-- Auto -->
    <a-entity class="vr-btn picker-btn" data-pick-row="auto" data-pick-value="auto" position="0.0 -0.46 0.01"
              geometry="primitive:plane;width:0.80;height:0.16" material="color:#3776c2;opacity:0.95">
      <a-text value="Auto (re-detect)" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
    </a-entity>
  </a-entity>

  <!-- Laser controllers for raycast-clicking the panel buttons. -->
```

- [ ] **Step 3: Add open/close JS for the picker**

Edit the inline JS. Add a new `vrAction === 'format'` branch in the existing `vrAction` switch, and a `vrFormatPicker` element reference at the top of the IIFE.

Find the IIFE preamble (the `const` declarations near the top):

```
old_string:
    const btn   = document.getElementById('enterVR');
    const scene = document.getElementById('vrScene');
    const wrap  = document.querySelector('.wrap');
    const video = document.getElementById('sceneVideo');
    if (!btn || !scene || !wrap || !video) return;

new_string:
    const btn          = document.getElementById('enterVR');
    const scene        = document.getElementById('vrScene');
    const wrap         = document.querySelector('.wrap');
    const video        = document.getElementById('sceneVideo');
    const picker       = document.getElementById('vrFormatPicker');
    if (!btn || !scene || !wrap || !video) return;
```

Find the `vrAction` function:

```
old_string:
      } else if (action === 'exit') {
        try { scene.exitVR(); } catch (e) { console.warn('stash-vr: exitVR failed', e); }
      }
    }

new_string:
      } else if (action === 'exit') {
        try { scene.exitVR(); } catch (e) { console.warn('stash-vr: exitVR failed', e); }
      } else if (action === 'format') {
        if (picker) {
          const visible = picker.getAttribute('visible');
          picker.setAttribute('visible', !visible);
        }
      }
    }
```

A-Frame's `getAttribute('visible')` returns a boolean for the `visible` component, and `setAttribute('visible', bool)` toggles it. Tap on Format toggles the picker's visibility.

- [ ] **Step 4: Build verify**

```
$env:Path += ";C:\Program Files\Go\bin"
go vet ./...
go build ./...
```

Clean.

- [ ] **Step 5: Source-grep verify**

Against `c:\dev\stash-vr\internal\static\browse_scene.gohtml`:

- Pattern `data-action="format"` matches exactly once (the new playback-panel button).
- Pattern `id="vrFormatPicker"` matches exactly once.
- Pattern `class="vr-btn picker-btn"` matches exactly **10** times (2 Type + 4 Degree + 3 Stereo + 1 Auto).
- Pattern `data-pick-row="type"` matches exactly twice.
- Pattern `data-pick-row="degree"` matches exactly 4 times.
- Pattern `data-pick-row="stereo"` matches exactly 3 times.
- Pattern `data-pick-row="auto"` matches exactly once.
- Pattern `action === 'format'` matches exactly once.

- [ ] **Step 6: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: add Format button + projection picker markup

Adds a Format button to the in-VR playback panel (panel widens from
1.6m to 1.85m to fit a fifth button). Adds the vrFormatPicker entity
with three rows of mutually-exclusive buttons (Type / Degree / Stereo)
+ an Auto button, hidden by default.

Tapping Format toggles picker.setAttribute('visible', ...). No tap
handlers on the preset buttons yet — those wire up in the next commit."
```

---

## Task 6: Picker tap handlers + apply behavior + POST + active-highlight + invalid-combo disabling

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

The big behavior commit. Adds:
- Picker state (`type`, `degree`, `stereo`) — initialized from server-rendered Projection.
- Tap handlers for the 10 picker buttons.
- `applyPickerState()` that maps state → Projection, updates `<a-scene>` data-stereo and `vrFisheye` data-fov, toggles render-entity visibility, resets `boundVR` flags, calls `applyAll`, and updates per-row active highlighting.
- POST to `/browse/scene/{id}/projection` with single-in-flight lock.
- Disabled-state styling for buttons that are invalid given the current Type (Cinema/360 disabled when FishEye; 200 disabled when Normal).
- Auto button handling.

- [ ] **Step 1: Add picker state, helpers, tap handlers, and POST in the IIFE**

Edit `c:\dev\stash-vr\internal\static\browse_scene.gohtml`. Insert a new block of JS just before the existing `function show2D()` definition.

Find this region:

```
old_string:
    document.querySelectorAll('.vr-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        vrAction(btn.dataset.action || btn.getAttribute('data-action'));
      });
    });

    function show2D() {
```

Replace with (inserting picker JS between the click registration and `show2D`):

```
new_string:
    document.querySelectorAll('.vr-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        const action = btn.dataset.action || btn.getAttribute('data-action');
        if (action) {
          vrAction(action);
          return;
        }
        const row = btn.dataset.pickRow;
        const value = btn.dataset.pickValue;
        if (row) handlePickerTap(row, value);
      });
    });

    // ============================================================
    // Picker state + handlers.
    //
    // Initial state derived from the server-rendered Projection
    // (read off <a-scene>'s data-stereo, vrFisheye's data-fov, and
    // the visibility of the four render entities). The user picks
    // one button per row; on every tap we (a) re-bind the renderer
    // locally, (b) POST the new selection so Stash tags update,
    // (c) update the active-highlight per row.
    // ============================================================

    const sceneId = (function() {
      const m = location.pathname.match(/\/browse\/scene\/([^\/]+)/);
      return m ? m[1] : '';
    })();
    const pickerState = computeInitialPickerState();

    // computeInitialPickerState reads the server-rendered Projection
    // off the <a-scene> attributes and the four render entities to
    // determine which preset is currently active. Used both for the
    // initial active-highlight and as the revert target for Auto.
    function computeInitialPickerState() {
      const stereoAttr = scene.dataset.stereo || '';
      const fisheyeEl = document.getElementById('vrFisheye');
      const sphere180 = document.getElementById('vrSphere180');
      const sphere360 = document.getElementById('vrSphere360');
      const flat      = document.getElementById('vrFlat');
      let type = 'Normal', degree = '180', stereo = 'SBS';
      if (flat && flat.getAttribute('visible')) {
        type = 'Normal'; degree = 'Cinema'; stereo = '2D';
      } else if (fisheyeEl && fisheyeEl.getAttribute('visible')) {
        type = 'FishEye';
        const fov = parseFloat(fisheyeEl.dataset.fov || '180');
        degree = (fov >= 200 ? '200' : '180');
      } else if (sphere360 && sphere360.getAttribute('visible')) {
        type = 'Normal'; degree = '360';
      } else if (sphere180 && sphere180.getAttribute('visible')) {
        type = 'Normal'; degree = '180';
      }
      if (degree === 'Cinema') {
        stereo = '2D';
      } else if (stereoAttr === 'sbs') {
        stereo = 'SBS';
      } else if (stereoAttr === 'tb') {
        stereo = 'TB';
      } else {
        stereo = '2D';
      }
      return { type, degree, stereo, initial: null };
    }
    pickerState.initial = { type: pickerState.type, degree: pickerState.degree, stereo: pickerState.stereo };

    // Apply current pickerState: update data attrs, toggle entity
    // visibility, reset boundVR flags so applyAll rebinds, then
    // recompute the per-row highlights and disabled state.
    function applyPickerState() {
      const isFishEye = pickerState.type === 'FishEye';
      const isCinema  = pickerState.degree === 'Cinema';
      const isNormal  = pickerState.type === 'Normal';

      // Cinema forces stereo=2D regardless of the user's Stereo pick.
      const effectiveStereo = isCinema ? '2D' : pickerState.stereo;
      const stereoData = effectiveStereo === 'SBS' ? 'sbs' :
                         effectiveStereo === 'TB'  ? 'tb'  : '';
      scene.dataset.stereo = stereoData;

      // Decide which render entity is active.
      let activeId = 'vrFlat';
      if (!isCinema) {
        if (isFishEye) {
          activeId = 'vrFisheye';
          const fovEl = document.getElementById('vrFisheye');
          if (fovEl) fovEl.dataset.fov = (pickerState.degree === '200' ? '200' : '180');
        } else if (isNormal && pickerState.degree === '360') {
          activeId = 'vrSphere360';
        } else if (isNormal && pickerState.degree === '180') {
          activeId = 'vrSphere180';
        }
      }
      ['vrSphere180', 'vrSphere360', 'vrFisheye', 'vrFlat'].forEach(id => {
        const el = document.getElementById(id);
        if (!el) return;
        el.setAttribute('visible', id === activeId);
        // Reset bind flag so applyAll re-creates the material with
        // the right offsets (sphere) or re-creates the shader
        // (fisheye) for the new geometry/fov.
        const mesh = el.getObject3D('mesh');
        if (mesh) mesh.userData.boundVR = false;
      });
      applyAll();

      updatePickerHighlights();
      updatePickerDisabled();
    }

    function updatePickerHighlights() {
      document.querySelectorAll('.picker-btn').forEach(b => {
        const row = b.dataset.pickRow;
        const value = b.dataset.pickValue;
        const isActive = (row !== 'auto' && pickerState[row] === value);
        b.setAttribute('material', 'color: ' + (isActive ? '#3776c2' : '#2c5282') + '; opacity: 0.95');
      });
    }

    function isInvalidCombo(row, value) {
      // Cinema is only valid with Type=Normal.
      if (row === 'degree' && value === 'Cinema' && pickerState.type === 'FishEye') return true;
      // 200 is only valid with Type=FishEye.
      if (row === 'degree' && value === '200' && pickerState.type === 'Normal') return true;
      // 360 is only valid with Type=Normal.
      if (row === 'degree' && value === '360' && pickerState.type === 'FishEye') return true;
      // When degree is Cinema, all stereo other than 2D is moot but not "invalid" per se
      // — leave them tappable; tapping a Stereo button while Cinema doesn't change render
      // (Cinema forces 2D in applyPickerState). No styling change for these.
      return false;
    }

    function updatePickerDisabled() {
      document.querySelectorAll('.picker-btn').forEach(b => {
        const row = b.dataset.pickRow;
        const value = b.dataset.pickValue;
        if (row === 'auto') return;
        const disabled = isInvalidCombo(row, value);
        if (disabled) {
          b.setAttribute('material', 'color: #444; opacity: 0.4');
          b.dataset.disabled = '1';
        } else {
          delete b.dataset.disabled;
          // Active-highlight setter in updatePickerHighlights will
          // set the right color; this branch only ensures non-disabled
          // buttons aren't stuck at the disabled color.
        }
      });
    }

    // Single in-flight lock for POST. Rapid taps coalesce — the latest
    // pickerState wins; intermediate POSTs that haven't started yet
    // are skipped.
    let postInFlight = false;
    let postPending  = null;
    function postProjection(body) {
      postPending = body;
      if (postInFlight) return;
      const send = () => {
        const next = postPending;
        if (!next) { postInFlight = false; return; }
        postPending = null;
        postInFlight = true;
        fetch('/browse/scene/' + encodeURIComponent(sceneId) + '/projection', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(next)
        }).then(resp => {
          if (!resp.ok) console.warn('stash-vr: projection POST returned', resp.status);
        }).catch(err => {
          console.warn('stash-vr: projection POST failed', err);
        }).finally(() => {
          postInFlight = false;
          if (postPending) send();
        });
      };
      send();
    }

    function handlePickerTap(row, value) {
      // Auto: revert pickerState to initial, re-apply, POST {auto:true}.
      if (row === 'auto') {
        pickerState.type   = pickerState.initial.type;
        pickerState.degree = pickerState.initial.degree;
        pickerState.stereo = pickerState.initial.stereo;
        applyPickerState();
        postProjection({ auto: true });
        return;
      }
      // Disabled buttons no-op.
      const btnEl = document.querySelector('.picker-btn[data-pick-row="' + row + '"][data-pick-value="' + value + '"]');
      if (btnEl && btnEl.dataset.disabled === '1') return;
      // Update state.
      pickerState[row] = value;
      // Picking Type may invalidate current Degree — auto-correct to a
      // valid neighbor so the resulting Projection is renderable.
      if (row === 'type') {
        if (value === 'FishEye' && (pickerState.degree === 'Cinema' || pickerState.degree === '360')) {
          pickerState.degree = '180';
        }
        if (value === 'Normal' && pickerState.degree === '200') {
          pickerState.degree = '180';
        }
      }
      applyPickerState();
      postProjection({
        type:   pickerState.type,
        degree: pickerState.degree,
        stereo: pickerState.stereo
      });
    }

    // Initial highlight + disabled-state pass once A-Frame is ready.
    scene.addEventListener('loaded', function() {
      updatePickerHighlights();
      updatePickerDisabled();
    });

    function show2D() {
```

- [ ] **Step 2: Build verify**

```
$env:Path += ";C:\Program Files\Go\bin"
go vet ./...
go build ./...
```

Clean.

- [ ] **Step 3: Source-grep verify**

Against `c:\dev\stash-vr\internal\static\browse_scene.gohtml`:

- Pattern `function handlePickerTap` matches exactly once.
- Pattern `function applyPickerState` matches exactly once.
- Pattern `function postProjection` matches exactly once.
- Pattern `function isInvalidCombo` matches exactly once.
- Pattern `function updatePickerHighlights` matches exactly once.
- Pattern `function updatePickerDisabled` matches exactly once.
- Pattern `function computeInitialPickerState` matches exactly once.
- Pattern `'/browse/scene/' \+ encodeURIComponent\(sceneId\)` matches exactly once.
- Pattern `pickerState\.initial` matches multiple times (set + read in Auto handler).
- Pattern `boundVR = false` matches exactly once (the reset in `applyPickerState`).

- [ ] **Step 4: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: wire projection picker tap handlers + POST

Picker now functional. Each tap on a Type/Degree/Stereo button updates
local state, recomputes the active render entity, toggles visibility
of the four render entities, resets boundVR so applyAll rebinds the
material/shader for the new combo, updates per-row active highlight,
and POSTs the new selection to /browse/scene/{id}/projection.

Auto button reverts pickerState to the server-rendered initial values
and POSTs {auto:true}. Single in-flight POST lock — rapid taps
coalesce to the latest selection.

Invalid combinations (Cinema with FishEye, 200 with Normal, 360 with
FishEye) gray out the offending Degree button and ignore taps on it.
Picking Type=FishEye while Degree is Cinema/360 auto-corrects to 180."
```

---

## Task 7: On-headset validation (manual, hand off to user)

This task is manual — the controller agent should hand it off to the user with the test plan below.

**Files:**
- Create: `docs/superpowers/research/2026-05-08-m3b-result/result.md`

- [ ] **Step 1: Restart the stash-vr binary**

PowerShell:
```
$env:Path += ";C:\Program Files\Go\bin"
go build -o stash-vr.exe ./cmd/stash-vr
.\stash-vr.exe
```

- [ ] **Step 2: Walk the validation matrix on Quest 3 / Meta Browser**

| # | Step | Pass/Fail criteria |
|---|---|---|
| 1 | Open scene 5535 (V-shape on M3a). Click Enter VR. | M3a-rendered V-shape is present (this is the baseline). |
| 2 | Click Format on the playback panel. | Picker panel becomes visible above playback bar. Three rows of buttons + Auto are tappable. |
| 3 | Pick `FishEye + 200° + SBS`. | Renderer rebinds live (no page reload). V-shape is gone. The Degree row shows `200°` highlighted, Type shows `FishEye`, Stereo shows `SBS`. |
| 4 | Exit VR. Reload the page. | `<a-scene>` data-stereo is `sbs`. The Format button label says "Enter VR" not "Watch on virtual screen". Click Enter VR; auto-detection now picks the new projection (the scene is tagged `VR_MKX200`+`VR_SBS`). The Format menu's active highlights match. |
| 5 | Tap Auto. | Renderer reverts to the initial server-detected Projection (likely DOME+SBS via M2 fallback). `VR_*` projection tags are removed from Stash (verify by reloading the page; Format menu's active highlights now reflect the auto-detected state). |
| 6 | On a known DOME+SBS scene that worked correctly in M3a: open Format, verify active highlight is `Normal / 180° / SBS`. Pick `Normal / 180° / TB`. | Renderer rebinds to TB stereo (vertical eye split). After reload, scene is tagged `VR_DOME`+`VR_TB`. |
| 7 | Pick Cinema. | Renderer drops to flat virtual screen. After reload, scene has no `VR_*` projection tags. |
| 8 | Type=FishEye selected. | Cinema and 360° Degree buttons are greyed out (#444, opacity 0.4). Tapping them does nothing. |
| 9 | Type=Normal selected. | 200° Degree button is greyed out. Tapping does nothing. |
| 10 | Picking Type=FishEye while Degree was Cinema. | Degree auto-corrects to 180°. |
| 11 | Regression: M2/M3a behavior. | Audio still in sync; no first-frame black flash; in-VR play/pause/-10s/+10s/exit work; non-VR scenes still render flat virtual screen. |

- [ ] **Step 3: Write the result artifact**

Create `c:\dev\stash-vr\docs\superpowers\research\2026-05-08-m3b-result\result.md`. One PASS/FAIL line per matrix row, plus a free-form "Surprises" section. Recommendation: green-light the IPD slider follow-up + M3c controller mappings, or block on a re-spec if anything FAILed.

- [ ] **Step 4: Commit the result artifact**

```
git add docs/superpowers/research/2026-05-08-m3b-result/result.md
git commit -m "browse: M3b in-VR projection picker on-headset validation result"
```

---

## Self-review against spec

**Spec coverage check:**

- Spec §2 success criterion 1 (Format button on playback panel) → Task 5 Step 1.
- Spec §2 success criterion 2 (Tapping Format toggles picker) → Task 5 Step 3.
- Spec §2 success criterion 3 (3-row picker mirrors SKYBOX, currently-active highlighted) → Task 5 Step 2 (markup) + Task 6 Step 1 (`updatePickerHighlights`).
- Spec §2 success criterion 4 (live re-bind + POST on tap) → Task 6 Step 1 (`handlePickerTap` + `applyPickerState` + `postProjection`).
- Spec §2 success criterion 5 (invalid combos disabled) → Task 6 Step 1 (`isInvalidCombo` + `updatePickerDisabled`).
- Spec §2 success criterion 6 (Auto removes all `VR_*` projection tags) → Task 6 Step 1 (`handlePickerTap` Auto branch posts `{auto:true}`) + Task 3's handler.
- Spec §2 success criterion 7 (after write-back, reload picks up new projection) → Task 1 (constants migration; detection now matches the written `VR_`-prefixed tags) + Task 3 (handler writes the right tags).
- Spec §2 success criterion 8 (close picker by tapping Format again) → Task 5 Step 3 (`vrAction === 'format'` toggles).
- Spec §2 success criterion 9 (M2/M3a behavior preserved) → Tasks 4-6 leave M2/M3a touchpoints unchanged; Task 7 Step 2 row 11 verifies on-headset.
- Spec §3 (constants migration) → Task 1.
- Spec §4.1 (trigger button position) → Task 5 Step 1 (panel widened to 1.85m, 5 buttons positioned at -0.75/-0.45/-0.15/+0.15/+0.45).
- Spec §4.2 (picker layout: three rows + Auto) → Task 5 Step 2.
- Spec §4.3 (three-field → Projection mapping) → Task 6 Step 1 (`applyPickerState`) and Task 3 (`projectionFromRequest`).
- Spec §4.4 (apply behavior: state update → rebind → POST) → Task 6 Step 1.
- Spec §4.5 (Auto behavior) → Task 6 Step 1 Auto branch + Task 3 handler Auto branch.
- Spec §5.1 (POST endpoint shape) → Task 3.
- Spec §5.2 (handler file location) → Task 3.
- Spec §5.3 (constants migration propagates to /deovr, /heresphere) → Task 1 (no edits to /deovr or /heresphere needed).
- Spec §5.4 (no new GraphQL operations) → confirmed; reuses `library.UpdateTags`.
- Spec §6 (file table) → matches plan File Structure.
- Spec §7 (validation) → Task 7.
- Spec §8 risks (multi-entity DOM, in-flight tag writes, Stash unreachable, mistagged-stays-mistagged-until-user-picks, constants migration breaks bare-tag libraries, picker disabled-button state, boundVR reset cost) → addressed by Tasks 4 (multi-entity) + 6 (single in-flight, error-tolerant, disabled-state, boundVR=false on apply).
- Spec §9 (what stays untouched) → no edits to `library.Service`, `internal.Detect`, `/deovr`, `/heresphere` (constants only), M1 surfaces, A-Frame.

No spec gaps.

**Type / API consistency check:**

- `Projection{Geometry, FOV, Stereo}` consistent across `projection.go::Detect`, `projection.go::TagsForProjection`, `scene_projection.go::projectionFromRequest`.
- Field values: Geometry strings `"equirectangular"` / `"fisheye"` / `""`; Stereo strings `"sbs"` / `"tb"` / `""`. Used identically in Go and JS.
- Tag constant names `TagVR_DOME` / `TagVR_SPHERE` / etc. consistent across `legend.go`, `Detect`, `TagsForProjection`, `sceneProjectionHandler`.
- Picker JSON shape `{type, degree, stereo, auto}` consistent between client (`postProjection`) and server (`projectionRequest`).
- Picker state field names `type`, `degree`, `stereo` consistent across `pickerState`, `handlePickerTap`, `applyPickerState`, `computeInitialPickerState`, `postProjection`.
- HTML id values `vrSphere180` / `vrSphere360` / `vrFisheye` / `vrFlat` consistent across template, JS bind functions, JS visibility-toggle loop.
- A-Frame data attributes `data-stereo` (on `<a-scene>`), `data-fov` (on `vrFisheye`), `data-action` (on playback `vr-btn`s), `data-pick-row` / `data-pick-value` (on picker `vr-btn`s) — each used consistently.

**Placeholder scan:**

No "TBD", no "TODO", no "fill in details", no "similar to Task N", no "handle edge cases" without code. Every code step has a complete code block. Every command has the exact invocation.
