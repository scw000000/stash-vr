# WebXR VR Player Spike Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a minimal HTML page on stash-vr that renders a 180° SBS VR video on Quest 3 in WebXR immersive mode, behind an `ENABLE_SPIKE` flag, to answer the binary question: *can we run our own WebXR player inside Quest 3's webview?*

**Architecture:** A new `/spike/{sceneId}` chi subrouter mounted only when `ENABLE_SPIKE=true`. The handler renders one Go template (embedded in `static.Fs`) containing inline JS that calls the existing `/deovr/{id}` JSON endpoint to fetch the direct stream URL + projection metadata, then constructs an A-Frame scene with two videosphere entities (one per eye) using the third-party `aframe-stereo-component` for L/R layer routing. No build pipeline, no bundler — A-Frame and the stereo component load from CDN via pinned `<script>` tags.

**Tech Stack:** Go 1.24 (existing), chi/v5 (existing), `html/template` (existing pattern), embed.FS (existing pattern), [A-Frame 1.7.x](https://aframe.io/) via CDN, [aframe-stereo-component](https://github.com/oscarmarinmiro/aframe-stereo-component) via CDN.

**Spec:** [docs/superpowers/specs/2026-05-08-webxr-vr-player-spike-design.md](../specs/2026-05-08-webxr-vr-player-spike-design.md)

**Project conventions to honor:**
- No test suite per [CLAUDE.md](../../../CLAUDE.md). "TDD" here means manual verification after each change: `go vet ./...`, `go build ./...`, `curl`, then visual check.
- Lowercase commit prefixes following recent style: `spike: <message>`.
- Don't commit unless the user explicitly asks.

---

## File structure

**Created:**
- `internal/api/spike/router.go` — chi subrouter, `httpHandler` struct (mirrors `internal/api/browse/router.go`).
- `internal/api/spike/page.go` — handler that renders the template, baking `sceneId` into the page.
- `internal/static/spike.gohtml` — embedded HTML + inline JS + A-Frame entities.
- `docs/superpowers/research/2026-05-08-webxr-spike/checklist.md` — one-page manual headset validation checklist.
- `docs/superpowers/research/2026-05-08-webxr-spike/result.md` — outcome report stub (user fills in after running checklist).

**Modified:**
- `internal/api/router.go` — mount `/spike` subrouter only when config flag is on.
- `internal/config/application.go` — new `EnableSpike` field + `ENABLE_SPIKE` env / pflag.

**Not touched:** `/deovr`, `/heresphere`, `/browse`, `library.Service`, GraphQL, generated client, auto-section logic.

---

## Task 0: Pre-spike WebXR availability check (MANUAL — no code)

**Files:** none.

**Purpose:** § 3 of the spec. Determine whether the spike's target browser is DeoVR's in-VR browser or Quest's Meta Browser. Five-minute test, must happen before Task 1.

- [ ] **Step 1: On Quest 3, open DeoVR app → in-VR browser → navigate to `https://immersive-web.github.io/webxr-samples/immersive-vr-session.html`**

Click the "Enter VR" button. Expected outcomes:
- Button enabled and entry succeeds (full immersive view) → DeoVR's in-VR browser is the target. Note this in Step 3.
- Button disabled / fails → DeoVR's in-VR browser does NOT support WebXR. Continue to Step 2.

- [ ] **Step 2: On Quest 3, open Meta Browser → navigate to the same URL**

Click "Enter VR". Expected outcomes:
- Entry succeeds → Meta Browser is the target. Document this in Step 3 with the implication that users would open stash-vr in Meta Browser (different UX from DeoVR app).
- Entry also fails → kill spike. Skip remaining tasks; produce a one-line outcome at `docs/superpowers/research/2026-05-08-webxr-spike/result.md` saying "WebXR unavailable in both browsers; spike fails at Step 0". Surface to user; fall back to deovr.com-style flat library design (spec § 8).

- [ ] **Step 3: Record outcome in `docs/superpowers/research/2026-05-08-webxr-spike/step0.md`**

Create the file with one of these three exact contents:

```markdown
# Step 0 outcome

Target browser: DeoVR in-VR browser
WebXR immersive-vr session: confirmed working at immersive-web.github.io
Decision: proceed with Task 1
```

OR

```markdown
# Step 0 outcome

Target browser: Meta Browser
WebXR immersive-vr session: confirmed working at immersive-web.github.io
DeoVR in-VR browser: WebXR unavailable
Decision: proceed with Task 1; flag Meta-Browser-only target in result.md
```

OR

```markdown
# Step 0 outcome

Target browser: NONE
WebXR immersive-vr session: failed in both DeoVR and Meta Browser
Decision: spike FAILS at step 0; skip Tasks 1-7; fall back to flat-library design
```

- [ ] **Step 4: Commit (only if user requests)**

```
git add docs/superpowers/research/2026-05-08-webxr-spike/step0.md
git commit -m "spike: step 0 — WebXR availability check"
```

---

## Task 1: Add `EnableSpike` config flag

**Files:**
- Modify: `internal/config/application.go`

- [ ] **Step 1: Add the env key constant**

In [internal/config/application.go](../../../internal/config/application.go), inside the `const ( ... )` block at the top (around line 35, after `envKeyAggregateLimit`), add:

```go
	envKeyEnableSpike            = "ENABLE_SPIKE"
```

- [ ] **Step 2: Add the struct field**

In the `ApplicationConfig` struct (around line 63, after `AggregateLimit`), add:

```go
	EnableSpike            bool
```

- [ ] **Step 3: Register the pflag**

In `Init()`, after the `AggregateLimit` pflag block (around line 142), add:

```go
	pflag.Bool(envKeyEnableSpike, false, "Enable the /spike/{sceneId} WebXR player feasibility route (default off; only used during 2026-05-08 spike)")
	_ = viper.BindPFlag(envKeyEnableSpike, pflag.Lookup(envKeyEnableSpike))
```

- [ ] **Step 4: Wire the value into the struct**

After the `AggregateLimit` assignment in `Init()` (around line 180), add:

```go
	applicationConfig.EnableSpike = viper.GetBool(envKeyEnableSpike)
```

- [ ] **Step 5: Verify build**

Run:
```
go vet ./...
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Verify the flag is registered**

Run:
```
go run ./cmd/stash-vr --help 2>&1 | findstr ENABLE_SPIKE
```

Expected: a single line showing `--ENABLE_SPIKE` with the description text. (PowerShell uses `findstr`, not `grep`. On bash use `grep ENABLE_SPIKE`.)

- [ ] **Step 7: Commit (only if user requests)**

```
git add internal/config/application.go
git commit -m "config: add ENABLE_SPIKE flag for WebXR spike route"
```

---

## Task 2: Create the spike package skeleton (router + stub handler)

**Files:**
- Create: `internal/api/spike/router.go`
- Create: `internal/api/spike/page.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create `internal/api/spike/router.go`**

```go
package spike

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"stash-vr/internal/library"
)

type httpHandler struct {
	libraryService *library.Service
}

func Router(libraryService *library.Service) http.Handler {
	h := httpHandler{libraryService: libraryService}
	r := chi.NewRouter()
	r.Get("/{sceneId}", h.pageHandler)
	return r
}
```

- [ ] **Step 2: Create `internal/api/spike/page.go` with a stub handler**

```go
package spike

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *httpHandler) pageHandler(w http.ResponseWriter, r *http.Request) {
	sceneID := chi.URLParam(r, "sceneId")
	if sceneID == "" {
		http.NotFound(w, r)
		return
	}
	// Task 3 will replace this with template rendering.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("spike stub for sceneId=" + sceneID))
}
```

- [ ] **Step 3: Wire the spike subrouter into `internal/api/router.go`, gated by config**

In [internal/api/router.go](../../../internal/api/router.go):

(a) Add import:

```go
	"stash-vr/internal/api/spike"
```

(Place it alphabetically with the other `stash-vr/internal/api/*` imports, between `"stash-vr/internal/api/heresphere"` and `"stash-vr/internal/api/web"`.)

(b) After the existing `router.Mount("/browse", ...)` line (currently line 33), add:

```go
	if config.Application().EnableSpike {
		router.Mount("/spike", logMod("spike", spike.Router(libraryService)))
	}
```

- [ ] **Step 4: Verify build**

Run:
```
go vet ./...
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Verify the route is gated correctly (manual)**

In one terminal, start stash-vr WITHOUT the flag:
```
go run ./cmd/stash-vr --STASH_GRAPHQL_URL=<your stash graphql url>
```

In another terminal, curl the spike route:
```
curl -i http://localhost:9666/spike/1
```

Expected: HTTP 404 (route not mounted).

Stop stash-vr (Ctrl+C). Restart WITH the flag:
```
go run ./cmd/stash-vr --STASH_GRAPHQL_URL=<your stash graphql url> --ENABLE_SPIKE=true
```

Curl again:
```
curl -i http://localhost:9666/spike/1
```

Expected: HTTP 200 with body `spike stub for sceneId=1`.

- [ ] **Step 6: Commit (only if user requests)**

```
git add internal/api/spike/router.go internal/api/spike/page.go internal/api/router.go
git commit -m "spike: scaffold /spike subrouter behind ENABLE_SPIKE flag"
```

---

## Task 3: Render an HTML template with the scene ID

**Files:**
- Create: `internal/static/spike.gohtml`
- Modify: `internal/api/spike/page.go`

- [ ] **Step 1: Create `internal/static/spike.gohtml` (minimal page, A-Frame wired but no scene yet)**

Pin A-Frame 1.7.x — pick the latest 1.7.x release at implementation time and write the exact version string in the URL (do NOT use `latest`). At time of plan writing, `1.7.0` is the current stable; verify on https://aframe.io/releases/ before pasting.

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, user-scalable=no">
    <title>stash-vr WebXR spike — {{.SceneID}}</title>
    <script src="https://aframe.io/releases/1.7.0/aframe.min.js"></script>
    <style>
        body { margin: 0; background: #000; color: #ddd; font-family: sans-serif; }
        #info { padding: 12px; }
        #info code { color: #8af; }
        #status { color: #f88; }
    </style>
</head>
<body>
    <div id="info">
        <h1>stash-vr WebXR spike</h1>
        <p>Scene ID: <code>{{.SceneID}}</code></p>
        <p id="status">Initializing…</p>
    </div>

    <script>
        const SCENE_ID = {{ .SceneID | printf "%q" }};
        document.getElementById("status").textContent = "Loaded scene ID " + SCENE_ID + ". Task 4 will fetch metadata.";
    </script>
</body>
</html>
```

(`{{ .SceneID | printf "%q" }}` emits a properly JSON-escaped string literal — important if scene IDs ever contain quotes. The `static.Fs` embed already includes `*.gohtml`, so no embed change is needed.)

- [ ] **Step 2: Update `internal/api/spike/page.go` to render the template**

Replace the entire file with:

```go
package spike

import (
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/static"
)

var spikeTmpl = template.Must(template.ParseFS(static.Fs, "spike.gohtml"))

type pageData struct {
	SceneID string
}

func (h *httpHandler) pageHandler(w http.ResponseWriter, r *http.Request) {
	sceneID := chi.URLParam(r, "sceneId")
	if sceneID == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := spikeTmpl.Execute(w, pageData{SceneID: sceneID}); err != nil {
		log.Ctx(r.Context()).Err(err).Str("sceneId", sceneID).Msg("spike: render template")
	}
}
```

- [ ] **Step 3: Verify build**

```
go vet ./...
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Verify rendering (manual)**

Restart stash-vr with `--ENABLE_SPIKE=true`. Curl:
```
curl -s http://localhost:9666/spike/42 | findstr "Scene ID"
```

Expected: a line containing `Scene ID: <code>42</code>`.

In a desktop browser, open http://localhost:9666/spike/42 — expected: page title visible, "Loaded scene ID 42." status, no JS console errors related to A-Frame loading. (A-Frame may log VR-mode warnings on desktop; those are fine.)

- [ ] **Step 5: Commit (only if user requests)**

```
git add internal/static/spike.gohtml internal/api/spike/page.go
git commit -m "spike: render gohtml template with sceneId baked in"
```

---

## Task 4: Fetch /deovr metadata + start 2D playback

**Files:**
- Modify: `internal/static/spike.gohtml`

This task makes the video play in a regular `<video>` element on the page (no VR yet). Verifies that the `/deovr/{id}` JSON endpoint reachable, the direct stream URL works for byte-range, and audio+video frames render. This isolates one failure surface before adding A-Frame complexity.

- [ ] **Step 1: Replace `internal/static/spike.gohtml` with the version that fetches metadata and plays in 2D**

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, user-scalable=no">
    <title>stash-vr WebXR spike — {{.SceneID}}</title>
    <script src="https://aframe.io/releases/1.7.0/aframe.min.js"></script>
    <style>
        body { margin: 0; background: #000; color: #ddd; font-family: sans-serif; }
        #info { padding: 12px; }
        #info code { color: #8af; }
        #status { color: #f88; }
        #vrvideo { width: 50vw; max-width: 800px; background: #111; }
    </style>
</head>
<body>
    <div id="info">
        <h1>stash-vr WebXR spike</h1>
        <p>Scene ID: <code>{{.SceneID}}</code></p>
        <p>Title: <span id="title">…</span></p>
        <p>Projection: <code id="projection">…</code></p>
        <p id="status">Fetching metadata…</p>
        <video id="vrvideo" crossorigin="anonymous" playsinline preload="auto" controls></video>
    </div>

    <script>
        const SCENE_ID = {{ .SceneID | printf "%q" }};
        const statusEl = document.getElementById("status");
        const titleEl = document.getElementById("title");
        const projEl = document.getElementById("projection");
        const videoEl = document.getElementById("vrvideo");

        async function loadScene() {
            try {
                const resp = await fetch("/deovr/" + encodeURIComponent(SCENE_ID), { credentials: "omit" });
                if (!resp.ok) {
                    throw new Error("HTTP " + resp.status + " from /deovr/" + SCENE_ID);
                }
                const data = await resp.json();

                titleEl.textContent = data.title || "(no title)";
                projEl.textContent = "screenType=" + (data.screenType || "?") +
                                     " stereoMode=" + (data.stereoMode || "?") +
                                     " is3d=" + Boolean(data.is3d);

                // Pick the FIRST encoding's first source (direct-stream-first ordering
                // was established in commit ae5c6f2). encodings[0] = direct stream.
                const enc = (data.encodings && data.encodings[0]) || null;
                const src = enc && enc.videoSources && enc.videoSources[0] && enc.videoSources[0].url;
                if (!src) {
                    throw new Error("no video URL in /deovr response");
                }

                videoEl.src = src;
                statusEl.textContent = "Stream URL set. Press play on the video element.";

                // Stash projection metadata on window for Task 5 to use.
                window.SPIKE_META = {
                    sceneId: SCENE_ID,
                    title: data.title,
                    screenType: data.screenType,
                    stereoMode: data.stereoMode,
                    streamUrl: src,
                };
            } catch (err) {
                statusEl.textContent = "ERROR: " + err.message;
                console.error("spike: loadScene failed", err);
            }
        }

        loadScene();
    </script>
</body>
</html>
```

- [ ] **Step 2: Pick a real DOME or SBS scene ID for testing**

The spec § 6 (criteria 2 + 3) requires verifying audio + video on the user's library. Pick any real scene ID tagged DOME or SBS in the user's Stash. To find one:

```
curl -s http://localhost:9666/deovr | python -c "import sys,json; d=json.load(sys.stdin); ids=[v['id'] for s in d.get('scenes',[]) for v in s.get('list',[])][:5]; print(ids)"
```

Or open http://localhost:9666/browse in a desktop browser and copy a scene ID from the URL when clicking into one. Pick a small-bitrate scene if possible (faster iteration).

Record the chosen ID. The plan refers to it as `<TEST_SCENE_ID>` below.

- [ ] **Step 3: Verify build (no Go change, but template is reparsed at startup)**

```
go vet ./...
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Verify in desktop browser (criteria 2 + 3)**

Restart stash-vr with `--ENABLE_SPIKE=true`. Open http://localhost:9666/spike/<TEST_SCENE_ID> in a desktop Chromium.

Expected:
- Title and projection fields populate within ~1 second.
- Status: "Stream URL set. Press play on the video element."
- Click the video's play button. Audio plays. Video frames visible. Seeking via the timeline works (proves byte-range).
- DevTools Network tab: the request to `/deovr/<id>` returns 200 application/json. The video request returns 206 Partial Content from Stash.
- Console: no errors. (A-Frame may log component warnings; those are fine.)

If the video URL returns CORS errors when loaded from the browser (unlikely since stash-vr's `stash.ApiKeyed` URL points directly at Stash and the browser is loading the page from stash-vr — different origin from Stash):
- This is a *risk* called out in spec § 7. If it happens, do NOT inline-fix in this task. Stop, document the failure mode in `result.md`, and surface to the user before deciding whether to add a `/spike/stream/{id}` proxy.

- [ ] **Step 5: Commit (only if user requests)**

```
git add internal/static/spike.gohtml
git commit -m "spike: fetch /deovr metadata and play stream in 2D mode"
```

---

## Task 5: Wire stereo VR rendering with A-Frame

**Files:**
- Modify: `internal/static/spike.gohtml`

This task adds the actual A-Frame stereo videosphere and the WebXR Enter-VR button. Validates spec criteria 1 (WebXR enters), 4 (stereo correct), 5 (smooth at 90 Hz).

- [ ] **Step 1: Replace `internal/static/spike.gohtml` with the full stereo-videosphere version**

`aframe-stereo-component` is the third-party package that adds `stereo` attribute support (uses `THREE.Layers` to bind entities to per-eye cameras). Pin to a specific commit/version. At time of plan writing, the unpkg-served version is `aframe-stereo-component@1.0.7`; verify on https://github.com/oscarmarinmiro/aframe-stereo-component before pasting.

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, user-scalable=no">
    <title>stash-vr WebXR spike — {{.SceneID}}</title>
    <script src="https://aframe.io/releases/1.7.0/aframe.min.js"></script>
    <script src="https://unpkg.com/aframe-stereo-component@1.0.7/dist/aframe-stereo-component.min.js"></script>
    <style>
        html, body { margin: 0; padding: 0; background: #000; overflow: hidden; }
        #info { position: fixed; top: 0; left: 0; padding: 8px 12px; color: #ddd; font-family: sans-serif; font-size: 14px; z-index: 100; pointer-events: none; }
        #info code { color: #8af; }
        #status { color: #f88; }
        #playbtn { position: fixed; top: 50%; left: 50%; transform: translate(-50%,-50%); padding: 16px 32px; font-size: 18px; background: #245; color: #fff; border: 1px solid #468; border-radius: 4px; cursor: pointer; z-index: 100; }
        #playbtn:disabled { opacity: 0.5; cursor: default; }
    </style>
</head>
<body>
    <div id="info">
        <div>Scene <code>{{.SceneID}}</code> — <span id="title">…</span></div>
        <div id="projection">screenType=? stereoMode=?</div>
        <div id="status">Initializing…</div>
    </div>

    <button id="playbtn" disabled>Loading…</button>

    <a-scene id="ascene" vr-mode-ui="enabled: true" embedded background="color: #000" loading-screen="enabled: false" renderer="antialias: true; logarithmicDepthBuffer: true">
        <a-assets timeout="0">
            <video id="vrvideo" crossorigin="anonymous" playsinline preload="auto"></video>
        </a-assets>

        <a-camera id="cam" position="0 0 0" wasd-controls-enabled="false" look-controls></a-camera>

        <!--
            Two half-sphere geometries, one per eye. The 'stereo' component from
            aframe-stereo-component binds each entity to the corresponding camera
            layer (THREE.Layers). 'mode:half' means the texture is split L|R; the
            left-eye entity samples the left half of the frame, the right-eye
            entity samples the right half.

            Geometry: half-sphere (thetaLength=180) on the front hemisphere
            (phiStart=-90, phiLength=180). Inverted (scale -1 1 1) so the texture
            renders on the inside of the surface, facing the camera at the
            origin.

            Material: shader=flat (no lighting), src=#vrvideo (linked to the
            asset above), side=back (render only the inside surface), npot=true
            (allow non-power-of-two video textures).
        -->
        <a-entity id="leftSphere"
                  geometry="primitive: sphere; radius: 100; segmentsHeight: 64; segmentsWidth: 64; thetaLength: 180; phiStart: -90; phiLength: 180"
                  material="shader: flat; src: #vrvideo; side: back; npot: true"
                  scale="-1 1 1"
                  stereo="eye: left; mode: half"
                  visible="false"></a-entity>

        <a-entity id="rightSphere"
                  geometry="primitive: sphere; radius: 100; segmentsHeight: 64; segmentsWidth: 64; thetaLength: 180; phiStart: -90; phiLength: 180"
                  material="shader: flat; src: #vrvideo; side: back; npot: true"
                  scale="-1 1 1"
                  stereo="eye: right; mode: half"
                  visible="false"></a-entity>
    </a-scene>

    <script>
        const SCENE_ID = {{ .SceneID | printf "%q" }};
        const statusEl = document.getElementById("status");
        const titleEl = document.getElementById("title");
        const projEl = document.getElementById("projection");
        const videoEl = document.getElementById("vrvideo");
        const leftEl = document.getElementById("leftSphere");
        const rightEl = document.getElementById("rightSphere");
        const playBtn = document.getElementById("playbtn");

        async function loadScene() {
            try {
                const resp = await fetch("/deovr/" + encodeURIComponent(SCENE_ID), { credentials: "omit" });
                if (!resp.ok) {
                    throw new Error("HTTP " + resp.status + " from /deovr/" + SCENE_ID);
                }
                const data = await resp.json();

                titleEl.textContent = data.title || "(no title)";
                projEl.textContent = "screenType=" + (data.screenType || "?") +
                                     " stereoMode=" + (data.stereoMode || "?") +
                                     " is3d=" + Boolean(data.is3d);

                // Spike validates DOME + SBS only. Other projections fall through
                // with a warning rather than guessing.
                const sType = (data.screenType || "").toLowerCase();
                const sMode = (data.stereoMode || "").toLowerCase();
                if (sType !== "dome" || sMode !== "sbs") {
                    statusEl.textContent = "WARNING: this scene is " + sType + "/" + sMode +
                                           "; the spike only validates dome/sbs. Try another scene tagged DOME or SBS.";
                    // Continue anyway so the user can see what happens.
                }

                const enc = (data.encodings && data.encodings[0]) || null;
                const src = enc && enc.videoSources && enc.videoSources[0] && enc.videoSources[0].url;
                if (!src) {
                    throw new Error("no video URL in /deovr response");
                }

                videoEl.src = src;
                videoEl.load();

                // Reveal the spheres only after the video has enough data, so
                // we don't flash an empty texture.
                videoEl.addEventListener("loadeddata", () => {
                    leftEl.setAttribute("visible", "true");
                    rightEl.setAttribute("visible", "true");
                    playBtn.disabled = false;
                    playBtn.textContent = "Play (then click Enter VR ⏎)";
                    statusEl.textContent = "Ready. Press Play, then click the VR icon (bottom-right) to enter immersive mode.";
                }, { once: true });

                videoEl.addEventListener("error", (ev) => {
                    statusEl.textContent = "Video element error: " + (videoEl.error ? videoEl.error.message : "unknown");
                    console.error("spike: video error", videoEl.error, ev);
                });
            } catch (err) {
                statusEl.textContent = "ERROR: " + err.message;
                console.error("spike: loadScene failed", err);
            }
        }

        playBtn.addEventListener("click", async () => {
            try {
                await videoEl.play();
                playBtn.textContent = "Playing — click VR icon to enter";
            } catch (err) {
                statusEl.textContent = "play() failed: " + err.message;
                console.error("spike: play() failed", err);
            }
        });

        loadScene();
    </script>
</body>
</html>
```

- [ ] **Step 2: Verify build (template-only change, but reparsed at startup)**

```
go vet ./...
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Verify in desktop Chromium (sanity check before headset)**

Open http://localhost:9666/spike/<TEST_SCENE_ID> on desktop. Expected:
- "Ready. Press Play..." status.
- Click Play. Audio plays.
- Two videosphere geometries are visible in the A-Frame scene (you can drag-rotate with the mouse since we left `look-controls` enabled). One eye's view will look strange on a flat 2D screen — that's expected because each entity is bound to a per-eye camera layer; on flat-display fallback, you may see only one or only the unstereo'd composite. Don't overthink the desktop view; the headset is the real test.
- Console: no critical errors. Warnings about WebXR being unavailable on desktop are fine.

- [ ] **Step 4: Headset validation (the actual spike)**

Restart stash-vr listening on the LAN-reachable address (the user's existing setup uses `https://stash-vr.duckdns.org` which proxies to `:9666` via Caddy). On Quest 3, in the target browser identified in Task 0:

Navigate to `https://stash-vr.duckdns.org/spike/<TEST_SCENE_ID>`.

Run the validation checklist (Task 6) and record results.

- [ ] **Step 5: Commit (only if user requests)**

```
git add internal/static/spike.gohtml
git commit -m "spike: stereo videosphere via aframe-stereo-component for 180 SBS"
```

---

## Task 6: Write the headset validation checklist

**Files:**
- Create: `docs/superpowers/research/2026-05-08-webxr-spike/checklist.md`

- [ ] **Step 1: Create the checklist file**

```markdown
# WebXR spike — headset validation checklist

**Run on:** Quest 3 hardware, in the target browser identified in Step 0 (DeoVR in-VR browser or Meta Browser).

**URL to open:** `https://stash-vr.duckdns.org/spike/<TEST_SCENE_ID>`
(replace `<TEST_SCENE_ID>` with the DOME/SBS scene ID picked during Task 4)

For each criterion, mark PASS / FAIL / PARTIAL and add a one-line note. PARTIAL is fine — flag it for the next session to interpret.

## Criteria

- [ ] **Criterion 1: HTML route serves cleanly**
  - The page loads. Title bar shows the scene ID. "Ready" status appears within ~3 seconds of opening.
  - Result: PASS / FAIL / PARTIAL — note: ___

- [ ] **Criterion 2: Video plays in 2D before VR entry**
  - Click "Play". Audio is audible. Looking at the rendered A-Frame canvas, you can see motion (the videosphere texture updating). This proves byte-range streaming works.
  - Result: PASS / FAIL / PARTIAL — note: ___

- [ ] **Criterion 3: Immersive-vr session entry succeeds**
  - Click the VR icon (typically bottom-right of the A-Frame canvas).
  - Headset transitions to fully immersive view (you see the videosphere wrap around you).
  - Result: PASS / FAIL / PARTIAL — note: ___

- [ ] **Criterion 4: Stereo geometry is correct**
  - Close one eye. The other eye should see a coherent monoscopic image — NOT a side-by-side split with both halves visible.
  - Open both eyes. Depth should feel natural (foreground appears closer than background). NOT inverted (background appearing closer than foreground).
  - Result: PASS / FAIL / PARTIAL — note: ___

- [ ] **Criterion 5: Smooth at 90 Hz over 2 minutes**
  - Watch the video for at least 2 continuous minutes while moving your head.
  - Head-tracking lag should be imperceptible. No visible stutter. No thermal warning notification from the headset.
  - Result: PASS / FAIL / PARTIAL — note: ___

## Overall outcome

Mark one:
- [ ] All five PASS → green-light full project decomposition session.
- [ ] At least one FAIL → spike fails. Fall back to flat-library design (spec § 8).
- [ ] PARTIAL on at least one (no clear FAIL) → user judgment call. See `result.md`.

## Notes / surprises (free-form)

___
```

- [ ] **Step 2: Commit (only if user requests)**

```
git add docs/superpowers/research/2026-05-08-webxr-spike/checklist.md
git commit -m "spike: headset validation checklist"
```

---

## Task 7: Stub the result document

**Files:**
- Create: `docs/superpowers/research/2026-05-08-webxr-spike/result.md`

- [ ] **Step 1: Create the result stub**

```markdown
# WebXR spike — result

**Date run:** ___
**Stash-vr commit at time of run:** ___ (run `git rev-parse --short HEAD` and paste)
**Target browser (from Step 0):** ___
**Test scene ID:** ___
**Test scene tags / format:** ___ (DOME/SBS expected)

## Per-criterion results

Copy-paste from `checklist.md` after running on the headset.

| # | Criterion | Result | Note |
|---|---|---|---|
| 0 | WebXR available in target browser | | |
| 1 | HTML route serves cleanly | | |
| 2 | Video plays in 2D | | |
| 3 | Immersive-vr enters | | |
| 4 | Stereo geometry correct | | |
| 5 | Smooth at 90 Hz over 2 minutes | | |

## Overall verdict

- [ ] PASS — green-light full project. Schedule next design session to decompose milestones.
- [ ] FAIL — fall back to deovr.com-style flat library design (spec § 8).
- [ ] PARTIAL — user judgment required. Decision: ___

## Surprises / observations / risks for full project

(Free-form. Performance notes, format compatibility caveats, browser quirks, anything load-bearing for a follow-up decomposition session.)

___

## Recommendation for next session

(One paragraph. If PASS: what should the decomposition session prioritize first — data API, faceted UI, multi-format support? If FAIL: anything from the spike worth keeping, or full revert?)

___
```

- [ ] **Step 2: Commit (only if user requests)**

```
git add docs/superpowers/research/2026-05-08-webxr-spike/result.md
git commit -m "spike: result document stub for post-headset writeup"
```

---

## Risk handling reminders

These come from spec § 7 — surface them to the user *before* changing scope:

- **CORS on direct stream from spike page** (Task 4 Step 4): if Stash returns a CORS error when the spike page tries to load the stream URL, STOP. Do not add a proxy in this spike — document it as a finding in `result.md` and surface for user decision.
- **A-Frame stereo-component doesn't work as written** (Task 5 Step 1): if the videosphere texture doesn't split correctly between eyes, time-box one debugging attempt to 2 hours. If still broken, fall back to a custom THREE.Layers approach: the standard pattern is to set `entity.object3D.layers.set(1)` for the left-eye entity and `entity.object3D.layers.set(2)` for the right-eye entity, and ensure the corresponding camera enables those layers (A-Frame's WebXR camera assigns layer 1 to left eye, layer 2 to right). If THAT fails too, escalate.
- **WebXR enters but performance tanks** (Criterion 5): note specifics in `result.md` — file size, projection, decode bitrate. This is load-bearing data for the full project's player implementation.
- **Two-day cap reached without all criteria passing**: stop. Surface to user for extend-or-stop decision.

---

## Self-review (run after writing each task above)

This is the writer's checklist — ignore if you're the executor.

1. **Spec coverage:**
   - § 2 success criteria — covered by Tasks 4–6 (each criterion mapped to a verification step).
   - § 3 Step 0 — Task 0.
   - § 4 what we build — Tasks 1–5.
   - § 5 what we don't build — out of scope for plan; not a task.
   - § 6 validation procedure — Task 6.
   - § 7 risks — surfaced in "Risk handling reminders" above.
   - § 8 fallback plan — referenced in Task 0 and Task 6, but the fallback design itself is intentionally NOT planned here (spec § 8 says "would get its own short design + plan if the spike fails").
   - § 10 files-touched prediction — matches the file structure section above. The optional `internal/api/spike/stream.go` from § 10 is *not* a task here; it's gated on a Task 4 finding.
   - § 11 spike output artifacts — Tasks 6 and 7 (checklist.md + result.md). step0.md added in Task 0.

2. **Placeholders:** None. Each step has actual code or a precise manual verification action.

3. **Type consistency:** `httpHandler`, `pageData`, `SCENE_ID`, `SPIKE_META`, the env key `ENABLE_SPIKE`, the field `EnableSpike`, the route `/spike/{sceneId}` are all used consistently across tasks.

4. **Ambiguity:** A-Frame and stereo-component versions are pinned but with a "verify before pasting" instruction (CDN versions can move). Acceptable given this is a spike, not a long-lived release.
