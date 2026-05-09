# M3c SKYBOX-Style Controller Mappings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SKYBOX-style controller mappings to stash-vr's WebXR player so the user can drive playback, panel toggling, screen drag/scale, and recenter from controllers without raycast clicks. Per [the M3c spec](../specs/2026-05-08-m3c-skybox-controller-mappings.md).

**Architecture:** A new A-Frame component `m3c-controls` (in a new JS file `internal/static/m3c-controls.js`) handles all controller event input — trigger state machine, B/Y long-press, gamepad polling, drag-pose capture — and emits semantic events on `<a-scene>`. The existing inline IIFE in `internal/static/browse_scene.gohtml` listens for these events and applies them to DOM/video state. The playback panel + Format picker + a new help cheatsheet are wrapped under a single `<a-entity id="vrControlsRoot">` with `visible="false"` by default; both `<a-entity laser-controls>` are toggled in lockstep.

**Tech Stack:** A-Frame 1.7 (already vendored at `internal/static/vendor/aframe.min.js`), WebXR Gamepad API, Three.js (via `AFRAME.THREE`), Go `embed.FS` for static serving, `chi` router with `http.FileServerFS` catch-all.

**Test strategy:** No Go-side logic changes worth unit-testing in this milestone. Validation is build+manual: `go vet ./...` and `go build ./...` clean per commit, plus curl-level smoke checks where applicable, plus on-headset Quest 3 validation in Task 12. Browser-console smoke (firing fake events from devtools) is a quick way to verify the state machine without a headset.

**Conventions:**

- Local Windows build: `scripts\build-windows.bat` (per [memory](../../../C:/Users/scw00/.claude/projects/c--dev-stash-vr/memory/reference_build_script.md)). Don't use raw `go build` for local Windows binaries.
- Run server locally: `STASH_GRAPHQL_URL=http://localhost:9999/graphql ./stash-vr` (or the Windows .exe). Port defaults to 9666.
- Existing M3b functionality must keep working at every commit — never break the Format picker, raycast clicks, audio sync, or auto-detect.

---

### Task 1: Embed root `*.js` so new JS files are served

**Files:**
- Modify: `internal/static/static.go`

- [ ] **Step 1: Update the `//go:embed` directive**

The directive currently embeds gohtml, html, png, and `vendor/*.js` only. Add `*.js` so root-level JS files (next to `static.go`) are also embedded.

Change [internal/static/static.go:5](../../../internal/static/static.go#L5) from:

```go
//go:embed *.gohtml *.html *.png vendor/*.js
var Fs embed.FS
```

to:

```go
//go:embed *.gohtml *.html *.png *.js vendor/*.js
var Fs embed.FS
```

- [ ] **Step 2: Build to verify the directive parses**

Run from repo root:

```powershell
scripts\build-windows.bat --skip-dirty
```

Expected: build succeeds. (The directive now requires at least one matching `*.js` at root; we add that in Task 2 — for now `embed` may complain if no files match. If it errors with `pattern *.js: no matching files found`, hold the embed change until Task 2 has the stub file in place. Easiest path: do Tasks 1 and 2 as a single commit. See Task 2 Step 5.)

- [ ] **Step 3: Commit (deferred — bundle with Task 2)**

Don't commit yet. The embed glob and the stub `m3c-controls.js` go in one commit.

---

### Task 2: Create `m3c-controls.js` stub and reference it from the template

**Files:**
- Create: `internal/static/m3c-controls.js`
- Modify: `internal/static/browse_scene.gohtml` (add `<script src="/m3c-controls.js">` and `<a-entity m3c-controls>`)

- [ ] **Step 1: Create the empty A-Frame component**

Create `internal/static/m3c-controls.js` with:

```js
/* M3c controller mappings — see docs/superpowers/specs/2026-05-08-m3c-skybox-controller-mappings.md
 *
 * This component listens for controller input on <a-scene> and emits
 * semantic events (m3c:panel-toggle, m3c:play-pause, m3c:seek, m3c:scale,
 * m3c:drag-start/move/end, m3c:reset-short, m3c:reset-long). The inline
 * IIFE in browse_scene.gohtml owns DOM/video state mutation in response.
 */
(function() {
  if (typeof AFRAME === 'undefined') return;

  AFRAME.registerComponent('m3c-controls', {
    init: function() {
      // Filled in by later tasks. For now just confirm registration.
      console.log('m3c-controls: init');
    },
    tick: function(time, delta) {
      // Filled in by later tasks (gamepad polling).
    },
    remove: function() {
      // Filled in by later tasks (event-listener teardown).
    }
  });
})();
```

- [ ] **Step 2: Reference the script from the template**

In [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml), find the line:

```html
<script src="/vendor/aframe.min.js"></script>
```

(currently line 184). Add a new line after it:

```html
<script src="/vendor/aframe.min.js"></script>
<script src="/m3c-controls.js"></script>
```

Order matters — `AFRAME` must exist before our component registers.

- [ ] **Step 3: Add the host entity to `<a-scene>`**

In the same template, find the closing `</a-scene>` (currently line 182). Just before it (after the two `<a-entity laser-controls>` lines), insert:

```html
  <!-- M3c controller-mapping component. Hosts the trigger state machine,
       gamepad polling, and B/Y long-press timer. Emits semantic events
       on <a-scene>; the inline IIFE applies them. -->
  <a-entity m3c-controls></a-entity>
```

- [ ] **Step 4: Build and smoke-test serving**

Build:

```powershell
scripts\build-windows.bat --skip-dirty
```

Expected: build succeeds (the embed glob now has matching files).

Run the binary, then in a separate shell:

```powershell
curl http://localhost:9666/m3c-controls.js
```

Expected: 200 response with the stub JS content. Loading `/browse/scene/{any-id}` in a browser and checking the console should show `m3c-controls: init` once A-Frame loads.

- [ ] **Step 5: Commit Tasks 1+2 together**

```powershell
git add internal/static/static.go internal/static/m3c-controls.js internal/static/browse_scene.gohtml
git commit -m "m3c: embed *.js and add m3c-controls component skeleton"
```

---

### Task 3: Wrap controls under `vrControlsRoot`, hide by default, tie laser visibility

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

The playback panel (`vrControls`), Format picker (`vrFormatPicker`), and the help panel (added in Task 11) need to share visibility. Wrap them in a parent entity that we toggle as a unit, and tie the laser-controls' `visible` and `raycaster.enabled` to that same state.

- [ ] **Step 1: Add the wrapper entity**

In `browse_scene.gohtml`, find:

```html
  <!-- In-VR playback controls. Positioned below eye-line, tilted toward user. -->
  <a-entity id="vrControls" position="0 0.4 -1.5" rotation="-30 0 0">
```

(currently line 86–87). Replace with:

```html
  <!-- All in-VR control surfaces (playback panel, Format picker, help panel)
       live under this wrapper so M3c can toggle them as a unit. Hidden by
       default; trigger summons. -->
  <a-entity id="vrControlsRoot" visible="false">

  <!-- In-VR playback controls. Positioned below eye-line, tilted toward user. -->
  <a-entity id="vrControls" position="0 0.4 -1.5" rotation="-30 0 0">
```

Then find the closing `</a-entity>` of the Format picker — after the Auto button. Currently line 177:

```html
    </a-entity>
  </a-entity>

  <!-- Laser controllers for raycast-clicking the panel buttons. -->
```

Insert a new closing `</a-entity>` for `vrControlsRoot` between the picker close and the laser-controls comment:

```html
    </a-entity>
  </a-entity>

  </a-entity><!-- /vrControlsRoot -->

  <!-- Laser controllers for raycast-clicking the panel buttons. -->
```

- [ ] **Step 2: Set lasers initially hidden + raycaster disabled**

Find:

```html
  <a-entity laser-controls="hand: right" raycaster="objects: .vr-btn; far: 5"></a-entity>
  <a-entity laser-controls="hand: left"  raycaster="objects: .vr-btn; far: 5"></a-entity>
```

Replace with:

```html
  <a-entity id="vrLaserRight" laser-controls="hand: right" raycaster="objects: .vr-btn; far: 5; enabled: false" visible="false"></a-entity>
  <a-entity id="vrLaserLeft"  laser-controls="hand: left"  raycaster="objects: .vr-btn; far: 5; enabled: false" visible="false"></a-entity>
```

- [ ] **Step 3: Add `togglePanelAndLasers` IIFE helper**

In the inline `<script>` block, find the `vrAction` function (currently line 372). Above it, insert:

```js
    // Panel + lasers visibility is shared. M3c hides them by default and
    // toggles via m3c:panel-toggle (or programmatically on Enter VR /
    // Format-button taps). When hidden: lasers' raycaster also disabled
    // so stray controller poses don't fire phantom button clicks.
    function setPanelVisible(visible) {
      const root = document.getElementById('vrControlsRoot');
      const lr   = document.getElementById('vrLaserRight');
      const ll   = document.getElementById('vrLaserLeft');
      if (root) root.setAttribute('visible', visible ? 'true' : 'false');
      [lr, ll].forEach(el => {
        if (!el) return;
        el.setAttribute('visible', visible ? 'true' : 'false');
        const rc = el.getAttribute('raycaster') || {};
        // raycaster is a component; pass an object update.
        el.setAttribute('raycaster', Object.assign({}, rc, { enabled: !!visible }));
      });
    }
    function isPanelVisible() {
      const root = document.getElementById('vrControlsRoot');
      if (!root) return false;
      const v = root.getAttribute('visible');
      // A-Frame returns boolean true/false after parse; before parse it's a string.
      return v === true || v === 'true';
    }
```

- [ ] **Step 4: Make Format button a child-toggle (still works inside panel)**

The Format button currently toggles `vrFormatPicker.visible`. That continues to work — the picker is still inside `vrControlsRoot` and its own visibility is independent of the parent. No code change needed here. (The picker's `visible="false"` initial stays — it's hidden inside a hidden parent.)

- [ ] **Step 5: Build and smoke-test**

```powershell
scripts\build-windows.bat --skip-dirty
```

Run, then load `/browse/scene/<id>` in a browser. Inspect the DOM: verify `vrControlsRoot` exists with `visible="false"`, both lasers have `visible="false"` and the raycaster-enabled set false.

In the browser console:

```js
document.getElementById('vrControlsRoot').getAttribute('visible')
// => false
```

- [ ] **Step 6: Commit**

```powershell
git add internal/static/browse_scene.gohtml
git commit -m "m3c: wrap controls in vrControlsRoot, hidden by default, tie laser visibility"
```

---

### Task 4: Trigger state machine — drag branch

**Files:**
- Modify: `internal/static/m3c-controls.js`
- Modify: `internal/static/browse_scene.gohtml` (add `m3c:drag-*` handlers + `activeGeometry` helper)

Spec §4.1: on `triggerdown`, capture controller pose `P_c0` and time `t0`. While candidate is active, per tick: if pose-delta > 5 cm OR duration > 250 ms → it's a drag. Emit `m3c:drag-start`, then `m3c:drag-move` per tick with delta from `P_c0`. End on `triggerup` with `m3c:drag-end`.

- [ ] **Step 1: Implement the trigger pose-tracking and drag emission in m3c-controls.js**

Replace the empty `init`/`tick` body with:

```js
    init: function() {
      this.sceneEl = this.el.sceneEl || this.el;
      this.tmpVec = new AFRAME.THREE.Vector3();
      this.startPos = new AFRAME.THREE.Vector3();
      this.curPos = new AFRAME.THREE.Vector3();
      this.deltaPos = new AFRAME.THREE.Vector3();

      // Per-hand trigger state. handId ∈ {'right','left'}.
      // phase ∈ {'idle','candidate','drag'}.
      this.triggerState = {
        right: this._newTriggerState(),
        left:  this._newTriggerState()
      };

      // Drag thresholds (spec §4.1).
      this.DRAG_DIST_M = 0.05; // 5 cm
      this.DRAG_HOLD_MS = 250;

      this._onTriggerDown = this._onTriggerDown.bind(this);
      this._onTriggerUp   = this._onTriggerUp.bind(this);

      // Find both laser-controls entities (they emit triggerdown/up).
      this._lasers = {
        right: document.getElementById('vrLaserRight'),
        left:  document.getElementById('vrLaserLeft')
      };
      Object.keys(this._lasers).forEach(hand => {
        const el = this._lasers[hand];
        if (!el) return;
        el.addEventListener('triggerdown', e => this._onTriggerDown(hand, e));
        el.addEventListener('triggerup',   e => this._onTriggerUp(hand, e));
      });
    },

    _newTriggerState: function() {
      return {
        phase: 'idle',
        downTime: 0,
        startPos: new AFRAME.THREE.Vector3(),
        hadIntersection: false
      };
    },

    _getControllerPos: function(hand, out) {
      const el = this._lasers[hand];
      if (!el || !el.object3D) { out.set(0, 0, 0); return false; }
      el.object3D.getWorldPosition(out);
      return true;
    },

    _hasRaycastIntersection: function(hand) {
      const el = this._lasers[hand];
      if (!el) return false;
      const rc = el.components && el.components.raycaster;
      if (!rc) return false;
      const its = rc.intersections || [];
      return its.length > 0;
    },

    _onTriggerDown: function(hand) {
      const st = this.triggerState[hand];
      st.phase = 'candidate';
      st.downTime = performance.now();
      this._getControllerPos(hand, st.startPos);
      st.hadIntersection = this._hasRaycastIntersection(hand);
    },

    _onTriggerUp: function(hand) {
      const st = this.triggerState[hand];
      if (st.phase === 'drag') {
        this.sceneEl.emit('m3c:drag-end', { hand: hand });
      }
      // 'candidate' resolution is handled by Task 5; for now just reset.
      st.phase = 'idle';
    },

    tick: function(time, delta) {
      // Promote candidate → drag if either threshold crosses.
      ['right', 'left'].forEach(hand => {
        const st = this.triggerState[hand];
        if (st.phase === 'candidate') {
          const elapsed = performance.now() - st.downTime;
          this._getControllerPos(hand, this.curPos);
          this.deltaPos.subVectors(this.curPos, st.startPos);
          const dist = this.deltaPos.length();
          if (dist > this.DRAG_DIST_M || elapsed > this.DRAG_HOLD_MS) {
            st.phase = 'drag';
            this.sceneEl.emit('m3c:drag-start', { hand: hand });
          }
        }
        if (st.phase === 'drag') {
          this._getControllerPos(hand, this.curPos);
          this.deltaPos.subVectors(this.curPos, st.startPos);
          this.sceneEl.emit('m3c:drag-move', {
            hand: hand,
            dx: this.deltaPos.x,
            dy: this.deltaPos.y,
            dz: this.deltaPos.z
          });
          // Reset startPos to curPos so next tick's delta is incremental,
          // not cumulative. This makes the geometry follow the controller
          // 1:1 instead of accelerating.
          st.startPos.copy(this.curPos);
        }
      });
    },

    remove: function() {
      // Listeners attached to laser-controls entities are torn down by
      // their entity removal; nothing global to clean.
    }
```

- [ ] **Step 2: Add `activeGeometry` helper + drag handlers in IIFE**

In `browse_scene.gohtml`'s inline IIFE, find the `applyAll` definition. Below it (and below the `applyAll` event-listener wiring), add:

```js
    // Resolve the currently-rendering geometry entity. M3b ensures
    // exactly one of the four has visible="true" at any time.
    function activeGeometry() {
      const ids = ['vrSphere180', 'vrSphere360', 'vrFisheye', 'vrFlat'];
      for (let i = 0; i < ids.length; i++) {
        const el = document.getElementById(ids[i]);
        if (!el) continue;
        const v = el.getAttribute('visible');
        if (v === true || v === 'true') return el;
      }
      return null;
    }

    // M3c drag — translate the active geometry by per-tick controller
    // delta. Cinema plane re-aims at user after each translate.
    scene.addEventListener('m3c:drag-move', function(e) {
      const el = activeGeometry();
      if (!el || !el.object3D) return;
      el.object3D.position.x += e.detail.dx;
      el.object3D.position.y += e.detail.dy;
      el.object3D.position.z += e.detail.dz;
      if (el.id === 'vrFlat') {
        const cam = scene.camera;
        if (cam) el.object3D.lookAt(cam.position);
      }
    });
```

(`m3c:drag-start` and `m3c:drag-end` don't need IIFE handlers in the simplest form — start is just informational, end means the next `drag-move` won't come. If a future task needs to capture state on start/end, add then.)

- [ ] **Step 3: Build and smoke-test**

```powershell
scripts\build-windows.bat --skip-dirty
```

Run, load a scene page in browser. In console:

```js
// Fake a triggerdown/triggerup on the right laser to see candidate→drag promotion.
const lr = document.getElementById('vrLaserRight');
lr.dispatchEvent(new CustomEvent('triggerdown'));
// Wait >250ms, watch tick logs:
setTimeout(() => lr.dispatchEvent(new CustomEvent('triggerup')), 400);
// Listen for emitted drag events:
document.querySelector('a-scene').addEventListener('m3c:drag-start', e => console.log('drag-start', e.detail));
document.querySelector('a-scene').addEventListener('m3c:drag-move',  e => console.log('drag-move',  e.detail));
document.querySelector('a-scene').addEventListener('m3c:drag-end',   e => console.log('drag-end',   e.detail));
```

Expected: after the 250 ms timer, `m3c:drag-start` fires once. `m3c:drag-move` may fire on subsequent ticks (with all-zero delta since the laser entity isn't moving in browser). `m3c:drag-end` fires on `triggerup`. (Actual translation only matters on Quest 3 where the laser entity tracks a real controller.)

- [ ] **Step 4: Commit**

```powershell
git add internal/static/m3c-controls.js internal/static/browse_scene.gohtml
git commit -m "m3c: trigger state machine — drag branch"
```

---

### Task 5: Trigger state machine — single-click vs double-click vs button-click

**Files:**
- Modify: `internal/static/m3c-controls.js`
- Modify: `internal/static/browse_scene.gohtml` (add `m3c:panel-toggle` and `m3c:play-pause` handlers)

Spec §4.1: on `triggerup` before drag thresholds cross, the click is a candidate. If the original `triggerdown` had a raycaster intersection → A-Frame's existing click pipeline fires the button (do nothing extra). Otherwise wait 300 ms: a second `triggerdown` within 300 ms → double-click (`m3c:play-pause`); else → single-click (`m3c:panel-toggle`).

- [ ] **Step 1: Extend the trigger-up handler with click classification**

In `m3c-controls.js`, replace the existing `_onTriggerUp` body:

```js
    _onTriggerUp: function(hand) {
      const st = this.triggerState[hand];
      if (st.phase === 'drag') {
        this.sceneEl.emit('m3c:drag-end', { hand: hand });
        st.phase = 'idle';
        return;
      }
      if (st.phase !== 'candidate') {
        st.phase = 'idle';
        return;
      }
      // Candidate: triggerup before drag thresholds.
      // If it had a raycast intersection, A-Frame's click pipeline
      // already fires the button — do nothing extra.
      if (st.hadIntersection) {
        st.phase = 'idle';
        return;
      }
      // Otherwise: it's a click candidate. Defer for double-click window.
      const now = performance.now();
      if (this._pendingClick && (now - this._pendingClick.time) <= this.DOUBLE_CLICK_MS) {
        // Second click within window → double-click.
        clearTimeout(this._pendingClick.timer);
        this._pendingClick = null;
        this.sceneEl.emit('m3c:play-pause');
      } else {
        // Start a new pending; resolve as single-click after window.
        if (this._pendingClick) clearTimeout(this._pendingClick.timer);
        const pending = { time: now, timer: 0 };
        pending.timer = setTimeout(() => {
          if (this._pendingClick === pending) {
            this._pendingClick = null;
            this.sceneEl.emit('m3c:panel-toggle');
          }
        }, this.DOUBLE_CLICK_MS);
        this._pendingClick = pending;
      }
      st.phase = 'idle';
    },
```

In `init`, add the `DOUBLE_CLICK_MS` constant after `DRAG_HOLD_MS`:

```js
      this.DRAG_HOLD_MS = 250;
      this.DOUBLE_CLICK_MS = 300;
      this._pendingClick = null;
```

- [ ] **Step 2: Add IIFE handlers for panel-toggle and play-pause**

In `browse_scene.gohtml`'s inline IIFE, after the `m3c:drag-move` listener added in Task 4, add:

```js
    scene.addEventListener('m3c:panel-toggle', function() {
      setPanelVisible(!isPanelVisible());
    });
    scene.addEventListener('m3c:play-pause', function() {
      if (video.paused) {
        const p = video.play();
        if (p && p.catch) p.catch(err => console.warn('stash-vr: video play failed', err));
      } else {
        video.pause();
      }
    });
```

- [ ] **Step 3: Build and smoke-test in browser console**

```powershell
scripts\build-windows.bat --skip-dirty
```

In the browser, after loading a scene:

```js
const scene = document.querySelector('a-scene');
const lr = document.getElementById('vrLaserRight');

// Single-click: triggerdown then triggerup quickly. After 300ms, expect m3c:panel-toggle.
scene.addEventListener('m3c:panel-toggle', () => console.log('panel-toggle'));
scene.addEventListener('m3c:play-pause',   () => console.log('play-pause'));
lr.dispatchEvent(new CustomEvent('triggerdown'));
setTimeout(() => lr.dispatchEvent(new CustomEvent('triggerup')), 50);

// Double-click: two quick down/up pairs. Expect m3c:play-pause (no panel-toggle).
setTimeout(() => {
  lr.dispatchEvent(new CustomEvent('triggerdown'));
  setTimeout(() => lr.dispatchEvent(new CustomEvent('triggerup')), 30);
  setTimeout(() => {
    lr.dispatchEvent(new CustomEvent('triggerdown'));
    setTimeout(() => lr.dispatchEvent(new CustomEvent('triggerup')), 30);
  }, 100);
}, 1000);
```

Expected: first sequence → `panel-toggle` fires after ~300 ms. Second sequence → `play-pause` fires (no `panel-toggle`).

Verify panel visibility in DOM: `document.getElementById('vrControlsRoot').getAttribute('visible')` should flip on each toggle.

- [ ] **Step 4: Commit**

```powershell
git add internal/static/m3c-controls.js internal/static/browse_scene.gohtml
git commit -m "m3c: trigger state machine — single/double click classification"
```

---

### Task 6: A and X buttons mirror trigger clicks

**Files:**
- Modify: `internal/static/m3c-controls.js`

Spec §4.2: A and X mirror the trigger's click family only — single → `m3c:panel-toggle`, double → `m3c:play-pause`. They don't have raycasters, so they always go straight through (no hadIntersection branch). They don't have hold/drag.

- [ ] **Step 1: Wire A/X events**

A-Frame's `laser-controls` doesn't emit A/X events directly — those come from `oculus-touch-controls`, which is added implicitly by `laser-controls` on Quest. Listen on the same laser entities; A-button on the right hand emits `abuttondown`/`abuttonup`, X-button on the left hand emits `xbuttondown`/`xbuttonup`.

In `m3c-controls.js`'s `init`, after the trigger-event wiring in Step 1 of Task 4, add:

```js
      // A (right) and X (left) mirror trigger clicks — single → panel toggle,
      // double → play/pause. No raycast branch, no drag (only the trigger
      // captures pose during hold).
      this._onAxClickUp = this._onAxClickUp.bind(this);
      const rightLaser = this._lasers.right;
      const leftLaser  = this._lasers.left;
      if (rightLaser) {
        rightLaser.addEventListener('abuttonup', this._onAxClickUp);
      }
      if (leftLaser) {
        leftLaser.addEventListener('xbuttonup', this._onAxClickUp);
      }
```

- [ ] **Step 2: Implement `_onAxClickUp` to feed the same pending-click logic**

After `_onTriggerUp`, add:

```js
    _onAxClickUp: function() {
      // Same double-click window as the trigger. No raycast / no drag.
      const now = performance.now();
      if (this._pendingClick && (now - this._pendingClick.time) <= this.DOUBLE_CLICK_MS) {
        clearTimeout(this._pendingClick.timer);
        this._pendingClick = null;
        this.sceneEl.emit('m3c:play-pause');
        return;
      }
      if (this._pendingClick) clearTimeout(this._pendingClick.timer);
      const pending = { time: now, timer: 0 };
      pending.timer = setTimeout(() => {
        if (this._pendingClick === pending) {
          this._pendingClick = null;
          this.sceneEl.emit('m3c:panel-toggle');
        }
      }, this.DOUBLE_CLICK_MS);
      this._pendingClick = pending;
    },
```

(This duplicates the click-classification block from `_onTriggerUp`. Acceptable — extracting a helper is YAGNI for two callers; if a third lands later, refactor then.)

- [ ] **Step 3: Build and smoke-test**

```powershell
scripts\build-windows.bat --skip-dirty
```

In the browser console:

```js
const lr = document.getElementById('vrLaserRight');
const ll = document.getElementById('vrLaserLeft');

// A-button single-click → panel-toggle after 300ms.
lr.dispatchEvent(new CustomEvent('abuttonup'));

// X-button double-click → play-pause.
ll.dispatchEvent(new CustomEvent('xbuttonup'));
setTimeout(() => ll.dispatchEvent(new CustomEvent('xbuttonup')), 100);
```

Expected: same outputs as Task 5 — first → `panel-toggle`, second pair → `play-pause`.

- [ ] **Step 4: Commit**

```powershell
git add internal/static/m3c-controls.js
git commit -m "m3c: A and X buttons mirror trigger clicks"
```

---

### Task 7: B/Y short-press → mode-aware reset

**Files:**
- Modify: `internal/static/m3c-controls.js`
- Modify: `internal/static/browse_scene.gohtml` (add `m3c:reset-short` handler + `resetGeometry` + `recenter` helpers)

Spec §4.2: on `bbuttondown`/`ybuttondown` start a 500 ms timer. If released before 500 ms → emit `m3c:reset-short`. Long-press is Task 8.

- [ ] **Step 1: Wire B/Y events with short-press detection**

In `m3c-controls.js`'s `init`, after the A/X wiring, add:

```js
      // B (right) / Y (left) — short-press = reset, long-press = recenter (Task 8).
      this.LONG_PRESS_MS = 500;
      this._byState = {
        b: { downTime: 0, fired: false },
        y: { downTime: 0, fired: false }
      };
      this._onByDown = this._onByDown.bind(this);
      this._onByUp   = this._onByUp.bind(this);
      if (rightLaser) {
        rightLaser.addEventListener('bbuttondown', () => this._onByDown('b'));
        rightLaser.addEventListener('bbuttonup',   () => this._onByUp('b'));
      }
      if (leftLaser) {
        leftLaser.addEventListener('ybuttondown', () => this._onByDown('y'));
        leftLaser.addEventListener('ybuttonup',   () => this._onByUp('y'));
      }
```

- [ ] **Step 2: Implement the press handlers**

After `_onAxClickUp`, add:

```js
    _onByDown: function(which) {
      const st = this._byState[which];
      st.downTime = performance.now();
      st.fired = false;
    },
    _onByUp: function(which) {
      const st = this._byState[which];
      if (st.fired) {
        // Long-press already fired in tick(); skip the short-press emit.
        st.fired = false;
        return;
      }
      const elapsed = performance.now() - st.downTime;
      const mode = this._currentMode();
      if (elapsed < this.LONG_PRESS_MS) {
        this.sceneEl.emit('m3c:reset-short', { mode: mode });
      }
    },
    _currentMode: function() {
      // Cinema = the flat plane is the active geometry. Otherwise immersive.
      const flat = document.getElementById('vrFlat');
      if (!flat) return 'immersive';
      const v = flat.getAttribute('visible');
      return (v === true || v === 'true') ? 'cinema' : 'immersive';
    },
```

The long-press promotion in `tick` is added in Task 8; for now `st.fired` is never set true so every release fires `reset-short`.

- [ ] **Step 3: Add `m3c:reset-short` handler in IIFE**

In `browse_scene.gohtml`'s inline IIFE, add helpers and listener after the `m3c:play-pause` handler:

```js
    // Reset the active geometry to its template-default position+scale.
    // Defaults are static per the HTML: vrFlat at (0,1.6,-3) scale 1,
    // all others at (0,0,0) scale 1.
    function resetGeometry() {
      const el = activeGeometry();
      if (!el || !el.object3D) return;
      if (el.id === 'vrFlat') {
        el.object3D.position.set(0, 1.6, -3);
      } else {
        el.object3D.position.set(0, 0, 0);
      }
      el.object3D.scale.set(1, 1, 1);
      const cam = scene.camera;
      if (el.id === 'vrFlat' && cam) el.object3D.lookAt(cam.position);
    }

    // WebXR yaw recenter: capture current camera yaw, build inverse-yaw
    // offset, swap reference space. Pitch/roll left to the headset (it
    // reports them correctly).
    function recenterYaw() {
      const r = scene.renderer;
      if (!r || !r.xr || !r.xr.isPresenting) return;
      const session = r.xr.getSession();
      const baseSpace = r.xr.getReferenceSpace();
      if (!session || !baseSpace) return;
      const cam = r.xr.getCamera();
      if (!cam) return;
      // Extract yaw from camera world matrix.
      const e = cam.matrixWorld.elements;
      // Forward vector is -Z column (cols 8,9,10 are -Z). Yaw = atan2(forward.x, forward.z).
      const fx = -e[8], fz = -e[10];
      const yaw = Math.atan2(fx, fz);
      const sinH = Math.sin(-yaw / 2);
      const cosH = Math.cos(-yaw / 2);
      // Quaternion for rotation about Y axis by -yaw.
      const offset = new XRRigidTransform(
        { x: cam.position.x, y: 0, z: cam.position.z, w: 1 },
        { x: 0, y: sinH, z: 0, w: cosH }
      );
      try {
        r.xr.setReferenceSpace(baseSpace.getOffsetReferenceSpace(offset));
      } catch (err) {
        console.warn('stash-vr: recenter failed', err);
      }
    }

    scene.addEventListener('m3c:reset-short', function(e) {
      if (e.detail.mode === 'cinema') {
        resetGeometry();
      } else {
        recenterYaw();
      }
    });
```

- [ ] **Step 4: Build and smoke-test**

```powershell
scripts\build-windows.bat --skip-dirty
```

In browser console:

```js
const scene = document.querySelector('a-scene');
const lr = document.getElementById('vrLaserRight');

scene.addEventListener('m3c:reset-short', e => console.log('reset-short', e.detail));

// Short-press B (release within 500ms):
lr.dispatchEvent(new CustomEvent('bbuttondown'));
setTimeout(() => lr.dispatchEvent(new CustomEvent('bbuttonup')), 200);
// Expect: m3c:reset-short with mode='cinema' (since flat scene loaded by default if non-VR scene).
```

`recenterYaw()` only activates inside an XR session, so calling it in 2D mode is a no-op (the `r.xr.isPresenting` guard skips). That's expected.

For cinema: load a scene that renders flat (no VR tags). Move `vrFlat` programmatically:

```js
document.getElementById('vrFlat').object3D.position.x = 5;
// Then trigger reset-short (via fake bbuttonup) and verify position resets to (0, 1.6, -3).
```

- [ ] **Step 5: Commit**

```powershell
git add internal/static/m3c-controls.js internal/static/browse_scene.gohtml
git commit -m "m3c: B/Y short-press → mode-aware reset"
```

---

### Task 8: B/Y long-press → full recenter

**Files:**
- Modify: `internal/static/m3c-controls.js`
- Modify: `internal/static/browse_scene.gohtml` (add `m3c:reset-long` handler)

Spec §4.2: B/Y held ≥500 ms emits `m3c:reset-long` (full recenter both modes: yaw + geometry reset). Released after long-press fires must NOT also fire `reset-short`.

- [ ] **Step 1: Promote held B/Y to long-press in tick**

In `m3c-controls.js`, modify the `tick` function to also walk B/Y state:

```js
    tick: function(time, delta) {
      // Trigger candidate → drag promotion (existing).
      ['right', 'left'].forEach(hand => {
        const st = this.triggerState[hand];
        if (st.phase === 'candidate') {
          const elapsed = performance.now() - st.downTime;
          this._getControllerPos(hand, this.curPos);
          this.deltaPos.subVectors(this.curPos, st.startPos);
          const dist = this.deltaPos.length();
          if (dist > this.DRAG_DIST_M || elapsed > this.DRAG_HOLD_MS) {
            st.phase = 'drag';
            this.sceneEl.emit('m3c:drag-start', { hand: hand });
          }
        }
        if (st.phase === 'drag') {
          this._getControllerPos(hand, this.curPos);
          this.deltaPos.subVectors(this.curPos, st.startPos);
          this.sceneEl.emit('m3c:drag-move', {
            hand: hand,
            dx: this.deltaPos.x,
            dy: this.deltaPos.y,
            dz: this.deltaPos.z
          });
          st.startPos.copy(this.curPos);
        }
      });
      // B/Y long-press promotion.
      ['b', 'y'].forEach(which => {
        const st = this._byState[which];
        if (st.downTime === 0 || st.fired) return;
        if (performance.now() - st.downTime >= this.LONG_PRESS_MS) {
          st.fired = true;
          const mode = this._currentMode();
          this.sceneEl.emit('m3c:reset-long', { mode: mode });
        }
      });
    },
```

In `_onByDown`, ensure `downTime` is set non-zero (already done in Task 7's version). In `_onByUp`, the `if (st.fired)` early-return prevents double-firing — and we need to clear `downTime` so the tick loop doesn't keep checking after release:

Replace `_onByUp` body with:

```js
    _onByUp: function(which) {
      const st = this._byState[which];
      const wasFired = st.fired;
      const elapsed = performance.now() - st.downTime;
      st.downTime = 0;
      st.fired = false;
      if (wasFired) return; // long-press already fired
      if (elapsed < this.LONG_PRESS_MS) {
        const mode = this._currentMode();
        this.sceneEl.emit('m3c:reset-short', { mode: mode });
      }
    },
```

- [ ] **Step 2: Add `m3c:reset-long` IIFE handler**

In `browse_scene.gohtml`'s inline IIFE, after the `m3c:reset-short` listener:

```js
    scene.addEventListener('m3c:reset-long', function() {
      // Full recenter: yaw recenter + geometry reset, both modes.
      recenterYaw();
      resetGeometry();
    });
```

- [ ] **Step 3: Build and smoke-test**

```powershell
scripts\build-windows.bat --skip-dirty
```

In browser console:

```js
const scene = document.querySelector('a-scene');
const lr = document.getElementById('vrLaserRight');

scene.addEventListener('m3c:reset-short', e => console.log('reset-short', e.detail));
scene.addEventListener('m3c:reset-long',  e => console.log('reset-long',  e.detail));

// Short-press: down then up within 500ms → reset-short.
lr.dispatchEvent(new CustomEvent('bbuttondown'));
setTimeout(() => lr.dispatchEvent(new CustomEvent('bbuttonup')), 200);

// Long-press: down then wait > 500ms → reset-long fires automatically; up afterward does NOT also fire reset-short.
setTimeout(() => {
  lr.dispatchEvent(new CustomEvent('bbuttondown'));
  setTimeout(() => lr.dispatchEvent(new CustomEvent('bbuttonup')), 700);
}, 1000);
```

Expected: first sequence → one `reset-short`. Second sequence → one `reset-long` at ~500 ms; release at ~700 ms emits nothing additional.

- [ ] **Step 4: Commit**

```powershell
git add internal/static/m3c-controls.js internal/static/browse_scene.gohtml
git commit -m "m3c: B/Y long-press → full recenter"
```

---

### Task 9: Thumbstick X-axis discrete seek

**Files:**
- Modify: `internal/static/m3c-controls.js`
- Modify: `internal/static/browse_scene.gohtml` (add `m3c:seek` handler)

Spec §4.3: per tick, poll `gamepad.axes[2]` (or [0] depending on layout). When `|axis_x|` crosses 0.7 while armed, fire `m3c:seek` with sign and disarm. Re-arm when `|axis_x| < 0.3`.

Note: A-Frame's `tracked-controls` exposes the gamepad via `el.components['tracked-controls'].controller.axes`. Quest controller layout is `[thumbstick_x, thumbstick_y, ?, ?]` — typically axes[2] and axes[3] for thumbstick on `xr-standard`, axes[0] and [1] for legacy. Probe both: prefer non-zero axes pair.

- [ ] **Step 1: Add per-controller seek state and poll helper**

In `m3c-controls.js`'s `init`, after the B/Y wiring, add:

```js
      // Thumbstick state per hand. seekArmed=true means the next
      // |axis_x|>0.7 will fire a seek event.
      this.SEEK_TRIGGER = 0.7;
      this.SEEK_REARM   = 0.3;
      this.SEEK_SECONDS = 10;
      this.thumbState = {
        right: { seekArmed: true },
        left:  { seekArmed: true }
      };
```

After `_currentMode`, add a helper to read the gamepad axes:

```js
    _getThumbstick: function(hand) {
      const el = this._lasers[hand];
      if (!el) return null;
      const tc = el.components && el.components['tracked-controls'];
      const ctrl = tc && tc.controller;
      const axes = ctrl && ctrl.axes;
      if (!axes || axes.length < 2) return null;
      // xr-standard mapping puts thumbstick on axes[2],[3]; legacy on [0],[1].
      // Prefer the pair with larger magnitude (likely non-default).
      let x = 0, y = 0;
      if (axes.length >= 4) {
        x = axes[2] || 0;
        y = axes[3] || 0;
        if (Math.abs(x) < 0.001 && Math.abs(y) < 0.001) {
          x = axes[0] || 0;
          y = axes[1] || 0;
        }
      } else {
        x = axes[0] || 0;
        y = axes[1] || 0;
      }
      return { x: x, y: y };
    },
```

- [ ] **Step 2: Add seek polling to tick**

Append to `tick`'s end (after the B/Y long-press promotion block):

```js
      // Thumbstick X — discrete seek with arm/rearm.
      ['right', 'left'].forEach(hand => {
        const ts = this._getThumbstick(hand);
        if (!ts) return;
        const st = this.thumbState[hand];
        const ax = Math.abs(ts.x);
        if (st.seekArmed && ax > this.SEEK_TRIGGER) {
          const sign = ts.x > 0 ? 1 : -1;
          this.sceneEl.emit('m3c:seek', { sign: sign, seconds: this.SEEK_SECONDS });
          st.seekArmed = false;
        } else if (!st.seekArmed && ax < this.SEEK_REARM) {
          st.seekArmed = true;
        }
      });
```

- [ ] **Step 3: Add IIFE seek handler**

After the `m3c:reset-long` listener:

```js
    scene.addEventListener('m3c:seek', function(e) {
      if (isNaN(video.currentTime)) return;
      const delta = e.detail.sign * e.detail.seconds;
      if (delta < 0) {
        video.currentTime = Math.max(0, video.currentTime + delta);
      } else if (!isNaN(video.duration) && video.duration > 0) {
        video.currentTime = Math.min(video.duration - 0.1, video.currentTime + delta);
      } else {
        video.currentTime = video.currentTime + delta;
      }
    });
```

- [ ] **Step 4: Build and smoke-test**

```powershell
scripts\build-windows.bat --skip-dirty
```

In browser console (without a real gamepad we can't easily fake `axes` polling — Quest 3 validation in Task 12 covers this fully). Verify the wiring at least loads:

```js
const scene = document.querySelector('a-scene');
scene.addEventListener('m3c:seek', e => console.log('seek', e.detail));
// Manually emit to test the IIFE handler:
scene.emit('m3c:seek', { sign: 1, seconds: 10 });
// Expect: video.currentTime advances by 10s (verify before/after).
```

- [ ] **Step 5: Commit**

```powershell
git add internal/static/m3c-controls.js internal/static/browse_scene.gohtml
git commit -m "m3c: thumbstick X-axis discrete seek"
```

---

### Task 10: Thumbstick Y-axis continuous scale

**Files:**
- Modify: `internal/static/m3c-controls.js`
- Modify: `internal/static/browse_scene.gohtml` (add `m3c:scale` handler)

Spec §4.3: when `|axis_y| > 0.3` per tick, emit `m3c:scale` with factor `1 + 0.6 * axis_y * dt`. Handler multiplies active-geometry scale by factor and clamps [0.3, 5]. Convention: stick up (axis_y < 0 in standard mapping, but normalize to "positive = scale up") — since browser/WebXR axes are typically inverted-Y (up = negative), and Quest's xr-standard has stick-up as -1, I'll normalize sign so "stick up = scale UP" (intuitive zoom-in).

- [ ] **Step 1: Add scale-emit to tick**

Append to `tick`:

```js
      // Thumbstick Y — continuous scale of active geometry.
      // xr-standard: stick up reads as negative axis_y. Invert so positive = up = scale up.
      const dtSec = (delta || 0) / 1000;
      ['right', 'left'].forEach(hand => {
        const ts = this._getThumbstick(hand);
        if (!ts) return;
        const yNorm = -ts.y; // up = positive
        if (Math.abs(yNorm) > 0.3 && dtSec > 0) {
          const factor = 1 + 0.6 * yNorm * dtSec;
          this.sceneEl.emit('m3c:scale', { factor: factor });
        }
      });
```

(`SEEK_SECONDS` and the scale rate `0.6` could be tuned; per spec §4.3 these are starting values.)

- [ ] **Step 2: Add IIFE scale handler**

After the `m3c:seek` listener:

```js
    scene.addEventListener('m3c:scale', function(e) {
      const el = activeGeometry();
      if (!el || !el.object3D) return;
      const newScale = el.object3D.scale.x * e.detail.factor;
      const clamped = Math.max(0.3, Math.min(5.0, newScale));
      el.object3D.scale.setScalar(clamped);
    });
```

- [ ] **Step 3: Build and smoke-test**

```powershell
scripts\build-windows.bat --skip-dirty
```

In browser console:

```js
const scene = document.querySelector('a-scene');
scene.addEventListener('m3c:scale', e => console.log('scale', e.detail));
// Emit a scale-up factor:
scene.emit('m3c:scale', { factor: 1.5 });
// Verify activeGeometry().object3D.scale.x increases (clamped at 5).
// Then over-emit to hit clamp:
for (let i = 0; i < 20; i++) scene.emit('m3c:scale', { factor: 1.5 });
// Verify scale clamped to 5.
// Now scale down past 0.3 floor:
for (let i = 0; i < 20; i++) scene.emit('m3c:scale', { factor: 0.5 });
// Verify scale clamped to 0.3.
```

- [ ] **Step 4: Commit**

```powershell
git add internal/static/m3c-controls.js internal/static/browse_scene.gohtml
git commit -m "m3c: thumbstick Y-axis continuous scale with clamp"
```

---

### Task 11: Help "?" button on panel + cheatsheet sub-panel

**Files:**
- Modify: `internal/static/browse_scene.gohtml` (add help button + cheatsheet entity + handler)

Spec §4.7: A "?" button on the playback panel toggles a cheatsheet sub-panel listing the §3 binding table.

- [ ] **Step 1: Add the Help button to the playback panel**

In `browse_scene.gohtml`, find the playback-panel button row inside `<a-entity id="vrControls">`. The current width-1.85 plane has 5 buttons at positions -0.75, -0.45, -0.15, 0.15, 0.45 (Play/Pause, -10s, +10s, Format, Exit). Make room for a sixth at 0.75:

Find:

```html
    <a-entity class="vr-btn" data-action="exit" position="0.45 0 0.01"
              geometry="primitive:plane;width:0.28;height:0.28"
              material="color:#a01010;opacity:0.95">
      <a-text value="Exit VR" align="center" color="#fff" width="2.2" position="0 0 0.005"></a-text>
    </a-entity>
```

Adjust the `Exit` position from `0.45` to `0.75` and add the Help button at `0.45`. Replace the `Exit` block with:

```html
    <a-entity class="vr-btn" data-action="help" position="0.45 0 0.01"
              geometry="primitive:plane;width:0.28;height:0.28"
              material="color:#2c5282;opacity:0.95">
      <a-text value="?" align="center" color="#fff" width="3.5" position="0 0 0.005"></a-text>
    </a-entity>
    <a-entity class="vr-btn" data-action="exit" position="0.75 0 0.01"
              geometry="primitive:plane;width:0.28;height:0.28"
              material="color:#a01010;opacity:0.95">
      <a-text value="Exit VR" align="center" color="#fff" width="2.2" position="0 0 0.005"></a-text>
    </a-entity>
```

Also widen the backing plane. Find:

```html
    <a-plane width="1.85" height="0.4" color="#000" material="opacity:0.65"></a-plane>
```

Change `width="1.85"` to `width="2.15"` to fit the sixth button.

- [ ] **Step 2: Add the cheatsheet entity**

Inside `vrControlsRoot`, after `vrFormatPicker`'s closing `</a-entity>` and before the `</a-entity><!-- /vrControlsRoot -->` line, add:

```html
  <!-- M3c help cheatsheet. Hidden by default; toggled by the "?" button. -->
  <a-entity id="vrHelpPanel" position="0 1.4 -1.5" rotation="-15 0 0" visible="false">
    <a-plane width="2.4" height="1.5" color="#000" material="opacity:0.75"></a-plane>
    <a-text value="Controls" align="left" color="#fff" width="3.5" position="-1.1 0.65 0.01"></a-text>

    <a-text value="Trigger (no hit) · single"  align="left" color="#aaa" width="3" position="-1.1  0.42 0.01"></a-text>
    <a-text value="Toggle panel"               align="left" color="#fff" width="3" position=" 0.05 0.42 0.01"></a-text>
    <a-text value="Trigger · double"           align="left" color="#aaa" width="3" position="-1.1  0.30 0.01"></a-text>
    <a-text value="Play / Pause"               align="left" color="#fff" width="3" position=" 0.05 0.30 0.01"></a-text>
    <a-text value="Trigger · hold + move"      align="left" color="#aaa" width="3" position="-1.1  0.18 0.01"></a-text>
    <a-text value="Drag screen"                align="left" color="#fff" width="3" position=" 0.05 0.18 0.01"></a-text>
    <a-text value="A or X · single / double"   align="left" color="#aaa" width="3" position="-1.1  0.06 0.01"></a-text>
    <a-text value="Toggle panel / Play-Pause"  align="left" color="#fff" width="3" position=" 0.05 0.06 0.01"></a-text>
    <a-text value="Thumbstick L / R"           align="left" color="#aaa" width="3" position="-1.1 -0.06 0.01"></a-text>
    <a-text value="-10s / +10s"                align="left" color="#fff" width="3" position=" 0.05 -0.06 0.01"></a-text>
    <a-text value="Thumbstick U / D"           align="left" color="#aaa" width="3" position="-1.1 -0.18 0.01"></a-text>
    <a-text value="Zoom in / out"              align="left" color="#fff" width="3" position=" 0.05 -0.18 0.01"></a-text>
    <a-text value="B or Y · short"             align="left" color="#aaa" width="3" position="-1.1 -0.30 0.01"></a-text>
    <a-text value="Reset screen / Recenter"    align="left" color="#fff" width="3" position=" 0.05 -0.30 0.01"></a-text>
    <a-text value="B or Y · long-press"        align="left" color="#aaa" width="3" position="-1.1 -0.42 0.01"></a-text>
    <a-text value="Full recenter"              align="left" color="#fff" width="3" position=" 0.05 -0.42 0.01"></a-text>

    <a-entity class="vr-btn" data-action="help-close" position="1.05 0.65 0.01"
              geometry="primitive:plane;width:0.18;height:0.18" material="color:#a01010;opacity:0.95">
      <a-text value="X" align="center" color="#fff" width="3.5" position="0 0 0.005"></a-text>
    </a-entity>
  </a-entity>
```

- [ ] **Step 3: Wire the help-toggle action**

In the inline IIFE's `vrAction` function, currently the dispatch ladder ends after `format`. Add new branches for `help` and `help-close`. Replace the body of `vrAction` with:

```js
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
      } else if (action === 'format') {
        if (picker) {
          const visible = picker.getAttribute('visible');
          picker.setAttribute('visible', !visible);
        }
      } else if (action === 'help') {
        const help = document.getElementById('vrHelpPanel');
        if (help) {
          const visible = help.getAttribute('visible');
          help.setAttribute('visible', !visible);
        }
      } else if (action === 'help-close') {
        const help = document.getElementById('vrHelpPanel');
        if (help) help.setAttribute('visible', false);
      }
    }
```

- [ ] **Step 4: Build and smoke-test**

```powershell
scripts\build-windows.bat --skip-dirty
```

Load a scene in the browser. Inspect the DOM: `vrHelpPanel` exists with `visible="false"`. The Help "?" button is present in `vrControls`. In the console:

```js
document.querySelector('.vr-btn[data-action="help"]').dispatchEvent(new Event('click'));
// Verify vrHelpPanel.getAttribute('visible') === 'true' or true.
document.querySelector('.vr-btn[data-action="help-close"]').dispatchEvent(new Event('click'));
// Verify vrHelpPanel.visible flips back to false.
```

- [ ] **Step 5: Commit**

```powershell
git add internal/static/browse_scene.gohtml
git commit -m "m3c: help button + cheatsheet sub-panel"
```

---

### Task 12: Quest 3 validation + result.md

**Files:**
- Create: `docs/superpowers/research/2026-05-08-m3c-result/result.md`
- Create: `docs/superpowers/research/2026-05-08-m3c-result/checklist.md` (track-as-you-go)

This is the actual "does it work" gate. The browser-console smoke checks in Tasks 4–11 verify wiring; this task runs the spec §7 Quest 3 protocol on real hardware.

- [ ] **Step 1: Build a fresh binary**

```powershell
scripts\build-windows.bat
```

(Note: no `--skip-dirty` — production-equivalent build.)

- [ ] **Step 2: Run on the local server reachable from Quest 3**

Per the project's existing setup (HTTPS via Caddy + DuckDNS, per memory). Start the server, confirm reachable from the headset.

- [ ] **Step 3: Run the §7 protocol — cinema scene**

Open spec §7 and follow steps 1–15. Tick off each in `docs/superpowers/research/2026-05-08-m3c-result/checklist.md` as you go. Test on a known cinema scene (no VR tags, renders flat).

- [ ] **Step 4: Run the §7 protocol — immersive scene**

Steps 16–21. Use a known DOME 180° SBS scene (one that worked correctly in M3a/M3b).

- [ ] **Step 5: Run the §7 regression checks**

Steps 22–25. Audio sync, no first-frame flash, Format picker still works, M1 surfaces untouched.

- [ ] **Step 6: Write the result artifact**

Create `docs/superpowers/research/2026-05-08-m3c-result/result.md` with the same template as M3a/M3b's deferred result template (look at `docs/superpowers/research/2026-05-08-m2-webxr-result/result.md` for shape). Cover:

- What works as designed.
- What surprised you (good or bad).
- Any §8-listed risks that materialized; what was done.
- Whether the spec needs amending (e.g., the `0.6` scale rate felt too fast — bump to 0.4).
- Whether any deferred items moved from "non-goal" to "follow-up actually needed" (e.g., user found themselves forgetting bindings — promote first-entry overlay).

- [ ] **Step 7: Final commit**

```powershell
git add docs/superpowers/research/2026-05-08-m3c-result/
git commit -m "m3c: Quest 3 validation result"
```

---

## Self-review

**Spec coverage check.** Each spec section has a task:

- §1 Context — informational, no task.
- §2 Goal & non-goals — covered across all tasks; non-goals (tutorial overlay, IPD, voice, etc.) deliberately not implemented.
- §3 Binding table — Tasks 4–10 cover every row.
- §4.1 Trigger state machine — Task 4 (drag), Task 5 (click).
- §4.2 B/Y short/long — Tasks 7, 8.
- §4.3 Thumbstick polling — Tasks 9 (X), 10 (Y).
- §4.4 Active-geometry resolver — Task 4 Step 2.
- §4.5 Recenter — Task 7 Step 3 (`recenterYaw` helper); used by Tasks 7, 8.
- §4.6 Panel + laser visibility — Task 3.
- §4.7 Help button — Task 11.
- §5.1 Component event surface — Tasks 4–10 (events emitted on `<a-scene>`).
- §5.2 IIFE handlers — Tasks 4–11 (handlers for each event).
- §5.3 Static-file serving — Task 1.
- §5.4 No server changes — confirmed; no server-side files touched in any task.
- §6 Files touched — three files: `static.go`, `browse_scene.gohtml`, `m3c-controls.js`. All three modified across tasks.
- §7 Validation — Task 12.
- §8 Risks — flagged in the spec; revisit in Task 12 result.
- §9 What stays untouched — no task; nothing in M2/M3a/M3b is modified.
- §10 After this milestone — informational, no task.

**Placeholder scan:** No "TBD", "TODO", "implement later", or "fill in details" in any task body. All code blocks are complete and runnable.

**Type / signature consistency check:**

- `m3c:panel-toggle` — no detail (Task 5, 6). Handler in Task 5 reads no detail. ✓
- `m3c:play-pause` — no detail (Task 5, 6). Handler in Task 5 reads no detail. ✓
- `m3c:seek` — `{ sign, seconds }` (Task 9). Handler in Task 9 reads both. ✓
- `m3c:scale` — `{ factor }` (Task 10). Handler in Task 10 reads `factor`. ✓
- `m3c:drag-start/move/end` — `{ hand, dx, dy, dz }` for move; `{ hand }` for start/end (Task 4). Handler in Task 4 reads `dx/dy/dz`. ✓
- `m3c:reset-short` — `{ mode }` (Task 7, 8). Handler in Task 7 reads `mode`. ✓
- `m3c:reset-long` — `{ mode }` (Task 8). Handler in Task 8 reads no detail (acts identically in both modes). ✓ (mode is emitted but unused in handler — fine, future-proofs without harm.)
- `activeGeometry()` defined in Task 4, used by Tasks 4 (drag-move), 7 (reset-short cinema), 8 (reset-long), 10 (scale). Same name throughout. ✓
- `setPanelVisible(bool)` / `isPanelVisible()` defined in Task 3, used by Task 5. Same names. ✓
- `resetGeometry()` / `recenterYaw()` defined in Task 7, used by Task 8. Same names. ✓

**Scope check:** Single milestone, single component, three files modified. Tasks are 5–15 minutes each. No decomposition needed.

---

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-08-m3c-skybox-controller-mappings.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
