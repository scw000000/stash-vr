# Smooth Scrolling for VR Browse Panel — Design

**Date:** 2026-05-19
**Scope:** Replace the linear `velocity = stick * rate` model on both the cover grid and the filter columns with a two-tier, eased smooth-scroll model.

## Problem

Today both scroll surfaces use a linear, stickless-of-curve model:

- Grid: `scrollY += ts.y * 0.6 * dt` ([internal/static/browse_scene.gohtml:3749-3754](../../../internal/static/browse_scene.gohtml#L3749-L3754))
- Column: `scrollY += stickY * 0.6 * dt` ([internal/static/browse_scene.gohtml:2558-2567](../../../internal/static/browse_scene.gohtml#L2558-L2567))

Stick magnitude directly scales speed and the speed snaps on/off when the magnitude crosses the deadzone. There is no acceleration, no separation between "precise creep" and "fast skim," and releasing the stick is an abrupt halt instead of a coast.

## Goals

1. Non-linear speed control with smooth ease curves.
2. Two perceptible tiers: a slow, precision tier and a fast, skim tier, both surfaced through the same stick.
3. Fast tier caps at a "perception limit" — the highest speed at which covers / row labels remain individually identifiable.
4. Identical model on the cover grid and on every filter column.

## Non-goals

- Snap-to-row or scroll-locking behavior.
- Touchpad / momentum-flick interactions.
- Reusing the smoother for other axes (volume, scrub) — those already feel right.

## Model

State: one signed scalar `velocity` per surface (in m/s). One instance for the grid, one per filter column.

Each frame, given current stick value `s` (signed, -1..+1) and `dt`:

```
mag = |s|

// 1. Two-mode target speed
if mag < SCROLL_DEADZONE (0.30):         target = 0
elif mag >= SCROLL_FULL_THRESHOLD (0.85): target = sign(s) * MAX_FAST
else:                                     target = sign(s) * MAX_SLOW

// 2. Pick time constant τ
if velocity != 0 and sign(target) != sign(velocity):
    τ = TAU_BRAKE_REVERSE   (0.05s, reverse braking)
elif |target| < |velocity|:
    τ = TAU_RELEASE         (0.13s, settling to lower target)
elif |target| == MAX_FAST:
    τ = TAU_ACCEL_FAST      (0.50s, climbing to fast tier)
else:
    τ = TAU_ACCEL_SLOW      (0.20s, climbing to slow tier)

// 3. Frame-rate-independent exponential ease
alpha = 1 - exp(-dt / τ)
velocity += (target - velocity) * alpha

// 4. Snap-to-target to avoid asymptote crawl
if |velocity - target| < SCROLL_EPSILON (0.001): velocity = target

// 5. Integrate
delta = velocity * dt
scrollY = clamp(scrollY + delta, 0, max)
```

**Why first-order exponential, not smoothstep:** smoothstep needs a known
start/end time, but the target keeps changing as the user moves the stick.
First-order ease-toward-target produces smooth curves under any input change
with no discontinuities. Visually reads as "ease-in-out" because acceleration
is highest at the start of each leg and tapers off near the target.

**Frame-rate independence:** the `1 - exp(-dt/τ)` form means 72 Hz and 90 Hz
headsets produce identical feel.

**Time-to-target reference** (3τ ≈ 95%):

| Transition                                  | τ      | Time to 95% |
|---------------------------------------------|--------|-------------|
| Rest → MAX_SLOW (partial stick)             | 0.20s  | ~0.6s       |
| Rest → MAX_FAST (full stick from rest)      | 0.50s  | ~1.5s       |
| MAX_FAST → MAX_SLOW (stick reduced from max)| 0.13s  | ~0.4s       |
| Any speed → 0 (stick released to deadzone)  | 0.13s  | ~0.4s       |
| Reversal (sign flip)                        | 0.05s  | ~0.15s      |

## Constants

```js
// Thresholds (shared across surfaces)
const SCROLL_DEADZONE       = 0.30;  // matches existing scale-gate
const SCROLL_FULL_THRESHOLD = 0.85;  // ≥ this counts as "max stick"
const SCROLL_EPSILON        = 0.001; // m/s, snap-to-target threshold

// Time constants
const TAU_ACCEL_FAST    = 0.50;
const TAU_ACCEL_SLOW    = 0.20;
const TAU_RELEASE       = 0.13;
const TAU_BRAKE_REVERSE = 0.05;

// Per-surface max speeds (m/s) — starting values, tune in-headset
const GRID_MAX_SLOW   = 1.0;   // ~2.3 grid rows/sec at row pitch 0.43m
const GRID_MAX_FAST   = 2.0;   // ~4.6 grid rows/sec
const COLUMN_MAX_SLOW = 0.6;   // ~6 column rows/sec at row pitch 0.10m
const COLUMN_MAX_FAST = 1.5;   // ~15 column rows/sec
```

## Architecture

A single `makeScrollSmoother(maxSlow, maxFast)` factory closes over a private
`velocity` and exposes:

- `step(stick, dt) → delta` — runs the model above and returns the scroll
  delta (m) to apply this frame. The caller still owns `scrollY` and clamping
  to `[0, max]`.
- `reset()` — zeros `velocity`. Called when the panel hides, when the active
  laser leaves the panel, on filter change for a column, and on initial scene
  load.

One instance for the grid, one per filter column kind
(`{ performer, studio, tag }`). All grid instances share `GRID_*` constants;
all column instances share `COLUMN_*` constants.

## Integration

### Grid — `tickBrowseScroll` ([internal/static/browse_scene.gohtml:3684-3761](../../../internal/static/browse_scene.gohtml#L3684-L3761))

- Remove the `Math.abs(ts.y) < 0.3` early-return at line 3729. The smoother now
  handles deadzone — releasing the stick still needs `step()` calls every
  frame to coast the velocity down to zero.
- Replace lines 3749-3753 (rate, target, clamped) with:
  - `const delta = gridSmoother.step(ts.y, dt);`
  - `scrollY = Math.max(0, Math.min(scrollY + delta, browseMaxScroll));`
- Keep the no-op short-circuit (`if (Math.abs(delta) < tiny) return;`) to
  avoid an unnecessary `relayoutTiles()` while parked at target = 0.
- Lazy-load condition at line 3758 stays as-is.
- When the panel becomes invisible (line 3713) or no laser is active
  (line 3718), call `gridSmoother.reset()` before returning, so the held
  velocity does not resume when the panel reopens.

### Column — `applyListScroll` ([internal/static/browse_scene.gohtml:2558-2567](../../../internal/static/browse_scene.gohtml#L2558-L2567))

- Replace the linear `cur + stickY * 0.6 * dt` line with:
  - `const delta = columnSmoothers[kind].step(stickY, dt);`
  - `const target = Math.max(0, Math.min(cur + delta, max));`
- On filter change at line 1665 (which already zeroes
  `browseState.listScrollY[kind]`), also call
  `columnSmoothers[kind].reset()`.
- `tickBrowseScroll` already calls `applyListScroll` only when the laser is on
  the column, so when the laser leaves the column its smoother stops being
  ticked and the velocity sits frozen. Reset on the same edge as the grid:
  when the panel becomes invisible or no laser is active, reset *all* column
  smoothers in addition to the grid one.

### Deadzone behavior — flagged

The current early-return at `|ts.y| < 0.3` makes scroll stop instantly when
the stick is released or wanders below the deadzone. With the smoother,
sub-deadzone stick sets `target = 0` and the velocity decays over ~0.4s. This
is the requested "release ease-out" — call it out in any commit message so
reviewers expect the change.

## Edge cases

- **Stick never quite reaches full.** If the user holds at 0.83, target stays
  at MAX_SLOW and they never get to MAX_FAST. This is the intended
  "commit to push fully" behavior.
- **Stick wobbles across the full threshold.** Target flips MAX_SLOW ↔
  MAX_FAST each frame the user is on the edge. The MAX_FAST/MAX_SLOW ratio
  is only ~2×, and TAU_RELEASE (0.13s) plus TAU_ACCEL_FAST (0.50s) low-pass
  the flips so the visible speed sits somewhere between the two — no
  flicker. No special hysteresis needed at this τ.
- **Very small `dt`.** `1 - exp(-dt/τ)` → 0, no movement. Safe.
- **Very large `dt`** (e.g., tab backgrounded then refocused). The
  `dt` clamp at line 3732 (`Math.min(0.1, …)`) already caps catch-up at one
  frame, so the smoother cannot teleport the scroll. Keep this clamp.
- **Lazy-load trigger.** The condition `scrollY > browseMaxScroll - 0.5` is
  on position, not velocity. Smooth scrolling doesn't change when it fires;
  it does mean the trigger can fire even after the user releases the stick
  (during the coast). Acceptable — the buffer was designed for this case.

## Testing

No Go test changes. Verification is in-headset, on Quest 3:

1. Tap stick down briefly → grid creeps then coasts to a stop in ~0.4s
   (no abrupt halt).
2. Push stick down to ~half deflection → grid eases up to ~MAX_SLOW (1 m/s)
   in ~0.6s and holds.
3. Push to full deflection → grid continues building from MAX_SLOW past it
   toward MAX_FAST (2 m/s); total time to MAX_FAST from rest with full stick
   ~1.5s.
4. Release stick → grid coasts to stop in ~0.4s.
5. While at MAX_FAST, push stick the opposite way → grid brakes hard (~0.15s
   to zero) then accelerates in the new direction normally.
6. Same five sequences for each filter column (performer / studio / tag).
7. Switch active filter → next time you touch the column, no carried-over
   velocity.
8. Hide and reopen the browse panel mid-scroll → reopened panel is parked,
   no resumed motion.

## Risk

- Exponential easing introduces a 0.4-second "I released but it kept going"
  window — the requested behavior, but a reviewer not in the loop may file
  it as a bug.
- `tickBrowseScroll` runs every frame regardless of stick state once the
  deadzone early-return is removed. The added cost is a few math ops + the
  conditional skip — negligible at 90 Hz.

## Out of scope (not in this design)

- Per-user tunability of MAX_SLOW / MAX_FAST through config.json.
- Snap-to-cover or row-quantization on release.
- Applying the smoother to playback-panel scrub or volume sliders.
