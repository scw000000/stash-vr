# M3b design: in-VR projection picker + tag write-back

**Date:** 2026-05-08
**Status:** Drafting (`/brainstorming` session 2026-05-08).
**Predecessor:** [M3a multi-projection rendering spec](2026-05-08-m3a-multi-projection-rendering.md) — shipped commits `b2e2ac5..70f6ede`, plus the M2-fallback fix `1407b52`. Auto-detected projection works for the 15-combo matrix; mistagged scenes (e.g. fisheye encoded as DOME+SBS) still render incorrectly.
**Successor:** **M3c** — SKYBOX-style controller mappings (single/double/long-press, thumbstick, recenter). **M3b-followup** — IPD / stereo-separation slider, if user requests after M3b ships.
**Reference player:** Behavior parity with [SKYBOX](https://skybox.xyz/support#Watch-Videos) picture 2 (the format menu — three rows of mutually-exclusive buttons: Type, Degree, Stereo). Where this spec is ambiguous, default to what SKYBOX does.

---

## 1. Context (why this milestone)

M3a auto-detects projection from tags + filename keywords and renders the correct geometry. Validation uncovered two real-world cases the auto-detection can't handle alone:

- **Mistagged scenes.** Scene 5535 (KMPVR / SAVR-417) is tagged `DOME + SBS` but the source is fisheye-projected — auto-detection picks DOME, the renderer projects equirectangular, and the user sees a V-shape pole artifact. Per spec §3.3 of M3a, the structural fix is manual override; M3a deferred this to M3b.
- **Generic-VR-only tags.** Scenes with just a `VR` tag and no specific projection tag regressed under M3a's stricter detection. Commit `1407b52` restored M2's "any VR-substring tag → DOME+SBS" fallback as a safety net, but the user has no way to *correct* a guess that's wrong.

M3b adds an in-VR projection picker so the user can override per-scene from inside the headset. Picking a format immediately re-binds the renderer (live) and writes the corresponding `VR_*` tags back to Stash, so the next page load auto-detects correctly.

The user also asked for projection tag names to use the `VR_` prefix convention (`VR_DOME`, `VR_MKX200`, `VR_SBS`, etc.) — see §3 below. The codebase migrates the existing legend constants to the prefixed form; bare-form tags are no longer recognized.

## 2. Goal & non-goals

**Goal:** From inside WebXR on Quest 3, the user can open an in-VR picker that mirrors SKYBOX's three-field format menu (Type, Degree, Stereo). Each tap on a preset button immediately re-binds the renderer to use the new projection AND writes the corresponding `VR_*` projection tags to Stash via `library.UpdateTags`. An "Auto" button removes all explicit projection tags so the next load falls back to filename or generic-VR detection.

**Success criteria, manually verified on Quest 3 / Meta Browser:**

1. Inside VR, the existing playback panel shows a new "Format" button.
2. Tapping Format toggles the picker panel above the playback bar.
3. Picker shows three rows mirroring SKYBOX picture 2:
   - **Type:** Normal, FishEye.
   - **Degree:** Cinema, 180°, 200°, 360°.
   - **Stereo:** 2D, SBS, TB.
   - Plus a single **Auto** button.
   The currently-active selection in each row is visually highlighted.
4. Tapping a preset button:
   - Live re-binds the renderer (no page reload). The change is visible immediately in VR.
   - POSTs the new selection to `/browse/scene/{id}/projection`. The handler updates the scene's tags in Stash by removing all seven `VR_*` projection tags and adding the ones matching the new selection.
5. Invalid combinations are unreachable: when **Type=FishEye** is selected, the **Cinema** and **360°** Degree buttons are greyed out and not tappable; when **Cinema** is selected, the Type and Stereo selections are forced/locked.
6. Tapping **Auto** removes all seven `VR_*` projection tags from the scene and triggers re-detection via the page's auto-detect path.
7. After tag write-back, reloading the scene auto-detects the new projection — no override needed on subsequent visits.
8. The picker can be closed by tapping the Format button again or the close (✕) glyph on the picker panel.
9. Existing M2 + M3a + sync/flash-fix behavior preserved: single-video architecture, audio defaults on, in-VR play/pause/±10s/exit, scene clear-color background, tag detection across `/browse` + `/deovr` + `/heresphere`.

**Non-goals (deferred):**

- IPD / stereo-separation slider. M3b-followup if user requests after picker ships.
- SKYBOX-style controller mappings — single/double/long-press, hold-and-move-screen, B/Y reset, thumbstick rewind/zoom, Oculus button recenter. **M3c.**
- CUBEMAP / EAC support. M4 if ever — not in user's library.
- Per-scene IPD memory.
- Undo / history of tag changes.
- Migrating existing user-library scenes from bare-form to `VR_`-prefixed tags. Bare-form tags are simply no longer recognized by M3b's stricter constants; manual cleanup is on the user.

## 3. Tag convention migration: bare → `VR_`-prefixed

The seven projection-tag constants in [internal/api/internal/legend.go](../../../internal/api/internal/legend.go) move from bare names to `VR_`-prefixed:

```go
TagVR_DOME    = "VR_DOME"     // was "DOME"
TagVR_SPHERE  = "VR_SPHERE"   // was "SPHERE"
TagVR_FISHEYE = "VR_FISHEYE"  // was "FISHEYE"
TagVR_MKX200  = "VR_MKX200"   // was "MKX200"
TagVR_RF52    = "VR_RF52"     // was "RF52"
TagVR_SBS     = "VR_SBS"      // was "SBS"
TagVR_TB      = "VR_TB"       // was "TB"
```

Three handlers reference these constants — `/browse` (M3a's `internal.Detect`), `/deovr/videodata.go::set3DFormat`, and `/heresphere/videodata.go::set3DFormat`. Changing the constants updates all three at once.

**No bare-form fallback.** The user confirmed their library does not rely on bare-form tags; the codebase's previous bare-form constants were a default that wasn't actively used. Generic-VR substring detection (the `1407b52` fallback that catches a plain `VR` tag and assumes DOME+SBS) stays — that's not a bare-form projection tag, it's a coarse "this scene is VR" signal.

The `CUBEMAP` and `EAC` constants are not migrated because they're unused (deferred to M4 if ever).

The two `TagInput` test cases in `projection_test.go` that assert the bare form (`tag DOME`, `tag SPHERE`, etc.) update to use the prefixed names. The case-insensitivity test (`case-insensitive name`) keeps testing case-insensitivity but on the new prefixed form.

## 4. UI / UX

### 4.1 Trigger

Add a sixth button to the existing in-VR playback panel (after Play/Pause, ±10s, Exit). Label: "Format". Tapping toggles `vrFormatPicker` visibility (CSS `display: none` ↔ `display: ''` via `el.setAttribute('visible', ...)` on the A-Frame entity).

### 4.2 Picker layout

A new `<a-entity id="vrFormatPicker">`, hidden by default, positioned above the playback panel. Three rows of mutually-exclusive `vr-btn` buttons + a single Auto button:

```
┌──────────────────────────────────────────────────────────────┐
│  Format                                                  ✕   │
├──────────────────────────────────────────────────────────────┤
│  Type:    [ Normal ]   [ FishEye ]                           │
│  Degree:  [ Cinema ]   [ 180° ]   [ 200° ]   [ 360° ]        │
│  Stereo:  [ 2D ]       [ SBS ]    [ TB ]                     │
├──────────────────────────────────────────────────────────────┤
│                          [ Auto (re-detect) ]                │
└──────────────────────────────────────────────────────────────┘
```

Currently-active button in each row has a lighter fill (e.g. `material="color:#3776c2"` vs the default `#2c5282`). The close (✕) glyph and the Format button on the playback panel both close the picker.

### 4.3 Three-field → Projection mapping

Each tap updates one of three JS state fields (`type`, `degree`, `stereo`). The combined state derives a `Projection`:

| Type | Degree | Stereo | → Projection {Geometry, FOV, Stereo} |
|---|---|---|---|
| any | Cinema | any (forced 2D) | `{ "", 0, "" }` (flat virtual screen) |
| Normal | 180° | 2D | `{ "equirectangular", 180, "" }` |
| Normal | 180° | SBS | `{ "equirectangular", 180, "sbs" }` |
| Normal | 180° | TB | `{ "equirectangular", 180, "tb" }` |
| Normal | 360° | 2D | `{ "equirectangular", 360, "" }` |
| Normal | 360° | SBS | `{ "equirectangular", 360, "sbs" }` |
| Normal | 360° | TB | `{ "equirectangular", 360, "tb" }` |
| FishEye | 180° | 2D | `{ "fisheye", 180, "" }` |
| FishEye | 180° | SBS | `{ "fisheye", 180, "sbs" }` |
| FishEye | 180° | TB | `{ "fisheye", 180, "tb" }` |
| FishEye | 200° | 2D | `{ "fisheye", 200, "" }` |
| FishEye | 200° | SBS | `{ "fisheye", 200, "sbs" }` |
| FishEye | 200° | TB | `{ "fisheye", 200, "tb" }` |

**Invalid combos** (Normal + 200°, FishEye + Cinema, FishEye + 360°): the responsible Degree button is rendered but shown disabled (greyed out, not raycast-tappable) when the current Type makes it invalid. Cinema selected: Type and Stereo state are visually muted and have no effect, but selections are remembered so deselecting Cinema restores the prior Type/Stereo.

RF52 is not a separate row option. Per M3a §2 non-goals, RF52 renders as plain 180° fisheye in v1; the user picks `FishEye + 180° + SBS` if they want what RF52 historically meant. The `VR_RF52` tag stays as a recognized form for detection purposes — the picker just doesn't write `VR_RF52`. (Migration nuance: if a scene is tagged `VR_RF52` and the user opens the picker, the active highlight shows `FishEye + 180° + SBS`.)

### 4.4 Apply behavior

Per the brainstorm decision: **apply on every tap, no separate Save step.** Tap a preset → renderer rebinds + tag write-back fires.

Implementation outline:
1. Tap handler updates JS state (`type` / `degree` / `stereo`).
2. Maps the new state to a `Projection`.
3. Updates `<a-scene>` data attributes (`data-stereo`) and the active geometry entity's `data-fov` so the next render call picks them up.
4. Re-runs `applyAll()` after first clearing `mesh.userData.boundVR` on each apply target — this rebinds the right material to the right entity (sphere-equirectangular vs sphere-fisheye-shader vs flat plane). The `<a-scene>` template body is server-rendered to include all three entities (`vrSphere`, `vrFisheye`, `vrFlat`); only the one matching the active Projection is targeted by `applyAll()` based on its id-existence guards. **Change from M3a:** the template currently emits exactly *one* of the three entities based on the server-side Projection. M3b-revised: the template emits *all three* entities (sphere, fisheye, flat) and uses CSS `visible` toggling controlled by JS to show only the active one. This makes runtime override possible without a re-render of the page.
5. Fires a POST to `/browse/scene/{id}/projection` with `{ "type": "...", "degree": "...", "stereo": "..." }` (or `{ "auto": true }` for Auto). Single-in-flight lock client-side — rapid taps drop intermediate ones; last-tap-wins.
6. Highlights the now-active button in the tapped row, dims the previously-active one.

### 4.5 Auto button

Tapping Auto:
1. JS clears all override state — restores the server-rendered initial Projection (read from `<a-scene>`'s initial `data-*` attributes plus the active entity's `data-fov`).
2. Calls `applyAll()` to rebind to that initial Projection.
3. POSTs `/browse/scene/{id}/projection` with `{ "auto": true }`. Server removes all seven `VR_*` projection tags from the scene.
4. After the POST returns, the auto-detected projection on the *next* page load may differ from what's currently visible — auto-detect runs against the now-empty projection-tag set, falling through to filename keywords or the generic-VR fallback. The current session's render stays at the just-rebound state until the user picks something else or reloads.

## 5. Server side

### 5.1 New endpoint

POST `/browse/scene/{id}/projection`. Body is JSON, two shapes:

- Apply: `{ "type": "Normal" | "FishEye", "degree": "Cinema" | "180" | "200" | "360", "stereo": "2D" | "SBS" | "TB" }`
- Auto: `{ "auto": true }`

Handler:

1. Look up the scene via `library.GetScene(ctx, id, false)`.
2. **Auto branch:** drop all seven `VR_*` projection tags from the scene's current direct (non-ancestor) tags. Call `library.UpdateTags(ctx, id, remainingTags)`.
3. **Apply branch:** validate the three-field input. Compute the target tag set:
   - Start from current direct tags filtered to drop ancestor-injected ones (existing guard) AND drop the seven `VR_*` projection tags.
   - Map the three fields to the projection tags to add: `Type` → at most one of `VR_DOME`/`VR_SPHERE`/`VR_FISHEYE`/`VR_MKX200` per the mapping table in §4.3; `Stereo` → at most one of `VR_SBS`/`VR_TB` (or none for `2D`).
   - Cinema means "no VR" — add no projection tags. (The picker still POSTs the change so any prior projection tags are removed.)
   - Append the new tag names to the filtered set, calling `library.FindOrCreateTag` if the tag didn't exist (existing pattern in `library.UpdateTags`).
4. Return 204 (no content) on success, JSON error body + 4xx/5xx on failure.

### 5.2 Where the handler lives

New file `internal/api/browse/scene_projection.go` with the `sceneProjectionHandler` function. Wired in `internal/api/browse/router.go` next to the existing tag-mutation routes.

### 5.3 Constants migration

[internal/api/internal/legend.go](../../../internal/api/internal/legend.go) constants migrate per §3. Three downstream call sites — `internal.Detect`, `deovr.set3DFormat`, `heresphere.set3DFormat` — pick up the new values automatically.

### 5.4 No new GraphQL operations

`library.UpdateTags` and `library.FindOrCreateTag` already exist (used by HereSphere's tag parser and `/browse`'s chip add/remove handlers). Reused.

## 6. Files touched

| File | Change |
|---|---|
| `internal/api/internal/legend.go` | Migrate seven `TagVR_*` constants from bare to `VR_`-prefixed names. |
| `internal/api/internal/projection_test.go` | Update tag-pass test cases to use prefixed names (e.g. `Name: "DOME"` → `Name: "VR_DOME"`). The case-insensitive test stays. The "alias matches DOME" test stays meaningful — alias just needs to be `vr_dome` instead of `dome`. |
| `internal/api/browse/router.go` | Register `r.Post("/scene/{id}/projection", h.sceneProjectionHandler)`. |
| `internal/api/browse/scene_projection.go` | **New.** `sceneProjectionHandler` per §5. |
| `internal/static/browse_scene.gohtml` | Add "Format" button to playback panel. Add `<a-entity id="vrFormatPicker">` with the three rows + Auto. Emit *all three* render entities (`vrSphere`, `vrFisheye`, `vrFlat`) — not just the one matching server-detected Projection — so runtime override can switch between them via `visible` toggling. Update inline JS for: picker open/close, preset tap handler that updates state + data attributes + reapplies + POSTs, single-in-flight lock, active-highlight per row, disable invalid Degree based on current Type, Auto handler. |

**No** changes to `/deovr` or `/heresphere` files beyond what they pick up automatically from the legend-constant migration. **No** new vendored libraries. **No** new env vars. **No** genqlient regeneration.

## 7. Validation

Manual on Quest 3 / Meta Browser, per [CLAUDE.md](../../../CLAUDE.md). No project-wide test suite; M3a's `internal.Detect` unit tests stay relevant after the legend constant migration (test cases just update to use prefixed names).

**Build-level (every commit):**

- `go vet ./...` clean.
- `go build ./...` clean.
- `go test ./internal/api/internal/...` passes.

**Curl-level (sanity, before headset test):**

- `curl -s http://localhost:9666/browse/scene/<id>` HTML now contains `id="vrFormatPicker"` and the three row labels.
- `curl -X POST http://localhost:9666/browse/scene/<id>/projection -H 'Content-Type: application/json' -d '{"type":"FishEye","degree":"200","stereo":"SBS"}'` returns 204.
- After that POST, `curl http://localhost:9666/browse/scene/<id>` HTML reflects the new Projection (e.g. `data-stereo="sbs"` on `<a-scene>`, `data-fov="200"` on `vrFisheye`).
- `curl -X POST http://localhost:9666/browse/scene/<id>/projection -d '{"auto":true}'` returns 204; subsequent GET shows the scene's tag set has no `VR_*` projection tags.

**Quest 3 (the actual UX validation):**

1. Open scene 5535 (or any scene with a known projection mismatch). Verify the M3a-rendered V-shape is present.
2. Click Enter VR. Click Format on the playback panel. Picker appears.
3. Pick `FishEye + 200° + SBS`. Verify the renderer rebinds live (no page reload) and the V-shape is gone.
4. Exit VR. Reload the page. Verify auto-detection now picks the new projection (the scene is tagged `VR_MKX200` + `VR_SBS`).
5. Enter VR again. Open Format menu. Verify the active highlight is on `FishEye / 200° / SBS`.
6. Tap Auto. Verify renderer reverts to detected (likely DOME+SBS via M2 fallback if no other tags) and tags `VR_*` are removed from Stash.
7. On a known DOME+SBS scene that worked correctly in M3a: open Format, verify the active highlight is `Normal / 180° / SBS`. Pick `Normal / 180° / TB`. Verify renderer rebinds live; verify `VR_TB` is added to Stash and `VR_SBS` is removed.
8. Cinema test: pick Cinema. Verify renderer drops to flat virtual screen and all `VR_*` projection tags are removed.
9. M2/M3a regression checks: audio still in sync; no first-frame black flash; in-VR play/pause/±10s/exit still work; non-VR scenes still render flat.

**Validation artifact:** `docs/superpowers/research/2026-05-08-m3b-result/result.md`, same template as M3a's deferred result template.

## 8. Risks

- **Template change to emit all three render entities** (rather than one based on server detection) is the single biggest behavior shift in this spec. M3a-era behavior: only the equirectangular-180 entity emits when DOME+SBS is detected, so unrelated entities don't exist in the DOM. M3b: all three entities are always emitted, and JS toggles `visible`. Any A-Frame component that scans for VR meshes by id (raycaster, occluders, etc.) sees three meshes instead of one. Reviewing the existing scene markup, only `vr-btn` raycaster uses an id-class selector (`.vr-btn`), so the three video meshes don't interfere. Worth a quick on-headset spot-check that no raycast or interaction regressions surface from the multi-entity state.
- **Multiple in-flight tag writes.** Rapid tapping could race: tap 1 → POST 1 (`VR_DOME` + `VR_SBS`) → tap 2 → POST 2 (`VR_MKX200` + `VR_SBS`). If POST 1 finishes after POST 2 (network reordering), the final tag state is wrong. **Mitigation:** client-side single-in-flight lock — a new POST waits for the previous to settle, OR aborts the previous (last-tap-wins). The simpler approach (queue and replace the queued one with the latest tap) is sufficient.
- **Stash unreachable mid-session.** Picker tap rebinds the renderer locally but the POST fails. The user sees the right rendering but tags stay stale. Acceptable — error is logged in the console (which the user can pull up via Quest's Meta Browser devtools); next reload reverts to whatever auto-detection picks.
- **Scene 5535 still renders V-shape after picker fix until the *user* taps the right preset.** This isn't a bug; it's the whole point of the override. Worth highlighting in the validation artifact: the picker doesn't auto-correct mistagged scenes; the user has to know what the file actually is.
- **Constants migration breaks libraries that DO use bare-form tags.** Per the user's confirmation, no such tags exist in their library. If another stash-vr user has bare-form tags, they regress to flat virtual screen on M3b. Acceptable — a single user is the scope; the spec documents this explicitly. Mitigation if it ever surfaces for someone else: add the bare-form back as an alternate constant.
- **Picker disabled-button state.** When `Type=FishEye` is selected, the Cinema and 360° buttons need visible disabled styling (e.g. `material="color:#444;opacity:0.4"`). Raycasters need to either skip these buttons or the click handler needs to no-op on disabled buttons. The implementation plan picks one.
- **`mesh.userData.boundVR` reset on every override** means `THREE.VideoTexture` is recreated each time. Cheap (no new media decode — same `<video>` element underneath), but worth confirming on-headset that no GC pressure or texture-leak warning surfaces over many overrides.

## 9. What stays untouched

- M2 + M3a sync/flash/audio behavior: single-video architecture, no `muted` attribute on the page video, no re-mute on `exit-vr`, `<a-scene background="color:#111">`, in-VR play/pause/±10s/exit on `sceneVideo`.
- `library.Service`, `library.UpdateTags`, `library.FindOrCreateTag` — reused, not modified.
- `internal/api/internal/projection.go::Detect` — logic unchanged; only the constants it consults change names.
- `/deovr/videodata.go::set3DFormat` and `/heresphere/videodata.go::set3DFormat` — pick up the new constant values automatically; no source edits.
- All M1 surfaces (scene grid, sidebar, search, pagination, mutation forms — rating, favorite, tags chip add/remove, O-counter, organized).
- HTTPS / Caddy / DuckDNS, build flow.
- A-Frame vendored at `internal/static/vendor/aframe.min.js`. No new vendored libraries.

## 10. After this milestone

If M3b ships green:

- **M3b-followup:** IPD / stereo-separation slider, if user requests it. Adds a vec2 uniform `uIPDShift` (or pair of scalars) to both `applySphere`'s and `applyFisheye`'s onBeforeRender, mapped to a horizontal slider in the picker panel. Per-session only initially; per-scene persistence is a separate question.
- **M3c:** SKYBOX-style controller mappings — single-click toggle Format menu, double-click play/pause, hold-and-move-screen, B/Y reset, thumbstick rewind/zoom, Oculus button recenter. Layered on top of M3b's UI; doesn't replace it.

If M3b surfaces something unexpected — multi-entity DOM regresses interaction, picker raycast misses, tag write-back races stay visible despite the in-flight lock — pause and re-spec.
