# Spike design: WebXR VR player feasibility

**Date:** 2026-05-08
**Status:** Approved by user (`/brainstorming` session 2026-05-08).
**Successor:** This is a SPIKE. If it passes, a separate design session decomposes the full SLR-clone project into milestones. If it fails, fall back to the deovr.com-style flat-library design (notes inline at end of this doc).
**Time-box:** 2 days hard cap, regardless of progress.

---

## 1. Context (why this spike)

The user's headset is a Meta Quest 3 running the DeoVR app. stash-vr currently
serves DeoVR's "JSON mode" library at `/deovr`. With auto-section explosion
(per-performer, per-tag) the library renderer is unusable: 400+ packed sections
horizontally, plus CJK names rendering as tofu glyphs because the renderer's
font lacks CJK coverage.

The user's intuition was "match SLR's UX, including the right-side filter panel
(tags / performer / studio drill-down)." Two rounds of investigation establish
that this is **structurally not reachable** through any URL we control:

1. **DeoVR has four hardcoded modes** (forum thread 7896, post #2, verbatim:
   *"DeoVR can operate in four modes — SLR, DeoVR, Json and JillVR. All but
   Json modes are determined by the URL and are hardcoded"*). SLR mode keys
   off the `sexlikereal.com` host name in DeoVR's compiled binary. We cannot
   trigger it for `stash-vr.duckdns.org`.
2. **SLR's HTML payload contains zero DeoVR-specific markup.** Their right-
   panel UX is rendered by SLR's own ~413 KB Astro/Solid SPA + WebGL+WebXR
   player ([scripts/slr_videoplayer.js](../../../scripts/slr_videoplayer.js)).
   The only DeoVR string in all of SLR's production JS is a tooltip at line
   15212 (`"Open sexlikereal.com in DeoVR app or Meta/Safari browser"`). SLR
   is not "using a DeoVR HTML hook" — they are running their own self-
   contained webapp inside Quest's plain Chromium webview, using the open
   WebXR standard.
3. **DeoVR's JSON schema** ([hzrd149/deovr-json-schema](https://github.com/hzrd149/deovr-json-schema))
   provably cannot express facets / hierarchy / pagination / per-section
   URLs. The only drill-down primitive is per-item `video_url`. deovr.com's
   own `/deovr` endpoint ships **one flat 41,957-item `Library` section** at
   scale because the schema offers no other choice.
4. **HTML→playback fallbacks that used to work are currently dead** on
   Quest 3 / DeoVR 15.3.3545 (forum thread 10601). The `deovr://` deeplink
   has been a no-op from the in-VR browser since 2020 (forum thread 80,
   never answered).

Full evidence with file/line citations:
- [docs/superpowers/research/2026-05-08-deovr-shape/notes.md](../research/2026-05-08-deovr-shape/notes.md)
- [docs/superpowers/research/2026-05-08-slr-playback-hook/notes.md](../research/2026-05-08-slr-playback-hook/notes.md)

The conclusion: replicating SLR's UX **requires** building a self-contained
WebXR webapp like SLR did. There is no HTML-side opt-in to DeoVR's player.
This is a substantial project (~weeks-to-months for a basic version,
indefinite for parity). Before committing, we run a focused spike that
answers exactly one question: **does WebXR even work in our target browser
on Quest 3, and can we render an SBS 180° VR video inside it?**

## 2. Goal & success criteria

**Goal:** in 1–2 days max, prove or disprove that stash-vr can serve an
HTML page that renders a 180° SBS VR video on Quest 3 in WebXR immersive
mode, with playback launching via an "Enter VR" button.

**Five binary decision criteria, evaluated in order on Quest 3 hardware:**

1. WebXR `immersive-vr` session entry succeeds in our chosen target browser.
2. Stash-vr serves an HTML page at the new `/spike/{sceneId}` route, and the
   target browser renders it without crashing or showing blank.
3. The video element pulls byte-range frames from Stash's direct stream
   (the URL stash-vr already builds via `stash.GetDirectStream`) and starts
   playing — both audio and video.
4. L/R stereo geometry is correct: each eye sees its half of the SBS frame,
   no cross-eye, no inverted depth.
5. Head-tracking is smooth at the headset's refresh rate (90 Hz expected).
   No obvious stutter, no thermal warning over a 2-minute observation
   window.

**If all 5 pass →** green-light a separate design session that decomposes
the full SLR-clone project into milestones (data API, faceted UI, scene
grid, multi-format player, funscript, etc.). That session is NOT in scope
for this spike.

**If any fail →** diagnose, decide if the failure is fixable in <1 more
day. If not, stop and fall back to the deovr.com-style flat-library design
(see § 8).

**Hard time-box: 2 days.** After 2 days regardless of progress, stop and
assess.

## 3. Step 0: de-risk WebXR availability before any code

Two webviews on Quest 3 are candidates for rendering this:

- **DeoVR app's in-VR Chromium browser** — the handoff confirmed this is
  Chromium 144, but WebXR availability in DeoVR's webview build is *unknown*
  and could be the fundamental blocker.
- **Quest's Meta Browser** — confirmed full WebXR support; this is what
  every WebVR demo on the public web targets.

Before any code: navigate the headset to a public WebXR demo
(`https://immersive-web.github.io/webxr-samples/`, specifically the
`immersive-vr-session.html` sample) from each browser and confirm
immersive-vr session entry succeeds.

Five-minute test, no code. Outcomes:

- **Works in DeoVR's in-VR browser →** ideal, that's our target.
- **Works only in Meta Browser →** spike still proceeds, target shifts to
  Meta Browser. Implication for the longer-term project: users would open
  stash-vr in Meta Browser rather than DeoVR. Different UX, may or may not
  be acceptable to the user — flag this at spike-end as a decision point.
- **Works in neither →** kill the spike now, fall back to flat-library
  design.

## 4. What we build

1. **One new HTTP route on stash-vr:** `GET /spike/{sceneId}`.
   - Serves a Go-templated HTML page with the sceneId baked into a `<script>`
     tag as a JS const.
   - Behind a `--ENABLE_SPIKE` (or `ENABLE_SPIKE=true` env) flag,
     default-off, so it can't accidentally ship to users.
   - Mounted alongside the existing `/deovr`, `/heresphere`, `/browse`
     routers in [internal/api/router.go](../../../internal/api/router.go).

2. **A-Frame from CDN, single `<script>` tag.**
   - Pin a specific version string in the `<script src=>` URL at
     implementation time (whatever the then-current stable A-Frame
     release is). Don't use floating tags like `latest`. No build pipeline,
     no bundler, no TypeScript. Pure HTML + inline JS.
   - Choice rationale: A-Frame is declarative WebXR (~500 KB), built on
     Three.js, has community components for stereo video already
     (e.g. [aframe-stereo-component](https://github.com/oscarmarinmiro/aframe-stereo-component)
     or equivalent). Three.js direct would also work but requires more
     glue for a spike. If the spike passes, the full project may pivot to
     Three.js direct or keep A-Frame — that's a later decision.

3. **The page calls `/deovr/{id}`** (existing JSON endpoint at
   [internal/api/deovr/videodata.go](../../../internal/api/deovr/videodata.go))
   to fetch:
   - `encodings[].videoSources[].url` — direct stream URL (already built
     with API key appended via `stash.ApiKeyed`).
   - `screenType` — projection ("dome" for 180° half-sphere is the spike's
     target).
   - `stereoMode` — "sbs" for the spike's target.
   - `videoLength` — total duration.

   Reusing the existing endpoint avoids duplicating Stash query logic and
   keeps the spike's blast radius small.

4. **A-Frame scene** (declarative HTML, not procedural JS where possible):
   - For `screenType=dome` + `stereoMode=sbs`: render an inverted half-
     sphere geometry textured with a `<video>` element, with a stereo
     component that maps the left half of the frame to the left eye's
     camera and the right half to the right eye's camera.
   - Component pick: use [aframe-stereo-component](https://github.com/oscarmarinmiro/aframe-stereo-component)
     unless its API doesn't fit; fallback to a custom 30-line component
     that sets `THREE.Layers` per-eye (the standard pattern).
   - Black scene background, no skybox, nothing else in the world.

5. **"Enter VR" button** uses A-Frame's built-in `<a-scene vr-mode-ui>`
   default UI button, which calls `xrSession.requestSession('immersive-vr')`.
   No custom button needed.

6. **Asset placement:**
   - HTML template: a new file under [internal/static/](../../../internal/static/),
     e.g. `spike.gohtml`, embedded via the existing `embed.FS`.
   - No standalone JS file. All JS inline in the HTML for the spike.
   - Direct stream URL is already same-origin'able via the existing setup;
     if Stash's CORS is an issue, document it and we'll address (this is a
     known constraint, not a spike question to re-litigate).

7. **No UI polish.** Black background. Default A-Frame "Enter VR" button.
   Video controls hidden once in VR. Don't even style the page.

## 5. What we don't build (deferred to full project if spike passes)

Explicit non-goals — do NOT scope-creep into these during the spike:

- Faceted filter panel (tags / performers / studios drill-down).
- Scene grid, browse page, search, infinite scroll.
- Other projection formats: FISHEYE, MKX200, RF52, SPHERE 360°, CUBEMAP,
  EAC. (The user confirmed 180° SBS is the dominant format and the spike
  validates only this one.)
- Top-bottom (TB) stereo. (Same SBS pattern with a 90° axis change; trivial
  to add later if needed but not part of the spike.)
- Monoscopic (non-stereo) playback.
- 2D fallback player for desktop / non-VR browsers.
- Funscript / interactive timeline.
- Heatmap overlay (the existing stash-vr feature).
- In-VR controls UI: play/pause, seek scrubber, skip-intro buttons. The
  spike just plays from start; user takes off the headset to stop.
- Build pipeline, TypeScript, Vite, asset bundling, hot reload.
- Authentication, configuration UI, multi-user support.
- Tests. (Per [CLAUDE.md](../../../CLAUDE.md), the project has no test
  suite. The spike is verified by manual headset checklist, § 6.)
- HereSphere `/heresphere` changes.
- Changes to `/deovr` JSON output.
- Changes to auto-section logic.

If the spike passes, those non-goals become candidate milestones in the
follow-up decomposition session, not in this spike's scope.

## 6. Validation procedure

For the spike to count as "pass," each criterion below is verified live on
Quest 3 hardware. The implementation will produce a one-page checklist
(saved alongside the spike output) the user runs through on the headset.

| # | Criterion | How to verify |
|---|---|---|
| 0 | WebXR available in target browser | Step-0 test § 3: `immersive-web.github.io/webxr-samples/immersive-vr-session.html` enters immersive mode |
| 1 | HTML route serves cleanly | `curl https://stash-vr.duckdns.org/spike/{id}` from desktop returns 200 + non-empty HTML; reload from headset browser shows page without errors |
| 2 | Video starts playing in 2D mode | Audio audible and frames visible BEFORE clicking Enter VR (proves byte-range stream works) |
| 3 | Immersive-vr enters | "Enter VR" button → headset transitions to fully immersive stereo view |
| 4 | Stereo geometry correct | Manually close one eye: other eye sees a coherent monoscopic image (no SBS split visible). Open both eyes: depth feels natural, not inverted. |
| 5 | Smooth at 90 Hz | Head-tracking lag is imperceptible. No visible stutter. No thermal warning over a 2-minute continuous-playback observation window. |

If a criterion partially passes (e.g. stereo correct but there's mild
stutter), document the partial outcome and let the user decide whether to
treat it as pass or fail. The spike is a binary decision aid; ambiguous
results need user judgment.

## 7. Risks (specific to the spike, not the full project)

- **WebXR not available in DeoVR's in-VR browser.** Mitigation: Step 0
  surfaces this in 5 minutes. Pivot target browser to Meta Browser if
  needed (and surface the implication to the user).
- **CORS / same-origin issues fetching Stash's direct stream from the
  spike page.** Stash-vr already proxies covers via `/cover/{id}`; if the
  video stream has the same problem, document it and consider a `/stream/
  {id}` proxy as part of the spike (a few lines, reusing existing patterns).
- **A-Frame's stereo-component doesn't quite work for the user's specific
  SBS encoding.** Mitigation: ~30 lines of Three.js Layers code is the
  standard fallback. Time-boxed to 2 hours; if neither works, escalate.
- **Quest 3 thermal throttling on 8K direct streams.** Known constraint
  from prior work (commit `ae5c6f2` ordering). The spike uses the user's
  existing direct-stream URL, so it inherits the same behavior. If
  thermals tank within the 2-minute observation window, that's
  load-bearing data for the full-project decomposition.
- **Two-day cap not enough.** If on day 2 we're 90% there but not
  passing all five criteria, the user decides: extend or stop. Default is
  stop.

## 8. Fallback plan if the spike fails

If the spike fails (any of the 5 criteria, with no fix in scope), the
fallback design is the deovr.com-style flat-library, sketched here so we
don't lose it:

- `/deovr` emits the four already-implemented curated aggregates
  (Recently Added, Recently Played, Highly Rated, Unwatched, capped at
  `AggregateLimit`) plus each enabled saved Stash filter as one named
  section.
- If the user has zero saved filters, fall back to one flat `Library`
  section containing every VR scene (mirrors deovr.com's actual shape).
- Drop `auto:perf:*` and `auto:tag:*` auto-section types entirely.
- Drop env vars `AUTO_SECTIONS_PERFORMERS`, `AUTO_SECTIONS_TAGS`, and the
  `MIN_SCENES_PER_*` thresholds. Stale `config.json` records self-prune
  on next reconcile.
- Per-item shape stays at the deovr.com 4-key minimum.
- HereSphere inherits the same change automatically (same
  `library.Service.GetSections()` pipeline). Two-way tag sync surface
  untouched.

This fallback would get its own short design + plan if the spike fails;
not committing to it now.

## 9. Out-of-scope decisions explicitly deferred

The following are NOT decided in this spike and are intentionally pushed
to the follow-up session if the spike passes:

- Final framework choice (A-Frame vs Three.js vs Babylon vs other).
- Project structure for the SPA (single embedded bundle vs separate repo
  vs anything else).
- Authentication / multi-user.
- Funscript integration.
- Heatmap rendering inside VR.
- Format support beyond 180° SBS.
- Whether the full project lives inside stash-vr's Go binary or alongside
  it as a separate static-asset webapp.
- Migration path away from `/deovr` JSON. (Likely we keep `/deovr` JSON
  for users who want DeoVR's player AND ship the new webapp surface
  alongside, but that's not decided here.)

## 10. Files this spike will touch

Predicted, conservative:

- `internal/api/router.go` — mount the `/spike` subrouter.
- `internal/api/spike/router.go` — new file, ~20 lines.
- `internal/api/spike/page.go` — new file, ~30 lines (template handler).
- `internal/static/spike.gohtml` — new file, the HTML+A-Frame page.
- `internal/config/application.go` — new `EnableSpike` config field
  (env: `ENABLE_SPIKE`, default false).
- Possibly `internal/api/spike/stream.go` — new file, only if CORS/origin
  forces a `/spike/stream/{id}` proxy (uncertain until we test).

No changes to: `/deovr`, `/heresphere`, `/browse`, `library.Service`,
auto-section code, GraphQL documents, generated Stash client.

## 11. Spike output artifacts

Beyond the code itself, the spike produces:

- `docs/superpowers/research/2026-05-08-webxr-spike/checklist.md` — the
  one-page validation checklist for the user to run on the headset.
- `docs/superpowers/research/2026-05-08-webxr-spike/result.md` — outcome
  report (pass/fail per criterion, observations, recommendation for or
  against full-project green-light). Written after the user runs the
  checklist on the headset.

These artifacts exist regardless of pass or fail and are the input to the
next session.
