# Stash-VR follow-ups

Logged items not currently scheduled into M4a/M4b/M4c/M4d. Categorized by area. Each entry: source spec/handoff that mentioned it, brief description, and a rough sense of size.

This doc is the canonical "what's left" index. New follow-ups surfaced during work should be added here under the appropriate category. If you decide to schedule one, move it into a milestone spec and remove the entry here.

---

## VR rendering / projection

### M3a-followup: aspect-ratio fallback heuristic
Source: [M3a spec §3](specs/2026-05-08-m3a-multi-projection-rendering.md), [SKYBOX UI reference §3.3](research/2026-05-08-skybox-ui-reference/reference.md). When neither tag nor filename gives a projection clue, SKYBOX falls back to `aspect_ratio > 1.8 → SBS`, etc. Stash-vr doesn't implement this. Requires reading `Files[0].Width`/`Height` from GraphQL. Size: small.

### M3b-followup: IPD / stereo-separation slider
Source: [M3b spec §2](specs/2026-05-08-m3b-in-vr-projection-picker.md), [M3c spec §2](specs/2026-05-08-m3c-skybox-controller-mappings.md). An "Advanced Settings" sub-panel with an IPD slider that adjusts a `uIPDShift` uniform on sphere + fisheye materials. Per-session in v1; per-scene persistence is a separate question. Size: medium — one new sub-panel + shader plumbing.

### Pre-M4b-followup: intermittent diagonal black wedge ("V flash") on 8K VR scenes
Sources: M2 sync/flash result doc (scene 5535, SAVR-417 / KMPVR-彩-), M4b round-5 headset testing (scene 1842, KAVR-338 / kawaii). User reports a per-eye-mirrored diagonal black wedge in the upper FOV that appears intermittently during playback. Visible across all stash-vr render modes (cinema 2D, sphere180, etc.). Predates M4b — not a regression.

Cross-checks done (2026-05-09):
- Clean in Stash's native HTML5 player → stash-vr's render path is the differentiator.
- Pause makes it disappear → temporal effect tied to active decode + render, not burned-in source content. (The earlier M2 doc's "burned-in source content stretched by pole singularity" hypothesis is **incorrect** — it predicts pause would freeze the V on screen, the opposite of what the user observes. Discard.)
- Screen recording captures fully black → typical WebXR layer-recording limitation, not informative.

**Common factor across confirmed cases (queried via `curl -X POST localhost:9666/heresphere/{id}`):**

| | Scene 1842 (KAVR-338) | Scene 5535 (SAVR-417) |
|---|---|---|
| `Resolution:` tag | 4320p | 4320p |
| Quality tag | `#:8KVR`, `#:ハイクオリティVR` | `#:8KVR`, `#:ハイクオリティVR` |
| Studio | kawaii | KMPVR-彩- |
| Transcodes available | 4096 / 2160 / 1080 / 720 / 480 / 240 | 4096 / 2160 / 1080 / 720 / 480 / 240 |

Single common factor: **8K source resolution (4320p, `#:8KVR`)**. Studios differ.

Mechanism (best fit): Quest 3's hardware HEVC decoder can do 8K but operates near its ceiling at 8K + WebGL upload load. When the decoder occasionally misses the render-tick deadline, Three.js's `VideoTexture` uploads a partially-decoded frame to the GPU via `texImage2D`. The boundary between decoded and undecoded macroblock slices is diagonal for many encoder slice-order modes (FMO, wavefront parallel processing) — that's the wedge shape. Both eyes show mirrored artifacts because SBS VR180 mirrors content between halves of the same partially-decoded source frame. At 4K and below the decoder has slack and the artifact never appears.

Stash's native player is clean because HTML5 `<video>` is composited by the browser's hardware video pipeline directly, not through a CPU→GPU `texImage2D` step that can sample partial-decode buffers.

Fix path (small Go change in `internal/api/browse/scene.go` and/or `internal/stash/stream.go`):
- **Auto-downgrade to 4K transcode for 8K scenes.** Detect `Resolution:4320p` or `#:8KVR` on the scene; instead of `vd.SceneParts.Paths.Stream` (the direct/original stream), pick the 4K (`FOUR_K`) entry from `vd.SceneParts.SceneStreams`. Cost: 8K-capable hardware can't see full 8K through stash-vr even when it could in principle.
- **Per-scene quality selector in the in-VR Format picker.** Add a "Quality" row (Direct / 4K / 1080p). User opts down per scene. More UI; preserves direct-stream default.

Recommend the auto-downgrade for v1 (zero UI, eliminates the V on the bulk of cases) with the per-scene picker as a future opt-up. Size: ~2-3h Go change + one new format-picker row + manual in-headset verification on a known-affected scene.

## In-VR controller UX

### M3c-followup: drag/scale sensitivity tuning + config knob
Source: [M3c result](research/2026-05-08-m3c-result/result.md). Surfaced during Quest 3 validation — the immersive drag needed a hardcoded 50× delta multiplier, and Y-axis scale rate feels sub-optimal. Both should be tunable per projection (cinema vs immersive), exposed as panel control or `?sens=` query param. Size: small.

### M3c-followup: First-entry tutorial overlay
Source: [M3c spec §9](specs/2026-05-08-m3c-skybox-controller-mappings.md). Backing plane appears on first Enter VR with the §3 cheatsheet, auto-dismisses after first interaction, persisted via `localStorage` as already-seen. Only build if the user reports forgetting bindings. Size: small.

## Web-side UX (M4a follow-ups)

### Optimistic UX update for mutations
Source: [M4a spec §7](specs/2026-05-09-m4a-web-polish.md). Today every favorite/rating/tag mutation waits on the server round-trip. On slow networks the user sees a moment of unresponsiveness. Optimistic-first variant: client mutates DOM immediately, then reconciles with server response. Size: small but adds reconciliation logic.

### Server-pushed updates if Stash is mutated externally
Source: [M4a spec §9](specs/2026-05-09-m4a-web-polish.md). If the user edits a scene in Stash's own UI while stash-vr's page is open, the page won't reflect it without a reload. SSE or polling layer if it ever matters. Size: medium.

### Toast / confirmation UX on mutations
Source: [M4a spec §9](specs/2026-05-09-m4a-web-polish.md). v1 is silent on success. If the user reports doubt about whether a mutation worked, add a brief toast or transient highlight. Size: small.

### Extract `browse_scene.gohtml` CSS to a separate file
Source: [M4a spec §9](specs/2026-05-09-m4a-web-polish.md). Template approaching the size where inlined CSS is hostile to skim. Mechanical refactor. Size: small.

## VR control panel (M4b follow-ups)

### Advanced Settings sub-panel (3D offset, brightness, tilt, monoscopic)
Source: [M4b spec §2](specs/2026-05-09-m4b-vr-control-panel.md), [SKYBOX reference §4.3](research/2026-05-08-skybox-ui-reference/reference.md). v1 ships without the gear-icon panel. Add if any of these become real pain points. Size: medium.

### Multi-track audio selector
Source: [M4b spec §2](specs/2026-05-09-m4b-vr-control-panel.md). Stash scenes can have multiple audio tracks; v1 plays whichever is default. Picker would expose `audioTracks[]` from the `<video>` element. Size: small.

### Heatmap as scrub-bar background
Source: [M4b spec §2](specs/2026-05-09-m4b-vr-control-panel.md), [M4b spec §9](specs/2026-05-09-m4b-vr-control-panel.md). Use the existing `/cover/{id}` PNG (which already includes the heatmap band) as the scrub bar's background texture. ~1 hour. Size: small.

### Caption format support beyond VTT/SRT
Source: [M4b spec §2](specs/2026-05-09-m4b-vr-control-panel.md). v1 parser handles VTT and SRT (regex-shared). ASS/SSA and other formats return empty cue list. Add if the user has scenes with non-VTT/SRT captions. Size: medium per format.

## In-VR search/browse (M4c follow-ups)

### M4c-followup-α: Auto-next on video end
Source: [M4c spec §9](specs/2026-05-09-m4c-in-vr-search.md). When playback ends, automatically load the next tile from the current filtered list. Requires preserving the result list across the active scene. Pairs naturally with watch-resume below. Size: small-medium.

### M4c-followup-β: Scene previews on tile hover
Source: [M4c spec §9](specs/2026-05-09-m4c-in-vr-search.md). Hover a tile → it plays a 3-sec preview clip via the existing `Paths.Preview` Stash field (already in the GraphQL fragment). Size: small.

### M4c-followup-γ: Multi-select / queue building
Source: [M4c spec §9](specs/2026-05-09-m4c-in-vr-search.md). Long-press a tile → adds to a queue; queue plays sequentially. Size: medium.

### M4c-followup-δ: Saved-filter integration
Source: [M4c spec §9](specs/2026-05-09-m4c-in-vr-search.md). Surface user-defined Stash saved filters (the same ones the original `/filters` UI managed) as a 7th picker or top-level entry alongside Filters. Size: medium. Note: depends on whether `config/user.go` was kept after M4d.

### M4c-followup-ε: Sort options
Source: [M4c spec §9](specs/2026-05-09-m4c-in-vr-search.md). Newest / highest-rated / random / longest selectors on the browse top strip. v1 inherits `/browse`'s default order. Size: small (server-side filter param).

### M4c-followup-ζ: Multi-select for Performer/Tag pickers
Source: [M4c spec §9](specs/2026-05-09-m4c-in-vr-search.md). Single-select v1 is restrictive — many use cases want "scenes with Alice AND Bob," "scenes with `POV` AND `Outdoor`." Picker becomes multi-select; chip area shows multiple chips per kind. Server handles arrays. Size: medium.

### Voice search
Source: [M4c spec §2](specs/2026-05-09-m4c-in-vr-search.md). Quest 3 supports voice input via system. Could route into the search field. Size: medium (browser API discovery).

### Persistent search state across VR exits
Source: [M4c spec §2](specs/2026-05-09-m4c-in-vr-search.md). Re-entering VR currently resets search/filter/scroll position. Persist via `localStorage`. Size: small.

### Scrolling within deeply-long picker lists
Source: M4c plan task 7. Each filter column shows ~5 rows; scrolling handles longer lists. If a column has thousands of options (rare), virtualization (render only visible window) would be the next step. Size: small to medium depending on need.

## Watch-resume / playback memory

### Watch-resume / continue-watching
Source: [M1 spec §3](specs/2026-05-08-m1-browse-2d-player-search.md), [M2 spec §2](specs/2026-05-08-m2-webxr-vr-player.md). Stash supports `resume_time` mutation. Both 2D and VR players seek to last position on load and write back on pause/exit. Size: medium — touches both paths.

### End-of-video behavior beyond stop
Source: [M1 spec §3](specs/2026-05-08-m1-browse-2d-player-search.md). Currently 2D and VR both just stop at end. Possible behaviors: auto-next (paired with M4c-followup-α), loop (M4b's loop button covers this on demand), nothing (current). Size: small if just adding a setting.

## /browse 2D polish (originally part of M4 framing)

### Multi-select facet filtering on `/browse` 2D
Source: [M1 spec §3](specs/2026-05-08-m1-browse-2d-player-search.md). The 2D sidebar is single-select per facet. Multi-select would benefit feature parity with M4c-followup-ζ. Size: medium.

### In-page lightbox preview on the grid
Source: [M1 spec §3](specs/2026-05-08-m1-browse-2d-player-search.md). Hover or click a tile on `/browse` index → small preview overlay without navigating away. Size: small.

### CSS polish on `/browse` index/grid
Source: M1 handoff. The 2D grid was MVP-styled; visual refinement (spacing, typography, hover affordances) hasn't had a pass. Size: small per pass.

## Removed / abandoned

These were on the radar at one point but are no longer relevant given the project's direction. Listed for future reference; don't schedule.

- **CUBEMAP / EAC (YouTube) projection support.** [M3a spec](specs/2026-05-08-m3a-multi-projection-rendering.md), [M3b spec](specs/2026-05-08-m3b-in-vr-projection-picker.md). User has no CUBEMAP/EAC content; never reach. The constants `TagVR_CUBEMAP` / `TagVR_EAC` were left in `internal/api/internal/legend.go` for completeness; remove if `legend.go` is deleted in M4d.
- **DeoVR webview UX (tofu glyphs, 400+ sections)** — pivoted away from DeoVR entirely; M4d removes the surface that hosted these problems.
- **Funscript timeline / haptic device sync** — user explicitly de-scoped toy support.
