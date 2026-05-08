# M2 follow-up: collapse dual-video architecture to fix VR audio desync + black flash

**Date:** 2026-05-08
**Status:** Drafting (`/brainstorming` session 2026-05-08).
**Predecessor:** [M2 spec](2026-05-08-m2-webxr-vr-player.md) — shipped, validated; introduced the WebXR 180° SBS player.
**Successor:** SKYBOX-style in-VR UI overhaul (controller mappings, projection-format selector, IPD slider). Separate spec, written after this one ships.

---

## 1. Context (why this fix)

Two bugs reported on the M2 VR player after on-headset use:

1. **Audio drifts out of sync with video** during VR playback. Gets noticeable
   after a couple of minutes.
2. **Occasional black flash** on Enter VR — the videosphere renders empty for
   a fraction of a second before frames appear.

Both stem from the same architectural choice. The current
[browse_scene.gohtml](../../../internal/static/browse_scene.gohtml) uses two
`<video>` elements:

- `sceneVideo` — visible 2D player, audible, plays from page load.
- `vrTex` — hidden, muted, in `<a-assets>`. Drives the WebGL `VideoTexture`.

Both elements load the same proxied stream URL independently. On Enter VR,
`vrTex.currentTime` is set from `sceneVideo.currentTime` and `vrTex.play()` is
called. From that point on the two videos buffer, decode, and advance via
their own pipelines. They drift — that's the desync. And `vrTex` starts
decoding only on Enter VR, so for the first ~hundreds of ms its texture is
empty → that's the black flash.

The dual-video pattern is a fossil from earlier debugging:

- Commit `9931425` ("bind VR texture via `<a-assets>` to fix black-render")
  added `vrTex` to dodge a black-render issue when the original `sceneVideo`
  was the texture source.
- Commit `c3c1890` ("keep 2D `<video>` playing during VR for audio") kept
  `sceneVideo` running because `vrTex` is muted.

Both root causes those workarounds chased have since been fixed:

- **Cross-origin texture taint** — solved by the same-origin proxy in commit
  `fcd73dd` ([scene_stream.go](../../../internal/api/browse/scene_stream.go)).
- **`aframe-stereo-component` init-order TypeError** — bypassed in commit
  `1128ae1` by binding `THREE.VideoTexture` programmatically and dropping the
  stereo component entirely.

The dual-video workaround stayed; the bugs it now causes did not get cleaned up.

## 2. Goal & non-goals

**Goal:** the VR player uses a single `<video>` element. Audio matches video.
No first-frame black flash on Enter VR.

**Success criteria (binary, manually verified on Quest 3 / Meta Browser):**

1. On a DOME+SBS scene, enter VR mid-playback. Video continues from the
   current playback position. Audio is audible and matches the picture.
2. After 5+ minutes inside VR, audio is still tight against the picture (no
   perceptible drift). This is the original reported symptom.
3. On Enter VR, the videosphere is populated with the current frame
   immediately — no visible black flash, no empty half-sphere for any
   measurable duration.
4. Exit VR. The 2D player resumes at the position the user left VR at, with
   the muted/audible state consistent with how the page started (muted).
5. In-VR panel buttons (play/pause, ±10s, exit) behave as before, operating
   on the single video element.
6. M1 + M2 surfaces unaffected: rating, favorite, tags, O-counter,
   organized, sidebar, search, pagination, the Enter VR gating on
   DOME+SBS scenes.

**Non-goals (deferred):**

- SKYBOX-style controller mappings (A/X/trigger semantics, thumbstick
  rewind/fast-forward/zoom, B/Y reset, Oculus button recenter).
- Projection-format selector (Normal/FishEye/YouTube · 2D/SBS/TB ·
  Cinema/180°/360°).
- IPD / stereo-separation slider.
- Resolution selector / quality switching.
- Watch-resume, in-VR scrub bar with heatmap, in-VR metadata overlay.

All of those go in the follow-up SKYBOX-clone spec.

## 3. The fix

Collapse to a single `<video>` element.

**Template** ([internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml)):

- Delete the entire `<a-assets>` block, including the `vrTex` `<video>`.
- The existing `sceneVideo` `<video>` is unchanged in attributes (still
  `controls playsinline autoplay muted preload="metadata"`). It remains the
  single source of media for the page, both in 2D and as the VR texture
  source.

**Inline JS** (same file):

- `applySphere` / `applyFlat` build `THREE.VideoTexture(video)` from the
  `sceneVideo` element instead of `vrTex`. Everything else in those
  functions (the `BackSide` material, the per-eye `onBeforeRender` UV swap)
  is unchanged.
- The Enter-VR click handler unmutes the video before calling `enterVR()`:
  `video.muted = false`. The click is a user gesture, so autoplay policy
  permits the unmute.
- The `exit-vr` handler re-mutes: `video.muted = true`. This restores the
  page's initial state — if the user re-enters VR, or scrolls around the
  page, the 2D player isn't surprisingly audible.
- `hide2D` / `show2D` no longer touch `vrTex`. They keep `sceneVideo`
  visible (the existing `el !== video` guard); A-Frame's full-viewport
  `<a-scene>` covers it during VR, the browser keeps decoding it.
- The in-VR control panel's `vrAction` function operates on `video`
  instead of `vrTex` for play / pause / seek-back / seek-fwd. Exit is
  unchanged.

That's it. No Go changes. No new vendored files. No new env vars.

## 4. Why this fixes both bugs

**Audio desync.** With one `<video>` element, there is exactly one media
pipeline. `currentTime`, `paused`, decode position — all are properties of
the same DOM element. Nothing can drift relative to itself.

**First-frame black flash.** `sceneVideo` has been autoplaying (muted) since
page load, so by the time the user clicks Enter VR, frames are already
decoded. The first `THREE.VideoTexture.update()` after Enter VR uploads a
real frame, not an empty one.

## 5. Risks

- **Occluded-video throttling.** When `<a-scene>` covers the page, the
  `sceneVideo` element is visually occluded. Quest's Meta Browser is
  Chromium-based and shouldn't throttle decode for occluded elements, but
  this is the one behavior worth checking on-headset. Fallback if it does
  throttle: hoist `sceneVideo` inside `<a-assets>` (still one element,
  just declared as an A-Frame asset).
- **Residual black flash from A-Frame's enter-VR compositor.** A-Frame
  briefly clears the WebGL framebuffer when transitioning to immersive-vr.
  Single-video doesn't fix that — it only fixes the much-longer "vrTex
  hasn't decoded any frame yet" flash. If a one-frame clear-color flash
  remains, mitigation is `<a-scene background="color:#111">` to match the
  page chrome rather than default black.
- **`muted` toggle UX.** With current attributes, the 2D player
  autoplays silent. Click Enter VR → audio jumps from silent to current
  playback position. Slight surprise, but matches every other web video
  player's behavior. Acceptable for this milestone.
- **VideoTexture lifetime.** The texture is bound to `sceneVideo`. If
  future work swaps `sceneVideo.src` (e.g. for resolution switching), the
  texture needs to be recreated. Worth a one-line comment in the JS;
  out-of-scope to actually implement for this fix.

## 6. Files touched

| File | Change |
|---|---|
| `internal/static/browse_scene.gohtml` | Drop `<a-assets>` + `vrTex`. Switch `applySphere`/`applyFlat`/`vrAction` to reference `video` (the existing `sceneVideo`). Add `video.muted = false` on Enter VR click and `video.muted = true` on `exit-vr`. Remove `vrTex.currentTime` sync and `vrTex.play()` calls. ~25 lines deleted, ~5 lines changed. |

**No Go changes. No new files. No removed files.**

## 7. What stays untouched

- `internal/api/browse/scene_stream.go` — the proxy stays. It's the reason
  same-origin texture upload works at all.
- `internal/api/browse/scene.go`, `data.go` — `IsVR180SBS` detection,
  `DirectStreamURL` building, all M1/M2 server logic.
- The vendored `aframe.min.js`. (We removed `aframe-stereo-component.min.js`
  from the runtime path in commit `1128ae1` already.)
- Mutation handlers, sidebar, search, pagination, rating/favorite/tags/o/organized.
- `/deovr`, `/heresphere`, all JSON endpoints.

## 8. Validation plan

Manual on Quest 3 / Meta Browser, per [CLAUDE.md](../../../CLAUDE.md). No
test suite exists.

**Build-level:**

- `go vet ./...` clean.
- `go build ./...` clean.

**Curl-level (sanity):**

- `curl -s http://localhost:9666/browse/scene/<DOME-SBS-id>` HTML still
  contains `id="enterVR"`, `<a-scene`, `id="sceneVideo"`.
- HTML no longer contains `<a-assets>` or `id="vrTex"`.

**Quest 3 / Meta Browser (the actual fix verification):**

- Open a DOME+SBS scene. Click Enter VR.
- VR enters at the user's current 2D playback position. Audio plays from
  that position, in sync with the picture.
- No black flash on entry — the videosphere is textured immediately.
- Stay in VR 5+ minutes. Audio still in sync with picture.
- Use the in-VR panel: play/pause toggles correctly, ±10s seeks correctly,
  exit returns to 2D. After exit, 2D player resumes from VR's last position
  and is muted again.
- Re-enter VR. Sync still tight, no flash.

**Validation artifact:** `docs/superpowers/research/2026-05-08-m2-sync-flash-result/result.md`,
same template as M1's `2026-05-08-m1-browse-result/`.

## 9. After this milestone

If this ships green, the SKYBOX-style UI overhaul becomes the next spec:
projection-format selector (Normal/FishEye/YouTube tabs, 2D/SBS/TB layout
toggles, Cinema/180°/360° projections, Reset/Auto), IPD/stereo-separation
slider, controller-button mappings (single-click panel toggle, double-click
play/pause, hold-and-move screen, B/Y reset, thumbstick rewind/zoom, Oculus
recenter). That spec is bigger and benefits from the simpler architecture
this fix leaves behind.

If this surfaces something unexpected — occluded-video throttling on Meta
Browser, an A-Frame compositor flash that doesn't yield to the
`background` workaround — we re-spec before stacking the UI work on top.
