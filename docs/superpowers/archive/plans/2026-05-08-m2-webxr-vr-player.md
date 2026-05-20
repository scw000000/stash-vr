# M2 WebXR 180° SBS VR player Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a WebXR immersive-vr mode to `/browse/scene/{id}`, gated to scenes tagged `DOME` + `SBS`. The 2D player from M1 stays; an "Enter VR" button below it triggers a full-takeover A-Frame `<a-scene>` that renders the same direct stream URL as a stereoscopic half-sphere. Exit VR returns to the 2D layout with `currentTime` and `paused` preserved.

**Architecture:** Server-rendered Go templates throughout (no SPA, no build pipeline). A-Frame and `aframe-stereo-component` are vendored under `internal/static/vendor/` and shipped via the existing `embed.FS` + the existing root-mounted `http.FileServerFS` route. Detection is server-side: a single `IsVR180SBS bool` on `SceneDetailData` driven by an `apiinternal.TagVR_DOME` + `apiinternal.TagVR_SBS` check. The `<video>` element is shared between 2D and VR — A-Frame's `src="#sceneVideo"` references the same DOM element, so `currentTime` and `paused` carry across automatically.

**Tech Stack:** Go 1.24 (existing), chi/v5 (existing), `html/template` (existing pattern), `embed.FS` (existing pattern). New runtime deps: A-Frame 1.7.0 (~500 KB) and aframe-stereo-component 1.4.0 (~10 KB), both vendored as static assets.

**Spec:** [docs/superpowers/specs/2026-05-08-m2-webxr-vr-player.md](../specs/2026-05-08-m2-webxr-vr-player.md)

**Project conventions to honor:**
- No test suite per [CLAUDE.md](../../../CLAUDE.md). "TDD" here means manual verification after each task: `go vet ./...`, `go build ./...`, `curl`, then visual eyeball.
- Lowercase commit prefixes following recent style: `browse: <message>` (every commit in this plan touches browse functionality, even the vendoring which exists solely for the VR mode rendered by browse).
- The user has approved this plan; commit steps within tasks are authorized by that approval. Do NOT amend or rebase prior commits without further explicit user request.
- Direct-to-master per the user's M2 branch decision (matches M1).

---

## File structure

**Created:**
- `internal/static/vendor/aframe.min.js` — A-Frame 1.7.0 production build, ~1.28 MB uncompressed.
- `internal/static/vendor/aframe-stereo-component.min.js` — oscarmarinmiro/aframe-stereo-component 1.4.0, ~3 KB.
- `docs/superpowers/research/2026-05-08-m2-webxr-result/checklist.md` — Quest 3 validation checklist.
- `docs/superpowers/research/2026-05-08-m2-webxr-result/result.md` — result stub for user to fill in.

**Modified:**
- `internal/static/static.go` — extend `//go:embed` directive to include `vendor/*.js`.
- `internal/api/browse/data.go` — add `SceneDetailData.IsVR180SBS bool`.
- `internal/api/browse/scene.go` — fold DOME+SBS detection into the existing tag loop.
- `internal/static/browse_scene.gohtml` — add `id="sceneVideo"` to the `<video>`, add `{{if .IsVR180SBS}}` block with Enter VR button + `<a-scene>` markup + script tags + inline JS toggle, add CSS for `.btn-vr`.

**Not touched:** router, config, library service, GraphQL documents, `/deovr`, `/heresphere`, mutation handlers (`scene_post.go`), heatmap/cover proxy, sidebar JS, `/browse` index template, M1 search wiring.

**No new packages, no new env vars, no new chi routes.**

---

## Task 1: Vendor A-Frame and stereo component

**Files:**
- Create: `internal/static/vendor/aframe.min.js`
- Create: `internal/static/vendor/aframe-stereo-component.min.js`
- Modify: `internal/static/static.go`

This task ships the JS libraries the VR mode depends on, without any server-side or template changes. End state: hitting `/vendor/aframe.min.js` in a browser returns the library content. Splitting vendoring from the consuming-template change keeps each commit small and lets you confirm the embed glob works in isolation.

- [ ] **Step 1: Create the vendor directory and download A-Frame 1.7.0**

Run:
```
mkdir internal\static\vendor
curl.exe -L -o internal\static\vendor\aframe.min.js https://aframe.io/releases/1.7.0/aframe.min.js
```

Expected: `internal/static/vendor/aframe.min.js` exists and is roughly 500 KB.

Verify with:
```
dir internal\static\vendor\aframe.min.js
```

Expected: file size between 400 KB and 600 KB. If `curl.exe` is unavailable, use `Invoke-WebRequest -Uri https://aframe.io/releases/1.7.0/aframe.min.js -OutFile internal\static\vendor\aframe.min.js` from PowerShell.

- [ ] **Step 2: Download aframe-stereo-component 1.4.0**

Run:
```
curl.exe -L -o internal\static\vendor\aframe-stereo-component.min.js https://cdn.jsdelivr.net/npm/aframe-stereo-component@1.4.0/dist/aframe-stereo-component.min.js
```

Expected: `internal/static/vendor/aframe-stereo-component.min.js` exists and is roughly 10–20 KB.

Verify with:
```
dir internal\static\vendor\aframe-stereo-component.min.js
```

Expected: file size between 5 KB and 50 KB.

If jsdelivr returns 404 or the file is empty, fall back to unpkg:
```
curl.exe -L -o internal\static\vendor\aframe-stereo-component.min.js https://unpkg.com/aframe-stereo-component@1.4.0/dist/aframe-stereo-component.min.js
```

- [ ] **Step 3: Extend the embed directive in `internal/static/static.go`**

The current file is:

```go
package static

import "embed"

//go:embed *.gohtml *.html *.png
var Fs embed.FS
```

Replace with:

```go
package static

import "embed"

//go:embed *.gohtml *.html *.png vendor/*.js
var Fs embed.FS
```

(Single-line addition: append `vendor/*.js` to the existing glob.)

- [ ] **Step 4: Verify build**

Run:
```
go vet ./...
go build ./...
```

Expected: both exit code 0, no errors. If `go build` fails with "pattern vendor/*.js: no matching files", verify the vendor directory contents and re-run.

- [ ] **Step 5: Verify HTTP serving via curl**

Start stash-vr against the user's running Stash:
```
go run ./cmd/stash-vr --STASH_GRAPHQL_URL=http://192.168.1.183:9999/graphql
```

In another terminal:
```
curl.exe -sI http://localhost:9666/vendor/aframe.min.js
curl.exe -sI http://localhost:9666/vendor/aframe-stereo-component.min.js
```

Expected: both return `HTTP/1.1 200 OK`. Content-Type may be `application/javascript`, `text/javascript`, or `application/octet-stream` — any of those is fine; the browser handles the script tag based on the `<script>` element type, not the response Content-Type, in modern Chromium.

Also verify content size:
```
curl.exe -s http://localhost:9666/vendor/aframe.min.js | findstr /R "AFRAME"
```

Expected: at least one line of output (A-Frame's bundle contains the `AFRAME` global definition).

If the routes return 404, the embed glob didn't pick up the files — re-check Step 3 and re-run `go build`.

- [ ] **Step 6: Commit**

```
git add internal/static/vendor/aframe.min.js internal/static/vendor/aframe-stereo-component.min.js internal/static/static.go
git commit -m "browse: vendor A-Frame 1.7.0 + aframe-stereo-component 1.4.0 for VR mode"
```

---

## Task 2: Server-side `IsVR180SBS` detection

**Files:**
- Modify: `internal/api/browse/data.go`
- Modify: `internal/api/browse/scene.go`

This task adds the server detection without yet templating it. Verifiable by build cleanliness — the `IsVR180SBS` field becomes visible to the template only after Task 3.

- [ ] **Step 1: Add `IsVR180SBS` to `SceneDetailData` in `internal/api/browse/data.go`**

The current struct ends with:

```go
// SceneDetailData drives browse_scene.gohtml.
type SceneDetailData struct {
    ID           string
    Title        string
    ThumbnailURL string
    BackURL      string // from Referer, fallback "/browse"
    Performers   string
    Studio       string
    Date         string // YYYY-MM-DD or empty
    Duration     string
    Rating1to5   int    // 0 = unrated; 1..5 set
    IsFavorite   bool
    Tags         []string // tag names currently on the scene (chips), excluding favorite tag and ancestor-injected tags
    AllTagNames  []string // for the <datalist> autocomplete
    OCounter        int
    Organized       bool
    DirectStreamURL string
    ErrMessage      string

    // StarSlice is a 5-element placeholder used purely so the template can
    // {{range $i, $_ := .StarSlice}} 0..4 to render the five star buttons.
    StarSlice [5]struct{}
}
```

Add `IsVR180SBS bool` after `DirectStreamURL`:

```go
// SceneDetailData drives browse_scene.gohtml.
type SceneDetailData struct {
    ID           string
    Title        string
    ThumbnailURL string
    BackURL      string
    Performers   string
    Studio       string
    Date         string
    Duration     string
    Rating1to5   int
    IsFavorite   bool
    Tags         []string
    AllTagNames  []string
    OCounter        int
    Organized       bool
    DirectStreamURL string
    IsVR180SBS      bool
    ErrMessage      string

    StarSlice [5]struct{}
}
```

- [ ] **Step 2: Detect `DOME` + `SBS` in the existing tag loop in `internal/api/browse/scene.go`**

The current tag loop (around lines 75–89) is:

```go
favTag := config.Application().FavoriteTag
for _, t := range vd.SceneParts.Tags {
    if t == nil {
        continue
    }
    // Skip ancestor-injected tags (decorateTags adds these with prefix.SvrAncestor in Sort_name).
    if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
        continue
    }
    if favTag != "" && t.TagParts.Name == favTag {
        data.IsFavorite = true
        continue
    }
    data.Tags = append(data.Tags, t.TagParts.Name)
}
```

Replace with:

```go
favTag := config.Application().FavoriteTag
hasDome, hasSBS := false, false
for _, t := range vd.SceneParts.Tags {
    if t == nil {
        continue
    }
    name := t.TagParts.Name
    // Detect VR projection BEFORE the ancestor skip so an ancestor-injected
    // DOME or SBS tag still counts.
    if name == apiinternal.TagVR_DOME {
        hasDome = true
    }
    if name == apiinternal.TagVR_SBS {
        hasSBS = true
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
data.IsVR180SBS = hasDome && hasSBS
```

The `apiinternal` import alias is already present in scene.go's import block (it imports `apiinternal "stash-vr/internal/api/internal"`); no new import needed.

- [ ] **Step 3: Verify build**

Run:
```
go vet ./...
go build ./...
```

Expected: both exit code 0. If `go vet` complains about an unused variable, re-check Step 2's exact text (typically the issue is a copy-paste defect).

- [ ] **Step 4: Commit**

```
git add internal/api/browse/data.go internal/api/browse/scene.go
git commit -m "browse: detect DOME+SBS scenes for VR-eligible toggle"
```

---

## Task 3: Template — Enter VR button, `<a-scene>`, script tags, JS toggle

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

This task adds the VR markup, the script tags, the inline toggle JS, and the CSS for the Enter VR button. Non-VR scenes get zero new bytes — everything new is inside `{{if .IsVR180SBS}}`.

- [ ] **Step 1: Add `id="sceneVideo"` to the existing `<video>` element**

The current element (around line 51) is:

```html
<video class="player" controls playsinline autoplay muted preload="metadata" src="{{.DirectStreamURL}}"{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}></video>
```

Replace with (added `id="sceneVideo"`):

```html
<video id="sceneVideo" class="player" controls playsinline autoplay muted preload="metadata" src="{{.DirectStreamURL}}"{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}></video>
```

- [ ] **Step 2: Add the CSS rule for the Enter VR button**

Inside the existing `<style>` block, add a new rule at the end (just before `</style>`):

```css
.btn-vr { display: block; width: 100%; padding: 14px; margin-bottom: 24px; background: #2c5282; color: #fff; border: none; border-radius: 6px; font-size: 1.05rem; font-weight: 600; cursor: pointer; }
.btn-vr:hover { background: #3776c2; }
```

(Same color as the M1 `<button class="addtag">` palette — visually consistent with existing primary buttons.)

- [ ] **Step 3: Add the `{{if .IsVR180SBS}}` block immediately after the `<video>` element**

After the `</video>` line and the existing `{{else if .ThumbnailURL}}` / `{{end}}` block, insert the VR markup. Find this block (currently around lines 50–54):

```html
{{if .DirectStreamURL}}
<video id="sceneVideo" class="player" controls playsinline autoplay muted preload="metadata" src="{{.DirectStreamURL}}"{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}></video>
{{else if .ThumbnailURL}}
<img class="thumb" src="{{.ThumbnailURL}}" alt="">
{{end}}
```

Replace with (the existing video conditional unchanged, followed by the new `{{if .IsVR180SBS}}` block):

```html
{{if .DirectStreamURL}}
<video id="sceneVideo" class="player" controls playsinline autoplay muted preload="metadata" src="{{.DirectStreamURL}}"{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}></video>
{{else if .ThumbnailURL}}
<img class="thumb" src="{{.ThumbnailURL}}" alt="">
{{end}}

{{if .IsVR180SBS}}
<button id="enterVR" class="btn-vr" type="button">▥ Enter VR</button>

<a-scene id="vrScene" style="display:none" vr-mode-ui="enabled: true" loading-screen="enabled: false">
  <a-entity camera></a-entity>
  <a-entity stereo="eye:left;mode:half"
            geometry="primitive:sphere;radius:100;phiLength:180;thetaLength:180;segmentsWidth:64;segmentsHeight:64"
            material="src:#sceneVideo;shader:flat;side:back"
            rotation="0 90 0"></a-entity>
  <a-entity stereo="eye:right;mode:half"
            geometry="primitive:sphere;radius:100;phiLength:180;thetaLength:180;segmentsWidth:64;segmentsHeight:64"
            material="src:#sceneVideo;shader:flat;side:back"
            rotation="0 90 0"></a-entity>
</a-scene>

<script src="/vendor/aframe.min.js"></script>
<script src="/vendor/aframe-stereo-component.min.js"></script>
<script>
  (function() {
    const btn   = document.getElementById('enterVR');
    const scene = document.getElementById('vrScene');
    const wrap  = document.querySelector('.wrap');
    if (!btn || !scene || !wrap) return;

    function show2D() {
      [...wrap.children].forEach(el => { el.style.display = ''; });
      scene.style.display = 'none';
    }
    function hide2D() {
      [...wrap.children].forEach(el => {
        if (el !== scene) el.style.display = 'none';
      });
      scene.style.display = '';
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
{{end}}
```

**Why each part is shaped as it is:**

- `<a-scene style="display:none" vr-mode-ui="enabled: true" loading-screen="enabled: false">` — A-Frame's default fullscreen CSS (position:fixed, inset:0) takes effect when display is unset. `loading-screen` would otherwise flash a black overlay; we disable. `vr-mode-ui` enables A-Frame's built-in goggle button as a fallback (mostly hidden in immersive-vr).
- `<a-entity camera>` — single camera entity. WebXR session provides both eye views automatically; `aframe-stereo-component`'s `stereo="eye:..."` attribute on geometry entities filters per-eye rendering at the scene-graph level.
- Two geometry entities, one per eye — `aframe-stereo-component`'s standard pattern. Each is a 180° half-sphere (`phiLength:180; thetaLength:180`), `radius:100` for envelopment, `segmentsWidth:64; segmentsHeight:64` for smooth poles, `material="src:#sceneVideo; shader:flat; side:back"` to render the inside of the sphere with no lighting using the existing `<video>` as texture.
- `rotation="0 90 0"` — rotates the half-sphere so its center faces -Z (camera default). If the half-sphere ends up mis-oriented on the headset (user sees a black wall when looking forward, video to the side), try -90 or 0; final value confirmed during Quest 3 validation.
- `src="#sceneVideo"` — A-Frame resolves `#id` references against `document` globally, so the existing 2D `<video>` element (sibling to `<a-scene>` inside `.wrap`) is found and used as the texture source. No `<a-assets>` block needed.
- The inline JS reads element refs once, then defines `show2D()` / `hide2D()` that toggle `display` on every child of `.wrap`, with `<a-scene>` as the inverse case. The `enterVR()` Promise resolves when the WebXR session starts; on rejection (user cancels, no headset, etc.) we fall back to 2D. The `exit-vr` event fires when the session ends.

- [ ] **Step 4: Verify build**

Run:
```
go vet ./...
go build ./...
```

Expected: both exit code 0. The Go template parser is invoked at runtime, so a template syntax error won't show up here — only on first request.

- [ ] **Step 5: Verify HTTP rendering via curl**

Restart stash-vr (Ctrl+C the previous run, restart with the same args). You'll need two scene IDs from the user's library:
- `<DOME-SBS-ID>`: a scene tagged with both `DOME` and `SBS`.
- `<2D-ID>`: a scene with neither tag.

Ask the user for these IDs if not obvious from the index page, or pick the first scene shown at `http://localhost:9666/browse/tag/<id-of-DOME-tag>` (sidebar's tag list contains "DOME"). For `<2D-ID>`, any scene NOT in either tag works.

Then:
```
curl.exe -s "http://localhost:9666/browse/scene/<DOME-SBS-ID>" | findstr /C:"id=\"enterVR\""
curl.exe -s "http://localhost:9666/browse/scene/<DOME-SBS-ID>" | findstr /C:"<a-scene"
curl.exe -s "http://localhost:9666/browse/scene/<DOME-SBS-ID>" | findstr /C:"aframe.min.js"
curl.exe -s "http://localhost:9666/browse/scene/<DOME-SBS-ID>" | findstr /C:"id=\"sceneVideo\""
```

Expected: each returns one matching line.

```
curl.exe -s "http://localhost:9666/browse/scene/<2D-ID>" | findstr /C:"id=\"enterVR\""
curl.exe -s "http://localhost:9666/browse/scene/<2D-ID>" | findstr /C:"aframe.min.js"
curl.exe -s "http://localhost:9666/browse/scene/<2D-ID>" | findstr /C:"<a-scene"
```

Expected: each returns zero matches. (The `id="sceneVideo"` on the existing `<video>` element is unconditional, so it WILL match on a 2D scene; that's correct — it's only the new VR markup that's gated.)

If a curl returns the wrong shape, the `{{if .IsVR180SBS}}` gate is incorrect — re-check Task 2 step 2 and the template insertion in step 3.

- [ ] **Step 6: Visual eyeball in a desktop browser**

Open `http://localhost:9666/browse/scene/<DOME-SBS-ID>` in any modern desktop browser (Chrome / Firefox / Edge). Expected:
- 2D `<video>` plays at the top (M1 behavior preserved).
- Below the video, a blue "▥ Enter VR" button.
- Click it. The 2D video and metadata block hide. An A-Frame canvas takes over the viewport. (On a desktop without a connected headset, A-Frame won't enter immersive-vr — instead you see the half-sphere rendered in 2D inside the browser. That's still visual confirmation that the scene initialized and the texture bound. Drag-to-look-around uses the default `look-controls` if A-Frame's WASD/look mode is enabled by default; if not, you'll see a static view, which is also fine.)
- Press `ESC` or close the browser tab to exit. (A-Frame's `exit-vr` event fires when the WebXR session ends; in non-VR desktop mode there's no session, so the toggle stays in "VR mode" until you reload. This is acceptable for desktop verification — the headset is the real test environment.)
- Open `http://localhost:9666/browse/scene/<2D-ID>` in the same browser. Expected: 2D video, no Enter VR button, no A-Frame canvas. Page source (`Ctrl+U`) does not contain `aframe.min.js` or `<a-scene`.

If the Enter VR button is missing on the DOME+SBS scene, the server detection is wrong — re-check Task 2. If the button appears on the 2D scene, the template gate is wrong — re-check Step 3's `{{if .IsVR180SBS}}` placement.

- [ ] **Step 7: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: add WebXR Enter VR toggle for DOME+SBS scenes"
```

---

## Task 4: Final on-headset validation and writeup

**Files:**
- Create: `docs/superpowers/research/2026-05-08-m2-webxr-result/checklist.md`
- Create: `docs/superpowers/research/2026-05-08-m2-webxr-result/result.md`

This task validates the full M2 in the actual target environment (Quest 3 / Meta Browser) and produces an artifact that gates moving to M3.

- [ ] **Step 1: Create the checklist file at `docs/superpowers/research/2026-05-08-m2-webxr-result/checklist.md`**

Write the file with these exact contents:

```markdown
# M2 WebXR 180° SBS — Quest 3 Meta Browser checklist

**Run on:** Quest 3 hardware, Meta Browser (NOT DeoVR's in-VR browser; that's known not to support WebXR — confirmed in M2 spec § 1).

**URL to open:** `https://stash-vr.duckdns.org/browse`

For each criterion: PASS / FAIL / PARTIAL + one-line note.

## Detection / button gating

- [ ] On a scene tagged `DOME` + `SBS`, the "Enter VR" button is visible below the 2D player.
  - Result: ___ — note: ___

- [ ] On a scene tagged `DOME` only (no `SBS`) — if any in your library — the Enter VR button is NOT visible. (Skip if no such scene exists.)
  - Result: ___ — note: ___

- [ ] On a 2D scene (no `DOME` and no `SBS` tags), the Enter VR button is NOT visible.
  - Result: ___ — note: ___

- [ ] Page source on a 2D scene does NOT contain `aframe.min.js` (use Meta Browser's "View Page Source" if accessible, or skip if it's not).
  - Result: ___ — note: ___

## VR session lifecycle

- [ ] Click "Enter VR" — Meta Browser's WebXR consent prompt appears (the first time).
  - Result: ___ — note: ___

- [ ] Grant consent — page swaps to immersive-vr. Headset shows the half-sphere video.
  - Result: ___ — note: ___

- [ ] Stereo split is correct: closing one eye then the other shows different views (each eye sees its half of the SBS texture). If both eyes see the same image, stereo is broken.
  - Result: ___ — note: ___

- [ ] Looking left/right reveals the rest of the 180° field (not a black wall, not a duplicated front view).
  - Result: ___ — note: ___

- [ ] Looking behind shows blank/black (the half-sphere doesn't cover 360°). NOT a mirror copy of the front.
  - Result: ___ — note: ___

- [ ] Audio is audible (or unmute via Quest controller / headset menu if browser autoplay is muted).
  - Result: ___ — note: ___

## Playback continuity

- [ ] In 2D, scrub the video to 1:00. Click Enter VR. The VR scene starts playing at 1:00 (not at 0:00).
  - Result: ___ — note: ___

- [ ] Watch a few seconds in VR. Exit VR (close the WebXR session via Meta Browser's exit overlay, or remove the headset). The 2D player is visible again with `currentTime` at the position the VR session ended.
  - Result: ___ — note: ___

## Existing M1 features regression

- [ ] On the same VR-eligible scene, click a star — rating updates after page reload.
  - Result: ___ — note: ___

- [ ] Toggle favorite — state persists.
  - Result: ___ — note: ___

- [ ] Add a tag via the input — chip appears.
  - Result: ___ — note: ___

- [ ] Remove a tag — chip disappears.
  - Result: ___ — note: ___

- [ ] O-counter +/- — number updates.
  - Result: ___ — note: ___

- [ ] Organized toggle — state changes.
  - Result: ___ — note: ___

- [ ] M1 search at `/browse?q=...` still works.
  - Result: ___ — note: ___

- [ ] Sidebar performer / studio / tag click still navigates to the entity-filtered grid.
  - Result: ___ — note: ___

## Non-VR-scene baseline (M1 unchanged)

- [ ] Open a non-VR scene. 2D player works as in M1. No "Enter VR" button. No console errors related to A-Frame.
  - Result: ___ — note: ___

## Overall

- [ ] All checks PASS → proceed to M3 design.
- [ ] At least one FAIL → write up in result.md and surface to user.
```

- [ ] **Step 2: Create the result document stub at `docs/superpowers/research/2026-05-08-m2-webxr-result/result.md`**

Write the file with these exact contents:

```markdown
# M2 WebXR 180° SBS — result

**Date run:** _(fill in here)_
**Stash-vr commit:** _(fill in here — run `git rev-parse --short HEAD`)_
**Quest 3 firmware:** _(fill in here)_
**Meta Browser version:** _(fill in here)_
**Library size at time of run:** _(fill in here)_ scenes total, _(fill in here)_ tagged DOME+SBS

## Per-criterion results

Copy from `checklist.md` after running on the headset.

## Surprises / observations

(Free-form. Performance notes — frame drops at 8K? — autoplay-policy quirks, half-sphere orientation issues, stereo-component initialization warnings, anything else load-bearing for M3.)

_(fill in here)_

## Recommendation

- [ ] All PASS → green-light M3 (multi-format VR projections) design session.
- [ ] FAIL — re-spec needed because: _(fill in here)_

## Open M3 inputs from this milestone

(Things we learned during M2 that should inform M3 design — e.g., "rotation 0 90 0 was wrong, should be 0 -90 0", "stereo component v1.4.0 didn't initialize cleanly on A-Frame 1.7, had to fall back to vN", "8K SBS dropped frames; M3 should default to a transcoded resolution".)

_(fill in here)_
```

- [ ] **Step 3: Verify the files were created and contain content**

Run:
```
findstr /C:"M2 WebXR" docs/superpowers/research/2026-05-08-m2-webxr-result/checklist.md
findstr /C:"Recommendation" docs/superpowers/research/2026-05-08-m2-webxr-result/result.md
```

Expected: each command returns one matching line.

- [ ] **Step 4: Commit**

```
git add docs/superpowers/research/2026-05-08-m2-webxr-result/checklist.md docs/superpowers/research/2026-05-08-m2-webxr-result/result.md
git commit -m "browse: M2 validation checklist and result stub"
```

- [ ] **Step 5: HAND-OFF — user runs the checklist on Quest 3**

Stop here. Do NOT proceed to mark the entire plan complete.

Tell the user:

> "Implementation complete. Please put on the Quest 3, open `https://stash-vr.duckdns.org/browse` in Meta Browser, navigate to a scene tagged `DOME` + `SBS`, and run through `docs/superpowers/research/2026-05-08-m2-webxr-result/checklist.md`. Fill in `result.md` with the outcomes — especially anything in the 'Surprises' or 'half-sphere orientation' areas, since those are the things most likely to need adjustment. If anything fails or surprises you, paste the result and we'll triage before moving to M3."

---

## Risk handling reminders (from spec § 11)

These come from the M2 spec. Surface to the user *before* changing scope:

- **`aframe-stereo-component@1.4.0` may not initialize cleanly on A-Frame 1.7.** The component was last published in 2019 against A-Frame 0.x. If you see console errors like "stereo component schema invalid" or the geometry entities render mono, fall back: replace the two `<a-entity stereo="eye:left|right;...">` with two equivalent entities that use `visible-cam="left|right"` (a custom 30-line component you write inline). DO NOT proceed with broken stereo.
- **Half-sphere geometry orientation.** `rotation="0 90 0"` is a guess. If the user reports the front of the half-sphere isn't where the camera looks (looking forward shows black, video is to the side), try -90 or 0 and re-test on the headset.
- **`<a-videosphere src="#sceneVideo">` referencing an outside-of-`<a-assets>` element.** A-Frame 1.x supports this, but if the texture doesn't bind (sphere renders solid grey or untextured), fall back: wrap the existing `<video>` in `<a-assets>` (it will become invisible to the 2D mode, requiring a separate 2D `<video>` element — at the cost of duplicate playback state, which is the spec § 11's "if continuity breaks" fallback).
- **8K SBS frame drops.** Document and move on. M3 problem.
- **Autoplay policy.** WebXR session entry IS a user gesture, so playback should be allowed. If it hiccups, user pauses/plays via the headset's controller-summoned 2D bar.
- **Existing 2D mutations.** All M1 forms are unchanged; if any of them break, that's a regression caused by the template diff, not the VR feature itself — re-check Task 3 Step 3's surrounding markup.

---

## Self-review (writer's check, run after writing each task above)

This is the writer's checklist — ignore if you're the executor.

1. **Spec coverage:**
   - § 2 success criterion 1 (button on DOME+SBS scenes) — Task 2 detection + Task 3 template gate.
   - § 2 success criterion 2 (button absent on non-VR scenes) — Task 3 `{{if .IsVR180SBS}}` gate.
   - § 2 success criterion 3 (Enter VR hides 2D, shows `<a-scene>`, requests immersive-vr session) — Task 3 inline JS.
   - § 2 success criterion 4 (stereo render at user's playback position) — Task 3 markup using `src="#sceneVideo"` shares the same `<video>` element; A-Frame's video texture follows currentTime automatically.
   - § 2 success criterion 5 (exit returns to 2D with state preserved) — Task 3 `exit-vr` event handler restores display; the `<video>` element survives the toggle, preserving currentTime/paused trivially.
   - § 2 success criterion 6 (M1 features still work) — Task 3 doesn't touch any M1 forms; Task 4 checklist regression-tests them.
   - § 9 file list — exact match: vendor/aframe.min.js, vendor/aframe-stereo-component.min.js, static.go, browse/data.go, browse/scene.go, browse_scene.gohtml.
   - § 10 validation plan — Task 4 captures the Quest 3 / Meta Browser checklist; Task 1 step 5 + Task 3 step 5 cover the curl-level checks.

2. **Placeholders:** None. Each step has actual code or a precise manual verification action. The few `_(fill in here)_` blanks in the checklist/result files are intentional user-fill blanks, not plan placeholders.

3. **Type consistency:**
   - `IsVR180SBS` field used consistently in data.go + scene.go + browse_scene.gohtml.
   - `id="sceneVideo"` used consistently between `<video>` and `material="src:#sceneVideo"`.
   - `id="enterVR"` used consistently between `<button>` and JS handler.
   - `id="vrScene"` used consistently between `<a-scene>` and JS handler.
   - URL path `/vendor/*.js` used consistently in template + curl verification.
   - File path `internal/static/vendor/*.js` used consistently in `mkdir`, `curl`, `embed.FS` glob.
   - `apiinternal.TagVR_DOME` / `apiinternal.TagVR_SBS` — alias matches the existing import in scene.go.

4. **Ambiguity:**
   - "Final stereo-component API depends on the version we vendor" — disclaimed in spec § 4 and surfaced as a risk in Task 3 step 7 + the risks section. Plan commits to v1.4.0 with a fallback path on broken init.
   - Half-sphere `rotation` value — committed to `0 90 0` with a debug-on-headset note. Final value confirmed during Quest 3 validation in Task 4.
   - Texture binding via outside-`<a-assets>` `#id` reference — committed with a fallback recipe in the Risk section.
