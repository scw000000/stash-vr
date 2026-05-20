# M4b design: VR control panel polish

**Date:** 2026-05-09
**Status:** Drafting (brainstorming approved 2026-05-09).
**Predecessors:** [M3c SKYBOX-style controller mappings](2026-05-08-m3c-skybox-controller-mappings.md) (panel hide/show, controller bindings). [M4a web view polish](2026-05-09-m4a-web-polish.md) (clickable chips, AJAX mutations, no shared code with M4b).
**Successors:** **M4c** — in-VR search/browse: button on the panel summons a scene grid in 3D space.
**Reference player:** Behavior parity with [SKYBOX](https://skybox.xyz/support#Watch-Videos), per [the consolidated reference](../../research/2026-05-08-skybox-ui-reference/reference.md). M4b targets section §4.1 (playback bar) of that reference.

---

## 1. Context (why this milestone)

M3c's playback panel is a single-row strip: `Play/Pause | -10s | +10s | Format | ? | Exit`. It works but feels primitive next to SKYBOX's playback bar (reference §4.1), which the user has been clear stash-vr should mirror. The user's specific asks (item 4 of the M4 brainstorm) plus "all common video player features" call for: scrub bar with drag-to-seek, current-time display, scene-title display, scene-marker dots on the scrub bar, mute, captions, playback speed, and loop.

The ±10s buttons are dropped because M3c's thumbstick X already provides ±10s discrete seek and the new scrub bar handles fine seek by drag. Advanced Settings (3D offset / brightness / monoscopic) is dropped because Quest 3's system controls plus M3b's Format Stereo row already cover the practical use cases — re-add as M4b-followup if a real scene reveals the gap.

## 2. Goal & non-goals

**Goal:** From inside WebXR, the user has a control panel that exposes every common video-player control they'd expect from SKYBOX or HereSphere — scrub, time, title, scene markers, mute, captions, speed, loop — without leaving the immersive scene.

**Success criteria, manually verified on Quest 3 / Meta Browser:**

1. Summon the panel: scene title text top-left, current/total time top-right, scrub bar center, button row at bottom.
2. Scrub-bar drag (laser-grab the playhead and move): video seeks in real time. Release lands at visual position within ~1 s.
3. Scene markers appear as dots on the scrub bar at correct positions. Raycast-hover a dot → its title text appears as a floating label.
4. Mute button toggles audio.
5. Captions button opens a picker listing the scene's caption languages plus "Off". Pick a language → subtitles render below the cinema plane (cinema mode) or below center-of-gaze (immersive). Pick "Off" → disappear.
6. Speed button cycles `0.5x → 1.0x → 1.25x → 1.5x → 2.0x → 0.5x` with the label showing current rate. `video.playbackRate` is updated.
7. Loop button toggles `video.loop`. Highlight on when active.
8. Captions button is hidden when the scene has no captions.
9. M3c regressions absent: panel hide/show, geometry-drag on empty space, thumbstick ±10s seek, B/Y short-press reset, B/Y long-press full recenter.
10. M3a/M3b regressions absent: projection auto-detect, Format picker, three render entities, audio sync, no first-frame flash.

**Non-goals (deferred):**

- **Advanced Settings** (3D offset, brightness, tilt, monoscopic). M4b-followup if any of these become real pain points.
- **Multi-track audio selector.** v1 plays whatever stream's audio track is default.
- **Previous/Next scene buttons.** Comes naturally with M4c (the in-VR search). No playlist context in v1.
- **Auto-next on video end.** Possible follow-up.
- **Heatmap as scrub-bar background.** Easy to add as a textured plane behind the bar; deferred to keep this milestone focused. Become a quick follow-up after M4b lands.
- **Funscript timeline display.** No haptic device support is in scope.
- **±10s explicit buttons.** Removed; M3c thumbstick X covers the role.

## 3. Layout

The panel grows from M3c's single 0.4 m row to a three-row stack ~1.0 m tall, ~3.0 m wide.

```
┌────────────────────────────────────────────────────────┐
│  Scene Title                            0:12:34 / 1:23:45  │  Row 1: title + time     0.18m
├────────────────────────────────────────────────────────┤
│  ●━━━━━━━━━●━━━━━●━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━   │  Row 2: scrub bar         0.20m
├────────────────────────────────────────────────────────┤
│ 🔊━━●━━━ │ CC │ ▶/⏸ │ 1.0x │ 🔁 │ Format │ ? │ Exit  │  Row 3: vol slider + 7 buttons  0.30m
└────────────────────────────────────────────────────────┘
```

Width 3.2 m, height ~0.7 m visible content. The whole panel sits at the M3c position (`0 0.4 -1.5` rotated `-30 0 0`) and stays inside `vrControlsRoot` so M3c's panel-toggle still works as a unit.

Row 3 layout:
- **Volume widget** (left): icon + horizontal slider, 0.62 m wide total. Tap icon to mute/unmute; drag the slider thumb to set volume 0..1.
- **7 buttons** to the right of the volume widget, each 0.32 m wide with 0.04 m spacing: CC, Play/Pause, Speed (1.0x), Loop, Format, Help (?), Exit.

## 4. Behavior details

### 4.1 Scrub bar with drag-to-seek

DOM:

```html
<a-entity id="vrScrubBar" position="0 0.10 0.01">
  <a-plane class="vr-scrub-bg" width="2.8" height="0.05" color="#444"></a-plane>
  <a-plane id="vrScrubFill" class="vr-scrub-bg" width="..." height="0.05" color="#3776c2" position="..."></a-plane>
  <a-entity id="vrScrubMarkers"></a-entity>  <!-- populated from data -->
  <a-entity class="vr-btn vr-scrub" id="vrScrubThumb" position="0 0 0.005"
            geometry="primitive:plane;width:0.06;height:0.10" material="color:#fff"></a-entity>
</a-entity>
```

The `vr-scrub` class on the thumb is the opt-out signal that M3c's trigger state machine reads to skip the geometry-drag branch on triggerdown.

Tick handler updates the playhead position and the fill width from `video.currentTime / video.duration` every animation frame.

**Drag protocol:**

1. `triggerdown` while raycaster hit is on `.vr-scrub` (or `.vr-scrub-bg`): start scrub session. Capture initial cursor world-x; compute scrub-bar's world-x extent; record intent in M3c so its drag/click branches are skipped for this trigger.
2. Per tick during the session: convert the current raycaster intersection point to a `t` value in `[0, video.duration]`, write `video.currentTime = t` (throttled to ~80 ms between writes to avoid Stash-side seek thrash), and snap the playhead.
3. `triggerup`: final write of the throttled value.

Throttle implementation: simple `if (now - lastWrite > 80) { write; lastWrite = now; }` plus a guaranteed final write on `triggerup`.

**Click-to-seek (no drag):** if the user just clicks the bar without holding, the same code path applies — the single-frame raycaster intersection sets `currentTime` once. M3c's click-vs-drag distinction (5 cm or 250 ms threshold) doesn't apply on `.vr-scrub` because we always seek immediately.

### 4.2 Scene markers

`vd.SceneParts.SceneMarkers` carries `[{seconds, title, primary_tag, ...}]`. M4b consumes only `seconds` and `title`. For each marker the panel emits a small `<a-entity class="vr-btn vr-scrub-marker" data-marker-seconds=... data-marker-title="...">` dot at world-x `barLeft + (marker.seconds / duration) * barWidth`.

Hover (`raycaster-intersected` event): a floating `<a-text>` tooltip (`vrScrubMarkerTip`) attaches near the dot showing the marker title. Hide on `raycaster-intersected-cleared`.

Click (`triggerup` while hit on `.vr-scrub-marker`): sets `video.currentTime = marker.seconds`. Single-frame seek.

If `vd.SceneParts.SceneMarkers` is empty (or `vd.SceneParts.Files[0].Duration` is zero), markers row stays empty.

### 4.3 Title display

Top-left text node, server-rendered:

```html
<a-text id="vrTitle" value="{{.Title}}" align="left" color="#fff" width="3.0" position="-1.4 0.30 0.01"></a-text>
```

A-Frame's `<a-text>` truncates by default with `wrap-count`; the panel sets `wrap-count: 36` so longer titles ellipsize visually instead of wrapping into the time text region. (A-Frame doesn't natively ellipsize — long text gets cut at column. Acceptable.)

### 4.4 Time display

Top-right text node, JS-driven from `video.currentTime / video.duration`:

```html
<a-text id="vrTime" value="0:00 / 0:00" align="right" color="#fff" width="3.0" position="1.4 0.30 0.01"></a-text>
```

Tick handler updates the value once per second (not per frame — text re-renders are expensive).

Format helper: `formatTime(s) → "M:SS"` for durations < 1 h, else `"H:MM:SS"`.

### 4.5 Mute button

```html
<a-entity class="vr-btn" data-action="mute" position="-1.20 -0.20 0.01" ...>
  <a-text id="vrMuteIcon" value="🔊" .../>
</a-entity>

<!-- Volume slider track + thumb to the right of the icon. -->
<a-entity id="vrVolumeSlider" position="-0.96 -0.20 0.01">
  <a-plane class="vr-vol-track vr-btn" width="0.40" height="0.04"
           color="#444" material="opacity:0.95"></a-plane>
  <a-plane id="vrVolumeFill" width="0.40" height="0.04" color="#3776c2"
           material="opacity:0.95" position="0 0 0.001"></a-plane>
  <a-entity id="vrVolumeThumb" class="vr-btn vr-vol-thumb"
            geometry="primitive:plane;width:0.04;height:0.10"
            material="color:#fff" position="0.20 0 0.005"></a-entity>
</a-entity>
```

**Mute icon click:** `video.muted = !video.muted`. Update icon to 🔊 / 🔇.

**Volume slider:** behaves like the M4b scrub bar (§4.1) but operates on `video.volume` (0..1) instead of `currentTime`. Trigger-down on the thumb or the track starts a drag session that sets `video.volume = ratio` per tick, where `ratio = clamp((cursorLocalX + 0.20) / 0.40, 0, 1)`. Throttle is unnecessary (volume writes are cheap). Tap (no drag) at a position seeks to that volume immediately.

Volume thumb's X position updates per tick from `video.volume`. Mute is reflected by the icon only — the slider stays at its underlying-volume position so unmute restores level.

Volume initializes to 1.0 (max) on page load. Persists for the session in JS; not across page loads.

### 4.6 Captions

Button:

```html
<a-entity class="vr-btn" id="vrCCBtn" data-action="cc" position="-0.88 -0.20 0.01" ...>
  <a-text value="CC" .../>
</a-entity>
```

If `vd.SceneParts.Captions` is empty, the entity is not rendered (server-side `{{if .Captions}}`).

Click → toggle `vrSubtitlePicker` sub-panel:

```html
<a-entity id="vrSubtitlePicker" position="0 1.4 -1.5" rotation="-15 0 0" visible="false">
  <a-plane id="vrSubtitlePickerBg" width="2.0" height="..." color="#000" material="opacity:0.7"></a-plane>
  <a-text value="Subtitles" align="left" color="#fff" width="3" position="-0.9 ... 0.01"></a-text>
  <!-- "Off" + one button per language; JS computes positions with row wrap. -->
</a-entity>
```

Picker is hidden by default; toggled by the CC button (mutually exclusive with `vrFormatPicker` and `vrHelpPanel` so they don't overlap — share a `closeAllSubpanels()` helper).

**Layout:** buttons render in a wrapping flex of up to 4 per row. Background plane height grows with the number of rows. With 1 caption: 1 row ("Off" + lang). With 5 captions: 2 rows. With 12: 4 rows. No scrolling needed because real scenes rarely exceed ~10 languages.

On a language pick:

1. Fetch the caption file: `GET /browse/scene/{id}/caption?lang=...&type=...` (new proxy route that wraps the Stash caption URL with the API key, like `/cover/{id}` does for thumbnails). Returns the file body in whatever format Stash served — VTT or SRT typically.
2. Parse the cue list with `parseSubtitles(text)` — handles **both VTT and SRT**. The two formats share the timing line shape (`HH:MM:SS.mmm --> HH:MM:SS.mmm`) up to the millisecond separator (VTT uses `.`, SRT uses `,`); the parser regex matches `[\.,]` so both work without branching. SRT cue-number lines (e.g., `1`, `2` on their own line before each timing) don't match the regex and are skipped naturally. Other formats (e.g., ASS) are unsupported in v1 — the parser returns an empty cue list, the subtitle plane stays hidden.
3. Set `currentCues = parsed`, `currentLang = lang`. Hide the picker.

On Off:

1. Set `currentCues = []`, `currentLang = ""`. Hide the picker.

### 4.7 Subtitle rendering plane

Two parents depending on mode:

- **Cinema mode:** plane is a child of the cinema plane (`vrFlat`), positioned at its bottom edge (`0 -1.0 0.01` in plane-local coords).
- **Immersive (sphere/fisheye):** plane is a child of the camera, positioned in front-center-low (`0 -0.5 -1.5` in camera-local coords).

The active parent is set whenever the active geometry changes (M3b's projection swap path) — same hook as M3c's `activeGeometry()`.

Tick handler: every frame, find the active cue (`currentCues.find(c => c.start ≤ video.currentTime ≤ c.end)`), set the plane's text to the cue's `text`, set visibility to true. If no active cue, set visibility false.

```html
<a-entity id="vrSubtitlePlane" visible="false">
  <a-plane width="1.6" height="0.18" color="#000" material="opacity:0.5"></a-plane>
  <a-text id="vrSubtitleText" value="" align="center" color="#fff" width="3" position="0 0 0.005"></a-text>
</a-entity>
```

Reparenting on geometry change: detach from current parent, append to new parent's `<a-entity>`. A-Frame supports this via `el.parentNode.removeChild(el); newParent.appendChild(el);` or programmatic Three.js `obj.parent.remove(obj); newParent.add(obj);`. Use the Three.js path because it survives A-Frame's component lifecycle cleanly.

### 4.8 Playback speed

```html
<a-entity class="vr-btn" data-action="speed" position="-0.24 -0.20 0.01" ...>
  <a-text id="vrSpeedLabel" value="1.0x" .../>
</a-entity>
```

Click cycles through `[0.5, 1.0, 1.25, 1.5, 2.0]`. Sets `video.playbackRate = next`. Updates label text.

State persists for the session (in JS) but not across page loads.

### 4.9 Loop

```html
<a-entity class="vr-btn" id="vrLoopBtn" data-action="loop" position="0.08 -0.20 0.01" ...>
  <a-text value="🔁" .../>
</a-entity>
```

Click toggles `video.loop`. When on, button color highlights (e.g., `#3776c2` instead of `#2c5282`).

### 4.10 Existing buttons (Format, Help, Exit)

Unchanged from M3c. Positions shift in row 3 to make room for the new buttons:

| Position (x in panel-local) | Button |
|---|---|
| -1.20 | Mute |
| -0.88 | CC |
| -0.56 | Play/Pause |
| -0.24 | Speed |
|  0.08 | Loop |
|  0.40 | Format |
|  0.72 | Help |
|  1.04 | Exit |

## 5. Server / data path

### 5.1 GraphQL fragment additions

Verify [internal/stash/gql/documents/query.graphql](../../../internal/stash/gql/documents/query.graphql) `SceneParts` fragment includes:

- `scene_markers { seconds, title, primary_tag { ... } }` — already includes via `SceneMarkerParts`. Verify the fragment exposes `seconds` and `title`. If not, add.
- `captions { caption_type, language_code }` — already present (line 182-184).
- `paths { caption }` — already present (line 180).

If `seconds` is not on `SceneMarkerParts`, add it and regen.

### 5.2 New caption proxy route

```
GET /browse/scene/{id}/caption?lang={code}&type={type}
```

Wraps Stash's `paths.caption` URL with the API key, same pattern as `/cover/{id}`. Returns `text/vtt` body.

Lives in [internal/api/browse/router.go](../../../internal/api/browse/router.go) and a new `caption.go` handler. Uses `stash.ApiKeyed(*vd.SceneParts.Paths.Caption + "?lang=...&type=...")`.

### 5.3 Template data additions

`SceneDetailData` gains:

```go
Captions []CaptionRef
SceneMarkers []SceneMarker
```

```go
type CaptionRef struct {
    LanguageCode string
    CaptionType  string
}
type SceneMarker struct {
    Seconds float64
    Title   string
}
```

`scene.go` populates from `vd.SceneParts.Captions` and `vd.SceneParts.SceneMarkers`.

## 6. Files touched

```
internal/stash/gql/documents/query.graphql  <- verify SceneMarkerParts.seconds + .title; regen if needed
internal/api/browse/data.go                 <- CaptionRef, SceneMarker, fields on SceneDetailData
internal/api/browse/scene.go                <- populate new fields
internal/api/browse/router.go               <- mount /browse/scene/{id}/caption
internal/api/browse/caption.go (new)        <- caption proxy handler
internal/static/browse_scene.gohtml         <- expanded panel (3 rows), vrScrubBar, vrSubtitlePicker, vrSubtitlePlane,
                                              new buttons (mute, CC, speed, loop), tick handlers, VTT parser, scene markers
internal/static/m3c-controls.js             <- skip drag/click classification when triggerdown hits .vr-scrub or .vr-scrub-marker
```

No test suite. `go vet ./...` and `go build ./...` are the standard checks.

## 7. Risks

- **VTT parser scope creep.** The WebVTT spec is ~200 pages. v1 supports only `HH:MM:SS.mmm --> HH:MM:SS.mmm` cue lines plus plain text payloads. Styling, voice tags, regions, settings — all dropped. Unsupported cues are skipped; broken VTT files fall back to "no subtitles" silently.
- **playbackRate ≠ 1 makes audio scrub.** Some browsers preserve pitch by default; some don't. Use `video.preservesPitch = true` (Chromium-based, Meta Browser is Chromium → fine).
- **Scrub-bar throttle interaction with Stash byte-range seeks.** 80 ms throttle = max 12 seeks/sec; Stash should handle. If real testing shows thrash, raise to 150 ms.
- **Camera-attached subtitle plane parent change.** `el.object3D.parent.remove(el.object3D); newParent.add(el.object3D);` works at the Three.js level but bypasses A-Frame's scene graph. A-Frame may re-insert on tick. Workaround: keep the subtitle plane as a child of `<a-scene>` and update its world-position+rotation per tick to match camera-local (0,-0.5,-1.5) — slightly more work but no scene-graph fight.
- **Subtitle text wrapping.** A-Frame `<a-text>` wraps at `wrap-count`. Long cues may wrap to 3+ lines and overflow the plane. Mitigation: set wrap-count=40; let multi-line wrap happen; long cues are rare.
- **Quest 3 frame stability with full panel rendered + VTT tick + scrub tick.** All of these run cheap (text updates, simple math). Should hold 90 fps. If profile shows otherwise, throttle text updates to 4 Hz (250 ms).
- **`vr-scrub` class detection in M3c.** M3c's `_onTriggerDown` currently captures `raycaster.intersected`. Need to add a class probe — small change, isolated.
- **Scene markers density.** If a scene has 50 markers, dots blur. Cap visible dots at ~15 by sampling. v1 just renders all; revisit if a real scene shows the problem.

## 8. Validation

On Quest 3 / Meta Browser:

### A. Initial state
- [ ] Open a scene with multiple captions and ≥3 scene markers; click Enter VR. Panel hidden by default (M3c). Single-click trigger → panel summons.
- [ ] Title text top-left shows scene title (truncated if long).
- [ ] Time text top-right shows `0:00 / M:SS` then advances.
- [ ] Scrub bar with playhead at far left.
- [ ] ≥3 dots on the scrub bar at expected positions.
- [ ] Row 3 shows: volume widget (icon + slider) on the left, then 7 buttons (CC, Play/Pause, Speed, Loop, Format, Help, Exit).

### B. Scrub bar
- [ ] Laser-grab the playhead, drag right → video advances in real time.
- [ ] Drag left → rewinds.
- [ ] Release at a position → final time within ~1 s of visual position.
- [ ] Click bare scrub bar (no drag) → video jumps to that position.

### C. Scene markers
- [ ] Hover a marker dot → tooltip shows marker title.
- [ ] Click a dot → video jumps to its `seconds`.

### D. Volume
- [ ] Click 🔊 icon → audio mutes; icon changes to 🔇.
- [ ] Click again → unmutes.
- [ ] Drag volume slider thumb left → video volume drops continuously.
- [ ] Drag right → volume rises.
- [ ] Tap a position on the slider track (no drag) → volume jumps there.
- [ ] After mute then unmute → volume returns to previous slider position (not max).

### E. Captions
- [ ] Click CC → picker opens listing each language plus "Off". With ≥5 languages, buttons wrap to multiple rows; panel grows to fit.
- [ ] Pick a language → subtitles render below cinema plane (cinema scene) or below center-of-gaze (immersive).
- [ ] Switch to a different language → subtitles re-parse and update.
- [ ] On a scene whose caption track is SRT (not VTT) — pick the language → subtitles still render correctly (parser handles both formats).
- [ ] Pick "Off" → subtitles disappear.
- [ ] Open a scene with no captions → CC button is absent.

### F. Speed
- [ ] Click 1.0x → label cycles through `1.25x → 1.5x → 2.0x → 0.5x → 1.0x`.
- [ ] At 2.0x video plays fast with synced audio (chipmunk or pitch-preserved depending on browser).
- [ ] At 0.5x video slows.

### G. Loop
- [ ] Click 🔁 → button highlights.
- [ ] Wait until video ends → restarts at 0.
- [ ] Click again → off; video stops at end.

### H. Cross-mode
- [ ] In immersive sphere mode: sub plane tracks camera. Time, scrub, all buttons work.
- [ ] In fisheye mode: same.
- [ ] In cinema mode: sub plane sits at bottom of cinema plane.

### I. M3c regressions
- [ ] Trigger single-click on empty space toggles panel.
- [ ] Trigger double-click on empty space play/pauses.
- [ ] Trigger hold + move on empty space drags geometry (NOT triggered when starting on scrub bar).
- [ ] Thumbstick L/R = ±10s seek.
- [ ] Thumbstick U/D = scale.
- [ ] B/Y short = reset / recenter.
- [ ] B/Y long = full recenter.

### J. M3a/M3b regressions
- [ ] Format picker still works.
- [ ] Auto-detect produces correct projection on first load.
- [ ] Audio sync intact.
- [ ] No first-frame flash on Enter VR.

## 9. Open follow-ups for next milestones

- **Heatmap on scrub bar.** The cover image already overlays a heatmap PNG via [internal/api/heatmap](../../../internal/api/heatmap). Same PNG can become the scrub bar's background texture. ~1 hour.
- **Advanced Settings sub-panel.** 3D offset slider, brightness slider. Defer until a real scene reveals the gap.
- **Volume slider.** If "Mute or full" is too coarse.
- **SRT subtitle support.**
- **Auto-next on video end.** Pairs naturally with M4c (in-VR search context for "what's next").
- **Multi-track audio selector.**
