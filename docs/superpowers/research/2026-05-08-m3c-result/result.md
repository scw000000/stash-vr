# M3c SKYBOX-style controller mappings — result

**Date run:** _(fill in here)_
**Stash-vr commit:** _(fill in — `git rev-parse --short HEAD` at run time)_
**Quest 3 firmware:** _(fill in here)_
**Meta Browser version:** _(fill in here)_
**Test scenes used:**
- Cinema (no VR tags): _(scene id / title)_
- Immersive DOME 180° SBS: _(scene id / title)_
- Mistagged (V-shape) for picker regression: _(scene id / title — typically 5535 / SAVR-417 per M3b)_

## Per-criterion results

Copy from `checklist.md` after running on the headset.

## Surprises / observations

(Free-form. Anything load-bearing for follow-ups: latency feel, drag-feels-too-twitchy, scale-rate-feels-too-fast, recenter-misbehaves-on-Meta-Browser-edge-case, panel-summon-too-slow-at-400ms, etc.)

_(fill in here)_

## Open tuning decisions

These constants are easy to tweak in `internal/static/m3c-controls.js`'s `init`. Note any that should change after on-headset feel:

- `DRAG_DIST_M = 0.05` (5 cm threshold for drag promotion). Felt right? Too sensitive? Too sluggish? → ___
- `DRAG_HOLD_MS = 250` (drag-time threshold). Right? → ___
- `DOUBLE_CLICK_MS = 400` (double-click window). Spec was 300; widened for jitter. Feels good or sluggish? → ___
- `LONG_PRESS_MS = 500` (B/Y long-press). Right? → ___
- `SEEK_TRIGGER = 0.7` / `SEEK_REARM = 0.3` (thumbstick seek hysteresis). Right? → ___
- `SEEK_SECONDS = 10`. Match user expectation? → ___
- Y-scale rate constant `0.6` in `1 + 0.6 * yNorm * dtSec`. Too fast at 90 fps? Too slow? → ___
- Scale clamp `[0.3, 5.0]`. Floor too small (sphere clipping)? Ceiling too big (geometry escaping scene)? → ___

## Recommendation

- [ ] All-PASS or near-all-PASS → green-light next milestone (M3c-followup tutorial overlay if needed; M3b-followup IPD slider; or M4 CUBEMAP).
- [ ] FAIL — re-spec needed because: _(fill in here)_

## Open follow-ups for next milestones

(Things we learned during M3c that should inform what comes next.)

- **M3c-followup: drag/scale sensitivity tuning + config knob.** During Quest 3
  validation, immersive drag needed a 50× delta multiplier (currently hardcoded
  in `internal/static/browse_scene.gohtml`'s `m3c:drag-move` handler) for hand
  motion to be perceptible against a 100m sphere. Both drag and Y-axis scale
  rates feel sub-optimal. Bundle these into a tunable config (per-projection
  override would be ideal): drag-multiplier-cinema, drag-multiplier-immersive,
  scale-rate. Possibly expose via the panel as a "Sensitivity" sub-control or
  a `?sens=` query parameter on the scene URL.

## Risks that materialized (cross-reference spec §8)

For each spec §8 risk, note whether it actually showed up:

- Trigger state machine edge cases (rapid triple-click, drag-into-button, long-press-while-stick-held): _(observed? not observed? mitigation needed?)_
- WebXR `setReferenceSpace` quirks on Meta Browser: _(observed?)_
- Sphere/fisheye-quad scale clipping at extremes: _(observed?)_
- Drag-into-button false positive: _(observed?)_
- Quest Meta button unreachable: confirmed unreachable (not a regression — known constraint)
- Laser-hidden mode disorientation: _(noticeable?)_
- Component-vs-IIFE boundary creep: _(any actual creep?)_
- Panel-hidden-by-default UX surprise: _(any?)_
- Single-click latency from 400 ms double-click window: _(noticeable?)_
