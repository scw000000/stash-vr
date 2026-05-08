# M3a design: multi-projection VR rendering on /browse/scene/{id}

**Date:** 2026-05-08
**Status:** Drafting (`/brainstorming` session 2026-05-08).
**Predecessor:** [M2 follow-up sync/flash fix spec](2026-05-08-m2-vr-sync-flash-fix-design.md) — shipped, validated; collapsed dual-video, audio defaults on, scene clear-color background.
**Successor:** **M3b** — in-VR SKYBOX-style projection picker + IPD slider + tag write-back to Stash on manual override. **M3c** — SKYBOX-style controller mappings.
**Reference players:** Behavior parity with [SKYBOX](https://skybox.xyz/support#Watch-Videos) (filename-keyword detection, format menu) and HereSphere (tag-based detection via Stash, projection per scene). Where this spec is ambiguous, default to whatever those players do.

---

## 1. Context (why this milestone)

M2 shipped a WebXR `/browse/scene/{id}` VR player that handles only `DOME+SBS` (180° equirectangular side-by-side). The M2 sync/flash follow-up validated cleanly, but on-headset testing surfaced a known M2 limitation: scenes that aren't 180° equirectangular SBS either fall through to the flat virtual-screen fallback (best case) or render with severe distortion if they're tagged DOME+SBS but actually encoded differently (worst case — e.g. scene 5535 / SAVR-417 shows a downward-pointing V-shape of black at the top of view, almost certainly because the source is a different projection but the renderer treats it as equirectangular).

The user's library contains all of: DOME 180° equirectangular (SBS + TB), SPHERE 360° equirectangular (SBS + TB), and the Japanese-VR fisheye family — FISHEYE 180°/190°, MKX200 (200° fisheye), and RF52 (canted 180° fisheye). M3a closes the projection-coverage gap so all of these render correctly, matching how SKYBOX and HereSphere handle the same content.

M3a is a server-side detection change + a client-side rendering change. No UI surfaces are added — the in-VR projection picker, IPD slider, and manual override are M3b's territory, layered on top of M3a's detection.

## 2. Goal & non-goals

**Goal:** `/browse/scene/{id}` correctly renders any of the 15 combinations of `{DOME, SPHERE, FISHEYE, MKX200, RF52} × {SBS, TB, mono}` in WebXR. Detection is tag-first (alias-aware, mirroring `/deovr`'s `set3DFormat`), with SKYBOX-style filename-keyword fallback when no projection tag is present. A scene with no VR detection in either pass continues to render as the flat virtual-screen fallback.

**Success criteria, manually verified on Quest 3 / Meta Browser:**

1. For each of the 15 combos in §3, a scene from the user's library that matches that combo renders with the correct geometry and stereo split. Visually compared against SKYBOX or HereSphere as the reference player.
2. Scene 5535 (currently V-shape broken when forced into DOME+SBS) renders correctly *if* its filename or its tags identify the actual format. If it's tagged wrong AND named neutrally, it stays broken until M3b's manual override ships — that's expected.
3. A scene with no VR tags and no filename hints continues to render as the flat virtual screen (no regression of the M2 fallback).
4. All M2 + sync/flash-fix behavior preserved: single-video architecture, audio defaults on (no `muted` attribute on the page video), exit-VR does not re-mute, scene clear-color `#111` background, in-VR control panel (play/pause, ±10s, exit) functions on `sceneVideo`.

**Non-goals (deferred):**

- **M3b:** in-VR projection picker, IPD/eye-distance slider, manual per-scene format override, tag write-back to Stash when the user overrides (uses the existing `library.Service` tag-mutation methods that HereSphere's parser already calls).
- **M3c:** SKYBOX-style controller mappings — single/double/long-press semantics, hold-and-move-screen, B/Y reset, thumbstick rewind/zoom, Oculus button recenter.
- **M4 or never:** CUBEMAP and EAC (YouTube) projections — not present in the user's library per the brainstorm scoping question. Heatmap-on-VR-scrub-bar, watch-resume, resolution selector, `/deovr` and `/heresphere` retirement decisions.
- **Within M3a:** RF52 canted-fisheye math. RF52 renders as plain 180° fisheye for v1, which produces a small but visible stereo error. Properly canting RF52 (each eye's fisheye is rotated outward by a fixed angle) is a follow-up if the v1 stereo is uncomfortable. Not blocking ship.

## 3. Detection — match SKYBOX + HereSphere

Two passes. Tags first; filename-keyword fallback only if the tag pass produced no Geometry. Within either pass, more-specific projections win. Stereo is detected independently of geometry in both passes.

### 3.1 Tag pass (HereSphere convention)

Walk the scene's tags, matching with `util.StrSliceEquals(t.Name, t.Aliases, X)` (alias-aware, identical to [internal/api/deovr/videodata.go::set3DFormat](../../../internal/api/deovr/videodata.go)). Constants from [internal/api/internal/legend.go](../../../internal/api/internal/legend.go):

| Tag matched | Sets |
|---|---|
| `MKX200` | Geometry=`fisheye`, FOV=200 |
| `RF52` | Geometry=`fisheye`, FOV=180 (canting punted) |
| `FISHEYE` | Geometry=`fisheye`, FOV=180 |
| `SPHERE` | Geometry=`equirectangular`, FOV=360 |
| `DOME` | Geometry=`equirectangular`, FOV=180 |
| `SBS` | Stereo=`sbs` |
| `TB` | Stereo=`tb` |

Resolution rules:

- Most-specific Geometry wins: `MKX200 > RF52 > FISHEYE > SPHERE > DOME`.
- If both `SBS` and `TB` tags match, `SBS` wins (more common; matches M2 default).
- If neither stereo tag matches, `Stereo=""` (mono).
- If no Geometry tag matches, fall through to §3.2.

### 3.2 Filename keyword fallback (SKYBOX convention)

Case-insensitive scan of the file basename. Pulled directly from [SKYBOX's documented keyword list](https://skybox.xyz/support#Watch-Videos). First match per category wins.

| Keyword in basename | Sets |
|---|---|
| `MKX200` | Geometry=`fisheye`, FOV=200 |
| `FISHEYE200`, `_200_FISHEYE`, `Fisheye_200°` | Geometry=`fisheye`, FOV=200 |
| `FISHEYE190`, `_190_FISHEYE` | Geometry=`fisheye`, FOV=190 |
| `FISHEYE180`, `_180_FISHEYE`, bare `FISHEYE` | Geometry=`fisheye`, FOV=180 |
| `RF52` | Geometry=`fisheye`, FOV=180 |
| `_360`, `VR360`, `LR_360`, `TB_360`, `2D_360` | Geometry=`equirectangular`, FOV=360 |
| `_180`, `VR180`, `LR_180`, `TB_180`, `2D_180` | Geometry=`equirectangular`, FOV=180 |
| `LR_` prefix or `_LR_` | Stereo=`sbs` |
| `TB_` prefix or `_TB_` | Stereo=`tb` |
| `2D_` prefix or `_2D_` | Stereo=`""` (force mono) |

### 3.3 Conflict resolution

- **Tag and filename agree** → use the agreed result.
- **Tag pass yields a Geometry** → tag pass wins, filename pass is skipped entirely. This is the HereSphere precedence: tags are the explicit source of truth; filenames are best-effort.
- **No detection in either pass** → `Projection{}` zero value → flat virtual screen (current M2 fallback, no regression).
- **Mistagged scenes** (e.g. file is MKX200 but the user tagged it `DOME+SBS`): tag pass picks DOME+SBS, V-shape persists. Path to fix is M3b's manual override + tag write-back, not heuristics on top of conflicting metadata.

## 4. Architecture

### 4.1 Server-side: detection + data shape

A new package-internal file — [internal/api/internal/projection.go](../../../internal/api/internal/projection.go) (to-be-created) — defines:

```go
type Projection struct {
    Geometry string  // "equirectangular" | "fisheye" | ""  ("" = no VR)
    FOV      int     // 180, 190, 200, 360 (or 0 if Geometry=="")
    Stereo   string  // "sbs" | "tb" | ""  ("" = mono)
}

// Detect resolves a Projection from scene tags and (optionally) the file basename.
// Tag pass uses StrSliceEquals(t.Name, t.Aliases, ...) for alias-aware matching.
// Filename-keyword fallback runs only if the tag pass found no Geometry.
func Detect(tags []*gql.SceneTagPart, basename string) Projection
```

Replace the M2 `SceneDetailData.IsVR180SBS bool` (and the M2 follow-up `VRMode string` introduced in commit `efaf2c6`) with `Projection Projection`. Update [internal/api/browse/scene.go](../../../internal/api/browse/scene.go) to call `internal.Detect(vd.SceneParts.Tags, basename)` after the existing tag loop, where `basename` comes from `vd.SceneParts.Files[0].Basename` (defensive: empty string if no files).

### 4.2 Client-side: template + rendering paths

[internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml) branches on `.Projection.Geometry`:

- **`equirectangular` + FOV=180** — `SphereGeometry` with `phiStart:180, phiLength:180, thetaLength:180`. The current half-sphere; unchanged from M2.
- **`equirectangular` + FOV=360** — `SphereGeometry` with default `phiLength:360`, `thetaLength:180` (full sphere). `BackSide` material so the user is inside the sphere.
- **`fisheye` + FOV ∈ {180, 190, 200}** — `SphereGeometry` (full) with a custom `ShaderMaterial`. The fragment shader takes the per-fragment direction and the `uFOV` uniform, computes fisheye `(u, v)`, then applies the eye-specific UV offset/repeat (from uniforms set per-eye in `onBeforeRender`).
- **No Geometry** (empty) — existing flat plane, current M2 fallback.

A single video element (`sceneVideo`) is the texture source for every path — preserves the M2 sync-fix architecture. `THREE.VideoTexture(sceneVideo)` is created once per active geometry, idempotent via `mesh.userData.boundVR`.

### 4.3 Per-eye UV swap, generalized

The existing `onBeforeRender` updates `tex.offset/repeat` based on `cam === xrCam.cameras[N]`. New version reads `Stereo`-aware constants emitted as JS literals in the template (or hard-coded if simpler):

- `sbs`: left = `offset(0, 0)`, repeat = `(0.5, 1)`. Right = `offset(0.5, 0)`, repeat = `(0.5, 1)`.
- `tb`: left = `offset(0, 0)`, repeat = `(1, 0.5)`. Right = `offset(0, 0.5)`, repeat = `(1, 0.5)`.
- `mono`: both eyes = `offset(0, 0)`, repeat = `(1, 1)`.

For the fisheye `ShaderMaterial` path, the same `eyeOffset` and `eyeRepeat` are passed as `uniform vec2`s; the fragment shader applies them after computing the fisheye `(u, v)` and before the `texture2D()` sample.

### 4.4 Fisheye fragment shader (sketch)

Inline in `browse_scene.gohtml`. Equidistant fisheye projection:

```glsl
varying vec3 vDir;  // direction from sphere center, set by vertex shader

uniform sampler2D uMap;
uniform float uFOV;       // 180.0, 190.0, or 200.0 (degrees)
uniform vec2  uEyeOffset; // e.g. (0, 0) or (0.5, 0)
uniform vec2  uEyeRepeat; // e.g. (0.5, 1) or (1, 0.5)

void main() {
    vec3 d = normalize(vDir);
    float theta = acos(-d.z);                  // angle from forward (-Z)
    float maxTheta = radians(uFOV * 0.5);
    if (theta > maxTheta) discard;             // outside fisheye coverage
    float r = theta / maxTheta * 0.5;          // [0, 0.5]
    float phi = atan(d.y, d.x);
    vec2 uv = vec2(0.5 + r * cos(phi), 0.5 + r * sin(phi));
    uv = uv * uEyeRepeat + uEyeOffset;
    gl_FragColor = texture2D(uMap, uv);
}
```

The `discard` for `theta > maxTheta` keeps the back of the sphere from sampling garbage. RF52 cant would add a per-eye rotation to `d` before computing `theta`/`phi`; deferred to a follow-up commit.

## 5. Files touched

| File | Change |
|---|---|
| `internal/api/internal/projection.go` | **New.** `Projection` type + `Detect(tags, basename) Projection`. Tag pass uses `util.StrSliceEquals` against `TagVR_*` constants. Filename pass uses the §3.2 keyword tables. |
| `internal/api/browse/data.go` | Replace `IsVR180SBS bool` and `VRMode string` with `Projection internal.Projection`. |
| `internal/api/browse/scene.go` | Replace the existing 2-tag check with one call to `internal.Detect(vd.SceneParts.Tags, basename)`. Pass `vd.SceneParts.Files[0].Basename` (defensive nil/empty checks). |
| `internal/static/browse_scene.gohtml` | Branch the `<a-scene>` body on `.Projection.Geometry` × `.Projection.FOV` to emit half-sphere / full-sphere / fisheye-shader entity. Generalize the per-eye UV swap to handle SBS / TB / mono via template-emitted JS constants. Add the fisheye `ShaderMaterial` (vertex + fragment shaders, inline). |

**No new vendored files. No new env vars. No genqlient changes.** A-Frame stays at the version vendored in M2.

## 6. Validation

Manual on Quest 3 / Meta Browser, per [CLAUDE.md](../../../CLAUDE.md). No test suite.

**Build-level:**
- `go vet ./...` clean.
- `go build ./...` clean.

**Curl-level (sanity):**
- `curl -s http://localhost:9666/browse/scene/<DOME-SBS-id>` HTML still contains the expected M2 markers (`id="enterVR"`, `id="sceneVideo"`, `<a-scene`).
- `curl -s http://localhost:9666/browse/scene/<DOME-SBS-id>` HTML's `<a-scene>` body matches the equirectangular-180 template branch (i.e. has `phiStart:180`).
- `curl -s http://localhost:9666/browse/scene/<SPHERE-SBS-id>` `<a-scene>` body has full-sphere geometry (no `phiStart`/`phiLength`).
- `curl -s http://localhost:9666/browse/scene/<FISHEYE-SBS-id>` `<a-scene>` body emits the fisheye `ShaderMaterial` and the `uFOV` uniform set to 180.

**Quest 3 (the actual UX validation):**
- One representative scene from the user's library per combo. Open it in Meta Browser. Click Enter VR.
- Verify visually: the projection looks like SKYBOX would render the same file. Stereo split is correct (close one eye at a time; the two eyes should see slightly different views consistent with the parallax direction).
- DOME+SBS regression check: scene that worked in M2 still works.
- Flat-virtual-screen regression check: a scene with no VR tags still renders 2D.
- Audio still in sync after 5 minutes (M2 fix preserved).

**Validation artifact:** `docs/superpowers/research/2026-05-08-m3a-result/result.md`, same template as the M2 results.

## 7. Risks

- **Custom fragment shader on WebXR.** Three.js `ShaderMaterial` with WebXR is well-documented but the per-eye uniform update path is new for this codebase. The existing per-eye `onBeforeRender` swap pattern works because A-Frame uses the same material across both eye renders; we extend that pattern to also update shader uniforms (`uEyeOffset`, `uEyeRepeat`). If the shader fails to compile or render correctly on Meta Browser's Chromium, fall back to flat virtual screen for fisheye combos and ship without them; renderer architecture is otherwise unaffected.
- **Fisheye edge artifacts.** At the edge of the fisheye circle, texture sampling can pull in pixels outside the projected content. The `discard` for `theta > maxTheta` in the fragment shader handles the geometric edge, but linear-filter bleed from the SBS half-boundary may still produce a thin seam at the vertical centerline. Mitigation if visible: switch the texture to nearest-neighbor filtering near the seam, or shrink `uEyeRepeat.x` by ~1% to crop the seam pixel.
- **RF52 canting punted.** Renders as plain 180° fisheye. Stereo separation will be slightly off (each eye's content is canted ~5° outward in the source). User may or may not perceive this as wrong; if reports suggest it's uncomfortable, add cant in a follow-up commit (one extra rotation matrix per eye in the shader).
- **Detection over-confidence.** Filename keywords are heuristics. A file named `something_LR_180_normal_test.mp4` could match both `LR_180` (equirectangular 180 SBS) and bare `_180_` (also equirectangular 180 — consistent). Ambiguity within the §3.2 table is constructed to be self-consistent; cross-table ambiguity (e.g. a file with `MKX200` and `_180_360` both in the name) goes to first-match-wins per the table order.
- **Tag and filename disagree.** §3.3 states tag wins. If users complain after a scene auto-detects wrong, M3b's manual override + tag write-back is the structural answer; don't complicate the heuristic.
- **Scene 5535 may not improve after M3a alone.** If 5535 is tagged `DOME+SBS` and its filename doesn't contain a recognizable keyword, it'll still render as broken DOME+SBS. The fix path is then M3b's manual override + tag write-back, not heuristic improvements here. We'll find out at validation time.
- **Performance on full-sphere geometry.** A 64×64-segment full sphere with WebGL is cheap. A custom fragment shader doing `acos`/`atan` per fragment is also cheap on Quest 3 (modern adreno). 8K source remains the bottleneck per M2 §11; nothing M3a does makes it worse.

## 8. What stays untouched

- **Single-video architecture** from the M2 sync/flash fix. `THREE.VideoTexture(sceneVideo)`, no `vrTex`, no dual-stream sync. Audio defaults on per the user's request. `<a-scene background="color:#111">` for compositor flash mitigation.
- **In-VR control panel** (play/pause, ±10s, exit). Operates on `sceneVideo`. Unchanged.
- **All M1/M2 surfaces** — scene grid, sidebar, search, pagination, scene-detail metadata, mutation forms (rating, favorite, tags, O-counter, organized).
- `/deovr`, `/heresphere`, all JSON endpoints. `library.Service`, GraphQL client.
- `internal/api/internal/legend.go` constants — read; not modified.
- HTTPS / Caddy / DuckDNS, build flow, Go-only no-bundler pattern.

## 9. After this milestone

If M3a validates:

- **M3b** — in-VR projection picker (mimics SKYBOX's format menu from the user's reference screenshot 2: Normal/FishEye/YouTube tabs, 2D/SBS/TB stereo toggles, Cinema/180°/360° geometry buttons, Reset/Auto). IPD / eye-distance slider. Tag write-back to Stash on manual override (uses existing `library.Service` mutations). The override is what gets exposed; M3a's `Projection` becomes the default that the menu can edit.
- **M3c** — SKYBOX-style controller mappings (single/double/long-press, thumbstick rewind/zoom, B/Y reset, Oculus button recenter). Independent of M3b.

If M3a surfaces something unexpected (fisheye shader doesn't compile on Meta Browser, custom material breaks the per-eye UV-swap pattern, etc.), pause and re-spec. Fall back to ship-just-the-equirectangular-pieces if needed (DOME-TB, SPHERE+SBS, SPHERE+TB) and defer fisheye family to a smaller follow-up spec.
