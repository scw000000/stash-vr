# M5: WebXR Media Layers — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `THREE.VideoTexture` with WebXR Media Layers (`XRMediaBinding`) for sphere180 / sphere360 / cinema projections in stash-vr's web player, eliminating the V-flash artifact that 8K content produces today. Fisheye keeps its current shader path. The HUD continues to render via A-Frame's projection layer.

**Architecture:** Phase 0 (Task 1) is a 1-day diagnostic that determines why the spike's session ended up on `XRWebGLLayer baseLayer` despite `optionalFeatures: layers`. Phase 0 outcome is recorded in the spec and dictates which Phase 1 branch task runs (2A: A-Frame config fix; 2B: Three.js Layers opt-in; 2C: pre-session interception fallback). Phase 2 (Tasks 3–8) is common: build the production layer manager, integrate with the projection picker, fix subtitle anchoring in cinema mode, verify sleep-recovery interaction, remove the spike + diagnostic, and run the manual validation checklist.

**Tech Stack:** A-Frame 1.7 (WebXR component), Three.js (bundled inside `aframe.min.js`, including its `WebXRManager` and `XRWebGLBinding` wrappers), vanilla JavaScript inside the page's `<script>` IIFE, Go html/template (host page).

**Spec:** [docs/superpowers/specs/2026-05-09-m5-webxr-media-layers.md](../specs/2026-05-09-m5-webxr-media-layers.md)

**No tests in this project.** Verification is `go vet ./...`, `go build ./...`, and the on-headset checklist in spec §7.

**Prerequisite:** M4b round-5 + spike commits shipped (HEAD at or after `c053d10`). The current spike block in `internal/static/browse_scene.gohtml` (`spikeWebXRLayers` IIFE) gets replaced by the production layer manager in Task 3 — leave it in place until then so the user can keep using force mode if they want to retest.

**Branching:** Tasks 2A, 2B, 2C are mutually exclusive. Pick based on Phase 0 outcome recorded in the spec. Skip the others.

---

## Task 1: Phase 0 — diagnostic in-VR overlay

**Files:**
- Modify: `internal/static/browse_scene.gohtml`
- Modify: `docs/superpowers/specs/2026-05-09-m5-webxr-media-layers.md`

**Goal:** Surface the four runtime probes that determine which Phase 1 path applies — `session.enabledFeatures`, `renderState.layers.length`, `XRMediaBinding` presence, and `AFRAME.THREE.REVISION` — as a visible-in-VR text overlay so the user can read them without devtools. Then record the outcome in the spec.

The mapping from probe results to Phase 1 path:

| Probe result | Outcome | Phase 1 path |
|---|---|---|
| `'layers'` is **not** in `enabledFeatures` | A | **Task 2A** (A-Frame config fix) |
| `'layers'` **is** in `enabledFeatures` AND `layers.length === 0` | B | **Task 2B** (Three.js Layers opt-in) |
| Neither A nor B (e.g., `XRMediaBinding` missing entirely on this build) | C | **Task 2C** (session interception fallback) |

- [ ] **Step 1: Add the diagnostic block to the IIFE**

Open `internal/static/browse_scene.gohtml`. Find the existing spike block by searching for `spikeWebXRLayers` (anchored around line 1784). Insert a new IIFE **immediately above** the spike block (so the diagnostic always runs even when `?spike-layers` flag isn't set):

```javascript
    // ============================================================
    // M5 Phase 0 diagnostic — see specs/m5 §4.1.
    // Always-on (no URL flag); shows runtime probe values as a
    // camera-attached <a-text> overlay so the user can read them
    // without devtools. Tasks 2A/2B/2C diverge based on what this
    // reports. Removed in Task 7 once Phase 1 ships.
    // ============================================================
    (function m5Phase0Diagnostic() {
      let diagText = null;
      function ensureOverlay() {
        if (diagText) return diagText;
        diagText = document.createElement('a-text');
        diagText.setAttribute('id', 'm5DiagOverlay');
        diagText.setAttribute('value', 'M5 Phase 0\n(awaiting session)');
        diagText.setAttribute('color', '#0f0');
        diagText.setAttribute('align', 'left');
        diagText.setAttribute('width', '2');
        diagText.setAttribute('position', '-0.6 0.3 -1.5');
        diagText.setAttribute('visible', 'false');
        // Attach to the camera entity so the panel follows head pose.
        const cam = scene.querySelector('[camera]') || scene;
        cam.appendChild(diagText);
        return diagText;
      }
      scene.addEventListener('loaded', ensureOverlay);

      scene.addEventListener('enter-vr', function() {
        const overlay = ensureOverlay();
        const lines = ['M5 Phase 0'];
        let xrSession = null;
        try {
          xrSession = scene.renderer && scene.renderer.xr && scene.renderer.xr.getSession();
        } catch (_) {}
        if (!xrSession) {
          lines.push('NO SESSION');
        } else {
          const ef = xrSession.enabledFeatures
            ? Array.from(xrSession.enabledFeatures)
            : [];
          lines.push('features: ' + (ef.length ? ef.join(',') : '(none)'));
          const layersArr = xrSession.renderState && xrSession.renderState.layers;
          lines.push('layers count: ' + (layersArr ? layersArr.length : '0'));
          const baseLayer = xrSession.renderState && xrSession.renderState.baseLayer;
          lines.push('baseLayer: ' + (baseLayer ? 'yes' : 'no'));
        }
        lines.push('XRMediaBinding: ' + (typeof XRMediaBinding !== 'undefined' ? 'yes' : 'no'));
        const rev = (typeof AFRAME !== 'undefined' && AFRAME.THREE && AFRAME.THREE.REVISION)
          ? AFRAME.THREE.REVISION
          : '?';
        lines.push('THREE: r' + rev);
        overlay.setAttribute('text', 'value', lines.join('\n'));
        overlay.setAttribute('visible', 'true');
      });

      scene.addEventListener('exit-vr', function() {
        if (diagText) diagText.setAttribute('visible', 'false');
      });
    })();
```

The overlay sits in front of the camera at `(-0.6, 0.3, -1.5)` and shows five labeled lines. Five lines were chosen so the implementer doesn't have to ask "and the baseLayer state too?" — Outcome C is detectable from `baseLayer: yes` + `XRMediaBinding: no`.

- [ ] **Step 2: Verify build is clean**

Run: `go vet ./...`
Expected: no output.

Run: `go build ./...`
Expected: no output.

- [ ] **Step 3: Commit the diagnostic block**

```bash
git add internal/static/browse_scene.gohtml
git commit -m "$(cat <<'EOF'
m5: phase-0 in-VR diagnostic overlay (camera-attached <a-text>)

Surfaces session.enabledFeatures, renderState.layers length,
renderState.baseLayer presence, XRMediaBinding presence, and
AFRAME.THREE.REVISION as a five-line green text panel so the user can
read the Phase 0 probe values from inside VR without devtools. Used
to determine which Phase 1 path applies (A: config fix, B: Three.js
opt-in, C: session interception). Removed in Task 7 once Phase 1
ships.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 4: User runs the diagnostic on Quest 3**

Build and deploy as usual (`scripts\build-windows.bat` then redeploy the binary). Tell the user:

> Open ANY scene's browse page (e.g., `https://<your-stash-vr-host>/browse/scene/1842`). No URL flag needed. Click Enter VR. Read the green text overlay near the bottom-left of your view. Report back:
>
> 1. The full `features:` line (comma-separated list of strings)
> 2. The `layers count:` value
> 3. The `baseLayer:` value (yes/no)
> 4. The `XRMediaBinding:` value (yes/no)
> 5. The `THREE:` revision number

- [ ] **Step 5: Map the readings to the Phase 1 outcome**

Compare the user's report against the table at the top of this task:

- `XRMediaBinding: no` → **Outcome C**, proceed to **Task 2C**.
- `XRMediaBinding: yes` AND `'layers'` not in `features:` → **Outcome A**, proceed to **Task 2A**.
- `XRMediaBinding: yes` AND `'layers'` is in `features:` AND `layers count: 0` AND `baseLayer: yes` → **Outcome B**, proceed to **Task 2B**.
- `XRMediaBinding: yes` AND `'layers'` is in `features:` AND `layers count >= 1` → unexpected; the spike's `legacy-baselayer` bail wouldn't have fired in this case. Re-check the spike block; if the bail fired anyway, the bail condition is wrong. Proceed to Task 2B as the more likely path and adjust the spike's bail condition during Task 3's refactor.

- [ ] **Step 6: Append the outcome to the spec**

Edit [docs/superpowers/specs/2026-05-09-m5-webxr-media-layers.md](../specs/2026-05-09-m5-webxr-media-layers.md). Find the line `### 4.1 Phase 0 — Investigate why session ends up on baseLayer (timebox: 1 day)` and add a new subsection at the very end of §4.1, before §4.2 starts:

```markdown
### 4.1.1 Phase 0 result

Run on commit [SHA from Task 1 Step 3] on Quest 3 / Meta Browser [version if known] on 2026-05-09:

- `session.enabledFeatures`: `[verbatim list from user]`
- `renderState.layers.length`: `[number]`
- `renderState.baseLayer`: `[yes/no]`
- `XRMediaBinding`: `[yes/no]`
- `AFRAME.THREE.REVISION`: `r[N]`

**Outcome:** [A / B / C]
**Path forward:** Task 2[A/B/C] in the plan.
[One-paragraph note on what we learned and any quirks.]
```

- [ ] **Step 7: Commit the spec update**

```bash
git add docs/superpowers/specs/2026-05-09-m5-webxr-media-layers.md
git commit -m "$(cat <<'EOF'
docs(m5): record Phase 0 diagnostic findings

Outcome [A/B/C]; Phase 1 will use Task 2[A/B/C].

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2A: Phase 1 — A-Frame `optionalFeatures` config fix

**SKIP this task if Phase 0 outcome was B or C.**

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Outcome A means the browser supports `XRMediaBinding` but `'layers'` did not end up in `session.enabledFeatures`. The most likely cause: A-Frame's webxr component parsed the `optionalFeatures: layers` attribute but didn't actually pass it to `requestSession`, OR the browser silently denied the optional feature. The fix is one of the following — try in order until `enabledFeatures` includes `'layers'`.

- [ ] **Step 1: Try `requiredFeatures` instead of `optionalFeatures`**

Find the `<a-scene>` opening tag in the markup (around line 73). The current attribute is:

```html
webxr="optionalFeatures: layers"
```

Change to:

```html
webxr="requiredFeatures: layers"
```

`requiredFeatures` forces the browser to either grant the feature or refuse the session. If the browser supports it (which we proved via the spike's force mode), this should make A-Frame's session creation include it.

- [ ] **Step 2: Verify build is clean**

Run: `go vet ./...` then `go build ./...`. Both clean.

- [ ] **Step 3: Commit, deploy, ask user to re-run the Phase 0 diagnostic**

```bash
git add internal/static/browse_scene.gohtml
git commit -m "$(cat <<'EOF'
m5: switch webxr to requiredFeatures: layers

Outcome A: optionalFeatures may have been silently dropped. Force the
feature via requiredFeatures so the browser must grant it (or refuse
the session, which on Quest 3 it should not).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Tell the user:

> Re-run the diagnostic on the same scene and report the new readings.

- [ ] **Step 4: If `'layers'` is now in `features:` and `layers count >= 1`, proceed to Task 3.**

Update §4.1.1 of the spec to record the new readings ("after Task 2A Step 1") and proceed to Task 3.

- [ ] **Step 5: If still not working, try the alternate A-Frame component syntax**

Some A-Frame builds parse `requiredFeatures: layers` as a single string token instead of an array. Try the array-literal form:

```html
webxr="requiredFeatures: layers; optionalFeatures: local-floor, bounded-floor"
```

(Includes A-Frame's defaults for optionalFeatures so we don't accidentally drop them.)

If still not working after Step 5, move on to Task 2C (session interception). Update the spec result section accordingly.

---

## Task 2B: Phase 1 — Three.js Layers opt-in

**SKIP this task if Phase 0 outcome was A or C.**

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Outcome B means `'layers'` is granted but Three.js's `WebXRManager` still set up an `XRWebGLLayer baseLayer` instead of an `XRProjectionLayer`. Three.js r152+ auto-uses Layers when the feature is granted, so if our bundled Three.js (from Phase 0 Step 4 we now know the revision number) is older than r152, that's the issue. Otherwise, A-Frame's renderer setup is overriding the auto-detection.

- [ ] **Step 1: If THREE revision is < r152, the bundled A-Frame is too old**

A-Frame 1.7 should bundle Three.js r166+. If the diagnostic reported r150 or earlier, the bundled A-Frame in `internal/static/vendor/aframe.min.js` is older than expected. Confirm by re-checking the file's modification date (`ls -l internal/static/vendor/aframe.min.js`) and the A-Frame release notes.

If the bundle is too old, replace it with A-Frame 1.7.x latest. Download from `https://aframe.io/releases/1.7.0/aframe.min.js` (or whichever 1.7.x is current). Skip to Step 5.

If the bundle's THREE revision is r152+, continue to Step 2 — the issue is configuration, not version.

- [ ] **Step 2: Force WebXRManager to use a projection layer manually**

Three.js exposes `renderer.xr.setSession()` which decides between baseLayer and projection layer. We can intercept this on enter-vr by directly creating the projection layer ourselves before A-Frame's renderer finishes its setup. Add this **before** the `m5Phase0Diagnostic` block in the IIFE:

```javascript
    // ============================================================
    // M5 Outcome B fix — force Three.js to use a projection layer
    // when the 'layers' feature is granted but the bundled
    // WebXRManager doesn't auto-switch from baseLayer.
    //
    // Listens for 'sessionstart' on renderer.xr (fires once when
    // A-Frame finishes setting up the session). If the session has
    // 'layers' enabled but renderState is still on baseLayer, we
    // create our own projection layer + replace the renderState
    // accordingly. Three.js's renderer reads the projection layer's
    // framebuffer for subsequent renders.
    // ============================================================
    (function m5OutcomeBFix() {
      scene.addEventListener('enter-vr', function() {
        const xr = scene.renderer && scene.renderer.xr;
        if (!xr) return;
        const xrSession = xr.getSession();
        if (!xrSession) return;
        const ef = xrSession.enabledFeatures
          ? Array.from(xrSession.enabledFeatures)
          : [];
        if (!ef.includes('layers')) return;
        // Already on Layers? Nothing to do.
        const rs = xrSession.renderState;
        if (rs && rs.layers && rs.layers.length) return;
        if (!rs || !rs.baseLayer) return;
        // Three.js gave us a baseLayer despite 'layers' being enabled.
        // Replace with a projection layer.
        try {
          const gl = xr.getContext ? xr.getContext() : (scene.renderer.getContext && scene.renderer.getContext());
          if (!gl) { console.warn('m5: no GL context for projection layer creation'); return; }
          const glBinding = new XRWebGLBinding(xrSession, gl);
          const projLayer = glBinding.createProjectionLayer({
            colorFormat: gl.RGBA8 || 0x8058,
            depthFormat: gl.DEPTH_COMPONENT24 || 0x81A6,
            scaleFactor: 1.0
          });
          xrSession.updateRenderState({ layers: [projLayer] });
          // Three.js's WebGLRenderer needs to know to render into the
          // projection layer's framebuffer instead of the baseLayer's.
          // Most modern revisions (r152+) detect this by reading
          // renderState.layers in onAnimationFrame. If yours doesn't,
          // this step requires a Three.js patch — escalate to Task 2C.
          console.log('m5: Outcome B fix applied — switched to projection layer');
        } catch (err) {
          console.warn('m5: Outcome B fix failed', err);
        }
      });
    })();
```

- [ ] **Step 3: Verify build is clean**

Run: `go vet ./...` then `go build ./...`. Both clean.

- [ ] **Step 4: Commit, deploy, ask user to re-run the diagnostic**

```bash
git add internal/static/browse_scene.gohtml
git commit -m "$(cat <<'EOF'
m5: Outcome B fix — manually create projection layer on enter-vr

Three.js's WebXRManager fell through to baseLayer despite 'layers'
being granted. Create an XRProjectionLayer ourselves via XRWebGLBinding
and updateRenderState to use it. Three.js r152+ should pick up the new
framebuffer on its next animation frame.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Tell the user:

> Re-run the diagnostic on the same scene. Verify `layers count` is now ≥ 1 and the HUD still renders correctly (panel summons, video plays).

- [ ] **Step 5: Update spec §4.1.1 with the result; if HUD broke, escalate**

If the user reports that `layers count` is now ≥ 1 but the HUD doesn't render (black screen for non-video content), Three.js's renderer didn't switch to the new framebuffer — its WebGLRenderer is still pointed at the gone baseLayer. Escalate to Task 2C.

If both `layers count ≥ 1` AND HUD renders correctly: proceed to Task 3.

---

## Task 2C: Phase 1 — pre-session interception

**SKIP this task if Phase 0 outcome was A or B.**

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Outcome C means we need to manage the WebXR session creation ourselves before A-Frame initializes. We override `navigator.xr.requestSession` so that when A-Frame asks for a session, we wrap its options to include `requiredFeatures: ['layers']` and we create the projection layer + media layer directly. This is invasive but unblocks any path A-Frame's webxr component can't reach.

- [ ] **Step 1: Add the request interceptor BEFORE A-Frame loads**

Open `internal/static/browse_scene.gohtml`. Find the line `<script src="/vendor/aframe.min.js"></script>`. Add a new `<script>` block **immediately above** it (it must run before A-Frame parses):

```html
<script>
// M5 Outcome C: pre-session interceptor.
// Wrap navigator.xr.requestSession so any session A-Frame requests
// also includes 'layers' in requiredFeatures. The actual layer
// creation happens later in the IIFE (see m5LayerManager).
(function() {
  if (!navigator.xr || !navigator.xr.requestSession) return;
  const orig = navigator.xr.requestSession.bind(navigator.xr);
  navigator.xr.requestSession = function(mode, init) {
    init = init ? Object.assign({}, init) : {};
    init.requiredFeatures = (init.requiredFeatures || []).slice();
    if (init.requiredFeatures.indexOf('layers') === -1) {
      init.requiredFeatures.push('layers');
    }
    return orig(mode, init);
  };
})();
</script>
```

- [ ] **Step 2: Verify build is clean**

Run: `go vet ./...` then `go build ./...`. Both clean.

- [ ] **Step 3: Commit, deploy, ask user to re-run the diagnostic**

```bash
git add internal/static/browse_scene.gohtml
git commit -m "$(cat <<'EOF'
m5: Outcome C — pre-session interceptor for layers feature

Wrap navigator.xr.requestSession so any session A-Frame creates also
includes 'layers' in requiredFeatures. Combined with Task 3's
production layer manager, this routes the video through XRMediaBinding
without depending on A-Frame's webxr component config to pass through.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Tell the user:

> Re-run the diagnostic on the same scene. Verify `'layers'` is now in `features:` AND `layers count >= 1`.

- [ ] **Step 4: Update spec §4.1.1 with the result and proceed to Task 3**

If `layers count` is still 0 after the interceptor, Three.js's renderer is the blocker — apply Task 2B Step 2 (manual projection-layer creation) ON TOP of the interceptor. Both pieces work together.

Otherwise: proceed to Task 3.

---

## Task 3: Production layer manager + projection-aware lifecycle

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Replace the spike block (`spikeWebXRLayers` IIFE) with a production layer manager. The manager owns the lifecycle of the media layer: creates it on enter-vr (when supported), tears it down + rebuilds it on projection / stereo change, and tears it down on exit-vr. It also hides whichever geometry entity (`vrSphere180` / `vrSphere360` / `vrFlat`) corresponds to the active layer so the video isn't double-rendered.

- [ ] **Step 1: Update `applyPickerState` to track the active geometry id on `<a-scene>`**

The layer manager needs to know which projection is active. The current code reads the active entity by walking visibility, but with M5 the entity is hidden when its layer is active — so visibility no longer reflects intent.

Find `applyPickerState()` (around line 1574 in browse_scene.gohtml). Right after the line `scene.dataset.stereo = stereoData;` (around line 1583), add:

```javascript
      scene.dataset.geometry = activeId;
```

Then move the `let activeId = 'vrFlat';` block to BEFORE the `scene.dataset.stereo = ...;` line so the variable is in scope when we set the dataset attribute. Specifically, the block should now read:

```javascript
    function applyPickerState() {
      const isFishEye = pickerState.type === 'FishEye';
      const isCinema  = pickerState.degree === 'Cinema';
      const isNormal  = pickerState.type === 'Normal';

      // Cinema forces stereo=2D regardless of the user's Stereo pick.
      const effectiveStereo = isCinema ? '2D' : pickerState.stereo;
      const stereoData = effectiveStereo === 'SBS' ? 'sbs' :
                         effectiveStereo === 'TB'  ? 'tb'  : '';

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

      // Record state on <a-scene> so the M5 layer manager can resync
      // independently of object3D.visible (which it overrides).
      scene.dataset.stereo = stereoData;
      scene.dataset.geometry = activeId;

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

      // Notify the M5 layer manager so it can rebuild the media layer
      // for the new projection/stereo.
      if (window.m5SyncLayer) window.m5SyncLayer();
    }
```

This: (a) hoists the `activeId` decision so it can populate `scene.dataset.geometry`; (b) calls a global `m5SyncLayer` hook that the layer manager (added in Step 2) publishes.

- [ ] **Step 2: Add the layer manager IIFE (replaces the spike block)**

Find the `spikeWebXRLayers` IIFE (search for `spikeWebXRLayers`). DELETE the entire IIFE — both the function expression `(function spikeWebXRLayers() { ... })()` and any associated comments above it labeled "SPIKE: WebXR Media Layers". Replace with:

```javascript
    // ============================================================
    // M5 Layer Manager — owns the lifecycle of an XR media layer
    // bound to <video>. Active when:
    //   - XRMediaBinding is defined globally
    //   - The session has 'layers' in enabledFeatures
    //   - renderState.layers is non-empty (projection layer present)
    //   - The active geometry is sphere180 / sphere360 / cinema
    //
    // When active, hides the corresponding geometry entity so the
    // video isn't double-rendered. Fisheye (no native layer type)
    // keeps its current shader path; the manager goes idle.
    //
    // Exposes window.m5SyncLayer for projection-change notification
    // from applyPickerState. Recreates the layer when projection or
    // stereo changes.
    // ============================================================
    (function m5LayerManager() {
      let activeMediaLayer = null;
      let suppressedGeomId = null;
      let mediaBinding     = null;
      let refSpace         = null;
      let supported        = false;

      function buildLayer() {
        if (!mediaBinding || !refSpace) return null;
        const stereo = scene.dataset.stereo || '';
        const layout = stereo === 'sbs' ? 'stereo-left-right' :
                       stereo === 'tb'  ? 'stereo-top-bottom' :
                       'mono';
        const geomId = scene.dataset.geometry || 'vrFlat';
        if (geomId === 'vrSphere180') {
          return mediaBinding.createEquirectLayer(video, {
            space: refSpace,
            layout: layout,
            centralHorizontalAngle: Math.PI,
            upperVerticalAngle: Math.PI / 2,
            lowerVerticalAngle: -Math.PI / 2,
            radius: 0
          });
        }
        if (geomId === 'vrSphere360') {
          return mediaBinding.createEquirectLayer(video, {
            space: refSpace,
            layout: layout,
            centralHorizontalAngle: 2 * Math.PI,
            upperVerticalAngle: Math.PI / 2,
            lowerVerticalAngle: -Math.PI / 2,
            radius: 0
          });
        }
        if (geomId === 'vrFlat') {
          return mediaBinding.createQuadLayer(video, {
            space: refSpace,
            layout: layout,
            width: 4.0,
            height: 2.25,
            transform: new XRRigidTransform({ x: 0, y: 1.6, z: -3 })
          });
        }
        // vrFisheye or unknown: no layer.
        return null;
      }

      function teardown() {
        if (activeMediaLayer) {
          try { activeMediaLayer.destroy(); } catch (_) {}
          activeMediaLayer = null;
        }
        if (suppressedGeomId) {
          const el = document.getElementById(suppressedGeomId);
          if (el) el.setAttribute('visible', 'true');
          suppressedGeomId = null;
        }
      }

      function syncLayer() {
        if (!supported) return;
        teardown();
        const newLayer = buildLayer();
        if (!newLayer) return;
        const xrSession = scene.renderer && scene.renderer.xr && scene.renderer.xr.getSession();
        if (!xrSession) return;
        const existing = (xrSession.renderState && xrSession.renderState.layers) || [];
        const merged = [newLayer].concat(existing.filter(function(l) { return l !== newLayer; }));
        try {
          xrSession.updateRenderState({ layers: merged });
        } catch (err) {
          console.warn('m5: updateRenderState failed', err);
          try { newLayer.destroy(); } catch (_) {}
          return;
        }
        activeMediaLayer = newLayer;
        const geomId = scene.dataset.geometry || 'vrFlat';
        const el = document.getElementById(geomId);
        if (el) {
          el.setAttribute('visible', 'false');
          suppressedGeomId = geomId;
        }
      }
      window.m5SyncLayer = syncLayer;

      scene.addEventListener('enter-vr', function() {
        if (typeof XRMediaBinding === 'undefined') {
          console.log('m5: XRMediaBinding unavailable; falling back to VideoTexture path');
          return;
        }
        const xr = scene.renderer && scene.renderer.xr;
        if (!xr) return;
        const xrSession = xr.getSession();
        if (!xrSession) return;
        const ef = xrSession.enabledFeatures
          ? Array.from(xrSession.enabledFeatures)
          : [];
        if (!ef.includes('layers')) {
          console.log('m5: layers feature not granted; falling back');
          return;
        }
        const layersArr = xrSession.renderState && xrSession.renderState.layers;
        if (!layersArr || !layersArr.length) {
          console.log('m5: renderState.layers empty (baseLayer mode); falling back');
          return;
        }
        try {
          mediaBinding = new XRMediaBinding(xrSession);
          refSpace = xr.getReferenceSpace();
          supported = true;
          syncLayer();
          console.log('m5: layer manager active');
        } catch (err) {
          console.warn('m5: enter-vr setup failed', err);
          mediaBinding = null;
          refSpace = null;
          supported = false;
        }
      });

      scene.addEventListener('exit-vr', function() {
        teardown();
        mediaBinding = null;
        refSpace = null;
        supported = false;
      });
    })();
```

This block REPLACES the spike. After this commit, the URL flag `?spike-layers=*` does nothing (the spike code is gone). Force mode and indicator tints are also gone. The diagnostic overlay from Task 1 is still in place — Task 7 removes that.

- [ ] **Step 3: Verify build is clean**

Run: `go vet ./...` then `go build ./...`. Both clean.

- [ ] **Step 4: Manually verify on Quest 3 (sphere180 SBS)**

Build, deploy. Open scene 1842 (8K SBS). Click Enter VR. Expectations:

- Console (if you can read it via `chrome://inspect`) shows `m5: layer manager active`. The diagnostic overlay still shows `layers count: 1` (or higher if Task 2B's projection layer is present too).
- The video plays smoothly with NO V-flash artifact.
- The HUD (panel, scrub bar, controls) summons normally. Buttons all click.
- `vrSphere180` is invisible (`document.getElementById('vrSphere180').getAttribute('visible')` returns `false`); the layer is presenting the video instead.

If the V-flash returns or layer creation fails: revisit Phase 0's outcome.

- [ ] **Step 5: Manually verify projection switching**

In VR: open the format picker; switch to `Cinema` then back to `180`. Each transition should:
1. Tear down the existing equirect layer.
2. Build a new quad/equirect layer matching the new geometry.
3. Hide the new active geometry entity.
4. Show the previously-hidden entity (or now-fisheye target gets shown if user picks fisheye).

If switching to `FishEye`: the layer manager goes idle (no layer for fisheye), `vrFisheye` becomes visible and the existing shader path takes over.

- [ ] **Step 6: Commit Task 3**

```bash
git add internal/static/browse_scene.gohtml
git commit -m "$(cat <<'EOF'
m5: production layer manager (replaces spike)

Owns the XR media layer lifecycle: creates on enter-vr, syncs on
projection/stereo change via window.m5SyncLayer hook, tears down on
exit-vr. Hides the corresponding geometry entity (vrSphere180/360 or
vrFlat) when the layer is active so video isn't double-rendered.
Fisheye keeps its existing shader path — the manager goes idle.

applyPickerState now writes scene.dataset.geometry so the manager
can resync independently of object3D.visible (which it overrides).

Spike block (?spike-layers=force etc.) deleted; production path is
on-by-default when supported, falls back to VideoTexture otherwise.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Subtitle plane camera-anchor in cinema mode

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Today, `reparentSubtitlePlane()` parents the subtitle plane to `vrFlat.object3D` in cinema mode (positioning it at the bottom of the cinema plane). Under M5, when cinema mode uses an XRQuadLayer, `vrFlat` is hidden — meaning its `object3D.visible` is false but the layer is presenting at the same world position. The subtitle plane needs a fixed world-position anchor in cinema mode so it appears below the layer's video.

- [ ] **Step 1: Update `reparentSubtitlePlane` to handle the M5 cinema-mode case**

Find `reparentSubtitlePlane()` (around line 1317). Replace the function body with:

```javascript
    function reparentSubtitlePlane() {
      const subEl = document.getElementById('vrSubtitlePlane');
      if (!subEl || !subEl.object3D) return;
      const geomId = scene.dataset.geometry || '';
      let targetObj = null;

      // Cinema mode (vrFlat). Two sub-cases:
      //   - M5 layer-managed cinema: vrFlat is hidden; parent the
      //     subtitle plane to <a-scene> with a fixed world-position
      //     under where the quad layer renders (0, 0.6, -3).
      //   - Pre-M5 / fallback path: vrFlat renders the video plane
      //     directly; parent to vrFlat at its bottom edge.
      if (geomId === 'vrFlat') {
        const flatEl = document.getElementById('vrFlat');
        const flatVisible = flatEl && flatEl.object3D && flatEl.object3D.visible;
        if (flatVisible) {
          targetObj = flatEl.object3D;
          subEl.object3D.position.set(0, -1.0, 0.01);
        } else {
          // vrFlat is hidden -> M5 layer-managed cinema.
          targetObj = scene.object3D;
          subEl.object3D.position.set(0, 0.55, -3.0);
        }
      } else {
        // Immersive (sphere/fisheye) -> camera-attached, front-low.
        const cam = scene.camera;
        if (!cam) return;
        targetObj = cam;
        subEl.object3D.position.set(0, -0.5, -1.5);
      }

      if (subEl.object3D.parent !== targetObj) {
        if (subEl.object3D.parent) subEl.object3D.parent.remove(subEl.object3D);
        targetObj.add(subEl.object3D);
      }
    }
```

The new branch reads `scene.dataset.geometry` (set in Task 3 Step 1) instead of relying on `activeGeometry()` returning a non-null entity. The cinema M5 case parents to `<a-scene>` (world space) at `(0, 0.55, -3.0)` — directly below where the quad layer's bottom edge sits (layer at y=1.6, height=2.25 → bottom at y=0.475; subtitle plane height=0.18 centered → at y≈0.55 the plane sits flush below the layer).

- [ ] **Step 2: Verify build is clean**

Run: `go vet ./...` then `go build ./...`. Both clean.

- [ ] **Step 3: Manually verify subtitle anchoring**

Build, deploy. Open a scene **with captions** (any 4K test scene with at least one caption track). Enter VR. Switch to cinema mode. Open CC picker, select the caption language. Verify:

- Subtitles appear directly below the cinema video plane, horizontally centered.
- They do NOT float in random space.

Switch to sphere180 mode. Verify:

- Subtitles attach to the camera, appearing in front-low view as before.

Switch back to cinema mode. Verify:

- Subtitles re-anchor under the cinema plane.

- [ ] **Step 4: Commit**

```bash
git add internal/static/browse_scene.gohtml
git commit -m "$(cat <<'EOF'
m5: subtitle plane anchors to world space in M5 cinema mode

When the XRQuadLayer manages cinema, vrFlat.object3D.visible is false
and the existing reparent logic produced an invisible orphan. Detect
that case via scene.dataset.geometry + flat visibility, and parent the
subtitle plane to <a-scene> at a fixed world position (0, 0.55, -3.0)
that sits flush below the layer's bottom edge.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Sleep-recovery interaction check

**Files:**
- Modify (potentially): `internal/static/browse_scene.gohtml`

**Goal:** Verify that M4b round-5 sleep recovery (`refreshVideoTexture` + watchdog + `video.load()` fallback) still works correctly when M5 is active. The VideoTexture is no longer in use for layer-managed projections, so `teardownVideoTexture()` becomes a partial no-op. The decoder kick (pause/seek/play) is still useful and should resume the layer's video frames.

- [ ] **Step 1: Walk the sleep-recovery code paths to confirm correctness**

Open `internal/static/browse_scene.gohtml`. Find `refreshVideoTexture()` (added in M4b round-5; search for `refreshVideoTexture`). Read through it and the watchdog timer carefully.

For each branch (soft path, watchdog timeout → hard path, visibilitychange long-hidden trigger), confirm:

- The video element is the one bound to the XR media layer (it is — there's only one `<video id="sceneVideo">`).
- `video.pause()` followed by `video.currentTime = t` followed by `video.play()` will cause the compositor to receive new video frames (yes — the layer is bound to the HTMLVideoElement, not to a snapshot).
- `video.load()` fully resets the decoder pipeline; once `loadedmetadata` fires the layer should re-receive frames (yes; same source).

If any of those would break under M5, fix here. Likely no fix needed — the `<video>` element behavior is independent of how we render its frames.

- [ ] **Step 2: Add a tiny robustness tweak to `teardownVideoTexture` if needed**

`teardownVideoTexture` (M4b round-5) disposes the shared `VideoTexture` and the materials that reference it. Under M5, when a layer is active for a sphere/cinema projection, the geometry entity is hidden but its material may still hold the VideoTexture reference. Disposal during sleep recovery will dispose textures we're not actively using, which is harmless but potentially noisy.

If disposal logs warnings on the headset, guard the dispose call with a "is this geometry currently being layer-managed?" check. Otherwise leave as is.

If no fix needed, this task may produce no diff. That's OK.

- [ ] **Step 3: Manually verify sleep recovery on Quest 3 with M5 active**

Build, deploy. Open scene 1842 in sphere180 mode (M5 active). Play 30 seconds. Take off headset for 60 seconds. Put back on, re-enter VR. Verify:

- Video resumes playing within 1-2 seconds.
- No V flash, no stalled frame.
- HUD still summons.

Repeat with a 3-minute removal (deep sleep). Verify the watchdog `video.load()` fallback fires and video resumes.

- [ ] **Step 4: Commit (or skip commit if no diff)**

If Step 2 produced changes:

```bash
git add internal/static/browse_scene.gohtml
git commit -m "$(cat <<'EOF'
m5: tighten teardownVideoTexture under M5 layer-managed projections

[brief description of what was tightened]

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

If no changes were needed, note "no fix required" in the implementer's task report and proceed.

---

## Task 6: Fallback path verification on a non-Layers browser

**Files:**
- (read-only verification; no diff expected unless a fallback bug is found)

**Goal:** Verify that on a browser without WebXR Layers support (or without `XRMediaBinding`), stash-vr falls back to the existing `THREE.VideoTexture` path automatically with no regressions.

- [ ] **Step 1: Test on a non-Layers browser**

Open the same scene in:
- Desktop Chrome (no WebXR session at all — doesn't apply).
- Quest browser with Layers support disabled (if there's a way to flag-off; otherwise skip).

Most realistic test: temporarily edit Task 3's manager so its enter-vr handler always early-returns (simulate "no Layers"), build, deploy, retest. Verify scene plays via VideoTexture path as it did pre-M5 (V flash returns on 8K, but other behavior is intact).

Revert the temporary edit and rebuild.

- [ ] **Step 2: Confirm and proceed**

Note in the implementer's task report whether the fallback was exercised and how. No commit unless a bug was found.

---

## Task 7: Remove diagnostic overlay; clean up

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Phase 0 is over and the layer manager is shipping. Remove the diagnostic IIFE so it's not noise in production. Also remove any stale spike comments that survived Task 3's deletion.

- [ ] **Step 1: Delete the M5 Phase 0 diagnostic block**

Find the `m5Phase0Diagnostic` IIFE (added in Task 1) and delete it entirely, including its banner comment. Search for `m5Phase0Diagnostic` and remove from the opening comment through the closing `)();`.

- [ ] **Step 2: Search for any leftover `spike-layers` references and remove**

Run a grep for `spike-layers` and `spikeWebXRLayers` in `internal/static/browse_scene.gohtml`. Either should be zero matches after Task 3, but if any survived (e.g., a stale comment), delete them.

Also search for `tintMuteButton` and `spikeStatus` — these should also be gone. If any survive, delete.

- [ ] **Step 3: Verify build is clean**

Run: `go vet ./...` then `go build ./...`. Both clean.

- [ ] **Step 4: Commit**

```bash
git add internal/static/browse_scene.gohtml
git commit -m "$(cat <<'EOF'
m5: remove Phase 0 diagnostic + leftover spike artifacts

Production layer manager is shipping; the in-VR diagnostic overlay and
?spike-layers force-mode debugging are no longer needed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Manual validation pass on Quest 3

**Files:**
- (validation only; no diff)

**Goal:** Walk the spec §7 validation checklist on the user's Quest 3. Any failure means we haven't shipped — diagnose and either fix or revert.

- [ ] **Step 1: §7-A Core 8K acceptance**

- Scene 1842 (KAVR-338) in sphere180 + SBS: smooth playback for 5+ minutes, no V flash, HUD operates.
- Scene 5535 (SAVR-417) likewise.
- All M4b features still work on these scenes.

- [ ] **Step 2: §7-B Projection / stereo switching**

- 1842: sphere180 → sphere360. Layer recreates; video continues; no error.
- 1842: sbs → tb (artificial; just confirms layer rebuild). Then back to sbs.
- 1842: sphere180 → cinema. Quad layer attached at expected position; subtitle plane anchors correctly under the layer; HUD operates.
- 1842: cinema → fisheye (manually pick). Layer destroyed; fisheye shader path takes over; HUD still works.

- [ ] **Step 3: §7-C Fallback / non-Layers path**

Already covered in Task 6.

- [ ] **Step 4: §7-D Sleep recovery**

Already covered in Task 5.

- [ ] **Step 5: §7-E M4b regressions**

Open a 4K scene (any `Resolution:2160p` scene without `#:8KVR`). Walk the M4b §8 checklist subset that's testable in 5 minutes — title, time, scrub, drag, mute, speed, loop, format. All work.

- [ ] **Step 6: §7-F Other render modes**

- Sphere360 with mono content: plays via XREquirectLayer with `layout: 'mono'`.
- Cinema 2D with mono content: plays via XRQuadLayer.

- [ ] **Step 7: Append validation result to spec**

Edit [docs/superpowers/specs/2026-05-09-m5-webxr-media-layers.md](../specs/2026-05-09-m5-webxr-media-layers.md) and add at the very end:

```markdown
## 10. Validation result

Run on commit [HEAD SHA] on Quest 3 / Meta Browser [version] on [date]:

- §7-A: PASS / FAIL (specifics)
- §7-B: PASS / FAIL
- §7-C: PASS / FAIL
- §7-D: PASS / FAIL
- §7-E: PASS / FAIL
- §7-F: PASS / FAIL

[One paragraph summary; any open issues to log as followups.]
```

- [ ] **Step 8: Commit the validation result and ship**

```bash
git add docs/superpowers/specs/2026-05-09-m5-webxr-media-layers.md
git commit -m "$(cat <<'EOF'
docs(m5): record validation result

[summary line]

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

If all sections PASS: M5 is shipped. Move the V-flash followup entry from `docs/superpowers/followups.md` to a "completed" section (or delete it; the spec covers the resolution).

If any section FAILED: do NOT mark complete. Diagnose, write a fix, re-run validation. If the failure points to a deeper architectural issue, escalate to a new milestone — don't paper over it.

---

## Self-review checklist

- **Spec coverage:** Every spec §3 architecture point and §7 validation criterion maps to a task. Phase 0 covers §4.1; Phase 1 branches cover §4.2 / §4.3; Tasks 3–4 cover §3.1–§3.4; Task 5 covers §3.6; Task 6 covers §3.5; Task 8 covers §7.
- **No placeholders:** Every step has concrete code or concrete commands. The Phase 1 branch tasks (2A/2B/2C) describe the most-likely fix and explicitly say "if this doesn't work, escalate to Task 2C" — no TBDs.
- **Type / name consistency:** `m5SyncLayer` (window-global), `m5LayerManager` (IIFE name), `scene.dataset.geometry` (data attribute), `scene.dataset.stereo` (existing) used consistently across Tasks 3 + 4. `activeMediaLayer` / `suppressedGeomId` / `mediaBinding` / `refSpace` / `supported` are private to `m5LayerManager`.
- **Frequent commits:** Each task ends with a commit. Branched tasks (2A/2B/2C) commit per attempted fix. Phase 0 diagnostic + spec result are separate commits. Eight task-commits + Phase 0 spec commit + per-attempt commits in Task 2 = 10–14 commits.
- **YAGNI:** No subtitle layer (deferred to follow-ups). No fisheye-via-shader (out of scope). No bitrate-aware auto-quality (followup). No WebCodecs (non-goal).
- **Decision logic on user-facing branches:** Phase 0 → 2A/2B/2C is explicit. Tasks 3+ are common.
