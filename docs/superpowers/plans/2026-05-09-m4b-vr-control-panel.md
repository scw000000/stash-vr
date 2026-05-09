# M4b: VR control panel polish — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the M3c playback panel into a SKYBOX-parity HUD with scrub bar, scene markers, current/total time, scene title, mute, captions (with picker), playback speed, and loop toggle.

**Architecture:** Five tasks. Task 1 adds the server-side data plumbing (caption proxy + new fields on `SceneDetailData`). Task 2 grows the panel to three rows and wires the simple buttons (mute, speed, loop). Task 3 adds the top row's title + time text. Task 4 adds the scrub bar with drag-to-seek and scene-marker dots. Task 5 layers subtitles end-to-end (picker, VTT parser, camera-attached plane).

**Tech Stack:** Go 1.24, chi router, html/template, A-Frame 1.7, Three.js (via A-Frame), vanilla JS.

**Spec:** [docs/superpowers/specs/2026-05-09-m4b-vr-control-panel.md](../specs/2026-05-09-m4b-vr-control-panel.md)

**No tests in this project.** Verification is `go vet ./...`, `go build ./...`, and the manual browser steps in §8 of the spec at the end of each task.

**Prerequisite:** M4a should be merged. M4b doesn't touch M4a code, but the user-facing acceptance flow assumes the chip/AJAX UX is shipped.

---

## Task 1: Caption proxy route + data path additions

**Files:**
- Create: `internal/api/browse/caption.go`
- Modify: `internal/api/browse/router.go`
- Modify: `internal/api/browse/data.go`
- Modify: `internal/api/browse/scene.go`

**Goal:** Add the `/browse/scene/{id}/caption` proxy that wraps Stash's caption URL with the API key, and expose `Title`, `Captions`, and `SceneMarkers` on `SceneDetailData` so the template can render them.

- [ ] **Step 1: Add types and fields to data.go**

In [internal/api/browse/data.go](../../../internal/api/browse/data.go), add the new types just below `EntityRef` (which M4a defined):

```go
type CaptionRef struct {
	LanguageCode string `json:"languageCode"`
	CaptionType  string `json:"captionType"`
}

type SceneMarker struct {
	Seconds float64 `json:"seconds"`
	Title   string  `json:"title"`
}
```

In `SceneDetailData`, add three fields after the existing `Tags []EntityRef`:

```go
	Captions     []CaptionRef
	SceneMarkers []SceneMarker
	DurationSec  float64 // duration in seconds; 0 if unknown
```

(`Title` is already present.)

- [ ] **Step 2: Populate new fields in scene.go**

In [internal/api/browse/scene.go](../../../internal/api/browse/scene.go), after the existing tag-population block (around line 100), and after `data.Date` / `data.Duration` are set, add:

```go
	// Captions: language tracks for the subtitle picker.
	for _, c := range vd.SceneParts.Captions {
		if c == nil {
			continue
		}
		data.Captions = append(data.Captions, CaptionRef{
			LanguageCode: c.Language_code,
			CaptionType:  c.Caption_type,
		})
	}

	// Scene markers: chapter dots on the scrub bar.
	for _, m := range vd.SceneParts.Scene_markers {
		if m == nil {
			continue
		}
		data.SceneMarkers = append(data.SceneMarkers, SceneMarker{
			Seconds: m.Seconds,
			Title:   m.Title,
		})
	}

	// Duration in seconds: needed for marker positioning + scrub-bar math.
	if len(vd.SceneParts.Files) > 0 && vd.SceneParts.Files[0] != nil {
		data.DurationSec = vd.SceneParts.Files[0].Duration
	}
```

(Field names from genqlient match Go's snake-to-Camel conversion: `Language_code`, `Caption_type`, `Scene_markers`. Verify against `internal/stash/gql/generated.go`.)

- [ ] **Step 3: Create the caption proxy handler**

Create [internal/api/browse/caption.go](../../../internal/api/browse/caption.go):

```go
package browse

import (
	"io"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/stash"
)

// sceneCaptionHandler proxies Stash's caption file (typically VTT) for a
// given scene and language/type combo. Same pattern as the /cover/{id}
// proxy: same-origin so the browser can fetch without CORS, and the
// Stash API key is appended server-side.
func (h *httpHandler) sceneCaptionHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	q := r.URL.Query()
	lang := q.Get("lang")
	captionType := q.Get("type")
	if lang == "" || captionType == "" {
		http.Error(w, "lang and type required", http.StatusBadRequest)
		return
	}

	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil || vd.SceneParts.Paths == nil || vd.SceneParts.Paths.Caption == nil {
		http.NotFound(w, r)
		return
	}

	upstream := *vd.SceneParts.Paths.Caption + "?lang=" + url.QueryEscape(lang) + "&type=" + url.QueryEscape(captionType)
	upstream = stash.ApiKeyed(upstream)

	req, err := http.NewRequestWithContext(r.Context(), "GET", upstream, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: fetch caption upstream")
		http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: copy caption body")
	}
}
```

- [ ] **Step 4: Mount the new route**

In [internal/api/browse/router.go](../../../internal/api/browse/router.go), add:

```go
	r.Get("/scene/{id}/caption", h.sceneCaptionHandler)
```

just below the existing `r.Get("/scene/{id}/stream", ...)` line.

- [ ] **Step 5: Vet, build**

Run: `go vet ./...` then `go build ./...`

Expected: clean.

- [ ] **Step 6: Manual verify**

Build, run, open a scene that has captions in Stash. In a terminal:

```
curl -i "https://stash-vr.duckdns.org/browse/scene/<id>/caption?lang=en&type=vtt"
```

Expected: `HTTP/1.1 200 OK`, `Content-Type: text/vtt; charset=utf-8`, body is VTT text starting with `WEBVTT`.

- [ ] **Step 7: Commit**

```
git add internal/api/browse/data.go internal/api/browse/scene.go internal/api/browse/caption.go internal/api/browse/router.go
git commit -m "browse: caption proxy + Captions/SceneMarkers/DurationSec on SceneDetailData"
```

---

## Task 2: Panel layout expansion + simple buttons (mute, speed, loop)

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Grow the playback panel from one row to three. Wire the three "simple" buttons (mute, speed, loop) that don't depend on additional data plumbing. Title, time, scrub bar, and captions come in later tasks.

- [ ] **Step 1: Resize the panel background plane and reposition existing buttons**

Find the `vrControls` entity in [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml) (around line 92):

```html
<a-entity id="vrControls" position="0 0.4 -1.5" rotation="-30 0 0">
  <a-plane width="2.15" height="0.4" color="#000" material="opacity:0.65"></a-plane>
  ...
</a-entity>
```

Replace the inner content. The new panel is 3 rows tall (~0.7 m), 3.0 m wide. Existing buttons (Play/Pause, Format, Help, Exit) move to specific positions in row 3; new buttons (Mute, CC, Speed, Loop) get inserted; -10s/+10s are removed.

```html
<a-entity id="vrControls" position="0 0.4 -1.5" rotation="-30 0 0">
  <a-plane width="3.0" height="0.7" color="#000" material="opacity:0.65"></a-plane>

  <!-- Row 3 (bottom): 8 control buttons. Width 0.32m, gap 0.04m, total 8*0.32+7*0.04 = 2.84m. -->
  <a-entity class="vr-btn" data-action="mute" position="-1.20 -0.20 0.01"
            geometry="primitive:plane;width:0.32;height:0.20"
            material="color:#2c5282;opacity:0.95">
    <a-text id="vrMuteIcon" value="🔊" align="center" color="#fff" width="2" position="0 0 0.005"></a-text>
  </a-entity>
  <a-entity class="vr-btn" data-action="cc" id="vrCCBtn" position="-0.88 -0.20 0.01"
            geometry="primitive:plane;width:0.32;height:0.20"
            material="color:#2c5282;opacity:0.95"
            visible="{{if .Captions}}true{{else}}false{{end}}">
    <a-text value="CC" align="center" color="#fff" width="3" position="0 0 0.005"></a-text>
  </a-entity>
  <a-entity class="vr-btn" data-action="playpause" position="-0.56 -0.20 0.01"
            geometry="primitive:plane;width:0.32;height:0.20"
            material="color:#2c5282;opacity:0.95">
    <a-text value="Play/Pause" align="center" color="#fff" width="2.2" position="0 0 0.005"></a-text>
  </a-entity>
  <a-entity class="vr-btn" data-action="speed" position="-0.24 -0.20 0.01"
            geometry="primitive:plane;width:0.32;height:0.20"
            material="color:#2c5282;opacity:0.95">
    <a-text id="vrSpeedLabel" value="1.0x" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
  </a-entity>
  <a-entity class="vr-btn" data-action="loop" id="vrLoopBtn" position="0.08 -0.20 0.01"
            geometry="primitive:plane;width:0.32;height:0.20"
            material="color:#2c5282;opacity:0.95">
    <a-text value="🔁" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
  </a-entity>
  <a-entity class="vr-btn" data-action="format" position="0.40 -0.20 0.01"
            geometry="primitive:plane;width:0.32;height:0.20"
            material="color:#2c5282;opacity:0.95">
    <a-text value="Format" align="center" color="#fff" width="2.2" position="0 0 0.005"></a-text>
  </a-entity>
  <a-entity class="vr-btn" data-action="help" position="0.72 -0.20 0.01"
            geometry="primitive:plane;width:0.32;height:0.20"
            material="color:#2c5282;opacity:0.95">
    <a-text value="?" align="center" color="#fff" width="3.5" position="0 0 0.005"></a-text>
  </a-entity>
  <a-entity class="vr-btn" data-action="exit" position="1.04 -0.20 0.01"
            geometry="primitive:plane;width:0.32;height:0.20"
            material="color:#a01010;opacity:0.95">
    <a-text value="Exit VR" align="center" color="#fff" width="2.2" position="0 0 0.005"></a-text>
  </a-entity>

  <!-- Row 1 + Row 2 placeholders (Tasks 3 + 4 fill these). Empty for now. -->
</a-entity>
```

The `-10s` and `+10s` buttons are deleted (M3c thumbstick covers ±10s).

- [ ] **Step 2: Wire up mute, speed, loop in the existing vrAction switch**

Find the `vrAction` function (around line 604):

```javascript
function vrAction(action) {
  if (action === 'playpause') {
    ...
  } else if (action === 'seek-back') {
    ...
  } else if (action === 'seek-fwd') {
    ...
  } else if (action === 'exit') {
    ...
  } else if (action === 'format') {
    ...
  } else if (action === 'help') {
    ...
  } else if (action === 'help-close') {
    ...
  }
}
```

Replace with:

```javascript
const SPEEDS = [0.5, 1.0, 1.25, 1.5, 2.0];
let speedIdx = 1; // start at 1.0x

function updateMuteIcon() {
  const el = document.getElementById('vrMuteIcon');
  if (el) el.setAttribute('value', video.muted ? '🔇' : '🔊');
}
function updateLoopBtn() {
  const el = document.getElementById('vrLoopBtn');
  if (el) el.setAttribute('material', 'color: ' + (video.loop ? '#3776c2' : '#2c5282') + '; opacity:0.95');
}
function updateSpeedLabel() {
  const el = document.getElementById('vrSpeedLabel');
  if (el) el.setAttribute('value', SPEEDS[speedIdx].toFixed(SPEEDS[speedIdx] === Math.floor(SPEEDS[speedIdx]) ? 1 : 2) + 'x');
}

function vrAction(action) {
  if (action === 'playpause') {
    if (video.paused) {
      const p = video.play();
      if (p && p.catch) p.catch(err => console.warn('stash-vr: video play failed', err));
    } else {
      video.pause();
    }
  } else if (action === 'mute') {
    video.muted = !video.muted;
    updateMuteIcon();
  } else if (action === 'speed') {
    speedIdx = (speedIdx + 1) % SPEEDS.length;
    video.playbackRate = SPEEDS[speedIdx];
    if ('preservesPitch' in video) video.preservesPitch = true;
    updateSpeedLabel();
  } else if (action === 'loop') {
    video.loop = !video.loop;
    updateLoopBtn();
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
  } else if (action === 'cc') {
    // Wired in Task 5.
  }
}
```

- [ ] **Step 3: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build the binary, run, open `/browse/scene/{id}` for a scene, click "Enter VR." Single-click trigger to summon panel. Verify:

- Panel is wider, three rows tall.
- 8 buttons visible in row 3 (or 7 if the scene has no captions — CC is hidden via `visible="false"`).
- Click 🔊 → audio mutes; icon flips to 🔇.
- Click 1.0x → label cycles `1.25x → 1.5x → 2.0x → 0.5x → 1.0x`. Audio plays at chosen rate.
- Click 🔁 → button highlights blue; video loops on end.
- Click again → loop off.
- Existing Play/Pause, Format, Help, Exit still work.
- M3c regressions: panel hide/show, geometry-drag on empty space, thumbstick L/R seek (now the only -10/+10 path) all work.

- [ ] **Step 4: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "m4b: panel grows to 3 rows; add mute, speed, loop buttons; drop -10s/+10s"
```

---

## Task 3: Title display + current/total time

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Top row of the panel shows the scene title (left) and current/total time (right), updating once per second.

- [ ] **Step 1: Add the title and time text nodes**

In the `vrControls` entity (Task 2's panel), inside the `<a-plane>` background, add two `<a-text>` nodes near the top:

```html
  <!-- Row 1 (top): title left, time right. -->
  <a-text id="vrTitle" value="{{.Title}}" align="left" color="#fff" width="3.0"
          position="-1.4 0.30 0.01" wrap-count="36"></a-text>
  <a-text id="vrTime" value="0:00 / 0:00" align="right" color="#fff" width="3.0"
          position="1.4 0.30 0.01"></a-text>
```

(Insert these just after the `<a-plane>` background and before the row 3 buttons.)

- [ ] **Step 2: Add the time tick handler**

In the existing IIFE, after the `vrAction` definition and `updateMuteIcon`/etc., add a tick handler that updates `vrTime` once per second:

```javascript
function formatTime(s) {
  if (!isFinite(s) || s < 0) s = 0;
  const total = Math.floor(s);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const sec = total % 60;
  if (h > 0) return h + ':' + String(m).padStart(2, '0') + ':' + String(sec).padStart(2, '0');
  return m + ':' + String(sec).padStart(2, '0');
}

let lastTimeUpdateMs = 0;
function tickTimeDisplay(nowMs) {
  if (nowMs - lastTimeUpdateMs < 1000) return;
  lastTimeUpdateMs = nowMs;
  const el = document.getElementById('vrTime');
  if (!el) return;
  const cur = formatTime(video.currentTime || 0);
  const dur = formatTime(video.duration || 0);
  el.setAttribute('value', cur + ' / ' + dur);
}

// Single rAF loop drives all per-frame updates. Tasks 4 and 5 hook
// additional handlers into this loop.
function rafLoop() {
  const now = performance.now();
  tickTimeDisplay(now);
  requestAnimationFrame(rafLoop);
}
requestAnimationFrame(rafLoop);
```

A-Frame's `<a-scene>` emits a `tick` event with `e.detail.time` (ms since scene start). We throttle to ~1 Hz to keep `<a-text>` re-renders cheap.

- [ ] **Step 3: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build, run, open a scene. Click Enter VR, summon panel. Verify:

- Title visible top-left (truncated visually if very long; A-Frame doesn't ellipsize but cuts at column).
- Time top-right displays `0:01 / M:SS`, updating each second.
- Pause video → time stays.
- Seek with thumbstick → time updates on next tick (~1s).

- [ ] **Step 4: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "m4b: panel row 1 — scene title + current/total time"
```

---

## Task 4: Scrub bar with drag + scene markers

**Files:**
- Modify: `internal/static/browse_scene.gohtml`
- Modify: `internal/static/m3c-controls.js`

**Goal:** Add a draggable scrub bar that seeks `<video>` in real time, plus dot markers from `vd.SceneParts.SceneMarkers`. M3c's geometry-drag is suppressed when the trigger-down hits a `.vr-scrub` element.

- [ ] **Step 1: Add scrub-bar entities to the panel**

In `vrControls`, between the title/time row and the button row, add the scrub bar:

```html
  <!-- Row 2: scrub bar. Total bar width 2.8m, sits at panel-y 0.05.
       The thumb is the laser-clickable interactive element (.vr-scrub).
       The bar background is also clickable (.vr-scrub-bg → seek to position). -->
  <a-entity id="vrScrubBar" position="0 0.05 0.01">
    <a-plane class="vr-scrub-bg vr-btn" data-action="scrub-bg" width="2.8" height="0.04"
             color="#444" material="opacity:0.95"></a-plane>
    <a-plane id="vrScrubFill" width="0.0" height="0.04" color="#3776c2"
             material="opacity:0.95" position="-1.4 0 0.001"></a-plane>
    <a-entity id="vrScrubMarkers"></a-entity>
    <a-entity id="vrScrubThumb" class="vr-btn vr-scrub" data-action="scrub"
              geometry="primitive:plane;width:0.05;height:0.10"
              material="color:#fff" position="-1.4 0 0.005"></a-entity>
  </a-entity>
```

- [ ] **Step 2: Render scene markers from server data**

In the JS IIFE, add a function that builds the marker dots from `{{.SceneMarkers}}` and `{{.DurationSec}}`. Add this just before `tickTimeDisplay`:

```javascript
const SCRUB_BAR_W = 2.8;
const SCRUB_BAR_LEFT = -1.4;
const sceneMarkers = (function() {
  // Server inlines the markers as a JSON literal. Empty if none.
  return JSON.parse({{ marshal .SceneMarkers }});
})();
const sceneDurationSec = parseFloat({{ .DurationSec | printf "%f" }}) || 0;

function tForMarker(secs) {
  if (!sceneDurationSec) return 0;
  return SCRUB_BAR_LEFT + (secs / sceneDurationSec) * SCRUB_BAR_W;
}

function buildSceneMarkerDots() {
  const root = document.getElementById('vrScrubMarkers');
  if (!root) return;
  while (root.firstChild) root.removeChild(root.firstChild);
  if (!sceneDurationSec || !sceneMarkers.length) return;
  sceneMarkers.forEach(m => {
    const x = tForMarker(m.seconds);
    const dot = document.createElement('a-entity');
    dot.classList.add('vr-btn', 'vr-scrub-marker');
    dot.setAttribute('data-marker-seconds', m.seconds);
    dot.setAttribute('data-marker-title', m.title || '');
    dot.setAttribute('geometry', 'primitive:circle;radius:0.03');
    dot.setAttribute('material', 'color:#f7b500;opacity:0.95');
    dot.setAttribute('position', x.toFixed(3) + ' 0 0.003');
    root.appendChild(dot);
  });
}
scene.addEventListener('loaded', buildSceneMarkerDots);
```

The `{{ marshal .SceneMarkers }}` template func emits the slice as JSON. Add the `marshal` func to the template's FuncMap:

In [internal/api/browse/scene.go](../../../internal/api/browse/scene.go), find:

```go
var sceneTmpl = template.Must(template.New("browse_scene.gohtml").Funcs(template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"le":  func(a, b int) bool { return a <= b },
}).ParseFS(static.Fs, "browse_scene.gohtml"))
```

Add `"marshal"`:

```go
import "encoding/json"
// ...
var sceneTmpl = template.Must(template.New("browse_scene.gohtml").Funcs(template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"le":  func(a, b int) bool { return a <= b },
	"marshal": func(v interface{}) (template.JS, error) {
		b, err := json.Marshal(v)
		return template.JS(b), err
	},
}).ParseFS(static.Fs, "browse_scene.gohtml"))
```

- [ ] **Step 3: Add the scrub-bar tick handler**

The fill width and thumb position update from `video.currentTime / video.duration` every frame. Add this near `tickTimeDisplay`:

```javascript
function tickScrubBar() {
  if (!sceneDurationSec && (!isFinite(video.duration) || video.duration === 0)) return;
  const dur = video.duration || sceneDurationSec;
  const t = Math.max(0, Math.min(dur, video.currentTime));
  const ratio = t / dur;
  const fillW = ratio * SCRUB_BAR_W;
  const fill = document.getElementById('vrScrubFill');
  const thumb = document.getElementById('vrScrubThumb');
  if (fill) {
    // Plane width is centered on its position, so we set width and shift x.
    fill.setAttribute('width', fillW);
    fill.setAttribute('position', { x: SCRUB_BAR_LEFT + fillW / 2, y: 0, z: 0.001 });
  }
  if (thumb) {
    thumb.setAttribute('position', { x: SCRUB_BAR_LEFT + fillW, y: 0, z: 0.005 });
  }
}

// Update the rAF loop from Task 3 to also drive the scrub bar tick.
// Replace the rafLoop body with:
function rafLoop() {
  const now = performance.now();
  tickTimeDisplay(now);
  tickScrubBar();
  requestAnimationFrame(rafLoop);
}
```

- [ ] **Step 4: Suppress M3c's geometry-drag for scrub trigger-down**

In [internal/static/m3c-controls.js](../../../internal/static/m3c-controls.js), find the trigger-down handler that captures the raycaster intersection. Add a check: if the intersected entity has class `.vr-scrub` or `.vr-scrub-marker` or `.vr-scrub-bg`, don't start a drag candidate — instead, mark this trigger as a scrub-only event and bypass the drag/click classification.

Locate the function (search for `_onTriggerDown`):

```javascript
_onTriggerDown: function(evt) {
  // ... existing code that captures pose, time, and raycast hit
  const hit = this._raycastHit(evt);
  if (hit && hit.classList.contains('vr-btn')) {
    // existing button-click branch
  }
  // existing drag-candidate start
}
```

Add a class-list probe at the top:

```javascript
_onTriggerDown: function(evt) {
  const hit = this._raycastHit(evt);
  if (hit && (hit.classList.contains('vr-scrub') || hit.classList.contains('vr-scrub-bg') || hit.classList.contains('vr-scrub-marker'))) {
    // Scrub session — bypass M3c's drag/click classification entirely.
    // The scrub-bar JS in browse_scene.gohtml handles the drag itself.
    this._scrubActive = true;
    return;
  }
  // ... rest of existing logic unchanged
}
```

And in `_onTriggerUp`:

```javascript
_onTriggerUp: function(evt) {
  if (this._scrubActive) {
    this._scrubActive = false;
    return;
  }
  // ... existing logic
}
```

(Read the actual M3c file to identify exact handler names; the patches above are conceptual.)

- [ ] **Step 5: Add the scrub-bar drag handler in browse_scene.gohtml**

Add the drag implementation in the JS IIFE near `bindForm` / `vrAction`:

```javascript
// Scrub bar drag: triggerdown on .vr-scrub or .vr-scrub-bg starts a
// session that writes video.currentTime per frame (throttled).
let scrubActive = false;
let scrubLastWriteMs = 0;
const SCRUB_THROTTLE_MS = 80;

function cursorToScrubT(intersection) {
  // intersection.point is a world-space Vector3.
  // Scrub bar lives inside vrControls (rotated -30deg around X). We need the
  // X coordinate in panel-local space. The simplest robust path: compute
  // local-x via the bar entity's matrixWorld inverse.
  const bar = document.getElementById('vrScrubBar');
  if (!bar || !bar.object3D) return null;
  const local = bar.object3D.worldToLocal(intersection.point.clone());
  const x = local.x;
  const ratio = Math.max(0, Math.min(1, (x - SCRUB_BAR_LEFT) / SCRUB_BAR_W));
  const dur = video.duration || sceneDurationSec;
  if (!dur) return null;
  return ratio * dur;
}

function setVideoTime(t, throttle) {
  const now = performance.now();
  if (throttle && now - scrubLastWriteMs < SCRUB_THROTTLE_MS) return;
  scrubLastWriteMs = now;
  video.currentTime = t;
}

document.querySelectorAll('.vr-scrub, .vr-scrub-bg, .vr-scrub-marker').forEach(el => {
  el.addEventListener('triggerdown', function(evt) {
    scrubActive = true;
    const intersection = evt.detail && evt.detail.intersection;
    if (intersection) {
      // Marker click: jump to marker.seconds and finish (no drag).
      if (el.classList.contains('vr-scrub-marker')) {
        const secs = parseFloat(el.dataset.markerSeconds);
        if (!isNaN(secs)) setVideoTime(secs, false);
        scrubActive = false;
        return;
      }
      const t = cursorToScrubT(intersection);
      if (t !== null) setVideoTime(t, false);
    }
  });
});

scene.addEventListener('raycaster-intersection', function(evt) {
  if (!scrubActive) return;
  const intersections = evt.detail.intersections || [];
  if (!intersections.length) return;
  const t = cursorToScrubT(intersections[0]);
  if (t !== null) setVideoTime(t, true);
});

document.querySelectorAll('.vr-scrub, .vr-scrub-bg').forEach(el => {
  el.addEventListener('triggerup', function() {
    if (!scrubActive) return;
    scrubActive = false;
    // Final write happens via the next tick or already-throttled write; no
    // explicit final-write needed because the bar tick also tracks current
    // video.currentTime.
  });
});

// Marker tooltip: a single floating <a-text> follows the hovered marker.
const markerTip = (function() {
  const t = document.createElement('a-text');
  t.setAttribute('id', 'vrScrubMarkerTip');
  t.setAttribute('color', '#fff');
  t.setAttribute('width', '3');
  t.setAttribute('align', 'center');
  t.setAttribute('value', '');
  t.setAttribute('visible', 'false');
  t.setAttribute('position', '0 0.10 0.01');
  document.getElementById('vrScrubBar').appendChild(t);
  return t;
})();

document.querySelectorAll('.vr-scrub-marker').forEach(el => {
  el.addEventListener('raycaster-intersected', function() {
    markerTip.setAttribute('value', el.dataset.markerTitle);
    const pos = el.getAttribute('position');
    markerTip.setAttribute('position', pos.x + ' 0.10 0.01');
    markerTip.setAttribute('visible', 'true');
  });
  el.addEventListener('raycaster-intersected-cleared', function() {
    markerTip.setAttribute('visible', 'false');
  });
});
```

(`raycaster-intersection` events: A-Frame's raycaster fires this on each tick that the ray intersects something. Reading `evt.detail.intersections` gives the world-space hit point, which we feed to `cursorToScrubT`.)

- [ ] **Step 6: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean. (The `marshal` template func change requires the import; ensure `encoding/json` is in scene.go's imports.)

Build, run, open a scene with markers and a known duration. Click Enter VR, summon panel. Verify:

- Scrub bar visible row 2.
- Playhead (white thumb) advances as video plays.
- Fill (blue) follows.
- ≥1 marker dot (yellow) visible if scene has markers.
- Hover marker → tooltip with title appears.
- Click marker → video jumps to that timestamp.
- Click bare scrub bar at a position → video jumps there.
- Hold trigger on the thumb and move controller → video seeks in real time. Release → final position holds.
- M3c regressions: trigger on empty space still toggles panel; geometry-drag on empty space still works (NOT triggered when starting on the bar).

- [ ] **Step 7: Commit**

```
git add internal/api/browse/scene.go internal/static/browse_scene.gohtml internal/static/m3c-controls.js
git commit -m "m4b: scrub bar with drag, scene-marker dots, marker tooltip"
```

---

## Task 5: Subtitles end-to-end

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** CC button opens a language picker. Selecting a language fetches the VTT, parses cues, and renders the active cue text on a plane parented to the cinema-plane in cinema mode or to the camera in immersive.

- [ ] **Step 1: Add the subtitle picker sub-panel**

Inside `vrControlsRoot` (alongside `vrFormatPicker` and `vrHelpPanel`), add:

```html
<a-entity id="vrSubtitlePicker" position="0 1.4 -1.5" rotation="-15 0 0" visible="false">
  <a-plane width="2.0" height="0.6" color="#000" material="opacity:0.7"></a-plane>
  <a-text value="Subtitles" align="left" color="#fff" width="3" position="-0.85 0.22 0.01"></a-text>

  <a-entity class="vr-btn vr-cc-pick" data-cc-lang="" position="-0.7 0.05 0.01"
            geometry="primitive:plane;width:0.4;height:0.16"
            material="color:#2c5282;opacity:0.95">
    <a-text value="Off" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
  </a-entity>

  {{range $i, $c := .Captions}}
  <a-entity class="vr-btn vr-cc-pick" data-cc-lang="{{$c.LanguageCode}}" data-cc-type="{{$c.CaptionType}}"
            position="{{add (mul $i 1) -3 | mul 0.42 | sub 0.3}} -0.15 0.01"
            geometry="primitive:plane;width:0.4;height:0.16"
            material="color:#2c5282;opacity:0.95">
    <a-text value="{{$c.LanguageCode}}" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
  </a-entity>
  {{end}}
</a-entity>
```

The position math is awkward; replace with a simpler approach — emit each language button at a fixed-spacing layout:

```html
{{range $i, $c := .Captions}}
<a-entity class="vr-btn vr-cc-pick" data-cc-lang="{{$c.LanguageCode}}" data-cc-type="{{$c.CaptionType}}"
          position="{{ccPickerX $i}} -0.15 0.01"
          geometry="primitive:plane;width:0.4;height:0.16"
          material="color:#2c5282;opacity:0.95">
  <a-text value="{{$c.LanguageCode}}" align="center" color="#fff" width="2.5" position="0 0 0.005"></a-text>
</a-entity>
{{end}}
```

Add the `ccPickerX` template func to FuncMap in [scene.go](../../../internal/api/browse/scene.go):

```go
"ccPickerX": func(i int) string {
    // 4 buttons per row, spacing 0.42, leftmost at x=-0.7. Wrap to next row at i>=4.
    col := i % 4
    return fmt.Sprintf("%.2f", -0.7 + float64(col) * 0.42)
},
```

(The simpler choice: cap at 4 visible languages on one row. v1 doesn't paginate; if you have 5+ caption languages, the 5th overlaps. Mitigate as M4b-followup.)

- [ ] **Step 2: Add the subtitle plane**

Inside `<a-scene>`, add the subtitle render plane. It starts as a child of `<a-scene>` and gets reparented at runtime depending on active geometry:

```html
<a-entity id="vrSubtitlePlane" visible="false">
  <a-plane width="1.6" height="0.18" color="#000" material="opacity:0.5"></a-plane>
  <a-text id="vrSubtitleText" value="" align="center" color="#fff" width="3" position="0 0 0.005"></a-text>
</a-entity>
```

- [ ] **Step 3: Wire CC button + picker handlers**

In the JS IIFE, replace the `'cc'` placeholder branch in `vrAction`:

```javascript
} else if (action === 'cc') {
  const picker = document.getElementById('vrSubtitlePicker');
  if (picker) {
    const visible = picker.getAttribute('visible');
    picker.setAttribute('visible', !visible);
  }
}
```

Add the picker's tap handler — the existing `.vr-btn` click loop already routes via `data-action`, but `.vr-cc-pick` uses `data-cc-lang` instead. Add a parallel branch in the click handler:

Find the existing block:

```javascript
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
```

Replace with:

```javascript
document.querySelectorAll('.vr-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    const action = btn.dataset.action || btn.getAttribute('data-action');
    if (action) {
      vrAction(action);
      return;
    }
    const row = btn.dataset.pickRow;
    const value = btn.dataset.pickValue;
    if (row) { handlePickerTap(row, value); return; }
    const ccLang = btn.dataset.ccLang;
    if (typeof ccLang !== 'undefined') {
      const ccType = btn.dataset.ccType || 'vtt';
      handleCCPick(ccLang, ccType);
      return;
    }
  });
});
```

- [ ] **Step 4: Implement `handleCCPick`, the VTT parser, and the cue tick**

Add the subtitle implementation in the IIFE:

```javascript
let currentCues = []; // [{start, end, text}]
let currentLang = '';

function parseVTT(text) {
  // Minimal VTT parser. Reads "HH:MM:SS.mmm --> HH:MM:SS.mmm" + payload until blank line.
  const cues = [];
  const lines = text.split(/\r?\n/);
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const m = line.match(/(\d{2}):(\d{2}):(\d{2})[\.,](\d{3})\s+-->\s+(\d{2}):(\d{2}):(\d{2})[\.,](\d{3})/);
    if (!m) continue;
    const start = (+m[1]) * 3600 + (+m[2]) * 60 + (+m[3]) + (+m[4]) / 1000;
    const end   = (+m[5]) * 3600 + (+m[6]) * 60 + (+m[7]) + (+m[8]) / 1000;
    const payloadLines = [];
    i++;
    while (i < lines.length && lines[i].trim() !== '') {
      // Strip simple HTML/style tags, keep text.
      payloadLines.push(lines[i].replace(/<[^>]+>/g, ''));
      i++;
    }
    cues.push({ start, end, text: payloadLines.join('\n') });
  }
  return cues;
}

function handleCCPick(lang, type) {
  const picker = document.getElementById('vrSubtitlePicker');
  if (picker) picker.setAttribute('visible', false);
  if (!lang) {
    currentCues = []; currentLang = '';
    return;
  }
  fetch('/browse/scene/' + encodeURIComponent(sceneId) + '/caption?lang=' + encodeURIComponent(lang) + '&type=' + encodeURIComponent(type))
    .then(r => r.text())
    .then(text => {
      currentCues = parseVTT(text);
      currentLang = lang;
    })
    .catch(err => console.warn('stash-vr: caption fetch failed', err));
}

// Reparent the subtitle plane based on active geometry. Cinema → child
// of vrFlat; immersive → child of camera.
function reparentSubtitlePlane() {
  const subEl = document.getElementById('vrSubtitlePlane');
  if (!subEl || !subEl.object3D) return;
  const active = activeGeometry();
  let target = null;
  if (active && active.id === 'vrFlat') {
    // Below cinema plane.
    target = active;
    subEl.object3D.position.set(0, -1.0, 0.01);
  } else {
    // Camera-attached, below center of gaze.
    const cam = scene.camera;
    if (!cam) return;
    target = { object3D: cam };
    subEl.object3D.position.set(0, -0.5, -1.5);
  }
  // Reparent at the Three.js level.
  if (subEl.object3D.parent !== target.object3D) {
    if (subEl.object3D.parent) subEl.object3D.parent.remove(subEl.object3D);
    target.object3D.add(subEl.object3D);
  }
}

function tickSubtitles() {
  const plane = document.getElementById('vrSubtitlePlane');
  if (!plane) return;
  if (!currentCues.length) {
    plane.setAttribute('visible', 'false');
    return;
  }
  reparentSubtitlePlane();
  const t = video.currentTime || 0;
  let active = null;
  for (let i = 0; i < currentCues.length; i++) {
    if (t >= currentCues[i].start && t <= currentCues[i].end) { active = currentCues[i]; break; }
  }
  if (active) {
    document.getElementById('vrSubtitleText').setAttribute('value', active.text);
    plane.setAttribute('visible', 'true');
  } else {
    plane.setAttribute('visible', 'false');
  }
}

// Update the rAF loop from Task 4 to also drive subtitle ticks.
// Replace the rafLoop body with:
function rafLoop() {
  const now = performance.now();
  tickTimeDisplay(now);
  tickScrubBar();
  tickSubtitles();
  requestAnimationFrame(rafLoop);
}
```

- [ ] **Step 5: Vet, build, manually verify**

Run: `go vet ./...` then `go build ./...` — expect clean.

Build, run, open a scene with captions in Stash. Click Enter VR, summon panel. Verify:

- CC button visible (not hidden via `{{if .Captions}}`).
- Tap CC → picker opens with "Off" + each available language.
- Tap a language → picker closes; after a brief fetch, subtitles appear at the bottom of the cinema plane (cinema mode) or below center-of-gaze (immersive).
- Subtitles update as video plays through cues.
- Switch language → re-fetches and updates.
- Tap Off → subtitles disappear.
- Open a scene with no captions → CC button is absent (hidden via template `visible="false"`).
- M3c, M3a, M3b, Task 2-4 regressions intact.

- [ ] **Step 6: Commit**

```
git add internal/static/browse_scene.gohtml internal/api/browse/scene.go
git commit -m "m4b: subtitles — picker, VTT parser, mode-aware subtitle plane"
```

---

## Self-review checklist

- **Spec coverage:** Title (Task 3) + time (Task 3) + scrub bar (Task 4) + scene markers (Task 4) + mute (Task 2) + CC (Task 5) + speed (Task 2) + loop (Task 2) all mapped. Caption proxy + data path (Task 1) is the prerequisite. M3c integration (Task 4 step 4) handles the `.vr-scrub` opt-out.
- **No placeholders:** Each task has concrete code, exact paths, exact commands. The Task 4 m3c-controls.js patch is described conceptually because the caller must read the existing handler names — flagged in the step.
- **Type consistency:** `CaptionRef`, `SceneMarker` defined in Task 1, used in Tasks 4 and 5 via template + JSON.
- **Frequent commits:** One per task. Five commits total.
- **YAGNI:** No advanced settings, no volume slider, no SRT support, no chapter generation. All explicit non-goals from the spec.
