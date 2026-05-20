# M2 follow-up: VR sync/flash + audio defaults — result

**Date run:** 2026-05-08
**Stash-vr commit at end:** `b6842b7` (`browse: VR audio defaults to on, scene clear-color mitigates flash`)
**Predecessor commit:** `0d866bb` (`browse: collapse VR to single <video> to fix audio sync + black flash`)
**Spec:** [2026-05-08-m2-vr-sync-flash-fix-design.md](../../specs/2026-05-08-m2-vr-sync-flash-fix-design.md)
**Plan:** [2026-05-08-m2-vr-sync-flash-fix.md](../../plans/2026-05-08-m2-vr-sync-flash-fix.md)
**Hardware:** Quest 3 / Meta Browser

## Per-criterion results

Mapped to spec §2 success criteria:

| # | Criterion | Result |
|---|---|---|
| 1 | Enter VR mid-playback. Video continues from current 2D position. Audio audible and matches picture. | PASS |
| 2 | After 5+ minutes inside VR, audio is still tight against picture (no perceptible drift). | PASS — original drift symptom is gone. |
| 3 | On Enter VR, the videosphere is populated with the current frame immediately — no first-frame black flash from empty texture. | PASS — first-frame flash gone. See "Surprises" for a different upper-screen artifact unrelated to the dual-video black flash. |
| 4 | Exit VR. 2D player resumes at the position the user left VR at. Muted/audible state consistent with how the page started. | PASS — and per follow-up commit `b6842b7`, mute state now persists across exit-VR rather than being forced back to muted (user request). |
| 5 | In-VR panel buttons (play/pause, ±10s, exit) work. | PASS |
| 6 | M1 + M2 surfaces unaffected. | PASS — rating, favorite, tags, O-counter, organized, sidebar, search, pagination, Enter-VR gating all unchanged. |

## Surprises / observations

**Audio default flipped per user request.** Spec §3 specified `<video ... autoplay muted>` with `video.muted = false` on Enter VR and `video.muted = true` on exit-VR. After on-headset use the user wanted audio on by default and the muted state to persist across the 2D ↔ VR boundary. Commit `b6842b7` dropped the `muted` HTML attribute entirely, dropped both JS muted-toggles, and added `video.play()` to the Enter VR click handler so VR auto-starts even from a paused 2D state (the click is a user gesture, autoplay-with-sound is permitted). Spec wording is now stale on this point — see "Open M3 inputs" below.

**Upper-screen V-shape black artifact, video-content-dependent.** A separate visual issue surfaced during validation: on certain VR scenes (notably scene id 5535, KMPVR / SAVR-417), a downward-pointing V-shape of black appears at the upper part of the user's view. Other VR-tagged scenes do not show it. The 2D thumbnail of the affected scene shows rectangular SBS halves (not circular fisheye), which suggests the cause is **burned-in source content** at the top edge of the frame — studio watermarks, scene-title cards, or a thin letterbox bar — being stretched into a wedge by the spherical pole singularity, rather than a projection mismatch.

The user chose to defer this to M3 (SKYBOX-style projection / format work) rather than ship a stop-gap UV crop in this fix. A texture crop of the top ~3% of the source was discussed but not landed because it would slightly truncate vertical FOV on clean sources to fix a problem that only some sources have. The proper fix is per-scene projection awareness, which is M3 scope.

**A-Frame compositor clear-color mitigation landed.** Commit `b6842b7` set `<a-scene background="color: #111">` to address residual frame-edge flashes from the WebGL compositor. Pre-existing spec §5 listed this as the fallback for "residual black flash from A-Frame's enter-VR clear" — promoted to default since it is essentially free and harmless on clean sources.

## Recommendation

- [x] All in-scope criteria PASS → green-light **M3 (SKYBOX-style projection selector + IPD/eye-distance + controller-mapping UI overhaul)** design session.
- [ ] FAIL — re-spec needed because: N/A.

## Open M3 inputs from this milestone

- **Per-scene projection override is necessary, not a nice-to-have.** Some scenes tagged DOME+SBS have content (watermarks, fisheye-extended frame masking, top-edge burn-in) that renders incorrectly on a vanilla equirectangular half-sphere. Detection by tag alone is insufficient; M3 should support a manual override (matches SKYBOX's "Auto / Reset / format buttons" UX in the user-supplied reference image).
- **Filename keyword detection** (à la SKYBOX: `MKX200`, `FISHEYE190`, `FISHEYE200`, `LR_180`, `2D_180`, `RF52`, `CANTED`, etc.) is a pragmatic auto-detect strategy that works without re-tagging the user's library.
- **Texture cropping is a viable per-projection mitigation.** A small UV inset (`offset.y = 0.03; repeat.y = 0.94`) hides top-edge artifacts at the cost of vertical FOV. Worth exposing as an opt-in tweak in the M3 in-VR menu.
- **The single-video architecture from this fix simplifies M3.** With one media element, the in-VR panel's play/pause/seek targeting is straightforward, and a future IPD slider only needs to adjust camera offsets (or texture sub-rect offsets), not coordinate two media pipelines.
- **Audio-defaults-on is the new baseline for M3.** No `muted` attribute on the page video, no muted-toggle on Enter/Exit VR. M3 should not regress this.
- **The clear-color background (`#111`) mitigates only compositor-side flashes, not source-side artifacts.** The V-shape on scene 5535 was unaffected by the background change. M3's projection-aware rendering needs to handle source-side issues independently.

## Out-of-spec changes shipped

- **`b6842b7`** dropped `muted` from `<video>` and dropped both muted-toggles in JS, replacing them with `video.play()` in the Enter-VR click handler. Reason: user request after on-headset use. Rationale documented above and in the commit message.
- **`b6842b7`** added `<a-scene background="color: #111">`. Reason: spec §5 fallback for residual compositor clear-color flashes.

Both changes are user-requested follow-ups and are noted here so the spec/plan diff can be reconciled when M3 begins.
