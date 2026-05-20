# todo

Master backlog for stash-vr. Pulled from M1–M5 plans and `docs/superpowers/followups.md` (the canonical archive — see it for full context on each item). All M1–M4 milestones have shipped; M5 is the only in-progress milestone.

## Active work

### M5 — WebXR Media Layers (in progress)
Plan: [docs/superpowers/plans/2026-05-09-m5-webxr-media-layers.md](docs/superpowers/plans/2026-05-09-m5-webxr-media-layers.md). Phase 0 diagnostic and Task 3 production layer manager landed; remaining tasks (verify per current code state):
- Task 4: Subtitle plane camera-anchor in cinema mode when XRQuadLayer is active.
- Task 5: Sleep-recovery interaction check — confirm M4b watchdog still works with M5 layer path.
- Task 6: Fallback-path verification on a non-Layers browser.
- Task 7: Remove diagnostic overlay; clean up spike comments.
- Task 8: Manual validation pass on Quest 3 (spec §7 checklist).

## Smooth scroll (M4d) — most recent
Just shipped (`9cfbe86..7177422`). Open follow-ups:
- **Grid scroll frame-rate hitch** (highest, user-flagged in headset): port grid to column's translate-the-container pattern. Columns are buttery because [browse_scene.gohtml:2523-2543](internal/static/browse_scene.gohtml#L2523-L2543) just nudges `object3D.position.y`; the grid still does full `relayoutTiles()` per frame. Medium.
- `gridSmoother.reset()` on `fetchGrid(true)` — one-line; fixes "stuck at bottom" after mid-scroll filter change. Trivial.
- Per-user `MAX_SLOW` / `MAX_FAST` tuning via `config.json` or panel slider. Small.
- Extract smoother block (~95 lines) to its own JS asset once a static-JS pipeline exists. Small.
- Expose smoothers on `window` for devtools introspection. Trivial.
- Edge-triggered cross-surface resets (instead of per-frame idempotent reset). Small.

## VR rendering / projection

### Pre-M4b — V-flash on 8K VR scenes (active)
Intermittent diagonal black wedge on 8K source content (4320p / `#:8KVR`). Root cause: HEVC decoder misses render-tick → partial-frame `texImage2D` upload. Recommended v1: auto-downgrade 8K to 4K transcode in `internal/api/browse/scene.go` / `internal/stash/stream.go`. Size: 2–3h Go change + in-headset verify.

### M3a-followup — aspect-ratio fallback heuristic
Read `Files[0].Width`/`Height` and apply SKYBOX-style fallback when no tag/filename projection clue. Small.

### M3b-followup — IPD / stereo-separation slider
Advanced Settings sub-panel with `uIPDShift` uniform on sphere + fisheye materials. Medium.

## In-VR controller UX

### M3c-followup — drag/scale sensitivity tuning + config knob
Hardcoded 50× delta multiplier and Y-axis scale rate need tunability per projection. Small.

### M3c-followup — first-entry tutorial overlay
Cheatsheet on first Enter VR, `localStorage`-persisted. Build only if user requests. Small.

## Web-side UX (M4a follow-ups)
- Optimistic UX update for mutations (small + reconciliation).
- Server-pushed updates if Stash is mutated externally (SSE/polling, medium).
- Toast / confirmation UX on mutations (small).
- Extract `browse_scene.gohtml` CSS to a separate file (small, mechanical).

## VR control panel (M4b follow-ups)
- Advanced Settings sub-panel (3D offset / brightness / tilt / monoscopic) — medium.
- Multi-track audio selector — small.
- Heatmap as scrub-bar background (reuse existing `/cover/{id}` PNG) — small, ~1h.
- Caption format support beyond VTT/SRT (ASS/SSA etc.) — medium per format.

## In-VR search/browse (M4c follow-ups)
- M4c-followup-α: Auto-next on video end — small-medium.
- M4c-followup-β: Scene previews on tile hover via `Paths.Preview` — small.
- M4c-followup-γ: Multi-select / queue building (long-press) — medium.
- M4c-followup-δ: Saved-filter integration as a 7th picker — medium.
- M4c-followup-ε: Sort options (newest / rated / random / longest) — small.
- M4c-followup-ζ: Multi-select for Performer/Tag pickers — medium.
- Voice search via Quest system input — medium.
- Persistent search state across VR exits (`localStorage`) — small.
- Virtualization for deeply-long picker lists — small to medium.

## Watch-resume / playback memory
- Watch-resume / continue-watching (Stash `resume_time` mutation; both 2D + VR paths) — medium.
- End-of-video behavior beyond stop (auto-next / loop / nothing as a setting) — small.

## /browse 2D polish
- Multi-select facet filtering on `/browse` 2D (parity with M4c-followup-ζ) — medium.
- In-page lightbox preview on the grid — small.
- CSS polish pass on `/browse` index/grid (spacing, typography, hover) — small per pass.

## Notes
- `docs/superpowers/followups.md` is the canonical archive with full reasoning, sources, and size estimates per item.
- "Removed / abandoned" entries (CUBEMAP/EAC support, DeoVR webview UX, funscript) are out of scope and not listed here.
- This file is the working list for at-a-glance prioritization; promote an entry into a milestone spec when it's time to schedule.
