# todo

Master backlog for stash-vr, triaged by domain. See `docs/superpowers/followups.md` for full context on each entry (sources, sizing rationale, decisions taken).

## config — user-tweakable knobs

- **Smooth-scroll speed caps** — expose `GRID_MAX_SLOW` / `GRID_MAX_FAST` / `COLUMN_MAX_SLOW` / `COLUMN_MAX_FAST` via `config.json` or a control-panel slider. Today they live in source. Small.
- **Controller drag / scale sensitivity** — M3c hardcoded a 50× delta multiplier for immersive drag; Y-axis scale rate feels sub-optimal. Make tunable per projection (cinema vs immersive), exposed as panel control or `?sens=` query param. Small.
- **IPD / stereo-separation slider** — Advanced Settings sub-panel with a `uIPDShift` uniform on sphere + fisheye materials. Per-session in v1; per-scene persistence is a follow-up question. Medium (sub-panel + shader plumbing).
- **Advanced Settings sub-panel** — gear-icon panel for 3D offset / brightness / tilt / monoscopic. M4b shipped without it; add if any of these become real pain points. Medium. (Naturally absorbs the IPD slider above.)
- **First-entry tutorial overlay** — backing plane with controller cheatsheet on first Enter VR, auto-dismisses, `localStorage`-persisted. Build only if the user reports forgetting bindings. Small.

## playback — player features

- **Watch-resume / continue-watching** — both 2D and VR seek to last position on load; write back on pause/exit via Stash's `resume_time` mutation. Medium (touches both player paths).
- **End-of-video behavior** — currently both players just stop. Add a setting: auto-next / loop / nothing. Small if just adding a setting; pairs with auto-next below.
- **Auto-next on video end** — when playback ends, automatically load the next tile from the current filtered list. Requires preserving the result list across active scene. Small-medium. Pairs with watch-resume.
- **Multi-track audio selector** — expose `audioTracks[]` from the `<video>` element when scenes have multiple audio tracks; v1 plays the default. Small.
- **Heatmap as scrub-bar background** — reuse existing `/cover/{id}` PNG (already includes heatmap band) as scrub-bar texture. ~1h, small.
- **Caption format support beyond VTT/SRT** — v1 handles VTT and SRT (regex-shared); ASS/SSA etc. return empty cue lists. Add if user has scenes with non-VTT/SRT captions. Medium per format.

## rendering — VR projection / render path

- **M5 — in progress.** Plan: [docs/superpowers/plans/2026-05-09-m5-webxr-media-layers.md](docs/superpowers/plans/2026-05-09-m5-webxr-media-layers.md). Phase 0 + Task 3 shipped; remaining (verify against code state):
  - Task 4: subtitle plane camera-anchor in cinema mode (when XRQuadLayer is active, `vrFlat` is hidden — need a fixed world-position anchor below the layer).
  - Task 5: sleep-recovery interaction check — confirm M4b watchdog still works with layer path.
  - Task 6: fallback-path verification on a non-Layers browser.
  - Task 7: remove diagnostic overlay; clean up spike comments.
  - Task 8: manual validation pass on Quest 3 (spec §7 checklist).
- **Aspect-ratio fallback heuristic** — when neither tag nor filename gives a projection clue, apply SKYBOX-style fallback (`aspect_ratio > 1.8 → SBS`, etc.). Read `Files[0].Width`/`Height` from GraphQL. Small.

## browse — search / discovery / grid features

- **Scene previews on tile hover** — hover tile → play 3s preview clip via existing `Paths.Preview` Stash field (already in GraphQL fragment). Small.
- **Multi-select / queue building** — long-press a tile adds to a queue; queue plays sequentially. Medium.
- **Saved-filter integration** — surface user-defined Stash saved filters (the same ones the original `/filters` UI managed) as a 7th picker or top-level entry alongside Filters. Medium. Depends on whether `config/user.go` was kept after M4d.
- **Sort options** — Newest / highest-rated / random / longest selectors on the browse top strip. Small (server-side filter param).
- **Multi-select for Performer/Tag pickers** — single-select v1 is restrictive ("scenes with Alice AND Bob" / "POV AND Outdoor"). Picker becomes multi-select; chip area shows multiple chips per kind. Medium.
- **Voice search** — Quest 3 supports voice input via system; could route into the search field. Medium (browser API discovery).
- **Persistent search state across VR exits** — re-entering VR currently resets search/filter/scroll. Persist via `localStorage`. Small.
- **Virtualization for deeply-long picker lists** — current ~5-row column handles typical lists; thousands of options (rare) would benefit from render-only-visible-window. Small to medium.

## scroll — smooth-scroll polish & grid feel

- **🔴 Grid scroll frame-rate hitch** (user-flagged in headset, highest priority). Port the grid to the column's translate-the-container pattern. Columns are buttery because [browse_scene.gohtml:2523-2543](internal/static/browse_scene.gohtml#L2523-L2543) (`updateColumnScroll`) just nudges `content.object3D.position.y`; the grid still does full `relayoutTiles()` every frame. Put all tiles in one A-Frame entity; set `object3D.position.y` per frame; recompute positions only on row/col change, fetch, or virtualization-boundary cross. Clip-plane culling already handles visibility. Medium.
- **`gridSmoother.reset()` on `fetchGrid(true)`** — one-line fix; mid-scroll filter change currently leaves velocity, grid feels "stuck at the new bottom" for ~0.4s. Trivial. Defer until user confirms it's perceptible.
- **Extract smoother block to its own JS asset** — ~95 self-contained lines in a 4000+ line `.gohtml`. Clean lift candidate once a static-JS pipeline exists. Pairs with the "extract browse_scene.gohtml CSS" item under web below. Small.
- **Expose smoothers on `window` for devtools introspection** — plan assumed `gridSmoother._peekVelocity()` is console-callable, but it's IIFE-local. Add `window.svDebugSmoothers = ...`. Trivial.
- **Edge-triggered cross-surface resets** — `Object.values(columnSmoothers).forEach(s => s.reset())` currently runs every frame the laser is on the grid (and symmetric). Idempotent but wasteful; track last surface and reset only on transition. Small.

## web — 2D web view (`/browse`, `browse_scene` non-VR)

- **Optimistic UX update for mutations** — favorite/rating/tag mutations wait on server round-trip; on slow networks user sees unresponsiveness. Client mutates DOM immediately, reconciles on response. Small + reconciliation.
- **Server-pushed updates** — if Stash is mutated externally while stash-vr page is open, page doesn't reflect it without reload. SSE or polling. Medium.
- **Toast / confirmation UX on mutations** — v1 is silent on success. Brief toast or transient highlight if user reports doubt. Small.
- **Extract `browse_scene.gohtml` CSS** — template approaching the size where inlined CSS is hostile to skim. Mechanical refactor. Small.
- **Multi-select facet filtering on `/browse` 2D** — sidebar is single-select per facet; feature parity with the in-VR multi-select picker. Medium.
- **In-page lightbox preview on the grid** — hover or click a tile on `/browse` → small preview overlay without navigating away. Small.
- **CSS polish on `/browse` index/grid** — MVP-styled; visual refinement (spacing, typography, hover affordances) hasn't had a pass. Small per pass.

## Notes

- `docs/superpowers/followups.md` is the canonical archive with full reasoning, sources, and decisions for each entry. This file is the working list for at-a-glance prioritization.
- Promote an item into a milestone spec when it's time to schedule.
- The "Removed / abandoned" section in `followups.md` (CUBEMAP/EAC projection, DeoVR webview UX, funscript toy support) is not listed here.
