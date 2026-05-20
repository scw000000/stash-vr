# M3c SKYBOX-style controller mappings — Quest 3 Meta Browser checklist

**Run on:** Quest 3 hardware, Meta Browser. The new controller bindings rely on `triggerdown`/`triggerup`, `a/b/x/ybuttondown/up`, and `gamepad.axes` — all of which Meta Browser exposes via WebXR + the `tracked-controls` A-Frame component. Other VR browsers (e.g., DeoVR's in-VR browser) are out of scope.

**URL to open:** `https://stash-vr.duckdns.org/browse`

**Build:** `C:\Users\scw00\Downloads\stash-vr.exe` (built 2026-05-08; commit will be filled in at validation time).

For each criterion: PASS / FAIL / PARTIAL + one-line note.

## A. Initial state on Enter VR

- [ ] Open a cinema scene (no VR tags). Click Enter VR. Both lasers are HIDDEN; playback panel is HIDDEN. Only the controller models (Quest's default oculus-touch model) are visible.
  - Result: ___ — note: ___

- [ ] Repeat on an immersive scene (DOME 180° SBS). Same hidden-by-default state.
  - Result: ___ — note: ___

## B. Trigger — single click in empty space

- [ ] Single-click right trigger pointing at empty space. After ~400 ms, panel + both lasers appear.
  - Result: ___ — note: ___

- [ ] Single-click again at empty space. Panel + lasers hide.
  - Result: ___ — note: ___

- [ ] Repeat with LEFT trigger — same behavior.
  - Result: ___ — note: ___

## C. Trigger — single click on a panel button (M3b regression)

- [ ] With panel visible, point laser at the +10s button and single-click trigger. Video advances 10 s. Panel STAYS visible (no panel toggle).
  - Result: ___ — note: ___

- [ ] Same with -10s, Play/Pause, Format, Help, Exit. All fire without toggling panel.
  - Result: ___ — note: ___

## D. Trigger — double click

- [ ] Double-click right trigger pointing at empty space (≤400 ms apart). Video toggles play/pause. No panel-toggle fires.
  - Result: ___ — note: ___

- [ ] Same with left trigger.
  - Result: ___ — note: ___

- [ ] Try a deliberately-too-slow double-click (~600 ms apart). First click should fire panel-toggle; second click should fire panel-toggle again (or play-pause if it fell within window — note observed behavior).
  - Result: ___ — note: ___

## E. A and X buttons

- [ ] Single-click A (right). Panel toggles same as trigger.
  - Result: ___ — note: ___

- [ ] Single-click X (left). Panel toggles.
  - Result: ___ — note: ___

- [ ] Double-click A. Play/pause.
  - Result: ___ — note: ___

- [ ] Double-click X. Play/pause.
  - Result: ___ — note: ___

- [ ] Trigger-up + A-up rapid (cross-input double click). Should fire play/pause (unified `_pendingClick`).
  - Result: ___ — note: ___

## F. Trigger hold + drag

### F.1 Cinema mode (flat 2D scene)

- [ ] Hold right trigger and move controller in any direction. The cinema plane translates with the controller. Plane keeps facing user (lookAt).
  - Result: ___ — note: ___

- [ ] Release. Plane drops in place.
  - Result: ___ — note: ___

- [ ] Drag with both hands simultaneously. Confirm geometry follows one of them sensibly (or jitters predictably between both — both is acceptable).
  - Result: ___ — note: ___

### F.2 Immersive (DOME 180° SBS)

- [ ] Hold right trigger and move controller. Sphere translates with the controller (the 360° projection's "center" shifts).
  - Result: ___ — note: ___

- [ ] Confirm the sphere drag feels usable, not nauseating.
  - Result: ___ — note: ___

## G. Thumbstick L/R seek

- [ ] Push right thumbstick fully RIGHT (past 0.7). Video advances 10 s once.
  - Result: ___ — note: ___

- [ ] Hold past 0.7. NO additional seeks fire (single-shot).
  - Result: ___ — note: ___

- [ ] Release to ~0, push right past 0.7 again. Another 10 s seek fires.
  - Result: ___ — note: ___

- [ ] Push left thumbstick fully LEFT. Video rewinds 10 s.
  - Result: ___ — note: ___

- [ ] Push thumbstick partially (e.g., 0.5). NO seek fires (below threshold).
  - Result: ___ — note: ___

## H. Thumbstick U/D scale

### H.1 Cinema mode

- [ ] Push thumbstick UP and hold. Cinema plane scales up smoothly. Stop within range.
  - Result: ___ — note: ___

- [ ] Push DOWN. Scales down.
  - Result: ___ — note: ___

- [ ] Push UP and hold long enough to hit 5× clamp. Verify it stops growing.
  - Result: ___ — note: ___

- [ ] Push DOWN long enough to hit 0.3× clamp. Verify it stops shrinking.
  - Result: ___ — note: ___

### H.2 Immersive

- [ ] Push UP. Sphere scales up (zoom-equivalent — content fills more of FOV).
  - Result: ___ — note: ___

- [ ] Push DOWN. Sphere scales down.
  - Result: ___ — note: ___

- [ ] Hit the 5× clamp; verify it stops.
  - Result: ___ — note: ___

- [ ] Hit the 0.3× clamp; verify the sphere's back face doesn't clip the camera (uncomfortable visual).
  - Result: ___ — note: ___

### H.3 Dual-stick scale

- [ ] Push BOTH thumbsticks UP simultaneously. Scale-up speed is faster than one stick (~2× linear, not compounded).
  - Result: ___ — note: ___

## I. B and Y reset

### I.1 Cinema short-press

- [ ] After dragging cinema plane somewhere weird, short-press B (release before 500 ms). Plane snaps back to (0, 1.6, -3) scale 1.
  - Result: ___ — note: ___

- [ ] Same with Y.
  - Result: ___ — note: ___

### I.2 Immersive short-press

- [ ] After turning around or recentering, short-press B in immersive mode. View yaw recenters (you face forward again).
  - Result: ___ — note: ___

- [ ] Same with Y.
  - Result: ___ — note: ___

### I.3 Long-press (both modes)

- [ ] Hold B for ≥500 ms. View recenters AND geometry resets. (In cinema: plane returns to default and you face forward; in immersive: yaw recenters AND sphere returns to default scale/position.)
  - Result: ___ — note: ___

- [ ] Release after long-press. NO additional reset-short fires.
  - Result: ___ — note: ___

- [ ] Same with Y.
  - Result: ___ — note: ___

## J. Help cheatsheet

- [ ] Tap "?" button on playback panel. Cheatsheet appears with all 8 binding rows from spec §3.
  - Result: ___ — note: ___

- [ ] Read the rows; verify each input/action matches actual implementation.
  - Result: ___ — note: ___

- [ ] Tap "X" close button on cheatsheet. Cheatsheet hides.
  - Result: ___ — note: ___

- [ ] Tap "?" again. Cheatsheet re-opens.
  - Result: ___ — note: ___

- [ ] Tap "?" while Format picker is open. Both visible (overlapping). Note observation; decide if mutual exclusion needs to be added.
  - Result: ___ — note: ___

## K. M3a/M3b regression

- [ ] On an immersive scene with the V-shape projection mismatch (e.g., scene 5535 from M3b notes), the M3a auto-detection still runs and produces the original (wrong) projection on first load.
  - Result: ___ — note: ___

- [ ] Open Format picker. Pick `FishEye + 200° + SBS`. Renderer rebinds. V-shape gone.
  - Result: ___ — note: ___

- [ ] Reload scene. Auto-detect picks the corrected projection (tags written back from the picker).
  - Result: ___ — note: ___

- [ ] Audio sync: video plays with synced audio, no drift over 30 s.
  - Result: ___ — note: ___

- [ ] No first-frame black flash on Enter VR.
  - Result: ___ — note: ___

- [ ] M1 surfaces (scene grid, sidebar, search) load and behave normally.
  - Result: ___ — note: ___

## L. Edge cases / surprises

- [ ] Re-enter VR (exit, then click Enter VR again). Verify panel and lasers are hidden again on second entry. (If they retain prior visibility state, file as a follow-up.)
  - Result: ___ — note: ___

- [ ] Hold trigger for a drag while ALSO pushing thumbstick Y (drag + scale at the same time). Both behave concurrently? Or does one block the other?
  - Result: ___ — note: ___

- [ ] Open the help cheatsheet, then double-click trigger pointing at empty space. Does play/pause fire (the trigger is on empty space) or does the cheatsheet eat the click?
  - Result: ___ — note: ___

- [ ] Anything else weird? Free-form.
  - ___
