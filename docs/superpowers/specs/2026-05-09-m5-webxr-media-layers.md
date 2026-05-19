# M5: WebXR Media Layers for high-resolution video

**Date:** 2026-05-09
**Status:** Drafting (brainstormed 2026-05-09)
**Predecessors:** [M3a multi-projection rendering](2026-05-08-m3a-multi-projection-rendering.md), [M3b in-VR projection picker](2026-05-08-m3b-in-vr-projection-picker.md), [M4b VR control panel](2026-05-09-m4b-vr-control-panel.md)
**Successor:** TBD
**Spike result:** Commit `c053d10` (`?spike-layers=force`) confirmed on Quest 3 + Meta Browser that routing the video through `XRMediaBinding.createEquirectLayer` eliminates the diagonal black "V flash" that VideoTexture produces on 8K content.

---

## 1. Context (why this milestone)

`THREE.VideoTexture` uploads each video frame from the `<video>` element to GPU memory via `gl.texImage2D` per render tick. At 8K resolution (~33 megapixels per frame, ~133 MB at RGBA) this upload pressure on the Quest 3's shared SoC bandwidth occasionally lets a partially-decoded frame reach the texture, producing the diagonal-wedge artifact users have reported on KAVR-338 (scene 1842), SAVR-417 (scene 5535), and other 8K-tagged content. Cross-checks proved this is browser-pipeline-specific: HereSphere on the same hardware plays the same files cleanly because it uses Android `MediaCodec` + `SurfaceTexture` direct compositor path, bypassing CPU↔GPU memory traffic entirely.

WebXR Media Layers (`XRMediaBinding`) is the browser-side equivalent: the compositor reads frames directly from the `<video>` element and presents them through a dedicated layer, with no `texImage2D` intermediate step. Meta has published Quest 2 samples running 8K @ 90fps via this API. The M5 spike confirmed it works on the user's Quest 3 specifically. M5 productizes the spike into a default render path.

## 2. Goal & non-goals

**Goal:** When the browser supports WebXR Layers, route stash-vr's VR video playback through `XRMediaBinding` instead of `THREE.VideoTexture` for projections that have a corresponding layer type. Preserve the existing HUD (control panel, scrub bar, subtitles, format picker, etc.) on top of the media layer.

**Success criteria, manually verified on Quest 3 / Meta Browser:**
1. Scene 1842 (KAVR-338, 8K HEVC) plays smoothly in sphere180 + SBS mode without the V-flash artifact.
2. Scene 5535 (SAVR-417) likewise.
3. The control panel summons and operates as before; all M4b features (scrub, markers, captions, mute, speed, loop, etc.) work.
4. Re-entering VR after sleep > 30 s recovers cleanly (M4b round-5 sleep recovery still applies).
5. Switching projection mode (cinema ↔ sphere180 ↔ sphere360) re-binds the appropriate layer type.
6. Switching stereo mode (mono ↔ SBS ↔ TB) updates the layer's layout.
7. Browsers without Layers support fall back to today's VideoTexture path automatically — no regression for non-Quest browsers or older Meta Browser builds.

**Non-goals:**

- **Fisheye projection.** Fisheye keeps its existing `ShaderMaterial` path with custom UV math. WebXR Layers has no native fisheye layer type; pre-warping fisheye to equirect on the GPU is a substantial separate engineering effort (deferred to M-followup if real demand surfaces). Documented limitation: 8K fisheye scenes will still exhibit the V flash in stash-vr's web player; user is recommended to view those in HereSphere.
- **WebCodecs API.** `XRMediaBinding` reads from the existing `<video>` element directly. Manual `VideoDecoder` / `VideoFrame` orchestration via WebCodecs is unnecessary here and adds significant complexity (custom demux, A/V sync, seek logic) for no measured benefit.
- **Cross-browser parity.** Quest 3 / Meta Browser is the only target where (a) users hit the 8K issue and (b) the Layers API is reliably available. Other browsers (desktop Chrome, Firefox, Safari) fall back to VideoTexture without UI to indicate the difference.
- **Subtitle quality upgrade.** Subtitles continue to render through A-Frame as text on a plane in the HUD layer. Crisper subtitle rendering via a dedicated `XRQuadLayer` per cue is interesting but separate.

## 3. Architecture

### 3.1 Layer mapping per projection

| Source projection (`Projection.Geometry`, `Projection.FOV`) | Layer type | Key parameters |
|---|---|---|
| `equirectangular`, FOV 180 | `XREquirectLayer` | `centralHorizontalAngle: π`, `upperVerticalAngle: π/2`, `lowerVerticalAngle: -π/2`, `radius: 0` |
| `equirectangular`, FOV 360 | `XREquirectLayer` | `centralHorizontalAngle: 2π`, vertical angles same |
| `fisheye`, any FOV | **No layer** — keep `ShaderMaterial` path | n/a |
| no projection (cinema/flat) | `XRQuadLayer` | `width: 4`, `height: 2.25`, `transform: XRRigidTransform(0, 1.6, -3)` |

Stereo layout maps directly from `Projection.Stereo` to the WebXR Layers `layout` parameter:

| stash-vr `Projection.Stereo` | WebXR `layout` |
|---|---|
| `''` (mono) | `'mono'` |
| `'sbs'` | `'stereo-left-right'` |
| `'tb'` | `'stereo-top-bottom'` |

### 3.2 HUD coexistence — the central question

The spike's force mode worked because it replaced A-Frame's `XRWebGLLayer baseLayer` with `[mediaLayer]` outright, dropping the projection target Three.js was rendering into. For production the session must be in **Layers mode** (`renderState.layers` populated, no `baseLayer`) so we can compose `[mediaLayer, projectionLayer]` — media behind, projection (HUD) in front.

Three.js's `WebXRManager.setSession()` *does* auto-create an `XRProjectionLayer` instead of an `XRWebGLLayer baseLayer` when `session.enabledFeatures.includes('layers')`. The spike's `legacy-baselayer` bail-out demonstrates this didn't happen in our setup despite `webxr="optionalFeatures: layers"` being on `<a-scene>`. Phase 0 of M5 (§4.1) is dedicated to finding why and fixing it.

Once the session is in Layers mode, the integration is small:

```
A-Frame's session  ─┬─ XRProjectionLayer  ──→ Three.js renders HUD
                    └─ XRMediaLayer (ours)──→ compositor reads <video> directly
                       (created/destroyed on projection / stereo change)
```

The four geometry entities (`vrSphere180`, `vrSphere360`, `vrFisheye`, `vrFlat`) each currently bind `sharedTex` to a material. Under M5: when a layer is active for that geometry, the entity is set `visible="false"` (no double-render). When the user switches projection (e.g., to fisheye, which has no layer), the layer is destroyed and the entity goes back to its current shader/material path.

### 3.3 Subtitles

Subtitles continue as today: an `<a-text>` in `vrSubtitlePlane` rendered through Three.js (i.e., the projection layer / HUD). The current re-parenting logic (`reparentSubtitlePlane()` in [browse_scene.gohtml](../../../internal/static/browse_scene.gohtml)) already chooses parent based on active geometry: `vrFlat` for cinema, camera for immersive. With M5, when cinema mode is active and `vrFlat.visible = false`, the subtitle plane needs a different anchor in cinema mode — pin it to the camera with a fixed forward-down offset matching the bottom of where the cinema video appears. This is a 5-line change in `reparentSubtitlePlane`.

### 3.4 Per-projection / per-stereo updates

`XREquirectLayer` and `XRQuadLayer` parameters (`layout`, angles, dimensions) are set at creation and not updated at runtime. When the user changes projection or stereo via the format picker, the existing media layer is destroyed (`layer.destroy()`) and a new one created with the new parameters, then `updateRenderState({layers: [...]})` is called.

The format-picker code (M3b) currently swaps which geometry entity has `visible: true`. M5 hooks into the same trigger: after the geometry swap, also rebuild the layer.

### 3.5 Fallback path

Feature detection at session start:

1. Was `'layers'` in `session.enabledFeatures`? (Granted)
2. Is `XRMediaBinding` defined globally?

If either is false, we stay entirely on the existing VideoTexture path. The four geometry entities continue to render via their current materials/shaders. No UI indication; this is silent graceful degradation.

If both are true, we enter Layers mode for non-fisheye projections.

### 3.6 Sleep-recovery interaction

M4b round-5 sleep recovery does `teardownVideoTexture()` + `rebindActiveGeometry()` + decoder kick. With M5 active, the VideoTexture isn't in use for the active layer-managed projections, so `teardownVideoTexture` becomes a no-op for those. The decoder kick (`video.pause / seek / play`) is still useful — the `<video>` element is the source for both paths and benefits from the kick after deep sleep. The `webglcontextlost`/`restored` handlers also still apply for the projection layer (which IS WebGL) and for fisheye.

## 4. Implementation phases

### 4.1 Phase 0 — Investigate why session ends up on baseLayer (timebox: 1 day)

The spike sets `webxr="optionalFeatures: layers"` on `<a-scene>` but `session.renderState.layers` came back empty, indicating Three.js fell through to baseLayer. Phase 0 finds out which step in the chain failed:

**Diagnostic 1.** Add temporary logging in the spike (or a separate small debug page) that, on `enter-vr`, prints `JSON.stringify(session.enabledFeatures)` to console. The user reads this via Quest USB devtools (`chrome://inspect`) — we'll provide step-by-step USB setup instructions.

**Diagnostic 2.** Inspect bundled Three.js version in `internal/static/vendor/aframe.min.js`. The Layers code path lives in `WebXRManager.setSession()`. Check whether the bundled Three.js version actually contains the auto-projection-layer logic (Three.js r152+).

**Decision tree:**

- **A.** `enabledFeatures` doesn't include `'layers'` → A-Frame's webxr component isn't passing the feature to `requestSession`, or the browser is denying it. Look at A-Frame webxr component schema; verify the `optionalFeatures` parsing. Test with `requiredFeatures: layers` to see if browser explicitly denies. Outcome: usually a config fix.
- **B.** `enabledFeatures` includes `'layers'` but Three.js still set up baseLayer → bundled Three.js is older than r152 OR the renderer needs an explicit opt-in. Outcome: either bump A-Frame, or apply a small monkey-patch on `renderer.xr` before A-Frame initializes.
- **C.** Something stranger → escalate; consider Approach 1 (pre-session interception) as fallback. The spike's force mode proves we can get at the layers API; we'd just be doing more of the work ourselves.

Phase 0 ends with a one-paragraph result note appended to this spec.

### 4.1.1 Phase 0 result

Run on commit `c58998d` (+ uncommitted refresh-on-interval refinement to the diagnostic) on Quest 3 / Meta Browser on 2026-05-19:

- `session.enabledFeatures`: `local, viewer, layers, web-xr, local-floor` (`'layers'` present)
- `renderState.layers.length`: `0` at A-Frame's `enter-vr` event; settles to `1` by t≈1–4s
- `renderState.baseLayer`: `no` (both at t=0 and at settled state)
- `XRMediaBinding`: `yes`
- `AFRAME.THREE.REVISION`: `r173`

**Outcome:** None of A / B / C. Three.js's `WebXRManager` *does* auto-create an `XRProjectionLayer` when `'layers'` is granted (no `baseLayer` is ever set; `renderState.layers` reaches length 1 once setup completes). The spike's `legacy-baselayer` bail kept firing because it read `renderState.layers` synchronously at A-Frame's `enter-vr` event, which fires *before* Three.js has populated the projection layer. A single read at that instant shows `layers:0 + baseLayer:no` — neither type set — and the spike's `!existingLayers.length` check incorrectly classified that as legacy mode.

**Path forward:** Skip Task 2A/2B/2C. Proceed directly to **Task 3** (production layer manager). The only adjustment vs. the plan as written: the manager's `enter-vr` handler must defer its `renderState.layers` read until Three.js has populated the projection layer — either via a `requestAnimationFrame` poll (wait until `renderState.layers.length >= 1` or a short timeout elapses), or by hooking `renderer.xr`'s own `sessionstart` event. The same deferral pattern should replace the spike's bail condition during Task 3's refactor.

### 4.2 Phase 1 — Production layer integration (assumes Phase 0 finds a tractable fix)

Files modified:

- `internal/static/browse_scene.gohtml`
  - Replace the spike block with production layer-management code (factor out the `createEquirectLayer` / `createQuadLayer` logic, hook into projection/stereo change, manage layer lifecycle on enter-vr / exit-vr / projection-change).
  - Update `reparentSubtitlePlane` to anchor to camera in cinema mode when `vrFlat` is layer-managed.
  - Wire feature detection so non-Layers sessions keep the current path.
  - Apply the Phase 0 fix (e.g., adjust `<a-scene webxr=...>` config, monkey-patch `renderer.xr`, etc.).
- Potentially `internal/static/m3c-controls.js` — if any laser/raycaster setup needs to be adjusted relative to the new layer ordering.

No server-side Go changes are anticipated.

### 4.3 Phase 1-alt — Session interception (if Phase 0 finds a deeper blocker)

Drop the `<a-scene webxr=...>` request and intercept session creation: override `navigator.xr.requestSession` in a small init script that runs before A-Frame loads, ensuring `requiredFeatures: ['layers']` is included. Then post-session, manually create the projection layer + media layer pair, configure Three.js's renderer to use the projection layer's framebuffer, and add the media layer to `renderState.layers`. This is Approach 1 from brainstorming. Significantly more code; we use it only if Phase 0 outcome is C.

## 5. Files touched

```
internal/static/browse_scene.gohtml                   <- Layer creation/teardown, feature detection, subtitle reparent, Phase 0 fix
internal/static/m3c-controls.js                       <- (maybe) raycaster/laser visibility tweaks
docs/superpowers/specs/2026-05-09-m5-webxr-media-layers.md  <- this spec, with Phase 0 result appended
docs/superpowers/plans/2026-05-09-m5-webxr-media-layers.md  <- implementation plan from writing-plans skill
```

No server-side Go changes anticipated. The existing `Projection` detection / `SceneStreams` handling stays as-is.

## 6. Risks

- **Phase 0 outcome unknown.** If Three.js fundamentally doesn't auto-use projection layers in the bundled A-Frame version, we may need to bump A-Frame or apply a monkey-patch. Either path adds risk and might require dependency updates we'd otherwise skip in this milestone. Mitigation: timebox Phase 0 to 1 day; if no clear path emerges, escalate to Approach 1 (Phase 1-alt).
- **Layer recreation on projection/stereo change has a brief discontinuity.** Destroying and re-creating an XR layer mid-session may produce a one-frame black flash. Acceptable — the user explicitly initiated the change. M3b's projection-change UX already has a brief pause.
- **Quest 3 Meta Browser version requirements.** WebXR Layers and `XRMediaBinding` are well-supported on recent Meta Browser builds, but old builds may lack the API. Feature detection (§3.5) handles this.
- **HUD layer ordering.** When `renderState.layers = [mediaLayer, projectionLayer]`, the rendering order is back-to-front; media behind, HUD in front. Validate that all HUD elements (panel, format picker, help, subtitle picker, subtitle plane, scrub bar, marker tooltip) appear correctly on top of the media layer with no z-fighting or partial occlusion.
- **Subtitle plane in cinema mode without `vrFlat`.** The current re-parent logic anchors subtitle plane to `vrFlat.object3D`. Under M5, `vrFlat` is hidden in cinema mode. New cinema-mode anchor: camera-attached at fixed offset, similar to immersive mode's anchoring but with offset matching the bottom of the cinema layer's apparent position. Validate that subtitles appear under the cinema video, not floating awkwardly.
- **Format picker mid-session swap correctness.** The current picker updates `data-stereo` / `data-geometry` on `<a-scene>`, then swaps which geometry entity has `visible: true`. M5 must additionally tear down old layer + create new one in the same code path.

## 7. Validation

On Quest 3 / Meta Browser:

### A. Core 8K acceptance
- [ ] Scene 1842 (KAVR-338) in sphere180 + SBS mode: video plays smoothly for 5+ minutes, no V flash.
- [ ] Scene 5535 (SAVR-417) likewise.
- [ ] Both scenes' HUD: panel summons; scrub bar tracks; markers visible & clickable; CC button hidden (no captions); play/pause/format/help/exit work; speed and loop work; mute and volume work; format picker can switch projection.

### B. Projection / stereo switching
- [ ] In scene 1842, open format picker; switch sphere180 → sphere360. Layer recreates; video continues; no error.
- [ ] In scene 1842, switch sbs → tb (artificial test, won't look right but validates layer rebuild). Then back to sbs.
- [ ] Switch sphere180 → cinema. Quad layer attached at expected position; subtitle plane anchors correctly; HUD operates.
- [ ] Switch cinema → fisheye (manually pick fisheye). Layer destroyed; fisheye shader path takes over; HUD still works; V flash returns on 8K (documented limitation).

### C. Fallback / non-Layers path
- [ ] On a desktop browser without Layers support, the same scenes play via the legacy VideoTexture path; no regression vs M4b behavior. M4b features still work.

### D. Sleep recovery
- [ ] Sleep > 30 s, re-enter VR. Video continues smoothly. Layer is recreated as needed.
- [ ] Sleep > 2 minutes (deep sleep). M4b round-5 hard refresh still triggers; video resumes.

### E. M4b regressions
- [ ] All M4b §8 validation criteria still pass on a non-8K scene (a Resolution:2160p or smaller scene).

### F. Other render modes
- [ ] Sphere360 with mono content (e.g., a regular VR360 scene without stereo): plays via XREquirectLayer with `layout: 'mono'`.
- [ ] Cinema 2D with mono content: plays via XRQuadLayer.

## 8. Open questions

- **Q1.** Does A-Frame's `optionalFeatures` attribute parser preserve the request through to `requestSession`? Phase 0 D1 answers this.
- **Q2.** Does the bundled Three.js auto-use projection layers when `'layers'` is granted? Phase 0 D2 answers this.
- **Q3.** Does `XRMediaBinding` exist on Meta Browser at the user's current version? Spike force mode answered yes; document the minimum version we tested.
- **Q4.** Does layer destroy + recreate cause any audio glitch (decoder mid-frame)? Test in validation §B.
- **Q5.** Is there any scenario where `XREquirectLayer` for VR180 SBS shows a content-mirror artifact different from the current sphere-mesh path? Validate carefully — the current sphere has a U-flip in its onBeforeRender; the layer-side may differ.

## 9. Out-of-scope / future follow-ups

- **Fisheye via shader pre-warp.** If 8K fisheye support becomes a real ask, design a shader that pre-warps to equirect into a render target, then displays via XREquirectLayer. Substantial work.
- **Subtitle layer.** Per-cue XRQuadLayer for crisper subtitle text at compositor resolution. Interesting but premature.
- **Cylinder layer** for a curved cinema screen (XRCylinderLayer). Could be a Format-picker option ("Curved screen"). Easy add once Layers integration is in place.
- **Bitrate-aware auto-quality.** If the user's hardware can't keep up even with Layers (extreme bitrates), auto-pick a transcoded resolution from `vd.SceneParts.SceneStreams`. The original M4b-followup-8K-downgrade idea, retained as a sensible follow-up for users without Layers support.
