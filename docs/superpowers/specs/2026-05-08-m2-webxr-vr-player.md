# M2 design: WebXR 180° SBS VR player on /browse/scene/{id}

**Date:** 2026-05-08
**Status:** Approved by user (`/brainstorming` session 2026-05-08).
**Predecessor:** [M1 spec](2026-05-08-m1-browse-2d-player-search.md) — shipped, validated on Quest 3 / Meta Browser.
**Successor:** M3 = multi-format VR (FISHEYE, MKX200, RF52, SPHERE 360°, TB).
M4 = sync polish + decisions on legacy `/deovr`, `/heresphere`. Each gets its
own spec/plan/implementation cycle later.

---

## 1. Context (why this milestone)

Earlier sessions established and ratified in M1:

1. WebXR works in Quest's **Meta Browser** but NOT in DeoVR's in-VR Chromium
   webview. Forward UX target is therefore Meta Browser. Empirical retest
   2026-05-08 against
   `https://immersive-web.github.io/webxr-samples/immersive-vr-session.html`.
2. SLR's right-panel UX cannot be replicated from any URL we control —
   DeoVR's "SLR mode" is hardcoded on the `sexlikereal.com` host name in the
   compiled app. Full evidence:
   [docs/superpowers/research/2026-05-08-slr-playback-hook/notes.md](../research/2026-05-08-slr-playback-hook/notes.md).
3. M1 closed the 2D-playback + catalog-search gaps in `/browse`. The page is
   self-sufficient inside Meta Browser today, except that VR-tagged scenes
   only play in 2D.

This milestone adds an **immersive-vr** mode to the existing scene detail
page, gated to scenes tagged `DOME` + `SBS`. The "extend `/browse`
incrementally" framing the user picked for M1 carries forward: server-rendered
Go templates throughout, no SPA framework, no build pipeline. A-Frame is
loaded as two vendored `<script>` tags only when the page actually exposes the
VR affordance.

## 2. Goal & non-goals

**Goal:** make `/browse/scene/{id}` render a 180° SBS scene in WebXR
immersive-vr mode on Quest 3 / Meta Browser, while preserving M1's 2D player
for the same scene.

**Success criteria (binary, manually verified on Quest 3 / Meta Browser):**

1. On a scene tagged `DOME` + `SBS`, scene detail shows the M1 2D player AND
   an "Enter VR" button below the video.
2. On a scene NOT tagged `DOME` + `SBS`, the page is identical to M1 — no
   "Enter VR" button rendered, no A-Frame scripts loaded.
3. Clicking "Enter VR" hides the 2D UI, reveals an `<a-scene>` half-sphere
   videosphere using the same `<video>` element as the texture source, and
   requests `navigator.xr.requestSession('immersive-vr')`.
4. Inside the VR session, the scene plays in stereo (left eye sees left half
   of the SBS texture, right eye sees right half) at the user's chosen
   playback position.
5. Exiting the VR session returns the page to its M1 layout. The `<video>`
   element's `currentTime` and `paused` state are preserved across the
   transition (both directions).
6. All M1 features still work on the page: 2D player, rating stars, favorite,
   tag add/remove, O-counter, organized toggle, search, sidebar, pagination.

**Non-goals (deferred to later milestones):**

- Other VR projections — FISHEYE, MKX200, RF52, SPHERE 360°, TB stereo. (M3.)
- VR-internal scrubbing UI, in-VR metadata overlay, in-VR funscript timeline,
  heatmap on a VR scrub bar. (M4 if at all.)
- Watch-resume / continue-watching. (M4.)
- Resolution selector / quality switching. HTML5 `<video>` doesn't support
  in-player switching without HLS/DASH; Stash doesn't segment by default.
- Decisions on the fate of `/deovr` and `/heresphere` JSON endpoints. They
  stay running.
- Multi-select facets in the sidebar. (Out of scope, possibly never.)
- Performance tuning for 8K streams. We test as-is on the user's library; if
  frames drop on 8K, we document and move on.

## 3. UI / UX flow

1. User opens `https://stash-vr.duckdns.org/browse/scene/{id}` in Quest's
   Meta Browser. Page renders the M1 2D layout: thumbnail-poster `<video>`
   at top with browser-native controls, then title, metadata, mutation
   buttons.
2. If the scene is VR-eligible (tag check from § 5), a single button is
   rendered below the `<video>`:
   ```
   ▥ Enter VR
   ```
3. User clicks the button. JS handler:
   - Sets `display: none` on the existing `.wrap > *` blocks except the
     `<a-scene>` overlay (or specifically: hides the 2D player + metadata
     container, leaves the `<a-scene>` visible in its full-viewport form).
   - Calls `aframe`'s scene `enterVR()`. A-Frame internally drives
     `navigator.xr.requestSession('immersive-vr')`. The user's click counts
     as the gesture WebXR requires.
4. The headset enters VR. The user sees a half-sphere with the video
   texture-mapped, split into left and right eye views by the stereo
   component. Default head-tracking lets them look around the 180°.
5. To exit, the user invokes Meta Browser's "Exit VR" UI (system-level
   close button on the WebXR overlay) OR removes the headset (Quest's
   proximity sensor ends the session). A-Frame fires `exit-vr` event.
6. JS handler restores `display` on the 2D blocks, sets `display: none` on
   `<a-scene>`. The `<video>` element survived in the DOM; its `currentTime`
   and `paused` state are intact.

**No "Exit VR" button is rendered as 2D HTML.** Once in immersive-vr,
2D HTML is not visible. Exit is via the headset's system overlay (always
available) or by removing the headset. This mirrors how every other WebXR
demo ships.

**End-of-video behavior:** the video stops. No auto-next, no loop. Same as
M1.

## 4. VR scene structure

The `<a-scene>` block lives inside the `{{if .IsVR180SBS}}` template guard,
so non-VR scenes never see the markup or the script tags.

```html
<video id="sceneVideo" class="player" controls playsinline preload="metadata"
       src="{{.DirectStreamURL}}"
       {{if .ThumbnailURL}}poster="{{.ThumbnailURL}}"{{end}}></video>

{{if .IsVR180SBS}}
<button id="enterVR" class="btn-vr" type="button">▥ Enter VR</button>

<a-scene id="vrScene" embedded
         style="display:none; position:fixed; inset:0; width:100vw; height:100vh; z-index:10"
         vr-mode-ui="enabled: true"
         loading-screen="enabled: false"
         renderer="antialias: true">
  <a-entity camera stereo-cam="eye:left"></a-entity>
  <a-videosphere src="#sceneVideo" stereo="eye:left;mode:half"
                 rotation="0 -90 0" radius="100"></a-videosphere>
  <a-videosphere src="#sceneVideo" stereo="eye:right;mode:half"
                 rotation="0 -90 0" radius="100"></a-videosphere>
</a-scene>

<script src="/vendor/aframe.min.js"></script>
<script src="/vendor/aframe-stereo-component.min.js"></script>
<script>
  (function(){
    const btn   = document.getElementById('enterVR');
    const scene = document.getElementById('vrScene');
    const wrap  = document.querySelector('.wrap');
    const video = document.getElementById('sceneVideo');

    function show2D() {
      scene.style.display = 'none';
      // Restore inline children of .wrap (we hid them all on enter).
      [...wrap.children].forEach(el => { el.style.display = ''; });
    }
    function hide2D() {
      [...wrap.children].forEach(el => {
        if (el !== scene) el.style.display = 'none';
      });
      scene.style.display = '';
    }

    btn.addEventListener('click', () => {
      hide2D();
      // A-Frame's enterVR() returns a Promise; bubbles a user gesture.
      scene.enterVR().catch(err => {
        console.warn('enterVR failed', err);
        show2D();
      });
    });
    scene.addEventListener('exit-vr', show2D);
  })();
</script>
{{end}}
```

**Key design points:**

- **Single shared `<video>` element.** A-Frame's `<a-videosphere src="#sceneVideo">`
  references the existing element by id. There is exactly one media stream
  for both 2D and VR; `currentTime` and `paused` carry across automatically.
- **Two `<a-videosphere>` entities, one per eye.** The
  `aframe-stereo-component` reads `stereo="eye:left;mode:half"` and
  configures the entity to render only on the left-eye camera with UV
  coordinates pointing at the left half of the texture. Same logic mirrored
  for `right`.
- **`rotation="0 -90 0"`.** Half-sphere geometry by default faces +Z; for
  the user's straight-ahead look to align with the centre of the SBS texture,
  we rotate −90° around Y. Final exact value confirmed during implementation.
- **`radius="100"`.** Standard WebXR convention — large enough that the user
  feels enveloped without depth cues from the geometry edge.
- **No A-Frame loading screen, no in-scene cursor.** We're not building a
  pointer UX inside VR for M2.
- **Exact `aframe-stereo-component` API** depends on the version we vendor.
  The two-`<a-videosphere>` pattern shown is the most common; some versions
  bundle `<a-videosphere>` with a single `stereo` mixin that internally
  duplicates. The implementation plan locks the exact version and pattern.

## 5. Detection (server)

The server checks `vd.SceneParts.Tags` for both `DOME` and `SBS` and sets a
single boolean on `SceneDetailData`:

```go
// in internal/api/browse/scene.go, after the existing tag loop
hasDome, hasSBS := false, false
for _, t := range vd.SceneParts.Tags {
    if t == nil {
        continue
    }
    if t.TagParts.Name == internal.TagVR_DOME {
        hasDome = true
    }
    if t.TagParts.Name == internal.TagVR_SBS {
        hasSBS = true
    }
}
data.IsVR180SBS = hasDome && hasSBS
```

**Constants reused:** `internal.TagVR_DOME` and `internal.TagVR_SBS` from
[internal/api/internal/legend.go](../../../internal/api/internal/legend.go) —
already used by `/deovr` and `/heresphere` for the same purpose. Same
contract, no drift.

**No alias matching.** `set3DFormat` in `internal/api/deovr/videodata.go`
uses `util.StrSliceEquals(t.Name, t.Aliases, internal.TagVR_DOME)` which
matches either the tag name or any of its aliases. M2 checks the name only,
matching what the user has on their library. If alias support is needed
later, switching to `util.StrSliceEquals` is one line.

**No ancestor-tag walking.** The existing scene detail loop already skips
`prefix.SvrAncestor`-prefixed tags — we don't filter the VR detection on
ancestor status separately because `DOME` / `SBS` are leaf tags in the
user's library convention.

## 6. Library hosting (vendor)

A-Frame and the stereo component live under `internal/static/vendor/` and
ship inside the Go binary via `embed.FS` — same mechanism as `icon.png`,
`browse.gohtml`, `browse_scene.gohtml`.

**Files added:**
- `internal/static/vendor/aframe.min.js` — A-Frame production build,
  ~500 KB. Pinned version (1.7.x at time of writing; final version locked
  in the implementation plan).
- `internal/static/vendor/aframe-stereo-component.min.js` — `oscarmarinmiro/aframe-stereo-component`
  minified build, ~10 KB. Final version locked in the plan.

**Embed glob:** the existing `embed.FS` in
[internal/static/static.go](../../../internal/static/static.go) currently
declares `//go:embed *.gohtml *.html *.png`. Extend the directive to
include the new vendor subtree:

```go
//go:embed *.gohtml *.html *.png vendor/*.js
```

**Routing: no change.** [internal/api/router.go:41](../../../internal/api/router.go#L41)
already mounts the embed.FS at the URL root via
`router.Get("/*", http.FileServerFS(static.Fs).ServeHTTP)`. Once the
vendor files are embedded, they are reachable at `/vendor/aframe.min.js`
and `/vendor/aframe-stereo-component.min.js` automatically. No chi route
needs to be added.

**Why vendored, not CDN:**
- Self-contained. No headset-side internet dependency at page-load time.
- Deterministic. The user's binary is reproducible without external state.
- Supply-chain isolation. We ship a known-hash blob.
- Cost: ~510 KB binary growth, paid once. The user has explicitly accepted
  this trade-off.

## 7. What gets removed

Nothing. M2 is purely additive. The 2D player, mutation handlers, sidebar,
search — all unchanged. M1's surfaces stay byte-for-byte identical for
non-VR-tagged scenes.

## 8. What stays untouched

- All M1 surfaces: scene grid, sidebar, search, pagination, scene-detail
  metadata, mutation forms.
- `/deovr`, `/heresphere`, all JSON endpoints. M2 doesn't touch them.
- `library.Service`, GraphQL client, generated bindings.
- `internal/api/internal/legend.go` — we read constants, don't add or
  modify them.
- `internal/api/deovr/videodata.go::set3DFormat` and the heresphere
  equivalent — M2's tag check is a parallel, simpler version that doesn't
  alter existing logic.
- HTTPS / Caddy / DuckDNS setup.
- Build flow (Go-only, no JS bundler, no asset pipeline).

## 9. Files touched

| File | Change |
|---|---|
| `internal/static/vendor/aframe.min.js` | **Add.** Vendored A-Frame ~500 KB. |
| `internal/static/vendor/aframe-stereo-component.min.js` | **Add.** ~10 KB stereo component. |
| `internal/static/static.go` | Extend `//go:embed` directive from `*.gohtml *.html *.png` to add `vendor/*.js`. |
| `internal/api/browse/data.go` | Add `SceneDetailData.IsVR180SBS bool`. |
| `internal/api/browse/scene.go` | Detect `TagVR_DOME` + `TagVR_SBS` after the existing tag loop, set `data.IsVR180SBS`. |
| `internal/static/browse_scene.gohtml` | Add `id="sceneVideo"` to the existing `<video>`. Add `{{if .IsVR180SBS}}` block: Enter VR button, `<a-scene>` markup, two `<script>` tags, ~30-line inline JS for toggle. Add CSS for `.btn-vr`. |

**No new packages, no new env vars, no genqlient changes, no new
dependencies in `go.mod`.**

## 10. Validation plan

Manual verification only, per [CLAUDE.md](../../../CLAUDE.md).

**Build-level (each task):**
- `go vet ./...` clean.
- `go build ./...` clean.

**Curl-level (catches regressions cheaply):**
- `curl -sI http://localhost:9666/vendor/aframe.min.js` returns 200,
  `Content-Type: application/javascript` (or `text/javascript`).
- `curl -s http://localhost:9666/browse/scene/<DOME-SBS-id>` HTML contains
  `id="enterVR"` and `<a-scene`.
- `curl -s http://localhost:9666/browse/scene/<2D-id>` HTML does NOT contain
  `id="enterVR"`, `<a-scene`, or `aframe.min.js`.
- `curl -s http://localhost:9666/browse/scene/<DOME-SBS-id> | findstr /C:"id=\"sceneVideo\""`
  matches one line.

**Quest 3 / Meta Browser (the actual UX validation):**
- Open `https://stash-vr.duckdns.org/browse`. Pick a scene known to be
  tagged `DOME` + `SBS`. Open it.
- 2D player visible, "Enter VR" button visible below.
- Click "Enter VR" — page swaps to immersive-vr.
- Verify stereo: each eye sees a different half of the texture (look at the
  scene with one eye closed, then the other). If both eyes see the same
  thing, stereo split is broken.
- Verify projection: looking left/right reveals the rest of the 180°
  field. Looking behind shows the back of the half-sphere (a black or empty
  region — not a mirror copy).
- Scrub forward in 2D before entering VR — VR starts at the new position.
- Exit VR (system overlay or remove headset). 2D player returns at the VR
  player's last position.
- Open a scene that is NOT tagged DOME+SBS. Verify no "Enter VR" button.
  Verify page source does not include `aframe.min.js`.
- Re-test M1 mutations on the VR-eligible scene: rating, favorite, tag
  add/remove, O+/-, organized — all should still work because their forms
  are unchanged.

**Validation artifact:** `docs/superpowers/research/2026-05-08-m2-webxr-result/checklist.md`
+ `result.md`, same template as M1's `2026-05-08-m1-browse-result/`.

## 11. Risks (small but worth flagging)

- **8K SBS texture upload at 90 Hz.** Quest 3's GPU may drop frames on very
  large videos. Document if observed; 4K should be fine. Drop-resolution is
  a Stash-side concern (transcoding profile), not stash-vr's.
- **`aframe-stereo-component` API surface drift.** The component is
  community-maintained; the exact attribute API (`stereo="eye:left;mode:half"`
  shown in § 4) may differ from the version we vendor. Implementation plan
  pins the version and confirms the API by reading the vendored file before
  templating it.
- **Half-sphere geometry orientation.** A-Frame's default sphere geometry's
  +Z axis may not align with what the user perceives as "forward". The
  `rotation="0 -90 0"` in § 4 is a guess; final value adjusted on-headset
  during implementation.
- **`<a-videosphere>` rendering inside-out.** Common gotcha — sphere
  geometry's normals face outward by default, but we're inside the sphere.
  A-Frame's `<a-videosphere>` already inverts; if the stereo component
  re-inverts, we may need a `scale="-1 1 1"` workaround.
- **Autoplay policy in VR.** WebXR session entry IS a user gesture.
  Playback should be allowed. If it hiccups, user pauses/plays via the
  headset's controller-summoned 2D bar (rendered by the OS / browser, not
  by us).
- **Existing 2D player affected by hide/show toggle.** The toggle uses
  `display:none` rather than removing/re-mounting. The `<video>` element
  remains in the DOM the entire time, so `currentTime` and `paused`
  preserve trivially. If we accidentally re-mount, state is lost — the
  Implementation plan keeps the element stable.
- **Meta Browser console errors on A-Frame load.** If Meta Browser's
  Chromium build ships a feature A-Frame's CDN doesn't account for (or
  vice versa), we'll see startup errors. Cheap to spot — `console.log` in
  the inline script at toggle time. Failure mode: VR doesn't enter, 2D
  still plays, user reports error.

## 12. After this milestone

If M2 ships green, M3 starts: extend the projection support beyond DOME+SBS.
Working hypothesis is that the same `<a-scene>` markup pattern generalizes
to FISHEYE, MKX200, RF52, SPHERE 360°, TB stereo with different geometry +
stereo-component flags. Each format gets a tag check (we already have the
constants) and either a different `<a-videosphere>` configuration or a
custom geometry primitive. M3 may also introduce a single VR-format dropdown
for users to override mis-tagged scenes.

If M2 surfaces something unexpected — frames drop catastrophically on 8K,
A-Frame fails to enter immersive-vr from Meta Browser, stereo component is
incompatible with this build of A-Frame — we pause and re-spec before M3.
