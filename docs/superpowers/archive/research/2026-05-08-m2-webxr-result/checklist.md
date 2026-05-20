# M2 WebXR 180° SBS — Quest 3 Meta Browser checklist

**Run on:** Quest 3 hardware, Meta Browser (NOT DeoVR's in-VR browser; that's known not to support WebXR — confirmed in M2 spec § 1).

**URL to open:** `https://stash-vr.duckdns.org/browse`

For each criterion: PASS / FAIL / PARTIAL + one-line note.

## Detection / button gating

- [ ] On a scene tagged `DOME` + `SBS`, the "Enter VR" button is visible below the 2D player.
  - Result: ___ — note: ___

- [ ] On a scene tagged `DOME` only (no `SBS`) — if any in your library — the Enter VR button is NOT visible. (Skip if no such scene exists.)
  - Result: ___ — note: ___

- [ ] On a 2D scene (no `DOME` and no `SBS` tags), the Enter VR button is NOT visible.
  - Result: ___ — note: ___

- [ ] Page source on a 2D scene does NOT contain `aframe.min.js` (use Meta Browser's "View Page Source" if accessible, or skip if it's not).
  - Result: ___ — note: ___

## VR session lifecycle

- [ ] Click "Enter VR" — Meta Browser's WebXR consent prompt appears (the first time).
  - Result: ___ — note: ___

- [ ] Grant consent — page swaps to immersive-vr. Headset shows the half-sphere video.
  - Result: ___ — note: ___

- [ ] Stereo split is correct: closing one eye then the other shows different views (each eye sees its half of the SBS texture). If both eyes see the same image, stereo is broken.
  - Result: ___ — note: ___

- [ ] Looking left/right reveals the rest of the 180° field (not a black wall, not a duplicated front view).
  - Result: ___ — note: ___

- [ ] Looking behind shows blank/black (the half-sphere doesn't cover 360°). NOT a mirror copy of the front.
  - Result: ___ — note: ___

- [ ] Audio is audible (or unmute via Quest controller / headset menu if browser autoplay is muted).
  - Result: ___ — note: ___

## Playback continuity

- [ ] In 2D, scrub the video to 1:00. Click Enter VR. The VR scene starts playing at 1:00 (not at 0:00).
  - Result: ___ — note: ___

- [ ] Watch a few seconds in VR. Exit VR (close the WebXR session via Meta Browser's exit overlay, or remove the headset). The 2D player is visible again with `currentTime` at the position the VR session ended.
  - Result: ___ — note: ___

## Existing M1 features regression

- [ ] On the same VR-eligible scene, click a star — rating updates after page reload.
  - Result: ___ — note: ___

- [ ] Toggle favorite — state persists.
  - Result: ___ — note: ___

- [ ] Add a tag via the input — chip appears.
  - Result: ___ — note: ___

- [ ] Remove a tag — chip disappears.
  - Result: ___ — note: ___

- [ ] O-counter +/- — number updates.
  - Result: ___ — note: ___

- [ ] Organized toggle — state changes.
  - Result: ___ — note: ___

- [ ] M1 search at `/browse?q=...` still works.
  - Result: ___ — note: ___

- [ ] Sidebar performer / studio / tag click still navigates to the entity-filtered grid.
  - Result: ___ — note: ___

## Non-VR-scene baseline (M1 unchanged)

- [ ] Open a non-VR scene. 2D player works as in M1. No "Enter VR" button. No console errors related to A-Frame.
  - Result: ___ — note: ___

## Overall

- [ ] All checks PASS → proceed to M3 design.
- [ ] At least one FAIL → write up in result.md and surface to user.
