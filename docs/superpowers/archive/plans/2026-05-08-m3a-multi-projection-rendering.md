# M3a Multi-Projection VR Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render any of the 15 combinations of `{DOME, SPHERE, FISHEYE, MKX200, RF52} × {SBS, TB, mono}` correctly in the `/browse/scene/{id}` WebXR player. Tag-first detection (alias-aware, mirroring `/deovr` and `/heresphere`'s `set3DFormat` pattern) with SKYBOX-style filename-keyword fallback when no projection tag is present.

**Architecture:** New `internal/api/internal/projection.go` module exposes a `Projection` struct (`Geometry`/`FOV`/`Stereo`) and a pure `Detect(tags, basename)` function with unit tests. The browse handler replaces its current ad-hoc VR-string check with a single `Detect` call. The template branches on `Projection.Geometry × Projection.FOV` to emit equirectangular half-sphere, equirectangular full sphere, or sphere-with-fisheye-shader geometry. A single `<video>` element drives the texture for every path (preserves M2's sync-fix architecture). Per-eye UV swap is generalized via `<a-scene>` data attributes carrying SBS/TB/mono offsets.

**Spec:** [docs/superpowers/specs/2026-05-08-m3a-multi-projection-rendering.md](../specs/2026-05-08-m3a-multi-projection-rendering.md)

**Tech Stack:** Go 1.24 (`html/template`, no SPA framework, no JS bundler), A-Frame 1.7.0 (vendored at `internal/static/vendor/aframe.min.js`), Three.js (bundled inside A-Frame), inline GLSL fragment + vertex shaders, inline JS in the gohtml template. Per [CLAUDE.md](../../../CLAUDE.md), `go vet ./...` and `go build ./...` are the standard checks; no project-wide test suite exists, but Task 1 adds a focused test file for the new pure-function detection module.

---

## File Structure

| Path | Responsibility | Status |
|---|---|---|
| `internal/api/internal/projection.go` | New. `Projection` struct + `TagInput` adapter + `Detect(tags []TagInput, basename string) Projection`. Tag pass uses `util.StrSliceEquals` against the existing `TagVR_*` constants in [legend.go](../../../internal/api/internal/legend.go). Filename-keyword fallback runs only when the tag pass produces no Geometry. Pure function — no I/O, no GraphQL types. | Create in Task 1. |
| `internal/api/internal/projection_test.go` | New. Table-driven unit tests covering each tag, each filename keyword, conflict resolution, and the "no detection" fallback. | Create in Task 1. |
| `internal/api/browse/data.go` | Modify. Replace `VRMode string` field on `SceneDetailData` with `Projection internal.Projection`. | Task 2. |
| `internal/api/browse/scene.go` | Modify. Replace the inline ad-hoc VR detection (`is180SBS` plus the regex `vrFilenameRe`) with one call to `apiinternal.Detect(...)`. Drop the `vrFilenameRe` regex entirely (subsumed by the new module). | Task 2. |
| `internal/static/browse_scene.gohtml` | Modify across Tasks 2–5. Switch template branches from `.VRMode` to `.Projection.Geometry`/`.Projection.FOV`. Add SPHERE 360° geometry (Task 3), generalize SBS/TB/mono UV swap (Task 4), add fisheye `ShaderMaterial` path (Task 5). | Tasks 2–5. |

No new vendored files. No changes to `legend.go`, `genqlient`, `library.Service`, or any HTTP routing. M2's single-video architecture, audio-defaults-on, and `<a-scene background="color:#111">` are preserved across every task.

## Pre-flight

- [ ] **Step 0a: Confirm working directory and tree state**

Run: `git status` and `git log -1 --oneline`

Expected: working dir `c:\dev\stash-vr`, tree clean. Most recent commit is `f909b30 browse: VR sync/flash fix on-headset validation result` (the M2 follow-up validation result). If newer commits touch `browse_scene.gohtml`, `browse/scene.go`, `browse/data.go`, or `internal/api/internal/`, re-read the relevant file before applying edits — line numbers and exact strings in this plan assume that head.

- [ ] **Step 0b: Confirm Go is on PATH**

PowerShell: `Get-Command go` should resolve to a `go.exe` (typically `C:\Program Files\Go\bin\go.exe`). If not, prepend it for the session: `$env:Path += ";C:\Program Files\Go\bin"`. The Bash tool in this environment doesn't have Go on PATH; use PowerShell for Go commands.

---

## Task 1: Add `projection.go` module + unit tests (standalone, no integration yet)

**Files:**
- Create: `internal/api/internal/projection.go`
- Create: `internal/api/internal/projection_test.go`

This task is fully standalone. The module compiles, the tests pass, but no caller imports it. Wiring happens in Task 2. Doing this in two commits keeps each commit a working state.

- [ ] **Step 1: Write the failing test file**

Create `c:\dev\stash-vr\internal\api\internal\projection_test.go` with the contents below. This is the full test file — the implementer should not abbreviate. Tests are table-driven for compactness; each row exercises one detection rule.

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail (no implementation yet)**

PowerShell from `c:\dev\stash-vr`:
```
go test ./internal/api/internal/...
```

Expected: compile error (`undefined: TagInput`, `undefined: Detect`, `undefined: Projection`). Confirms the tests are wired and the module doesn't yet exist.

- [ ] **Step 3: Write the implementation**

Create `c:\dev\stash-vr\internal\api\internal\projection.go` with this exact content:

```go
package internal

import (
	"strings"

	"stash-vr/internal/util"
)

// Projection describes how a VR scene should be rendered. Empty Geometry
// means no VR detected — the renderer falls back to a flat virtual screen.
type Projection struct {
	Geometry string // "equirectangular" | "fisheye" | ""
	FOV      int    // 180, 190, 200, 360 (or 0 if Geometry is "")
	Stereo   string // "sbs" | "tb" | ""  ("" = mono)
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

	for _, t := range tags {
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
		p.Geometry, p.FOV = "fisheye", 180
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

	// Filename-keyword fallback. Only runs if no Geometry tag matched.
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
		p.Geometry, p.FOV = "fisheye", 180
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
	switch {
	case strings.Contains(lc, "_2d_") || strings.HasPrefix(lc, "2d_"):
		// Explicit mono. Leave p.Stereo empty.
	case strings.Contains(lc, "_lr_") || strings.HasPrefix(lc, "lr_"):
		p.Stereo = "sbs"
	case strings.Contains(lc, "_tb_") || strings.HasPrefix(lc, "tb_"):
		p.Stereo = "tb"
	}

	return p
}
```

- [ ] **Step 4: Run the tests to verify they pass**

PowerShell from `c:\dev\stash-vr`:
```
go test ./internal/api/internal/... -v
```

Expected: every subtest in `TestDetect` passes. `PASS` line, `ok stash-vr/internal/api/internal <duration>`. If any subtest fails, fix the implementation in `projection.go` (not the test) — the test cases are the spec.

- [ ] **Step 5: Run full build to confirm no regressions**

PowerShell from `c:\dev\stash-vr`:
```
$env:Path += ";C:\Program Files\Go\bin"
go vet ./...
go build ./...
```

Expected: both clean (no output, exit 0).

- [ ] **Step 6: Commit**

```
git add internal/api/internal/projection.go internal/api/internal/projection_test.go
git commit -m "browse: add Projection type and Detect function with tests

New internal/api/internal/projection.go module that resolves a
Projection (Geometry/FOV/Stereo) from scene tags and an optional file
basename. Tag pass uses util.StrSliceEquals for alias-aware matching
against the existing TagVR_* constants, mirroring the pattern in
deovr/videodata.go and heresphere/videodata.go. Filename keyword
fallback covers SKYBOX-style names (MKX200, FISHEYE190, _360, LR_, TB_,
etc.) and only runs when no projection tag matched. Tags win on
conflict.

Module is standalone — no caller imports it yet. Wiring into the browse
handler is the next commit."
```

---

## Task 2: Wire `Projection` into the browse handler — preserve M2 behavior

**Files:**
- Modify: `internal/api/browse/data.go`
- Modify: `internal/api/browse/scene.go`
- Modify: `internal/static/browse_scene.gohtml`

After this task, the existing M2 DOME+SBS path renders identically. New Projection values (SPHERE 360°, fisheye, etc.) detected by Task 1's module fall through to the existing flat virtual-screen fallback — they are wired up in Tasks 3–5. The point of doing this as a single commit is to land the data-model change without simultaneously changing rendering behavior, so any regression after this commit means the wiring is wrong, not the new geometries.

- [ ] **Step 1: Update `SceneDetailData` to carry `Projection` instead of `VRMode`**

Edit `c:\dev\stash-vr\internal\api\browse\data.go`. Replace this block (currently around line 65–67):

```
old_string:
	DirectStreamURL string
	VRMode          string // "180sbs" for stereo half-sphere; "flat" for flat plane in 3D space
	ErrMessage      string
```

with:

```
new_string:
	DirectStreamURL string
	Projection      apiinternal.Projection
	ErrMessage      string
```

Add the import. Replace the import block at the top of the file. The current `data.go` has no imports yet — add this import block right after `package browse`:

```
old_string:
package browse

// Entity is a sidebar row (performer / studio / tag).
new_string:
package browse

import (
	apiinternal "stash-vr/internal/api/internal"
)

// Entity is a sidebar row (performer / studio / tag).
```

- [ ] **Step 2: Replace the inline VR detection in `scene.go` with a single `apiinternal.Detect` call**

Edit `c:\dev\stash-vr\internal\api\browse\scene.go`. Two distinct edits.

**Edit 2a:** Remove the now-unused `vrFilenameRe` regex and the `regexp` import.

```
old_string:
import (
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/api/heatmap"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/config"
	"stash-vr/internal/prefix"
	"stash-vr/internal/static"
)

// vrFilenameRe matches common VR markers in a basename or path. Hand-rolled
// boundaries because Go's \b treats _ as a word char (so "_VR_" wouldn't
// satisfy \bVR\b). MKX matches MKX, MKX200, MKX220, etc.
var vrFilenameRe = regexp.MustCompile(`(?i)(^|[^a-z0-9])(180|360|MKX[0-9]*|FB360|FISHEYE|DOME|SBS|EAC|RF52)([^a-z0-9]|$)`)

new_string:
import (
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/api/heatmap"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/config"
	"stash-vr/internal/prefix"
	"stash-vr/internal/static"
)
```

**Edit 2b:** Replace the inline `is180SBS` detection block (currently lines 86–133 of `scene.go`, the entire span from `favTag := config.Application().FavoriteTag` through the `if is180SBS { data.VRMode = "180sbs" } else { data.VRMode = "flat" }` block) with a version that uses `apiinternal.Detect`. The favorite-detection and ancestor-skip logic still runs in the same loop because those concerns are unchanged; we only swap out the VR-projection detection.

```
old_string:
	favTag := config.Application().FavoriteTag
	is180SBS := false
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		name := t.TagParts.Name
		// Detect VR projection BEFORE the ancestor skip so an ancestor-injected
		// VR / DOME / SBS tag still counts. Match any tag whose name contains
		// "VR" (case-insensitive) — Stash's VR scrapers add tags like "VR",
		// "vr_180", etc. — and the explicit DOME / SBS projection tags.
		if !is180SBS {
			upper := strings.ToUpper(name)
			if strings.Contains(upper, "VR") || upper == apiinternal.TagVR_DOME || upper == apiinternal.TagVR_SBS {
				is180SBS = true
			}
		}
		// Skip ancestor-injected tags from the chip list.
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		if favTag != "" && name == favTag {
			data.IsFavorite = true
			continue
		}
		data.Tags = append(data.Tags, name)
	}
	// Filename heuristic backstop for libraries that don't tag at all.
	if !is180SBS {
		for _, f := range vd.SceneParts.Files {
			if f == nil {
				continue
			}
			if vrFilenameRe.MatchString(f.Basename) || vrFilenameRe.MatchString(f.Path) {
				is180SBS = true
				break
			}
		}
	}
	// Mode dispatch: VR scenes get the immersive 180° SBS sphere; everything
	// else gets a flat plane in 3D space (a virtual cinema). The Enter VR
	// button always shows when there's a stream — user can watch any video
	// in headset.
	if is180SBS {
		data.VRMode = "180sbs"
	} else {
		data.VRMode = "flat"
	}

new_string:
	favTag := config.Application().FavoriteTag
	tagInputs := make([]apiinternal.TagInput, 0, len(vd.SceneParts.Tags))
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		name := t.TagParts.Name
		// Collect every tag (including ancestor-injected) for projection
		// detection — an ancestor DOME tag is just as authoritative as a
		// direct one.
		tagInputs = append(tagInputs, apiinternal.TagInput{Name: name, Aliases: t.Aliases})
		// Skip ancestor-injected tags from the chip list.
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		if favTag != "" && name == favTag {
			data.IsFavorite = true
			continue
		}
		data.Tags = append(data.Tags, name)
	}
	basename := ""
	if len(vd.SceneParts.Files) > 0 && vd.SceneParts.Files[0] != nil {
		basename = vd.SceneParts.Files[0].Basename
	}
	data.Projection = apiinternal.Detect(tagInputs, basename)
```

- [ ] **Step 3: Update template to read `.Projection` instead of `.VRMode`**

Edit `c:\dev\stash-vr\internal\static\browse_scene.gohtml`. Two distinct edits.

**Edit 3a:** Update the Enter VR button label. The current button label switches on `.VRMode == "180sbs"`. Replace it to switch on `.Projection.Geometry` being non-empty. After Tasks 3–5 add new render paths, this label still works generically.

```
old_string:
<button id="enterVR" class="btn-vr" type="button">{{if eq .VRMode "180sbs"}}▥ Enter VR (180° SBS){{else}}▥ Watch on virtual screen{{end}}</button>

new_string:
<button id="enterVR" class="btn-vr" type="button">{{if .Projection.Geometry}}▥ Enter VR{{else}}▥ Watch on virtual screen{{end}}</button>
```

**Edit 3b:** Update the geometry branch inside `<a-scene>`. The current code emits `vrSphere` when `.VRMode == "180sbs"` and `vrFlat` otherwise. Switch the condition to read from `.Projection`. Only the M2 path (equirectangular + 180) emits the half-sphere; every other Projection value falls through to the flat plane for now (Tasks 3–5 will add the missing branches).

```
old_string:
{{if eq .VRMode "180sbs"}}
  <a-entity id="vrSphere"
            geometry="primitive:sphere;radius:100;phiStart:180;phiLength:180;thetaLength:180;segmentsWidth:64;segmentsHeight:64"></a-entity>
{{else}}
  <a-entity id="vrFlat"
            geometry="primitive:plane;width:4;height:2.25"
            position="0 1.6 -3"></a-entity>
{{end}}

new_string:
{{if and (eq .Projection.Geometry "equirectangular") (eq .Projection.FOV 180)}}
  <a-entity id="vrSphere"
            geometry="primitive:sphere;radius:100;phiStart:180;phiLength:180;thetaLength:180;segmentsWidth:64;segmentsHeight:64"></a-entity>
{{else}}
  <a-entity id="vrFlat"
            geometry="primitive:plane;width:4;height:2.25"
            position="0 1.6 -3"></a-entity>
{{end}}
```

- [ ] **Step 4: Build verify**

PowerShell from `c:\dev\stash-vr`:
```
$env:Path += ";C:\Program Files\Go\bin"
go vet ./...
go build ./...
go test ./internal/api/internal/...
```

Expected: all three commands clean (no output / `PASS`). The `go test` re-runs Task 1's tests since `projection.go` is now imported by `data.go` and `scene.go`.

- [ ] **Step 5: Source-grep verify**

Use the Grep tool against `c:\dev\stash-vr\internal\api\browse\scene.go`:

- `vrFilenameRe` should match nowhere (regex deleted).
- `is180SBS` should match nowhere (variable removed).
- `apiinternal.Detect` should match exactly once (the new call).
- `data.VRMode` should match nowhere.
- `data.Projection` should match exactly once (the assignment).

And against `c:\dev\stash-vr\internal\api\browse\data.go`:
- `VRMode` should match nowhere.
- `Projection ` (with trailing space) should match exactly once.

And against `c:\dev\stash-vr\internal\static\browse_scene.gohtml`:
- `.VRMode` should match nowhere.
- `.Projection.Geometry` should match exactly twice (button label condition, geometry branch condition).

If any check fails, the edits in Steps 1–3 are incomplete — re-read the file and fix.

- [ ] **Step 6: Commit**

```
git add internal/api/browse/data.go internal/api/browse/scene.go internal/static/browse_scene.gohtml
git commit -m "browse: wire Projection detection into render pipeline

Replace SceneDetailData.VRMode (and the inline ad-hoc 'contains VR' /
vrFilenameRe regex detection in scene.go) with a single apiinternal.Detect
call producing a structured Projection (Geometry/FOV/Stereo). The
template branches on .Projection.Geometry/.Projection.FOV instead of the
previous string mode.

This commit preserves M2 behavior exactly: only the equirectangular+180
branch emits the half-sphere geometry; all other Projection values
(SPHERE 360, fisheye, etc.) fall through to the flat virtual-screen
fallback. Subsequent commits add the SPHERE 360 (Task 3), TB/mono UV
swap (Task 4), and fisheye shader (Task 5) render paths."
```

---

## Task 3: Add SPHERE 360° equirectangular full-sphere geometry

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

Adds a third template branch alongside the existing equirectangular-180 and flat fallback. Per-eye UV swap stays SBS-only at this task — Task 4 generalizes to TB/mono.

- [ ] **Step 1: Add a SPHERE 360° geometry branch to the template**

Edit `c:\dev\stash-vr\internal\static\browse_scene.gohtml`. Replace the geometry-branch block from Task 2 (now reads `.Projection.Geometry == "equirectangular" && .Projection.FOV == 180`) with one that adds an `else if` for FOV == 360.

```
old_string:
{{if and (eq .Projection.Geometry "equirectangular") (eq .Projection.FOV 180)}}
  <a-entity id="vrSphere"
            geometry="primitive:sphere;radius:100;phiStart:180;phiLength:180;thetaLength:180;segmentsWidth:64;segmentsHeight:64"></a-entity>
{{else}}
  <a-entity id="vrFlat"
            geometry="primitive:plane;width:4;height:2.25"
            position="0 1.6 -3"></a-entity>
{{end}}

new_string:
{{if and (eq .Projection.Geometry "equirectangular") (eq .Projection.FOV 180)}}
  <a-entity id="vrSphere"
            geometry="primitive:sphere;radius:100;phiStart:180;phiLength:180;thetaLength:180;segmentsWidth:64;segmentsHeight:64"></a-entity>
{{else if and (eq .Projection.Geometry "equirectangular") (eq .Projection.FOV 360)}}
  <a-entity id="vrSphere"
            geometry="primitive:sphere;radius:100;segmentsWidth:64;segmentsHeight:64"></a-entity>
{{else}}
  <a-entity id="vrFlat"
            geometry="primitive:plane;width:4;height:2.25"
            position="0 1.6 -3"></a-entity>
{{end}}
```

The 360° entity reuses `id="vrSphere"` because the existing `applySphere` function in the inline IIFE looks up that id. Same id → same texture-binding code → no JS changes needed in this task. The geometry difference (no `phiStart`/`phiLength`/`thetaLength` → defaults to full sphere) is the only user-visible change.

- [ ] **Step 2: Build verify**

PowerShell from `c:\dev\stash-vr`:
```
$env:Path += ";C:\Program Files\Go\bin"
go vet ./...
go build ./...
```

Expected: clean.

- [ ] **Step 3: Curl verify (manual, optional but recommended)**

If you have access to the running server with a known SPHERE-tagged scene id, curl and grep:

```
curl -s http://localhost:9666/browse/scene/<SPHERE-id> | findstr /C:"primitive:sphere;radius:100;segmentsWidth"
```

Expected: matches the new full-sphere geometry line. If you don't have a live server, skip this step — Task 6's on-headset validation will catch geometry-emit regressions.

- [ ] **Step 4: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: render SPHERE 360 equirectangular projections

Add an {{else if}} template branch for Projection.Geometry ==
'equirectangular' && Projection.FOV == 360. Emits a full sphere (no
phiStart/phiLength/thetaLength) reusing the same vrSphere id so the
existing applySphere() texture binding works unchanged. Stereo split
stays SBS-only at this commit; TB and mono are added in the next task."
```

---

## Task 4: Generalize per-eye UV swap to handle SBS / TB / mono

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

Replaces the hard-coded SBS UV swap with a Stereo-aware version. Pulls the SBS/TB/mono offsets from `<a-scene>` data attributes that the template emits based on `.Projection.Stereo`.

- [ ] **Step 1: Emit `data-stereo` attribute on `<a-scene>`**

Edit `c:\dev\stash-vr\internal\static\browse_scene.gohtml`. Update the `<a-scene>` opening tag to carry the stereo mode as a data attribute. Find the line:

```
old_string:
<a-scene id="vrScene" style="display:none" vr-mode-ui="enabled: true" loading-screen="enabled: false" background="color: #111">

new_string:
<a-scene id="vrScene" style="display:none" vr-mode-ui="enabled: true" loading-screen="enabled: false" background="color: #111" data-stereo="{{.Projection.Stereo}}">
```

The attribute value is `"sbs"`, `"tb"`, or `""` (empty for mono). The JS reads it via `scene.dataset.stereo`.

- [ ] **Step 2: Replace the per-eye UV swap inside `applySphere`'s `onBeforeRender`**

Edit the `mesh.onBeforeRender = function(...)` block inside `applySphere`. The current logic hard-codes SBS offsets. Replace with Stereo-aware offsets that read from `scene.dataset.stereo`.

```
old_string:
      mesh.onBeforeRender = function(renderer, sceneObj, cam) {
        const xr = renderer.xr;
        if (!xr || !xr.isPresenting) {
          tex.offset.set(0, 0);
          tex.repeat.set(1, 1);
          return;
        }
        const xrCam = xr.getCamera();
        if (!xrCam || !xrCam.cameras || xrCam.cameras.length < 2) return;
        if (cam === xrCam.cameras[0]) {
          tex.offset.set(0, 0);    // left eye: left half of SBS texture
          tex.repeat.set(0.5, 1);
        } else if (cam === xrCam.cameras[1]) {
          tex.offset.set(0.5, 0);  // right eye: right half
          tex.repeat.set(0.5, 1);
        }
      };

new_string:
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
```

The inner comment block at the top of the IIFE that describes the SBS strategy is now slightly stale ("ONE half-sphere with the full SBS-encoded texture") — leave it; Task 5 rewrites it as part of the fisheye-path additions.

- [ ] **Step 3: Build verify**

```
$env:Path += ";C:\Program Files\Go\bin"
go vet ./...
go build ./...
```

Expected: clean.

- [ ] **Step 4: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: handle TB stereo and mono in per-eye UV swap

The <a-scene> element now carries data-stereo='sbs'|'tb'|'' from the
Projection.Stereo field. The applySphere onBeforeRender hook reads it
and configures tex.offset/repeat per eye accordingly: SBS splits
horizontally, TB splits vertically, mono (empty) shows the full
texture to both eyes."
```

---

## Task 5: Add fisheye `ShaderMaterial` path for FISHEYE / MKX200 / RF52

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

Adds a fourth template branch and an `applyFisheye` JS function with an inline GLSL shader. The shader takes the per-fragment direction, converts it to fisheye `(u, v)` based on `uFOV`, then applies eye-specific UV offset/repeat from uniforms set per-eye in `onBeforeRender`. RF52 canting is punted — RF52 renders as plain 180° fisheye (the spec's accepted v1 trade-off).

- [ ] **Step 1: Add a fisheye geometry branch to the template**

Edit `c:\dev\stash-vr\internal\static\browse_scene.gohtml`. Update the geometry-branch block one more time. The new branch is `Projection.Geometry == "fisheye"`; the FOV is forwarded to JS via a `data-fov` attribute on the entity. Place the new branch before the flat fallback.

```
old_string:
{{if and (eq .Projection.Geometry "equirectangular") (eq .Projection.FOV 180)}}
  <a-entity id="vrSphere"
            geometry="primitive:sphere;radius:100;phiStart:180;phiLength:180;thetaLength:180;segmentsWidth:64;segmentsHeight:64"></a-entity>
{{else if and (eq .Projection.Geometry "equirectangular") (eq .Projection.FOV 360)}}
  <a-entity id="vrSphere"
            geometry="primitive:sphere;radius:100;segmentsWidth:64;segmentsHeight:64"></a-entity>
{{else}}
  <a-entity id="vrFlat"
            geometry="primitive:plane;width:4;height:2.25"
            position="0 1.6 -3"></a-entity>
{{end}}

new_string:
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

- [ ] **Step 2: Add `applyFisheye` JS function with the shader, register it in `applyAll`**

Edit the inline `<script>` IIFE. Two sub-edits.

**Edit 2a:** Add the `applyFisheye` function. Insert it after `applyFlat()` and before `applyAll()`. Find this region (after Task 4's edits, `applyFlat` looks like the original — Task 4 only modified `applySphere`):

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
    function applyAll() {
      applySphere();
      applyFlat();
    }
    scene.addEventListener('loaded', applyAll);
    ['vrSphere', 'vrFlat'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.addEventListener('object3dset', applyAll);
    });

new_string:
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
    // applyFisheye binds a custom ShaderMaterial that converts each
    // fragment's direction (from sphere center) to a fisheye (u, v)
    // sample, then applies SBS/TB/mono eye offsets via uniforms set per
    // eye in onBeforeRender. uFOV in degrees comes from the entity's
    // data-fov attribute (180 / 190 / 200). Beyond the fisheye coverage,
    // fragments are discarded so the back of the sphere shows the
    // <a-scene> background color.
    function applyFisheye() {
      const el = document.getElementById('vrFisheye');
      if (!el || !window.AFRAME || !AFRAME.THREE) return;
      const mesh = el.getObject3D('mesh');
      if (!mesh || mesh.userData.boundVR) return;
      const tex = new AFRAME.THREE.VideoTexture(video);
      if (AFRAME.THREE.SRGBColorSpace) tex.colorSpace = AFRAME.THREE.SRGBColorSpace;
      const fov = parseFloat(el.dataset.fov || '180');
      const material = new AFRAME.THREE.ShaderMaterial({
        side: AFRAME.THREE.BackSide,
        uniforms: {
          uMap:       { value: tex },
          uFOV:       { value: fov },
          uEyeOffset: { value: new AFRAME.THREE.Vector2(0, 0) },
          uEyeRepeat: { value: new AFRAME.THREE.Vector2(1, 1) }
        },
        vertexShader: [
          'varying vec3 vDir;',
          'void main() {',
          '  vDir = normalize(position);',
          '  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);',
          '}'
        ].join('\n'),
        fragmentShader: [
          'precision highp float;',
          'varying vec3 vDir;',
          'uniform sampler2D uMap;',
          'uniform float uFOV;',
          'uniform vec2 uEyeOffset;',
          'uniform vec2 uEyeRepeat;',
          'void main() {',
          '  vec3 d = normalize(vDir);',
          '  float theta = acos(-d.z);',                // angle from -Z (forward)
          '  float maxTheta = radians(uFOV * 0.5);',
          '  if (theta > maxTheta) discard;',
          '  float r = (theta / maxTheta) * 0.5;',      // [0, 0.5]
          '  float phi = atan(d.y, d.x);',
          '  vec2 uv = vec2(0.5 + r * cos(phi), 0.5 + r * sin(phi));',
          '  uv = uv * uEyeRepeat + uEyeOffset;',
          '  gl_FragColor = texture2D(uMap, uv);',
          '}'
        ].join('\n')
      });
      mesh.material = material;
      mesh.onBeforeRender = function(renderer, sceneObj, cam) {
        const xr = renderer.xr;
        const stereo = scene.dataset.stereo || '';
        const u = material.uniforms;
        if (!xr || !xr.isPresenting || !stereo) {
          u.uEyeOffset.value.set(0, 0);
          u.uEyeRepeat.value.set(1, 1);
          return;
        }
        const xrCam = xr.getCamera();
        if (!xrCam || !xrCam.cameras || xrCam.cameras.length < 2) return;
        const isLeft = cam === xrCam.cameras[0];
        const isRight = cam === xrCam.cameras[1];
        if (!isLeft && !isRight) return;
        if (stereo === 'sbs') {
          u.uEyeRepeat.value.set(0.5, 1);
          u.uEyeOffset.value.set(isLeft ? 0 : 0.5, 0);
        } else if (stereo === 'tb') {
          u.uEyeRepeat.value.set(1, 0.5);
          u.uEyeOffset.value.set(0, isLeft ? 0 : 0.5);
        } else {
          u.uEyeOffset.value.set(0, 0);
          u.uEyeRepeat.value.set(1, 1);
        }
      };
      mesh.userData.boundVR = true;
    }
    function applyAll() {
      applySphere();
      applyFlat();
      applyFisheye();
    }
    scene.addEventListener('loaded', applyAll);
    ['vrSphere', 'vrFlat', 'vrFisheye'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.addEventListener('object3dset', applyAll);
    });
```

- [ ] **Step 3: Update the IIFE comment to reflect the three-path architecture**

The comment block at the top of the IIFE (above `applySphere`) currently describes a one-sphere-with-SBS strategy. Replace with one that mentions all three paths.

```
old_string:
    // Bind material + texture programmatically. aframe-stereo-component@1.4.0
    // doesn't work on A-Frame 1.7 (reads material as raw string at init), so
    // stereo is handled here directly.
    //
    // Single-video architecture: sceneVideo (the on-page <video>) is the
    // sole media element. THREE.VideoTexture reads frames from it; the
    // element also produces audio. One pipeline = no drift between audio
    // and picture, and the texture is non-empty on Enter VR because
    // sceneVideo will already be playing (or about to play, started by the
    // Enter VR click which is a user gesture).
    //
    // Strategy for SBS: ONE half-sphere with the full SBS-encoded texture.
    // WebXR renders the scene twice per frame (once per eye), and Three.js
    // calls mesh.onBeforeRender per render call with the active camera. We
    // swap tex.offset/repeat per eye so the left eye samples the left half
    // and the right eye samples the right half of the SBS texture.
    //
    // The half-sphere uses phiStart:180, phiLength:180 so it natively faces
    // -Z (camera forward) with U increasing left-to-right and V increasing
    // top-to-bottom — texture orientation matches the user's view, no
    // rotation needed.

new_string:
    // Bind material + texture programmatically. aframe-stereo-component@1.4.0
    // doesn't work on A-Frame 1.7 (reads material as raw string at init), so
    // stereo is handled here directly.
    //
    // Single-video architecture: sceneVideo (the on-page <video>) is the
    // sole media element. THREE.VideoTexture reads frames from it; the
    // element also produces audio. One pipeline = no drift between audio
    // and picture, and the texture is non-empty on Enter VR because
    // sceneVideo will already be playing (or about to play, started by the
    // Enter VR click which is a user gesture).
    //
    // Three rendering paths, picked by the server based on tags + filename:
    //   1. Equirectangular sphere (DOME 180°, SPHERE 360°). Standard
    //      MeshBasicMaterial. Per-eye UV swap on tex.offset/repeat handles
    //      SBS/TB/mono via the scene's data-stereo attribute.
    //   2. Fisheye sphere (FISHEYE 180°, MKX200 200°, RF52). Custom
    //      ShaderMaterial: fragment shader maps direction → fisheye (u,v),
    //      eye offsets applied via uniforms set per-eye in onBeforeRender.
    //   3. Flat plane (no VR detected). Plain MeshBasicMaterial, no
    //      stereo, sits in front of the user as a virtual cinema screen.
    //
    // For paths 1 and 2 the half-sphere / full-sphere geometry uses
    // phiStart/phiLength values that face -Z (camera forward). WebXR
    // renders the scene twice per frame (once per eye); Three.js calls
    // mesh.onBeforeRender per render call with the active camera, which
    // is how we know which eye to configure UV/offset uniforms for.

```

- [ ] **Step 4: Build verify**

```
$env:Path += ";C:\Program Files\Go\bin"
go vet ./...
go build ./...
go test ./internal/api/internal/...
```

Expected: clean, all detection tests still passing.

- [ ] **Step 5: Source-grep verify**

Use the Grep tool against `c:\dev\stash-vr\internal\static\browse_scene.gohtml`:

- `applyFisheye` should match exactly twice (function definition, call inside `applyAll`).
- `id="vrFisheye"` should match exactly once (template entity).
- `applySphere` should match exactly twice (function definition, call inside `applyAll`).
- `applyFlat` should match exactly twice.
- `vrSphere` should appear in two template branches and in the JS `forEach` array.

If any check fails, the edits in Steps 1–3 are incomplete.

- [ ] **Step 6: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: add fisheye shader path for FISHEYE/MKX200/RF52

Adds a third render path: a sphere with a custom ShaderMaterial whose
fragment shader maps each fragment's direction to a fisheye (u, v)
sample, then applies eye-specific UV offsets via uniforms updated per
eye in onBeforeRender. FOV (180/190/200) comes from a data-fov
attribute on the entity. Fragments outside the fisheye coverage discard
so the <a-scene> background color shows behind. RF52 canting is punted
to a follow-up commit; RF52 renders as plain 180 fisheye for v1."
```

---

## Task 6: On-headset validation + result artifact

This task is manual — the implementer cannot do it from a subagent. The controlling agent should hand it off to the human user with the test plan below.

**Files:**
- Create: `docs/superpowers/research/2026-05-08-m3a-result/result.md`

- [ ] **Step 1: Restart the stash-vr binary**

PowerShell:
```
$env:Path += ";C:\Program Files\Go\bin"
go build -o stash-vr.exe ./cmd/stash-vr
.\stash-vr.exe   # or however the user normally launches it
```

- [ ] **Step 2: Walk through the 15-combo matrix**

Open Quest 3 Meta Browser. For each row in the table below, navigate to a representative scene from the user's library and click Enter VR. Record PASS/FAIL.

| # | Combo | What to look for |
|---|---|---|
| 1 | DOME + SBS | (Regression check.) Same as M2: 180° half-sphere, horizontal eye split, audio in sync. |
| 2 | DOME + TB | 180° half-sphere, vertical eye split. Top half → left eye, bottom half → right eye. |
| 3 | DOME mono | 180° half-sphere, both eyes see the same image (no parallax). |
| 4 | SPHERE + SBS | Full 360° sphere; user can turn around and see content in every direction. Horizontal eye split. |
| 5 | SPHERE + TB | Full 360° sphere with vertical eye split. |
| 6 | SPHERE mono | Full 360° sphere, both eyes see the same image. |
| 7 | FISHEYE + SBS | 180° fisheye coverage (slightly less than the equirectangular case visually); behind-the-user is `#111` background. Horizontal eye split. |
| 8 | FISHEYE + TB | 180° fisheye, vertical eye split. |
| 9 | FISHEYE mono | 180° fisheye, both eyes see the same image. |
| 10 | MKX200 + SBS | 200° fisheye (slightly more coverage than 180°), horizontal eye split. |
| 11 | MKX200 + TB | 200° fisheye, vertical eye split. |
| 12 | MKX200 mono | 200° fisheye, mono. |
| 13 | RF52 + SBS | 180° fisheye, horizontal eye split. (Stereo separation may be slightly off — that's the punted canting; note in the result.) |
| 14 | RF52 + TB | 180° fisheye, vertical eye split. |
| 15 | RF52 mono | 180° fisheye, mono. |

**Plus regressions:**

- A scene with no VR tags and no VR-suggesting filename → renders as flat virtual screen (M2 fallback preserved).
- Scene 5535 (SAVR-417) — re-test. If it now renders correctly, the V-shape was fixed by the new detection (likely via filename keyword). If it still V-shapes, its tags + filename don't identify the actual format; the fix path is M3b's manual override.
- Audio still in sync after 5+ minutes in any VR projection (M2 sync fix preserved).
- Exit VR → 2D player resumes at the position the user left VR at, not re-muted (M2 follow-up preserved).

- [ ] **Step 3: Write the result artifact**

Create `docs/superpowers/research/2026-05-08-m3a-result/result.md` modeled on `docs/superpowers/research/2026-05-08-m2-sync-flash-result/result.md`. One PASS/FAIL line per matrix row above plus the regression rows. Free-form "Surprises" section for fisheye edge artifacts, RF52 canting noticeability, etc. Recommendation: green-light M3b or block on a re-spec.

- [ ] **Step 4: Commit the result**

```
git add docs/superpowers/research/2026-05-08-m3a-result/result.md
git commit -m "browse: M3a multi-projection on-headset validation result"
```

---

## Self-review against spec

**Spec coverage check:**

- Spec §2 success criterion 1 (each of 15 combos renders with correct geometry/stereo) → Task 6 Step 2 (matrix walk).
- Spec §2 success criterion 2 (scene 5535 fixed when filename/tag identifies format) → Task 6 Step 2 (regression bullet).
- Spec §2 success criterion 3 (no-VR-tag scenes still render flat) → Task 6 Step 2 (regression bullet) + Task 2's "preserve M2 behavior" framing.
- Spec §2 success criterion 4 (M2 + bug-fix behavior preserved) → Task 6 Step 2 (regression bullets) + each task's build verify.
- Spec §3.1 tag pass with `util.StrSliceEquals` → Task 1 Step 3 (`projection.go::Detect`).
- Spec §3.1 most-specific Geometry wins → Task 1 Step 1 covers this in tests; Step 3 implementation has the explicit ordered switch.
- Spec §3.1 SBS wins over TB → Task 1 Step 1 test "DOME+SBS+TB prefers SBS"; Step 3 implementation has the ordered switch.
- Spec §3.2 filename keyword fallback table → Task 1 Step 3 `detectFromFilename`.
- Spec §3.3 conflict resolution (tag wins) → Task 1 Step 3 "filename pass runs only when tag pass produced no Geometry".
- Spec §4.1 Projection struct + Detect signature → Task 1 Step 3.
- Spec §4.1 caller wires basename from `Files[0].Basename` → Task 2 Step 2 Edit 2b.
- Spec §4.2 equirectangular 180° branch → Task 2 Step 3 Edit 3b (preserved M2).
- Spec §4.2 equirectangular 360° branch → Task 3 Step 1.
- Spec §4.2 fisheye branch → Task 5 Step 1.
- Spec §4.2 no-VR fallback → Task 2 Step 3 Edit 3b (the `{{else}}` flat plane survives across all later tasks).
- Spec §4.3 generalized SBS/TB/mono UV swap → Task 4 Step 2.
- Spec §4.4 fisheye fragment shader → Task 5 Step 2 (verbatim shader code).
- Spec §5 file table → matches plan File Structure section.
- Spec §6 build-level verification → each task has a `go vet` / `go build` step.
- Spec §6 curl-level verification → Task 3 Step 3, plus Task 2 Step 5 source-grep checks.
- Spec §6 Quest 3 validation → Task 6.
- Spec §7 risks (custom shader, fisheye edge, RF52 canting punted, detection over-confidence, scene 5535 may not improve, performance) → addressed implicitly: shader has discard for theta>maxTheta, RF52 canting is explicitly punted in the commit message, scene 5535 is checked as a regression bullet not a hard pass criterion.
- Spec §8 what stays untouched (single-video, audio defaults, in-VR control panel, M1/M2 surfaces) → none of the tasks touch those code paths; preserved by construction.

No spec gaps.

**Type / API consistency check:**

- `apiinternal.Projection` (in projection.go) is the same type referenced from `data.go` (`Projection apiinternal.Projection`) and from the template (`.Projection.Geometry`, `.Projection.FOV`, `.Projection.Stereo`).
- `apiinternal.TagInput{Name, Aliases}` is the same struct constructed in `scene.go` Step 2 Edit 2b and consumed by `Detect` in `projection.go`.
- `apiinternal.Detect` signature is `func(tags []TagInput, basename string) Projection` everywhere it's called or tested.
- Geometry strings: `"equirectangular"` and `"fisheye"` consistently. Empty `""` consistently means "no VR".
- Stereo strings: `"sbs"`, `"tb"`, `""` (mono) consistently.
- HTML id values: `vrSphere` (used by both equirectangular branches and `applySphere`), `vrFisheye` (used by fisheye branch and `applyFisheye`), `vrFlat` (flat fallback and `applyFlat`).
- A-Frame data attributes: `data-stereo` on `<a-scene>` (Task 4); `data-fov` on `<a-entity id="vrFisheye">` (Task 5).

**Placeholder scan:**

No "TBD", no "TODO", no "implement later", no "fill in details", no "similar to Task N", no "handle edge cases" without code. Every code step has a complete code block. Every command has the exact invocation.

The "RF52 canting punted" wording appears in three places (spec, plan task 5 commit message, validation matrix row 13 note). That's intentional and consistent — it's not a placeholder, it's a documented v1 trade-off.
