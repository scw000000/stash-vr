# VR Audio-Sync + Black-Flash Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the dual-`<video>` architecture in `browse_scene.gohtml` with a single `<video>` element so the WebGL texture and the 2D player share one media pipeline. Eliminates audio-vs-picture drift in VR and the first-frame black flash on Enter VR.

**Architecture:** Delete the `<a-assets>` block and the `vrTex` `<video>` element. Bind `THREE.VideoTexture` to the existing on-page `sceneVideo` `<video>` element. Toggle `sceneVideo.muted` on the Enter-VR click handler (the click is a user gesture, autoplay policy permits unmute) and back to muted on `exit-vr`. The in-VR control panel buttons (play/pause, ±10s, exit) retarget to `sceneVideo`.

**Spec:** [docs/superpowers/specs/2026-05-08-m2-vr-sync-flash-fix-design.md](../specs/2026-05-08-m2-vr-sync-flash-fix-design.md)

**Tech Stack:** Go `html/template`, A-Frame 1.7.0 (already vendored at `internal/static/vendor/aframe.min.js`), Three.js (bundled inside A-Frame), inline JS. No build pipeline, no test suite (per [CLAUDE.md](../../../CLAUDE.md): `go vet` and `go build` are the only standard checks).

---

## File Structure

One file changed:

- **Modify:** `internal/static/browse_scene.gohtml`
  - Drop the `<a-assets>` block that declares `<video id="vrTex">`.
  - Drop the `const vrTex = ...` lookup in the inline JS.
  - Switch `applySphere` / `applyFlat` to bind the texture to the page's `sceneVideo` element.
  - Switch `vrAction` (in-VR panel button handler) play/pause/seek to operate on `sceneVideo`.
  - Drop `vrTex.currentTime`/`vrTex.play()`/`vrTex.pause()` from `hide2D` / `show2D`.
  - Add `video.muted = false` to the Enter-VR click handler.
  - Add `video.muted = true` to the `exit-vr` event handler.

No Go file changes. No new files. No deleted files. No new vendored assets. No new env vars.

## Pre-flight

- [ ] **Step 0a: Confirm working directory and branch**

Run: `git status` and `git rev-parse --abbrev-ref HEAD`

Expected: Working directory `c:\dev\stash-vr`, branch `master` (or any feature branch — irrelevant to the fix), tree clean. If dirty, stash or commit unrelated work before starting.

- [ ] **Step 0b: Capture the existing VR markup as a sanity baseline**

Run: `git log -1 --oneline -- internal/static/browse_scene.gohtml`

Expected: Most recent commit touching this file is `51f3757 browse: add in-VR playback control panel (play/pause, seek, exit)`. If a newer commit appears, re-read the file before editing — the line numbers and exact strings in this plan assume that commit is the current head of `browse_scene.gohtml`.

---

## Task 1: Collapse to a single `<video>` element

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

This task is one logical change — the template and the JS are tightly coupled (the JS references DOM IDs from the template). Splitting it across multiple commits would leave intermediate broken states (template removes `vrTex` while JS still references it, or vice versa). All edits in one commit.

- [ ] **Step 1: Remove the `<a-assets>` block and `vrTex` `<video>` from the template**

Use the Edit tool. Match the exact existing text (the `<a-assets>` block sits between the `<a-scene>` opening tag and the `<a-entity camera>`):

```
old_string:
<a-scene id="vrScene" style="display:none" vr-mode-ui="enabled: true" loading-screen="enabled: false">
  <a-assets timeout="5000">
    <video id="vrTex" src="{{.DirectStreamURL}}" preload="auto" muted playsinline loop></video>
  </a-assets>
  <a-entity camera position="0 1.6 0"></a-entity>

new_string:
<a-scene id="vrScene" style="display:none" vr-mode-ui="enabled: true" loading-screen="enabled: false">
  <a-entity camera position="0 1.6 0"></a-entity>
```

After this edit, the `<a-scene>` no longer declares any asset video. The texture source for the VR sphere/plane will come from the on-page `sceneVideo` element (bound in JS in Step 2).

- [ ] **Step 2: Replace the entire inline `<script>` IIFE with the single-video version**

The IIFE currently spans lines 108–250 of `browse_scene.gohtml` (immediately after `<script src="/vendor/aframe.min.js"></script>`). Use Edit to replace it.

```
old_string:
<script>
  (function() {
    const btn   = document.getElementById('enterVR');
    const scene = document.getElementById('vrScene');
    const wrap  = document.querySelector('.wrap');
    const video = document.getElementById('sceneVideo');
    const vrTex = document.getElementById('vrTex');
    if (!btn || !scene || !wrap) return;

    // Bind material + texture programmatically. aframe-stereo-component@1.4.0
    // doesn't work on A-Frame 1.7 (reads material as raw string at init), so
    // stereo is handled here directly.
    //
    // Strategy: ONE half-sphere with the full SBS-encoded texture. WebXR
    // renders the scene twice per frame (once per eye), and Three.js calls
    // mesh.onBeforeRender per render call with the active camera. We swap
    // tex.offset/repeat per eye so the left eye samples the left half and
    // the right eye samples the right half of the SBS texture.
    //
    // The half-sphere uses phiStart:180, phiLength:180 so it natively faces
    // -Z (camera forward) with U increasing left-to-right and V increasing
    // top-to-bottom — texture orientation matches the user's view, no
    // rotation needed.
    function applySphere() {
      const el = document.getElementById('vrSphere');
      if (!el || !vrTex || !window.AFRAME || !AFRAME.THREE) return;
      const mesh = el.getObject3D('mesh');
      if (!mesh || mesh.userData.boundVR) return;
      const tex = new AFRAME.THREE.VideoTexture(vrTex);
      if (AFRAME.THREE.SRGBColorSpace) tex.colorSpace = AFRAME.THREE.SRGBColorSpace;
      mesh.material = new AFRAME.THREE.MeshBasicMaterial({
        map: tex,
        side: AFRAME.THREE.BackSide
      });
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
      mesh.userData.boundVR = true;
    }
    function applyFlat() {
      const el = document.getElementById('vrFlat');
      if (!el || !vrTex || !window.AFRAME || !AFRAME.THREE) return;
      const mesh = el.getObject3D('mesh');
      if (!mesh || mesh.userData.boundVR) return;
      const tex = new AFRAME.THREE.VideoTexture(vrTex);
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

    // In-VR control panel: raycast clicks fire on .vr-btn entities (via
    // A-Frame's laser-controls + cursor pipeline). Each button has a
    // data-action attribute that maps to a vrTex playback verb.
    function vrAction(action) {
      if (action === 'playpause') {
        if (vrTex.paused) {
          const p = vrTex.play();
          if (p && p.catch) p.catch(err => console.warn('stash-vr: vrTex play failed', err));
        } else {
          vrTex.pause();
        }
      } else if (action === 'seek-back') {
        if (vrTex && !isNaN(vrTex.currentTime)) {
          vrTex.currentTime = Math.max(0, vrTex.currentTime - 10);
        }
      } else if (action === 'seek-fwd') {
        if (vrTex && !isNaN(vrTex.currentTime) && !isNaN(vrTex.duration) && vrTex.duration > 0) {
          vrTex.currentTime = Math.min(vrTex.duration - 0.1, vrTex.currentTime + 10);
        }
      } else if (action === 'exit') {
        try { scene.exitVR(); } catch (e) { console.warn('stash-vr: exitVR failed', e); }
      }
    }
    document.querySelectorAll('.vr-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        vrAction(btn.dataset.action || btn.getAttribute('data-action'));
      });
    });

    function show2D() {
      scene.style.display = 'none';
      [...wrap.children].forEach(el => {
        if (el !== scene) el.style.display = '';
      });
      // Stop the silent VR texture video on exit.
      if (vrTex) {
        try { vrTex.pause(); } catch (e) {}
      }
    }
    function hide2D() {
      applyAll();
      [...wrap.children].forEach(el => {
        if (el !== scene && el !== video) el.style.display = 'none';
      });
      scene.style.display = '';
      // Sync VR texture video to the 2D player's position and start it.
      // Keep sceneVideo playing in the background — it provides audio while
      // vrTex (muted) feeds the WebGL texture.
      if (video && vrTex) {
        try { vrTex.currentTime = video.currentTime; } catch (e) {}
        const p = vrTex.play();
        if (p && p.catch) p.catch(err => console.warn('stash-vr: vrTex.play failed', err));
      }
    }

    btn.addEventListener('click', () => {
      hide2D();
      if (typeof scene.enterVR !== 'function') {
        console.warn('stash-vr: a-scene not ready');
        show2D();
        return;
      }
      scene.enterVR().catch(err => {
        console.warn('stash-vr: enterVR failed', err);
        show2D();
      });
    });
    scene.addEventListener('exit-vr', show2D);
  })();
</script>

new_string:
<script>
  (function() {
    const btn   = document.getElementById('enterVR');
    const scene = document.getElementById('vrScene');
    const wrap  = document.querySelector('.wrap');
    const video = document.getElementById('sceneVideo');
    if (!btn || !scene || !wrap || !video) return;

    // Bind material + texture programmatically. aframe-stereo-component@1.4.0
    // doesn't work on A-Frame 1.7 (reads material as raw string at init), so
    // stereo is handled here directly.
    //
    // Single-video architecture: sceneVideo (the on-page <video>) is the
    // sole media element. THREE.VideoTexture reads frames from it; the
    // element also produces audio. One pipeline = no drift between audio
    // and picture, and the texture is non-empty on Enter VR because
    // sceneVideo has been autoplaying (muted) since page load.
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
      mesh.userData.boundVR = true;
    }
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

    // In-VR control panel: raycast clicks fire on .vr-btn entities (via
    // A-Frame's laser-controls + cursor pipeline). Each button has a
    // data-action attribute that maps to a sceneVideo playback verb.
    function vrAction(action) {
      if (action === 'playpause') {
        if (video.paused) {
          const p = video.play();
          if (p && p.catch) p.catch(err => console.warn('stash-vr: video play failed', err));
        } else {
          video.pause();
        }
      } else if (action === 'seek-back') {
        if (!isNaN(video.currentTime)) {
          video.currentTime = Math.max(0, video.currentTime - 10);
        }
      } else if (action === 'seek-fwd') {
        if (!isNaN(video.currentTime) && !isNaN(video.duration) && video.duration > 0) {
          video.currentTime = Math.min(video.duration - 0.1, video.currentTime + 10);
        }
      } else if (action === 'exit') {
        try { scene.exitVR(); } catch (e) { console.warn('stash-vr: exitVR failed', e); }
      }
    }
    document.querySelectorAll('.vr-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        vrAction(btn.dataset.action || btn.getAttribute('data-action'));
      });
    });

    function show2D() {
      scene.style.display = 'none';
      [...wrap.children].forEach(el => {
        if (el !== scene) el.style.display = '';
      });
      // Restore the page's initial muted state so re-entering VR or
      // scrolling around doesn't surprise the user with audible playback
      // from the 2D <video>.
      video.muted = true;
    }
    function hide2D() {
      applyAll();
      [...wrap.children].forEach(el => {
        if (el !== scene && el !== video) el.style.display = 'none';
      });
      scene.style.display = '';
    }

    btn.addEventListener('click', () => {
      hide2D();
      // The click is a user gesture — autoplay policy permits unmuting.
      // sceneVideo continues from its current position; do not touch
      // currentTime here (would cause a re-seek and possibly a black
      // frame).
      video.muted = false;
      if (typeof scene.enterVR !== 'function') {
        console.warn('stash-vr: a-scene not ready');
        show2D();
        return;
      }
      scene.enterVR().catch(err => {
        console.warn('stash-vr: enterVR failed', err);
        show2D();
      });
    });
    scene.addEventListener('exit-vr', show2D);
  })();
</script>
```

After this edit:
- `vrTex` is no longer referenced anywhere in the file.
- `applySphere` / `applyFlat` build the texture from `video` (`sceneVideo`).
- `vrAction` operates on `video` for play/pause/seek.
- `hide2D` no longer seeks/plays a second element.
- `show2D` re-mutes the page video (no `vrTex.pause()` to call any more).
- The Enter-VR click handler unmutes `video` between `hide2D()` and `scene.enterVR()`.

- [ ] **Step 3: Build verify**

Run from the repo root:

```
go vet ./...
go build ./...
```

Expected: both commands print nothing and exit 0. The change is template/JS only, so a Go regression here would mean an unrelated environment issue — investigate before continuing.

- [ ] **Step 4: Curl-level verify (server running)**

Start the server in one shell:

```
$env:STASH_GRAPHQL_URL = "http://localhost:9999/graphql"
go run ./cmd/stash-vr
```

(Substitute the user's real Stash URL/API key as already configured. If a `.env` or wrapper script is in use, source it instead.)

In a second shell, pick a known DOME+SBS scene id and confirm the served HTML:

```
curl -s http://localhost:9666/browse/scene/<DOME-SBS-id> | findstr /C:"id=\"sceneVideo\""
curl -s http://localhost:9666/browse/scene/<DOME-SBS-id> | findstr /C:"<a-scene"
curl -s http://localhost:9666/browse/scene/<DOME-SBS-id> | findstr /C:"id=\"enterVR\""
curl -s http://localhost:9666/browse/scene/<DOME-SBS-id> | findstr /C:"id=\"vrTex\""
curl -s http://localhost:9666/browse/scene/<DOME-SBS-id> | findstr /C:"<a-assets"
```

Expected:
- First three commands each print exactly one matching line.
- The last two commands print nothing and exit with `findstr` exit code 1 (i.e. `<a-assets>` and `id="vrTex"` are gone from the rendered HTML).

If any of these expectations fail, re-open `internal/static/browse_scene.gohtml` and verify the edits in Steps 1–2 actually applied — there should be no `vrTex` and no `<a-assets>` in the file.

Stop the server with Ctrl-C before continuing.

- [ ] **Step 5: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: collapse VR to single <video> to fix audio sync + black flash

Drop the dual-video architecture (sceneVideo + vrTex). One <video>
element now drives both the on-page 2D player and the WebGL texture, so
audio cannot drift against picture and the texture is populated on
Enter VR (sceneVideo has been autoplaying since page load). The Enter VR
click is a user gesture, so we unmute the video then; we re-mute on
exit-vr to restore the page's initial state."
```

(Pre-commit hook will append the `Co-Authored-By` trailer.)

---

## Task 2: On-headset validation on Quest 3 / Meta Browser

The actual fix verification is manual. Run after Task 1 is committed and the binary is restarted.

**Files:**
- Create: `docs/superpowers/research/2026-05-08-m2-sync-flash-result/result.md`

- [ ] **Step 1: Open a known DOME+SBS scene on Quest 3**

Open `https://stash-vr.duckdns.org/browse` in Quest's Meta Browser. Navigate to a scene tagged `DOME` + `SBS` (the same scene that was used for M2 validation). Wait for the 2D player to start its silent autoplay — confirm a thumbnail-sized preview is rendering (proves frames are decoding).

- [ ] **Step 2: Enter VR and verify audio + position parity**

Scrub the 2D player to a non-zero position (e.g., 30 seconds in) so we can tell the VR view didn't reset. Click "Enter VR".

Verify, on-headset:
- VR enters at the same playback position the 2D player was at (not 0).
- Audio plays from that position, audible, in sync with the picture (lip-sync test if there's dialogue, or pick a clip with sharp foley cues).
- No visible black flash on entry. The videosphere is textured immediately. (If there's still a one-frame clear-color flash, note it — it's an A-Frame compositor artifact, not the dual-video flash. Mitigation in spec §5.)

- [ ] **Step 3: Long-soak sync test**

Stay in VR, watching, for 5+ minutes without interaction. Listen for audio drift against picture. The original symptom was "noticeable after a couple of minutes" — 5 minutes is a generous margin.

Verify: audio remains tight against picture. No drift.

- [ ] **Step 4: In-VR control panel test**

While in VR, raycast onto each button on the in-VR control panel:
- Play/Pause: toggles playback state. Single tap pauses, single tap resumes. Audio mutes/unmutes accordingly.
- −10s: jumps back 10 seconds. Audio re-syncs to new position.
- +10s: jumps forward 10 seconds. Audio re-syncs.
- Exit VR: leaves immersive-vr, returns to 2D layout.

- [ ] **Step 5: Exit / re-enter test**

After Exit VR:
- 2D player is visible again, paused or playing at the position the user left VR at.
- 2D player is muted (the `show2D()` re-mute applied).
- Page mutation forms (rating, favorite, tags, O-counter, organized) still render and still work — click one to verify (rate the scene one star, then back to current rating).

Click Enter VR again. Verify:
- Sync is still tight from frame 1.
- No black flash.
- Audio plays.

- [ ] **Step 6: Negative test — non-VR scene unaffected**

Open a scene that is NOT tagged DOME+SBS. Verify:
- 2D player works.
- "Enter VR" button is rendered (it's gated only by `DirectStreamURL`, not by the VR-mode tags — same as before this change).
- Page source still does not contain `id="vrTex"` (the entire IIFE removed the reference; this is a sanity check).

- [ ] **Step 7: Write the result artifact**

Create `docs/superpowers/research/2026-05-08-m2-sync-flash-result/result.md` with one short section per Step (2–6), each marked PASS / FAIL with one or two sentences of detail. Format mirrors `docs/superpowers/research/2026-05-08-m2-webxr-result/` from M2.

If any step FAILed, do NOT close out the plan. Re-open the spec, identify whether the failure is in scope (a bug in this change) or a known risk (occluded-video throttling, A-Frame compositor flash), and decide whether to patch or document and ship. Update the spec's §5 risks accordingly.

- [ ] **Step 8: Commit the result artifact**

```
git add docs/superpowers/research/2026-05-08-m2-sync-flash-result/result.md
git commit -m "browse: VR sync/flash fix on-headset validation result"
```

---

## Self-review against spec

**Spec coverage:**
- Spec §3 fix #1 (drop `<a-assets>` + vrTex): Task 1 Step 1.
- Spec §3 fix #2 (`THREE.VideoTexture(sceneVideo)`): Task 1 Step 2 — `applySphere` and `applyFlat` updated.
- Spec §3 fix #3 (`video.muted = false` on Enter VR click): Task 1 Step 2 — click handler.
- Spec §3 fix #4 (`video.muted = true` on `exit-vr`): Task 1 Step 2 — `show2D`.
- Spec §3 fix #5 (`hide2D` / `show2D` drop vrTex): Task 1 Step 2 — both functions simplified.
- Spec §3 fix #6 (`vrAction` targets sceneVideo): Task 1 Step 2 — all four branches.
- Spec §6 file table (only `browse_scene.gohtml`): matches — no other files touched.
- Spec §8 build-level (`go vet` + `go build`): Task 1 Step 3.
- Spec §8 curl-level (`id="enterVR"`, `<a-scene`, `id="sceneVideo"` present; `<a-assets>` and `id="vrTex"` absent): Task 1 Step 4.
- Spec §8 Quest 3 validation (entry sync, 5+ min drift, in-VR panel, exit/re-enter, non-VR scene): Task 2 Steps 2–6.
- Spec §8 validation artifact: Task 2 Steps 7–8.

No spec gaps.

**Type / API consistency:** `video` refers to the same `HTMLVideoElement` (the on-page `sceneVideo`) in every function (`applySphere`, `applyFlat`, `vrAction`, `hide2D`, `show2D`, click handler). `vrTex` is removed in every place it appeared (template `<video>`, `const vrTex = ...`, both `apply*` functions, `vrAction`'s four branches, `hide2D`'s sync, `show2D`'s pause). `applyAll`, `vrAction`, `hide2D`, `show2D` signatures unchanged. Event names (`loaded`, `object3dset`, `exit-vr`, `click`) unchanged.

**Placeholder scan:** No "TBD", no "TODO", no "implement later", no "handle edge cases", no "similar to Task N". Every code step shows the exact code. Every command shows the exact invocation.
