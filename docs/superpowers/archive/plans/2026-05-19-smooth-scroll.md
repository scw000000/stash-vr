# Smooth Scrolling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the linear `stick * 0.6 m/s` scroll on the VR cover grid and filter columns with a two-tier eased scroll model (MAX_SLOW for partial stick, MAX_FAST for full stick, first-order exponential ease toward target).

**Architecture:** Add a `makeScrollSmoother(maxSlow, maxFast)` factory in [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml). One instance for the grid, one per filter column kind. Each instance owns a private `velocity` and exposes `step(stick, dt) → delta` and `reset()`. Wire it into `tickBrowseScroll` (grid) and `applyListScroll` (columns), and reset all smoothers on panel hide / no-laser / filter change.

**Tech Stack:** JavaScript embedded in a Go html/template (`internal/static/browse_scene.gohtml`). No JS test framework; the Go side has only `go vet ./...` and `go build ./...`. Verification is split: pure-math sanity checks via the browser console + behavioral verification in headset on Quest 3. Binary build uses `scripts\build-windows.bat`.

**Reference:** Full design in [docs/superpowers/specs/2026-05-19-smooth-scroll-design.md](../specs/2026-05-19-smooth-scroll-design.md).

---

## File Layout

All changes are in one file: [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml).

Three integration regions, in source order:
- **Constants + factory** — new block, inserted right before line 2683 (`const PANEL_USABLE_W`).
- **Column scroll** — `applyListScroll` at lines 2558-2568, and the filter-change reset at line 1665.
- **Grid scroll** — `tickBrowseScroll` at lines 3684-3761.

No new files. No new GraphQL operations. No Go changes.

---

## Task 1: Add the smoother factory + constants

**Files:**
- Modify: `internal/static/browse_scene.gohtml` (insert new block before line 2683)

- [ ] **Step 1: Insert the constants and factory before the existing tile-rendering constants block**

Find the line `const PANEL_USABLE_W = 3.4;` (around line 2683). Insert the following block immediately above it (i.e., between the closing `}` of `applyListScroll`'s sibling code and the `// Tile rendering geometry.` comment block):

```js
    // ─── Smooth scroll (M4d) ────────────────────────────────────────────
    // Shared two-tier eased velocity model used by the cover grid AND every
    // filter column. Stick magnitude picks a target tier (SLOW for partial
    // deflection, FAST for fully-mashed). First-order exponential ease
    // toward target produces a smooth "ease-in-out feel" under any input
    // changes. Frame-rate independent: 1 - exp(-dt/τ).
    //
    // Spec: docs/superpowers/specs/2026-05-19-smooth-scroll-design.md
    const SCROLL_DEADZONE       = 0.30;  // matches existing scale-gate
    const SCROLL_FULL_THRESHOLD = 0.85;  // ≥ this counts as "max stick"
    const SCROLL_EPSILON        = 0.001; // m/s — snap-to-target threshold

    const TAU_ACCEL_FAST    = 0.50;  // rest → MAX_FAST in ~1.5s
    const TAU_ACCEL_SLOW    = 0.20;  // rest → MAX_SLOW in ~0.6s
    const TAU_RELEASE       = 0.13;  // settle to lower target in ~0.4s
    const TAU_BRAKE_REVERSE = 0.05;  // sign-flip braking in ~0.15s

    const GRID_MAX_SLOW   = 1.0;   // ~2.3 grid rows/sec (row pitch ~0.43m)
    const GRID_MAX_FAST   = 2.0;   // ~4.6 grid rows/sec
    const COLUMN_MAX_SLOW = 0.6;   // ~6 column rows/sec (row pitch 0.10m)
    const COLUMN_MAX_FAST = 1.5;   // ~15 column rows/sec

    function makeScrollSmoother(maxSlow, maxFast) {
      let velocity = 0;
      return {
        // Advances velocity one frame and returns the scroll delta (m) to
        // apply this frame. Caller owns clamping to [0, max].
        step(stick, dt) {
          const mag = Math.abs(stick);

          // 1. Two-mode target speed
          let target;
          if (mag < SCROLL_DEADZONE) {
            target = 0;
          } else if (mag >= SCROLL_FULL_THRESHOLD) {
            target = Math.sign(stick) * maxFast;
          } else {
            target = Math.sign(stick) * maxSlow;
          }

          // 2. Pick τ. Order matters: reversal check first, then magnitude
          //    comparison, then which tier we're climbing toward.
          let tau;
          if (velocity !== 0 && target !== 0 && Math.sign(target) !== Math.sign(velocity)) {
            tau = TAU_BRAKE_REVERSE;
          } else if (Math.abs(target) < Math.abs(velocity)) {
            tau = TAU_RELEASE;
          } else if (Math.abs(target) >= maxFast - SCROLL_EPSILON) {
            tau = TAU_ACCEL_FAST;
          } else {
            tau = TAU_ACCEL_SLOW;
          }

          // 3. Frame-rate-independent exponential ease toward target
          const alpha = 1 - Math.exp(-dt / tau);
          velocity += (target - velocity) * alpha;

          // 4. Snap to exact target near asymptote
          if (Math.abs(velocity - target) < SCROLL_EPSILON) velocity = target;

          // 5. Return integrated delta
          return velocity * dt;
        },
        reset() { velocity = 0; },
        // Exposed for diagnostics overlay only — do NOT mutate from outside.
        _peekVelocity() { return velocity; },
      };
    }

    const gridSmoother = makeScrollSmoother(GRID_MAX_SLOW, GRID_MAX_FAST);
    const columnSmoothers = {
      performer: makeScrollSmoother(COLUMN_MAX_SLOW, COLUMN_MAX_FAST),
      studio:    makeScrollSmoother(COLUMN_MAX_SLOW, COLUMN_MAX_FAST),
      tag:       makeScrollSmoother(COLUMN_MAX_SLOW, COLUMN_MAX_FAST),
    };
    function resetAllScrollSmoothers() {
      gridSmoother.reset();
      columnSmoothers.performer.reset();
      columnSmoothers.studio.reset();
      columnSmoothers.tag.reset();
    }

```

- [ ] **Step 2: Verify the template still parses (no Go-side errors)**

Run: `go vet ./...`
Expected: no output (clean). If you see `template: ... unexpected ...`, you accidentally introduced a `{{` or `}}` token — check the inserted block for those (none should be present).

Run: `go build ./...`
Expected: no output.

- [ ] **Step 3: Sanity-check the math in a browser console**

Rebuild and run locally. From the Quest browser (or any browser pointed at `http://<host>:9999/browse_scene/<id>?…`), open devtools and paste:

```js
// Use a fresh smoother so other state doesn't interfere
const s = makeScrollSmoother(1.0, 2.0);

// (a) Rest → push partial stick (0.5) for ~0.6s; should approach MAX_SLOW=1.0
let v = 0;
for (let i = 0; i < 36; i++) v = s._peekVelocity(), s.step(0.5, 1/60);
console.log('partial 0.6s:', s._peekVelocity().toFixed(3), '(want ≈ 0.95)');

// (b) Continue pushing full stick (1.0) for another 1.5s; should approach MAX_FAST=2.0
for (let i = 0; i < 90; i++) s.step(1.0, 1/60);
console.log('full 1.5s after slow:', s._peekVelocity().toFixed(3), '(want ≈ 1.95)');

// (c) Release stick (0) for ~0.4s; should approach 0
for (let i = 0; i < 24; i++) s.step(0.0, 1/60);
console.log('release 0.4s:', s._peekVelocity().toFixed(3), '(want ≈ 0.04, snap-to-0 soon)');

// (d) Reversal: spin up to MAX_FAST again, then push opposite for 0.15s
for (let i = 0; i < 120; i++) s.step(1.0, 1/60);
console.log('back to MAX_FAST:', s._peekVelocity().toFixed(3), '(want ≈ 2.00)');
for (let i = 0; i < 9; i++) s.step(-1.0, 1/60);
console.log('after 0.15s reverse:', s._peekVelocity().toFixed(3), '(want ≈ -0.10 ish, sign flipped)');
```

Expected output (approximate; small variation OK):
```
partial 0.6s: 0.95  (want ≈ 0.95)
full 1.5s after slow: 1.95  (want ≈ 1.95)
release 0.4s: 0.04  (want ≈ 0.04, snap-to-0 soon)
back to MAX_FAST: 2.00  (want ≈ 2.00)
after 0.15s reverse: -0.10  (want ≈ -0.10 ish, sign flipped)
```

If any line is off by more than ~0.1, re-read your inserted block for typos in τ values or the sign-flip branch.

- [ ] **Step 4: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: add ScrollSmoother helper + constants

Pure-helper task — not yet wired into grid or column. Verified via
browser console: rest→MAX_SLOW, →MAX_FAST, release, reversal all
within tolerance of the design spec.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Wire smoother into the grid

**Files:**
- Modify: `internal/static/browse_scene.gohtml` lines 3713-3754 (`tickBrowseScroll`)

- [ ] **Step 1: Replace the panel-hidden / no-laser early-returns to also reset all smoothers**

Find this block in `tickBrowseScroll` (around line 3713):

```js
      if (!bpVis) {
        browseConsumeThumbY = false;
        browseLastScrollTickMs = 0;
        return;
      }
      if (!activeLaser) {
        browseConsumeThumbY = false;
        browseLastScrollTickMs = 0;
        return;
      }
```

Replace with:

```js
      if (!bpVis) {
        browseConsumeThumbY = false;
        browseLastScrollTickMs = 0;
        resetAllScrollSmoothers();
        return;
      }
      if (!activeLaser) {
        browseConsumeThumbY = false;
        browseLastScrollTickMs = 0;
        resetAllScrollSmoothers();
        return;
      }
```

This guarantees no stale velocity carries across panel-show / laser-leave events.

- [ ] **Step 2: Remove the deadzone early-return**

Find (around line 3729):

```js
      const ts = _readThumbstickAxes(activeLaser);
      if (!ts) { browseLastScrollTickMs = 0; return; }
      // xr-standard: stick-up reads as ts.y = -1, stick-down as +1.
      // Deadzone matches M3c's scale gate (0.3) so light drift doesn't scroll.
      if (Math.abs(ts.y) < 0.3) { browseLastScrollTickMs = 0; return; }
```

Replace with:

```js
      const ts = _readThumbstickAxes(activeLaser);
      if (!ts) { browseLastScrollTickMs = 0; return; }
      // xr-standard: stick-up reads as ts.y = -1, stick-down as +1.
      // Deadzone is now handled inside makeScrollSmoother (target = 0 below
      // 0.3) so the smoother can keep ticking and coast velocity to a stop
      // when the user releases. Do NOT early-return here.
```

This is the deliberate behavior change flagged in the spec — releasing the stick now coasts for ~0.4s instead of stopping instantly.

- [ ] **Step 3: Replace the linear rate math with the smoother call**

Find (around line 3744-3754):

```js
      // M4c Task 7 Step 6: route the scroll to whichever surface the laser
      // is hovering on. Pointing at a filter column → scroll that column's
      // option list; anywhere else on the browse panel → scroll the tile
      // grid (existing behavior).
      const scrollTarget = _laserBrowseScrollTarget(activeLaser);
      if (scrollTarget && scrollTarget.indexOf('list-') === 0) {
        applyListScroll(scrollTarget.substring(5), ts.y, dt);
        return;
      }

      // 0.6 m/s at full deflection. Stick DOWN (ts.y = +1) → scroll
      // forward (scrollY INCREASES → tiles shift up → next rows revealed
      // below). Stick UP reverses; scrollY clamps at 0 (top). Matches the
      // mobile "drag down to scroll down" convention requested by the user.
      const rate = 0.6;
      const target = scrollY + ts.y * rate * dt;
      const clamped = Math.max(0, Math.min(target, browseMaxScroll));
      if (Math.abs(clamped - scrollY) < 0.0001) return;
      scrollY = clamped;
      relayoutTiles();
```

Replace with:

```js
      // Route the scroll to whichever surface the laser is hovering on.
      // Pointing at a filter column → scroll that column's option list;
      // anywhere else on the browse panel → scroll the tile grid.
      const scrollTarget = _laserBrowseScrollTarget(activeLaser);
      if (scrollTarget && scrollTarget.indexOf('list-') === 0) {
        applyListScroll(scrollTarget.substring(5), ts.y, dt);
        // The grid smoother is not being ticked while the laser is on a
        // column — coast its velocity to zero so it doesn't resume when
        // the laser swings back to the grid.
        gridSmoother.reset();
        return;
      }
      // Inverse: ensure column smoothers don't retain velocity while the
      // laser is on the grid.
      columnSmoothers.performer.reset();
      columnSmoothers.studio.reset();
      columnSmoothers.tag.reset();

      // Stick DOWN (ts.y = +1) → scroll forward (scrollY INCREASES → tiles
      // shift up → next rows revealed below). Stick UP reverses; scrollY
      // clamps at 0 (top). Speed is now eased by gridSmoother — see the
      // smooth-scroll block above for the velocity model.
      const delta = gridSmoother.step(ts.y, dt);
      if (Math.abs(delta) < 0.00005) return;
      const clamped = Math.max(0, Math.min(scrollY + delta, browseMaxScroll));
      if (Math.abs(clamped - scrollY) < 0.0001) return;
      scrollY = clamped;
      relayoutTiles();
```

The two new `reset()` block-pairs ensure the cross-surface laser swing doesn't leak velocity in either direction.

- [ ] **Step 4: Rebuild and verify go vet + go build**

Run: `scripts\build-windows.bat`
Expected: builds without errors, producing the stash-vr binary.

If you skipped the script, the minimum check is:
Run: `go vet ./... && go build ./cmd/stash-vr`
Expected: no output.

- [ ] **Step 5: In-headset behavioral verification for the grid**

Start the binary, open the browse panel in Quest 3, and run each of these:

1. **Brief tap (down + immediate release).** Grid should creep forward a few cm then coast smoothly to a stop in ~0.4s. Pre-change it stopped instantly. ✅ if coast visible.
2. **Partial deflection (~half stick, held).** Grid should ease up to ~1 m/s over ~0.6s and hold. ✅ if no jerky ramp, no overshoot past ~2 covers/sec.
3. **Full deflection from rest (held).** Grid should ease past MAX_SLOW and continue building to ~2 m/s over ~1.5s total. ✅ if you can see the speed continue to climb after it reaches the partial-stick speed.
4. **Release after full deflection.** Coast to a stop in ~0.4s. ✅ if no abrupt halt.
5. **Reversal at full speed.** While scrolling down at MAX_FAST, snap stick up. Grid should brake hard (~0.15s) and pick up upward direction. ✅ if reversal feels tight, not floaty.
6. **Stick wobble across full threshold (~0.85).** Hold near edge. Speed should sit between MAX_SLOW and MAX_FAST, no perceptible flicker. ✅ if smooth.

If any check fails: open devtools, watch `gridSmoother._peekVelocity()` in console while you scroll, compare against expected.

- [ ] **Step 6: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: wire ScrollSmoother into grid scroll

Replaces the linear stick*0.6 model in tickBrowseScroll with the
two-tier eased smoother. Includes the deliberate behavior change
flagged in the spec: releasing the stick now coasts ~0.4s instead
of stopping instantly. Cross-surface laser swings reset the
opposite-side smoothers so velocity doesn't leak.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Wire smoother into the filter columns

**Files:**
- Modify: `internal/static/browse_scene.gohtml` lines 2558-2568 (`applyListScroll`)
- Modify: `internal/static/browse_scene.gohtml` line 1665 (filter-change reset)

- [ ] **Step 1: Replace the linear math in `applyListScroll`**

Find (lines 2558-2568):

```js
    function applyListScroll(kind, stickY, dt) {
      const max = maxListScrollY(kind);
      if (max <= 0) return;
      const cur = browseState.listScrollY[kind] || 0;
      // Stick DOWN (stickY = +1) → scrollY INCREASES → later rows reveal,
      // matching the tile-grid scroll convention from tickBrowseScroll.
      const target = Math.max(0, Math.min(cur + stickY * 0.6 * dt, max));
      if (Math.abs(target - cur) < 0.0001) return;
      browseState.listScrollY[kind] = target;
      updateColumnScroll(kind);
    }
```

Replace with:

```js
    function applyListScroll(kind, stickY, dt) {
      const max = maxListScrollY(kind);
      if (max <= 0) {
        // Column has no overflow — keep the smoother at rest so the user
        // can't build velocity that would surprise them after a filter
        // narrows the list back to overflow.
        columnSmoothers[kind].reset();
        return;
      }
      const cur = browseState.listScrollY[kind] || 0;
      // Stick DOWN (stickY = +1) → scrollY INCREASES → later rows reveal.
      // Speed is eased by columnSmoothers[kind] — see the smooth-scroll
      // block for the velocity model.
      const delta = columnSmoothers[kind].step(stickY, dt);
      const target = Math.max(0, Math.min(cur + delta, max));
      if (Math.abs(target - cur) < 0.0001) return;
      browseState.listScrollY[kind] = target;
      updateColumnScroll(kind);
    }
```

- [ ] **Step 2: Reset the column's smoother on filter-text change**

Find (line 1665, inside the `picker-` branch of the debounced input handler):

```js
            browseState.listScrollY[kind] = 0; // reset per-column scroll on filter change
            renderColumnList(kind);
```

Replace with:

```js
            browseState.listScrollY[kind] = 0; // reset per-column scroll on filter change
            columnSmoothers[kind].reset();     // zero held velocity too
            renderColumnList(kind);
```

- [ ] **Step 3: Rebuild**

Run: `scripts\build-windows.bat`
Expected: clean build.

- [ ] **Step 4: In-headset behavioral verification for each column**

For each of `performer`, `studio`, `tag` (point the laser at the column header / option rows):

1. **Brief tap.** Column row list creeps then coasts to stop in ~0.4s.
2. **Partial deflection held.** Eases up to ~0.6 m/s (~6 rows/sec, individually trackable).
3. **Full deflection from rest, held.** Continues building to ~1.5 m/s (~15 rows/sec).
4. **Release.** Coasts to stop in ~0.4s.
5. **Reversal mid-scroll.** Brakes tight, picks up other way.

Then:

6. **Type into the filter input while the column is scrolling.** The list re-renders narrowed AND the smoother is at rest (no leftover velocity making the freshly-narrowed list jump on next stick touch).
7. **Swing the laser from grid → column while the grid is at MAX_FAST.** The column smoother should start from zero (verify the column's first frame isn't already moving). The grid smoother should reset (swing back to the grid; no resumed motion).

- [ ] **Step 5: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: wire ScrollSmoother into filter column scroll

Replaces the linear stickY*0.6 model in applyListScroll with the
shared eased smoother (per-column instance). Resets the column's
smoother on filter-text narrow so a freshly-rendered list doesn't
inherit velocity from a different content set.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Final cross-surface verification

**Files:** none — verification only.

- [ ] **Step 1: Headset checklist — interaction edges**

In Quest 3 with the built binary, run through:

1. **Panel hide mid-scroll.** Scroll the grid at MAX_FAST, then close the browse panel (e.g., open the playback panel). Wait 1s. Reopen the browse panel. Grid must be parked — no resumed motion.
2. **Active laser swap mid-scroll.** Scroll the grid at MAX_FAST with the right laser. While still pushing the right stick down, also push the left stick down. Now release the right stick. The active-laser selection inside `tickBrowseScroll` should jump to the left laser; grid should keep scrolling under the new laser's input without a velocity reset.
3. **Filter applied mid-column-scroll.** Scroll a column to ~middle. Tap a different filter option to narrow the list (which calls `renderColumnList` and zeros `listScrollY`). Touch the stick again — column should start from zero velocity, not resume.
4. **Backgrounding the browser.** Scroll grid at MAX_FAST. Press the Quest home button briefly, then come back. The grid should not have teleported (the `dt` clamp at line 3732 protects this).
5. **Frame-rate dip.** If you can simulate it (developer settings → lock to 72Hz), verify the feel is identical to 90Hz. The exponential ease is frame-rate independent.

- [ ] **Step 2: Look at the debug overlay during a scroll**

Press the DBG button on the playback panel to enable the diagnostic text. While scrolling the grid, the `sy R=…` line shows the raw stick values — confirm `scroll=…` increases smoothly (not in linear-rate jumps), and `scroll` ≤ `browseMaxScroll`.

- [ ] **Step 3: No further commit needed — verification only**

If any check above failed, return to the affected task and fix. If all pass, the work is done.

---

## Out of scope (do not implement)

- Surface-specific user-tunable MAX values via `config.json`. The constants live in the source for now.
- Snap-to-row / row quantization on release.
- Applying the smoother to playback-panel scrub or volume sliders.
- Visualizing the velocity curve in the diagnostic overlay (only `_peekVelocity()` for console debug).
