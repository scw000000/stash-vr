# Handoff: M1 shipped, M2 (WebXR VR player) pending

**Date:** 2026-05-08
**Status:** M1 implementation complete, validated on Quest 3 / Meta Browser. M2 brainstorming + design + plan + implementation pending in a new session.
**User platform:** Meta Quest 3 (Horizon OS, stock). Stash + stash-vr on Windows 11. LAN-only network. Caddy reverse-proxies `https://stash-vr.duckdns.org/...` → `localhost:9666`.
**Branch:** `master` (no feature branch this round; user explicitly chose direct-to-master).
**Latest commit:** `e07d181`.

---

## TL;DR for the next agent

The previous session completed an unusual day's worth of work that re-architected the project's forward direction:

1. **Discovered** DeoVR's hardcoded SLR-mode and JSON-mode-only constraints
   make it impossible to replicate SLR's right-panel facet UX through any URL
   we control. **Don't relitigate.** Full evidence:
   `docs/superpowers/research/2026-05-08-deovr-shape/notes.md` and
   `docs/superpowers/research/2026-05-08-slr-playback-hook/notes.md`.
2. **Confirmed** WebXR is unavailable in DeoVR's in-VR Chromium browser but
   **fully available in Quest's Meta Browser**. Tested live against
   `https://immersive-web.github.io/webxr-samples/immersive-vr-session.html`.
3. **Pivoted target browser** from "DeoVR's webview" to "Quest's Meta Browser."
   Implication: stash-vr's new UX surfaces are standalone webapps, not extensions
   of DeoVR. `/deovr` and `/heresphere` JSON endpoints stay running for users
   who keep using those apps but are no longer load-bearing for the new UX.
4. **Decomposed** the post-DeoVR project into four milestones (user-value-first
   strategy):
   - **M1** (✅ shipped): /browse 2D player + catalog search; drop DeoVR launch UI.
   - **M2** (next): WebXR VR player layered onto `/browse/scene/{id}`. **180° SBS
     dominant first.**
   - **M3**: Multi-format VR (FISHEYE, MKX200, RF52, SPHERE 360°, TB stereo).
   - **M4**: Sync polish + funscript + heatmap-on-player + decisions on legacy
     `/deovr`, `/heresphere`.
5. **Shipped M1** in 8 commits on master (`76809bd` → `e07d181`). User validated
   on Quest 3 / Meta Browser; reports "most features tested, all work in web
   browser." Some existing-mutation regression checks (favorite, tag add/remove,
   O-counter, organized) were not exercised — handler code was untouched, so the
   risk is essentially zero, but worth a 30-second click-through if you (M2 agent)
   touch `browse_scene.gohtml` substantially.

The next session is M2: design, plan, implement. **Brainstorm first**, get user
approval, write spec, write plan, execute (subagent-driven). Don't go straight
to implementation.

---

## What's committed (M1, work to preserve)

8 commits added on top of the earlier `35a1b5d docs: handoff for next session...`:

```
e07d181 browse: preserve sidebar tab on search form submit
141de31 browse: tighten M1 validation checklist (empty-q baseline, seek criteria, placeholder rendering)
b43a6cb browse: M1 validation checklist and result stub
afd4145 browse: remove DeoVR play overlay from grid tiles
981b3bd browse: replace 'Play in DeoVR' with inline <video> player on scene detail
9db2265 browse: preserve sidebar tab in search Clear link
bae1651 browse: add catalog search input with form-based submit and clear link
76809bd browse: thread q query param through fetchSceneIDs for catalog search
20407d5 docs: M1 spec - /browse 2D player + catalog search
89a7a59 docs: spike design for WebXR VR player feasibility
```

What each does:

| Commit | What it does |
|---|---|
| `89a7a59` | Spike spec + research notes: established that DeoVR's in-VR browser doesn't support WebXR, SLR's UX is hardcoded to its hostname, the JSON schema can't express facets. The spike PLAN itself was written then abandoned (user pivoted to skip the spike). The orphan plan file `docs/superpowers/plans/2026-05-08-webxr-vr-player-spike.md` is still untracked on disk; ignore it. |
| `20407d5` | M1 spec: extend /browse with 2D player + catalog search. Server-rendered Go templates throughout, no SPA. |
| `76809bd` | Backend: `fetchSceneIDs` accepts `q string`, sets `FindFilterType.Q` when non-empty. Both `indexHandler` and `entityHandler` extract `searchQ := q.Get("q")` from URL and pass it. |
| `bae1651` | UI: search `<form method="GET">` above the grid in `browse.gohtml`, `SearchQuery` field on `PageData`, populated by both handlers, pre-fills the input. CSS for the input + Clear link. |
| `9db2265` | Fix: Clear link `href` preserves `?tab=...` so sidebar tab doesn't reset on clear. |
| `981b3bd` | 2D player: `<video controls playsinline autoplay muted preload="metadata" src="{{.DirectStreamURL}}" poster="{{.ThumbnailURL}}">` on `/browse/scene/{id}`, replacing the "Play in DeoVR" link. `SceneDetailData.DeoVRPlayURL` → `DirectStreamURL`. |
| `afd4145` | Remove `Card.DeoVRPlayURL` and the `▶` quickplay overlay from grid tiles. Tile click goes straight to scene detail. |
| `b43a6cb` | M1 validation checklist + result stub at `docs/superpowers/research/2026-05-08-m1-browse-result/`. |
| `141de31` | Polish: empty-q baseline test, seek scrubber fail criteria, replace `___` (renders as `<hr>`) with `_(fill in here)_`. |
| `e07d181` | Fix: search form has hidden `<input name="tab">` so submitting search preserves `?tab=...` (symmetric with Clear-link fix). |

---

## What M2 needs to be (the core ask)

Add a **WebXR VR player** for 180° SBS scenes to the `/browse/scene/{id}` page.
The user opens the scene in Quest's Meta Browser, sees the 2D player (already
shipping in M1), and gains a new **"Enter VR"** affordance that switches into
WebXR immersive-vr mode rendering the same direct stream URL as a stereo
videosphere.

**Format scope for M2:** ONLY the DOME (180° half-sphere) + SBS (side-by-side
stereo) combination. The other projections (FISHEYE, MKX200, RF52, SPHERE 360°,
TB stereo) are explicit non-goals for M2; they go to M3.

**The user has already chosen the M1 framing** ("extend /browse incrementally")
and that **strongly implies** M2 should also extend `/browse/scene/{id}` rather
than introduce a new route or a SPA. Default approach: layer A-Frame's `<a-scene>`
+ stereo videosphere into `browse_scene.gohtml` behind a feature toggle (CSS
class swap, query param like `?vr=1`, or a button click). But **brainstorm this
with the user before committing** — don't assume. They might want a separate
route, or might want to drop the 2D player when entering VR (full-takeover),
or might want to leave 2D and offer VR as an alternative.

---

## Hard-earned constraints (don't relitigate)

These were settled in the previous session with **file-grounded evidence** —
not just forum quotes:

| Constraint | Evidence |
|---|---|
| **WebXR works in Quest's Meta Browser** | Manually tested `https://immersive-web.github.io/webxr-samples/immersive-vr-session.html` — entered immersive-vr cleanly. |
| **WebXR does NOT work in DeoVR's in-VR Chromium browser** | Same test, same browser build — "VR not found." That browser is Chromium 144 but lacks WebXR API. |
| **SLR's right-panel facets cannot be replicated from a /deovr JSON endpoint** | `docs/superpowers/research/2026-05-08-deovr-shape/notes.md` — DeoVR's JSON schema (hzrd149/deovr-json-schema) has no fields for hierarchy / facets / pagination / per-section URLs. deovr.com itself ships one flat 41,957-item Library section. |
| **SLR's UX is rendered by SLR's own JS in DeoVR's hardcoded "SLR mode"** | `docs/superpowers/research/2026-05-08-slr-playback-hook/notes.md` — only ONE DeoVR string in all of SLR's production JS (a tooltip at line 15212 of slr_videoplayer.js). SLR's HTML payload is an empty Astro/Solid SPA shell. They use a 413 KB custom WebGL+WebXR player via `navigator.xr.isSessionSupported()` — the open WebXR standard, NOT a DeoVR app handoff. |
| **DeoVR's HTML→playback fallbacks are dead on Quest 3** | DeoVR forum thread 10601 (Quest 3 / DeoVR 15.3.3545) — direct `.mp4` link clicks no longer launch the player; show infinite spinner / file download. `deovr://` URL scheme has been a no-op from in-VR browser since 2020 (forum thread 80, never answered). |
| **Stash's transcoder hangs on large 8K MP4s** | Documented in earlier session's commit `ae5c6f2`. Direct stream is the workaround. M1 inherits this — `*vd.SceneParts.Paths.Stream` is the direct URL the `<video>` plays from. |
| **Self-signed HTTPS cert can't be installed on Quest 3** | Earlier session's HTTPS gauntlet (commit `11fa6b4`). Caddy + DuckDNS via Let's Encrypt is the working setup. The user's stash-vr is reachable at `https://stash-vr.duckdns.org/...`. |

**M2 should NOT** retry any of these — the answer is the answer. M2's
unknowns are different (see "Open questions" below).

---

## Working state of the user's environment

Unchanged from the prior handoff except for the tools-up-to-date M1
implementation:

- **stash-vr.exe** at `C:\Users\scw00\Downloads\stash-vr.exe` (user builds it
  via `scripts/build-windows.bat`). They were running it just now with
  `--AUTO_SECTIONS_PERFORMERS=true --AUTO_SECTIONS_TAGS=true
  --AUTO_SECTIONS_AGGREGATES=true` — those flags drive `/deovr` and
  `/heresphere` and are unchanged in M1. Auto-sections still produce 400+
  sections in `/deovr`'s output, which is unusable in DeoVR's library
  renderer; that's accepted as M4's eventual cleanup, not M2's concern.
- **Stash** at `10.0.0.4:9999`, Stash version `v0.31.1`.
- **Caddy** at `localhost:443/9666` reverse-proxies `https://stash-vr.duckdns.org/`
  → `http://localhost:9666/`. Cert auto-renews via DuckDNS DNS-01.
- **Quest 3** uses manual DNS (8.8.8.8 + 1.1.1.1) to bypass router rebinding
  filter.
- **Confirmed M2 target browser:** Quest's **Meta Browser** at
  `https://stash-vr.duckdns.org/browse`. NOT the DeoVR app's in-VR browser.

---

## What's open / what M2 needs to brainstorm

### Architectural decisions to lock down with the user

1. **Where does the VR player live?**
   - Option A: same `browse_scene.gohtml`, behind an "Enter VR" button that
     hides the 2D player and overlays an `<a-scene>` (full-takeover).
   - Option B: a parallel route like `/browse/scene/{id}/vr` that's a separate
     template, with a button on the 2D scene linking to it.
   - Option C: in-page modal-style takeover. Probably overkill.
   - Default lean: **A** (matches M1's "extend /browse incrementally" framing).
2. **A-Frame vs Three.js vs vanilla WebXR.**
   - A-Frame: declarative, faster to wire, uses ~500 KB CDN library, has
     community stereo components like `aframe-stereo-component`.
   - Three.js direct: more code, more control, smaller footprint if tree-shaken.
   - Vanilla WebXR: most code, finest control, no library overhead.
   - Default lean: **A-Frame** — fast to ship, good enough for 180° SBS, easy
     to swap to Three.js later if needed.
3. **How does the user enter VR?** A button on scene detail? Auto-prompt? A
   query param? Per WebXR spec, immersive-vr session entry must be triggered
   by a user gesture (button click), so it can't be auto-launched anyway.
4. **What does "exit VR" do?** Returns to the 2D scene detail page? Closes
   the tab?
5. **Performance expectations.** 8K SBS direct streams may stress Quest 3's
   GPU on a textured videosphere. Spike-style validation early in M2 will
   answer this.

### M1 carryover unknowns surfaced during testing

- The user reports "all work in web browser" — implying M1 was tested in a
  desktop browser AND on Quest 3 Meta Browser (per the AskUserQuestion
  answer). No hard regressions surfaced. Mutations handlers were not
  exercised end-to-end (favorite, tag add/remove, O-counter, organized) but
  the M1 diff didn't touch those handlers, so risk is near-zero.
- The user did NOT fill in `result.md` ("surprises / observations"). If you
  (M2 agent) want forward-looking M2 inputs (autoplay policy, byte-range
  behavior, perf), ask the user directly during brainstorming.

### Things to verify early in M2

- A-Frame scene loads in Meta Browser without errors (cheap — load any
  A-Frame demo URL on the headset).
- A-Frame's `<a-videosphere>` or `aframe-stereo-component` correctly splits
  L/R for SBS encoding (likely needs trial-and-error with one of your scenes).
- The `<video>` byte-range that already works for 2D also works as a texture
  source for `<a-videosphere>` (it should — same `<video>` element semantics).
- Quest 3 frame stability under 8K SBS texture upload at 90 Hz.

---

## Specific files M2 will likely touch

```
internal/static/browse_scene.gohtml        <- where to add the VR player UI
internal/api/browse/scene.go               <- if M2 needs new template fields (projection metadata, etc.)
internal/api/browse/data.go                <- if M2 adds fields to SceneDetailData
internal/api/internal/legend.go            <- TagVR_DOME, TagVR_SBS, etc. (already exist for /deovr; reuse)
internal/api/deovr/videodata.go            <- has set3DFormat() that maps Stash tags -> DeoVR screenType/stereoMode. M2 can extract that logic if it wants the same mapping in /browse/scene.
```

---

## Things the user has explicitly said they don't want

- "no, I want the SLR right panel ags / performer / studio drill-down" — they
  initially asked for SLR's right-panel facets, but accepted the evidence-
  backed answer that this is unreachable from a /deovr JSON endpoint. They
  pivoted away from "match SLR exactly" toward "build our own with the same
  feel but with the facets we already have via /browse's sidebar."
- They went from "build a custom WebXR VR player" with full SLR-clone scope
  (months) to "skip the spike, redesign the full project" (the pragmatic
  decomposition into M1-M4). They will push back on overengineered M2 plans.
- They picked "extend /browse incrementally (Recommended)" for M1 framing.
  Stay consistent in M2: extend `browse_scene.gohtml`, don't introduce a SPA
  framework.
- They prefer commits gated on explicit "go" / "commit" approval per CLAUDE.md.
  Treat plan-approval as authorization for that plan's commits, but ask
  before introducing new commits outside a plan.
- They prefer lowercase-prefixed commits (`browse: ...`, `library: ...`, etc.).

---

## First actions for the next session

1. **Read this file end-to-end before doing anything else.**
2. **Read `CLAUDE.md`** for project conventions.
3. **Skim `docs/superpowers/specs/2026-05-08-m1-browse-2d-player-search.md`** —
   the M1 spec that just shipped. That's the immediate context for M2.
4. **Skim** `docs/superpowers/research/2026-05-08-slr-playback-hook/notes.md`
   for the WebXR + SLR-mode constraints. Just the synthesis section is enough.
5. **Invoke `superpowers:brainstorming`** for M2.
6. **Ask the user** the architectural questions above (where does VR live,
   A-Frame vs Three.js, etc.). Don't assume answers.
7. **Spec → plan → implement** via the standard skills flow
   (`superpowers:writing-plans`, then `superpowers:subagent-driven-development`).
8. **Branch decision:** ask the user. They picked direct-on-master for M1; M2
   may want a feature branch since it's chunkier.

The user is software-savvy, exhausted by VR-platform constraint discovery
already, and pragmatic about scope. Don't propose anything that requires
more environment-specific testing on the headset than necessary — they will
push back hard. Show your reasoning before committing to a direction;
file-grounded evidence is the standard.

---

## Loose ends in the working tree (NOT blockers)

These exist as untracked files on master; the user has not yet decided
their fate. They don't block M2; just be aware:

- `docs/superpowers/plans/2026-05-08-webxr-vr-player-spike.md` — the orphan
  spike plan that was written then never executed. Could be deleted, or kept
  as historical record. M2 doesn't reference it.
- `scripts/slr_*.{html,js,css,pretty.js}`, `scripts/deovr_page.html` — the
  third-party SLR + deovr.com captures from the previous session's research.
  Used as evidence in `docs/superpowers/research/2026-05-08-slr-playback-hook/notes.md`
  (line numbers cited there). Probably fine to leave untracked; the research
  notes are the durable artifact.
- `devstash-vrscan_output.json` — user's own scan output, unrelated.
- `docs/superpowers/research/2026-05-08-m1-browse-result/result.md` is
  committed but **empty** (`_(fill in here)_` placeholders not filled in).
  If you want forward-looking M2 inputs from M1's run, ask the user
  directly during brainstorming rather than mining this file.
