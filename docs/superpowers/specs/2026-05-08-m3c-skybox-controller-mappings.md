# M3c design: SKYBOX-style controller mappings

**Date:** 2026-05-08
**Status:** Drafting (`/brainstorming` session 2026-05-08).
**Predecessor:** [M3b in-VR projection picker spec](2026-05-08-m3b-in-vr-projection-picker.md). M3b adds the Format picker and `/browse/scene/{id}/projection` endpoint; the playback panel is always-visible there. M3c flips the panel to hidden-by-default and lets the user drive it from the controllers.
**Successors:** **M3c-followup** — first-entry tutorial overlay, only if needed. **M3b-followup** — IPD slider in an Advanced Settings panel.
**Reference player:** Behavior parity with [SKYBOX](https://skybox.xyz/support#Watch-Videos), per the consolidated reference at [docs/superpowers/research/2026-05-08-skybox-ui-reference/reference.md](../research/2026-05-08-skybox-ui-reference/reference.md). Where SKYBOX is silent (specifically: drag and zoom in immersive mode), this spec extends the cinema mechanic in the most coherent way and notes the extension explicitly.

---

## 1. Context (why this milestone)

M3a/M3b shipped the rendering and Format-picker UI for VR scenes. The playback panel is currently a 2D HUD that's always visible — the user clicks buttons via raycast but has no controller-button or thumbstick shortcuts. SKYBOX's UX is the opposite: panel is hidden by default for full immersion, and a small set of controller shortcuts drive playback, panel toggling, screen positioning, and recenter. The user has been consistent across milestones that stash-vr's in-VR UX should mirror SKYBOX where practical.

M3c adds those controller shortcuts, hides the playback panel by default, ties laser-controller visibility to the panel state, and adds a help cheatsheet so the bindings are discoverable.

## 2. Goal & non-goals

**Goal:** From inside WebXR on Quest 3, the user can drive playback, panel toggling, screen drag/scale, and recenter from the controllers without ever pointing at the panel — though the panel is still reachable on demand and all M3b raycast clicks still work. Panel is hidden by default; trigger summons it; the laser controllers are only visible when the panel is.

**Success criteria, manually verified on Quest 3 / Meta Browser:**

1. Enter VR → playback panel and both laser controllers are hidden. Only the headset's own controller models are visible.
2. Single-click trigger (or A or X) in empty space → panel + lasers appear. Single-click again in empty space → both hide.
3. Single-click trigger on a `.vr-btn` raycast hit → button fires (existing M3b behavior preserved).
4. Double-click trigger (within 300 ms, in empty space) → video plays/pauses.
5. Trigger held + controller moved (>5 cm pose delta OR >250 ms hold) → drag of the active geometry (cinema plane / sphere / fisheye-quad) by controller pose delta. Translation only.
6. Thumbstick X-axis past 0.7 → ±10 s seek fires once. Re-arms when stick falls below 0.3.
7. Thumbstick Y-axis magnitude > 0.3 (held) → continuous scale of the active geometry, clamped 0.3×–5×.
8. B or Y short-press (released before 500 ms) → cinema: reset screen pose+scale; immersive: recenter yaw (re-orient projection so user faces forward).
9. B or Y long-press (≥500 ms) → full recenter both modes: yaw recenter + geometry pose+scale reset.
10. Help "?" button on the playback panel → opens a cheatsheet sub-panel listing the bindings from §3.
11. Format picker (M3b) still works inside the panel; tag write-back unaffected.
12. M2/M3a/M3b regressions absent: audio sync, no first-frame flash, projection auto-detect, three render entities, in-panel buttons.

**Non-goals (deferred):**

- First-entry tutorial overlay. M3c-followup if user reports forgetting bindings.
- Continuous-scrub thumbstick L/R. v1 is discrete ±10 s on threshold-cross.
- IPD slider / Advanced Settings panel. M3b-followup.
- Per-scene memory of dragged/scaled geometry pose.
- Intercepting Quest's Meta button (impossible from WebXR — see §8).
- Voice / hand-tracking input.
- Idle auto-hide of the panel. Panel only toggles via explicit user action.

## 3. Binding table

| Input | Cinema mode | Immersive (180°/360°/fisheye) |
|---|---|---|
| Trigger — single-click (raycast hit on `.vr-btn`) | Button click (M3b behavior) | Button click (M3b behavior) |
| Trigger — single-click (no raycast hit) | Toggle panel + lasers | Toggle panel + lasers |
| Trigger — double-click (≤300 ms apart, no raycast hit) | Play/pause | Play/pause |
| Trigger — hold + move controller | Drag cinema plane, translation only, plane keeps facing user | Drag sphere or fisheye-quad, translation only *(SKYBOX-extension where SKYBOX is undocumented)* |
| A or X — single-click | Toggle panel + lasers | Toggle panel + lasers |
| A or X — double-click (≤300 ms apart) | Play/pause | Play/pause |
| Thumbstick X-axis past ±0.7 (threshold-cross) | −10 s / +10 s | −10 s / +10 s |
| Thumbstick Y-axis magnitude > 0.3 (continuous, while held) | Scale screen, clamp 0.3×–5× | Scale sphere/fisheye-quad, clamp 0.3×–5× *(SKYBOX-extension)* |
| B / Y — short-press (<500 ms) | Reset screen pose+scale | Recenter yaw |
| B / Y — long-press (≥500 ms) | Full recenter (yaw + geometry reset) | Full recenter (yaw + geometry reset) |
| Help "?" button on panel | Open cheatsheet sub-panel | Open cheatsheet sub-panel |

Both hands' triggers/A/X/B/Y are equivalent — left and right behave identically, matching SKYBOX. Either thumbstick handles seek/scale.

A and X do not carry a raycaster, so they have no "raycast hit" branch — they always go straight to panel toggle / play-pause. Only the trigger has the raycast-hit branch (because only the trigger drives `.vr-btn` clicks per M3b's existing wiring). Hold+drag is trigger-only as well, because the drag mechanism captures controller pose during the trigger hold.

## 4. Behavior details

### 4.1 Trigger state machine

On `triggerdown`:
1. Capture controller pose `P_c0`, capture wall-clock time `t0`, capture whether raycaster has an intersection.
2. Start a candidate.

While candidate is active, per `tick`:
- If controller pose delta from `P_c0` exceeds **5 cm** OR `t_now - t0` exceeds **250 ms** → candidate becomes a **drag**. Emit `m3c:drag-start` with the active geometry id (resolved via §4.4). Continue emitting `m3c:drag-move` per tick with `P_c_now - P_c0` delta. End on `triggerup` with `m3c:drag-end`.

On `triggerup` before either threshold:
- If the original `triggerdown` had a raycaster intersection: a-Frame's existing click pipeline already fires the button; do nothing extra.
- Else: defer for **300 ms** as a candidate-click. If a second `triggerdown` arrives within 300 ms → it's a **double-click**: emit `m3c:play-pause`, cancel the candidate-click. Otherwise → emit `m3c:panel-toggle`.

### 4.2 B/Y short vs long press

On `bbuttondown` or `ybuttondown`: capture `t0`, start a 500 ms timer.
- If button is released before 500 ms → emit `m3c:reset-short`.
- If 500 ms elapses with button still held → emit `m3c:reset-long` (do not also fire `reset-short` on release).

A and X buttons mirror the trigger's click family only — they emit `m3c:panel-toggle` (single) or `m3c:play-pause` (double) via the same candidate-click pipeline as the trigger, but skipping the raycast-hit and drag branches (A/X have neither). This matches SKYBOX's "A/X/Trigger" grouping for clicks and the separate "B/Y" mapping for screen reset.

### 4.3 Thumbstick polling

A-Frame's `thumbstickmoved` event fires only on change and is too sparse for continuous scaling. The `m3c-controls` component polls `gamepad.axes` from both `tracked-controls` components every `tick`:

- **X-axis seek (discrete):** Track an "armed" flag per controller. While `|axis_x| < 0.3` → armed. When `axis_x > 0.7` while armed → emit `m3c:seek` with sign `+1`, disarm. When `axis_x < -0.7` while armed → emit `m3c:seek` with sign `-1`, disarm. Re-arm when `|axis_x|` falls below 0.3.
- **Y-axis scale (continuous):** When `|axis_y| > 0.3` → emit `m3c:scale` with delta `1 + 0.6 · axis_y · dt` per tick (where `dt` is seconds since last tick). The handler multiplies the active geometry's scale by this value, then clamps to [0.3, 5]. Direction convention: stick up = scale up (geometry larger).

Both controllers' axes are independent — pushing left thumbstick L and right thumbstick R simultaneously will fire two seek events (one in each direction). This is acceptable; in practice the user uses one stick at a time.

### 4.4 Active-geometry resolver

`m3c:scale` and `m3c:drag-*` need to know which entity is currently rendering. M3b's three entities `vrFlat`, `vrSphere180/360`, `vrFisheye` are all in the DOM but only one is `visible="true"` at a time. The resolver:

1. Read `<a-scene>`'s `data-stereo` and active-entity's `data-fov`.
2. Pick the entity whose `visible="true"`. Single helper, called per event.

Drag applies controller-delta translation to that entity's `position`. Scale multiplies that entity's `scale`. Cinema plane (`vrFlat`) additionally re-aims at the user via `lookAt(camera.position)` after each translate so it doesn't end up facing the wrong way.

### 4.5 Recenter

WebXR recenter via offset reference space:

```js
function recenter() {
  const xr = renderer.xr;
  if (!xr.isPresenting) return;
  const session = xr.getSession();
  const baseSpace = xr.getReferenceSpace();
  const cam = xr.getCamera();
  // Capture yaw only; ignore pitch/roll (the headset reports those correctly).
  const yaw = computeYaw(cam.matrixWorld);
  const offset = new XRRigidTransform(
    { x: cam.position.x, y: 0, z: cam.position.z },
    quaternionFromYaw(-yaw)
  );
  xr.setReferenceSpace(baseSpace.getOffsetReferenceSpace(offset));
}
```

`reset-short` in immersive calls `recenter()` only. `reset-short` in cinema calls a separate `resetGeometry(activeId)` that snaps the geometry's `position` and `scale` back to the values captured at Enter VR. `reset-long` calls both (in either mode).

### 4.6 Panel + laser visibility

The playback panel, the Format picker, and the help cheatsheet sub-panel live under a wrapper `<a-entity id="vrControlsRoot">`. Initial state on Enter VR: `visible="false"`. Both `<a-entity laser-controls>` entities also start at `visible="false"` and `raycaster="enabled: false"`.

`m3c:panel-toggle` flips:
- `vrControlsRoot.setAttribute('visible', !current)`.
- Both laser-controls' `visible` and their raycaster's `enabled`, mirroring panel state.

The Format picker's open-state is independent of the panel root: when the panel re-shows after being hidden, the picker stays in its last-set open/closed state. (Same for the help cheatsheet.)

### 4.7 Help "?" button

A new button on the playback panel, next to the Format button. Tapping toggles `<a-entity id="vrHelpPanel">` visibility. The help panel renders the §3 binding table as a-text rows on a backing plane. Layout is similar to the Format picker — same toggle-and-close pattern.

## 5. Implementation outline

### 5.1 New A-Frame component

`internal/static/m3c-controls.js`. Registered as `AFRAME.registerComponent('m3c-controls', { ... })`. Attached to a dedicated `<a-entity m3c-controls>` in the scene template (anywhere — the component is stateless w.r.t. its host entity's pose).

Component listens via `this.el.sceneEl.addEventListener` to:
- `triggerdown`, `triggerup` (both controllers).
- `abuttondown`, `xbuttondown` (mirror trigger click family).
- `bbuttondown`, `bbuttonup`, `ybuttondown`, `ybuttonup`.
- Per `tick`, reads `gamepad.axes` from `tracked-controls` components on both hands.

Component emits the following on `this.el.sceneEl`:

| Event | Detail |
|---|---|
| `m3c:panel-toggle` | none |
| `m3c:play-pause` | none |
| `m3c:seek` | `{ sign: -1 \| +1, seconds: 10 }` |
| `m3c:scale` | `{ factor: number }` (e.g. `1.012`) |
| `m3c:drag-start` | `{ activeId: string }` |
| `m3c:drag-move` | `{ activeId: string, dx: number, dy: number, dz: number }` |
| `m3c:drag-end` | `{ activeId: string }` |
| `m3c:reset-short` | `{ mode: 'cinema' \| 'immersive' }` |
| `m3c:reset-long` | `{ mode: 'cinema' \| 'immersive' }` |

The component does not touch DOM. Wiring belongs to the existing inline IIFE.

### 5.2 IIFE event handlers

The existing IIFE in `browse_scene.gohtml` adds listeners for the `m3c:*` events. Each handler is small:

```js
sceneEl.addEventListener('m3c:panel-toggle', () => togglePanelAndLasers());
sceneEl.addEventListener('m3c:play-pause', () => video.paused ? video.play() : video.pause());
sceneEl.addEventListener('m3c:seek', (e) => { video.currentTime += e.detail.sign * e.detail.seconds; });
sceneEl.addEventListener('m3c:scale', (e) => {
  const el = activeGeometry();
  const s = el.object3D.scale.x * e.detail.factor;
  const c = clamp(s, 0.3, 5.0);
  el.object3D.scale.setScalar(c);
});
// ... etc
```

`togglePanelAndLasers()` is the single function flipping `vrControlsRoot.visible` and both laser-controls' `visible` + `raycaster.enabled` together.

`activeGeometry()` resolves to one of `vrFlat` / `vrSphere180` / `vrSphere360` / `vrFisheye` based on which has `visible="true"`. Drag and reset helpers also use this.

`recenter()` and `resetGeometry()` per §4.5.

Initial geometry state (the position+scale at Enter VR) is captured into an in-memory snapshot when Enter VR is first triggered. `resetGeometry()` reads from that snapshot.

### 5.3 Static-file serving

`internal/api/router.go` already mounts `http.FileServerFS(static.Fs)` as the catch-all `Get("/*", ...)`, so any file present in the embed.FS is auto-served at its path. **However**, [internal/static/static.go](../../../internal/static/static.go) currently embeds only `*.gohtml *.html *.png vendor/*.js` — root-level `*.js` is NOT in the glob. M3c extends the directive to include `*.js` so `internal/static/m3c-controls.js` is served at `/m3c-controls.js`. One-character edit:

```go
//go:embed *.gohtml *.html *.png *.js vendor/*.js
```

### 5.4 No server changes

No new endpoints. No GraphQL ops. No env vars. No new vendored libraries.

## 6. Files touched

| File | Change |
|---|---|
| `internal/static/browse_scene.gohtml` | Wrap playback panel + Format picker + new help panel under `<a-entity id="vrControlsRoot" visible="false">`. Make both `<a-entity laser-controls>` start with `visible="false"` and `raycaster="enabled: false"`. Add `<a-entity m3c-controls>`. Add Help "?" button to playback panel. Add `<a-entity id="vrHelpPanel">` containing the cheatsheet (§3 table rendered as a-text rows on a backing plane). Extend inline JS with the `m3c:*` event handlers, `togglePanelAndLasers()`, `activeGeometry()`, `recenter()`, `resetGeometry()`, and the Enter-VR snapshot capture. |
| `internal/static/m3c-controls.js` | **New.** A-Frame component implementing trigger state machine, B/Y long-press timer, gamepad polling for X-axis discrete seek and Y-axis continuous scale, drag mechanic (controller pose delta capture and per-tick emission). Pure event-emitter; no DOM mutation. ~150–200 lines. |
| `internal/static/static.go` | Extend `//go:embed` directive to include root-level `*.js` so `m3c-controls.js` is embedded and auto-served by `internal/api/router.go`'s `http.FileServerFS(static.Fs)` catch-all. One-character change. |

**No** changes to `/deovr` or `/heresphere`. **No** changes to `library.Service`, `library.UpdateTags`, or M3b's `/browse/scene/{id}/projection` endpoint. **No** new vendored libraries. **No** env vars.

## 7. Validation

Manual on Quest 3 / Meta Browser, per [CLAUDE.md](../../../CLAUDE.md). The project has no test suite and controller behavior isn't unit-testable.

**Build-level (every commit):**

- `go vet ./...` clean.
- `go build ./...` clean (use `scripts\build-windows.bat` for local Windows builds, per memory).
- `curl http://localhost:9666/m3c-controls.js` returns 200 with non-empty body.
- `curl http://localhost:9666/browse/scene/<id>` HTML contains `<a-entity m3c-controls>` and `id="vrControlsRoot"` with `visible="false"`.

**Quest 3 — cinema scene first (a 2D-flat scene):**

1. Enter VR. Verify playback panel and both lasers are hidden; only controller models visible.
2. Single-click right trigger in empty space → panel + lasers appear.
3. Single-click empty space again → both hide.
4. Repeat with A button, X button, left trigger — all four behave identically.
5. Double-click trigger → video toggles play/pause.
6. Single-click trigger while panel is up, on a panel button (e.g. ±10 s) → button fires, panel stays up. (Existing M3b behavior preserved.)
7. Hold trigger and move controller → cinema plane translates with the controller; release drops in place.
8. Push thumbstick R past 0.7 → +10 s seek fires once. Release stick to ~0 and push R again → another +10 s. Holding past 0.7 does NOT keep firing.
9. Push thumbstick L past −0.7 → −10 s seek.
10. Push thumbstick up and hold → screen scales up smoothly. Release at the desired size.
11. Push down → scales down. Verify clamp at 0.3× (can't go smaller).
12. B button short-press → screen pose+scale reset to Enter-VR defaults.
13. Y button → same.
14. B button held ≥500 ms → headset yaw recenters AND screen resets. Release without firing extra reset-short.
15. Tap Help "?" on panel → cheatsheet appears with §3 bindings. Tap again → hides.

**Quest 3 — immersive scene (e.g. a DOME 180° SBS scene):**

16. Enter VR. Panel and lasers hidden.
17. Trigger summon panel; double-click play/pause; thumbstick L/R seek — all work as in cinema.
18. Hold trigger + move → sphere translates with controller. (SKYBOX-extension; verify it feels usable, not nauseating.)
19. Push thumbstick up/down → sphere scales (zoom-equivalent). Verify clamp at 0.3×–5×.
20. B short-press → sphere yaw recenters (re-orients to face user-forward). Geometry scale unchanged.
21. B long-press → yaw recenter + geometry pose+scale reset.

**Regression checks (both modes):**

22. Audio still in sync.
23. No first-frame black flash.
24. Format picker (M3b) still opens, preset taps still rebind + write-back.
25. M1 surfaces (scene grid, sidebar, search) untouched.

**Validation artifact:** `docs/superpowers/research/2026-05-08-m3c-result/result.md`, same template as M3b's deferred result template.

## 8. Risks

- **Trigger state machine edge cases.** Rapid triple-clicks, drag-into-button, B/Y long-press while a thumbstick gesture is active. Mitigation: keep the state machine simple, log every state transition in dev mode, exercise the §7 list of test cases on-headset before declaring done. If specific edge cases regress, add them to the validation artifact and adjust the machine.
- **WebXR `setReferenceSpace` quirks on Meta Browser.** Recenter via offset reference space is the standard recipe but Meta Browser's WebXR implementation has historical quirks. Mitigation: implement standard recipe first; if it misbehaves, fall back to manually rotating a wrapper `<a-entity>` that contains the camera rig (less clean but more controllable).
- **Sphere/fisheye-quad scale clipping.** Scaling the sphere very small could clip the camera into the back face; very large could push the sphere through the scene background. Mitigation: hard clamp at [0.3, 5]. Drag translation has no clamp — user can move the geometry far away if they want, and reset brings it back.
- **Drag-into-button false positive.** User aims at a button intending a click, but the controller drifts >5 cm during the press → state machine reclassifies as drag, button doesn't fire. Mitigation: 5 cm threshold is generous; A-Frame's existing click pipeline only fires when the raycaster maintains lock through triggerup, so a small jitter won't drift off the button anyway. If it surfaces, raise the drag-distance threshold or require a longer hold-duration.
- **Quest Meta button is unreachable.** WebXR cannot intercept the system button; "Oculus button = recenter" from SKYBOX has no analog. Mitigation: B/Y long-press is the substitute (covered in §3, §4.2). Spec is explicit.
- **Laser-hidden mode disorientation.** Without a laser line, user can't see what they're aiming at when the panel is hidden — fine for drag (uses controller pose, not raycast) and for click-to-summon (direction doesn't matter), but might feel odd on first try. Mitigation: A-Frame's default `oculus-touch-controls` model stays visible, so the user still sees the controller in their hand. Only the *laser line* is hidden. If users complain, surface an option to keep lasers visible.
- **Component-vs-IIFE boundary creep.** New component and existing inline IIFE both want to mutate panel/laser/geometry state. Mitigation enforced by spec: component emits semantic events ONLY; IIFE owns DOM mutation. Code review checks this.
- **Panel-hidden-by-default UX surprise.** M3b shipped with the panel always visible. M3c flips it. First-time use after upgrade may confuse the user — they enter VR, see no panel, don't know to click. Mitigation: this user designed the change, so will know. If it surprises in practice, M3c-followup adds the first-entry tutorial overlay we deferred.
- **Single-click latency from 300 ms double-click window.** Panel toggle has a perceptible delay (300 ms) before firing because we wait to see if a second click arrives. Mitigation: 300 ms is the SKYBOX-typical window — users have habituated. If it feels sluggish, shorten to 250 ms during validation and re-test.

## 9. What stays untouched

- M2 + M3a + M3b sync/flash/audio behavior: single-video architecture, no `muted` attribute on the page video, no re-mute on `exit-vr`, `<a-scene background="color:#111">`.
- `library.Service`, `library.UpdateTags`, `library.FindOrCreateTag` — unchanged.
- M3b's `/browse/scene/{id}/projection` endpoint, three render entities, Format picker, tag write-back.
- `internal/api/internal/projection.go::Detect` — unchanged.
- `/deovr/videodata.go::set3DFormat` and `/heresphere/videodata.go::set3DFormat`.
- All M1 surfaces (scene grid, sidebar, search, pagination, mutation forms — rating, favorite, tags chip add/remove, O-counter, organized).
- HTTPS / Caddy / DuckDNS, build flow, goreleaser config.
- A-Frame vendored at `internal/static/vendor/aframe.min.js`. No new vendored libraries.

## 10. After this milestone

If M3c ships green:

- **M3c-followup:** First-entry tutorial overlay, only if the user reports forgetting bindings in practice. Layout: a backing plane appears on Enter VR with the §3 cheatsheet; auto-dismisses after first interaction or on tap; persisted in `localStorage` as already-seen.
- **M3b-followup:** IPD slider in an Advanced Settings panel.
- **M4:** CUBEMAP / EAC support, if the user's library ever needs it.

If M3c surfaces something unexpected — recenter misbehaves, drag feels janky, state machine is flaky, panel-hidden-by-default disorients — pause and re-spec rather than patching in place.
