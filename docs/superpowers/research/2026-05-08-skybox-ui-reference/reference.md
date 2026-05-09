# SKYBOX VR Video Player — UI reference for stash-vr milestones

**Date assembled:** 2026-05-08
**Purpose:** Canonical reference for "what does SKYBOX do" when matching SKYBOX behavior in stash-vr's WebXR player. The user has been clear that stash-vr's in-VR UX should mirror SKYBOX/HereSphere conventions; this doc captures what was extractable so we don't have to re-research per milestone.
**Caveat:** SKYBOX's `https://skybox.xyz/support` is a JavaScript-only Vue SPA — `WebFetch`, `curl`, and Wayback Machine snapshots all return only the SPA shell (~1-3 KB) without rendered support content. The information below is assembled from:

1. Screenshots the user supplied earlier in this conversation showing the controller-bindings table and the format-menu layout (verbatim text from the SKYBOX support page rendered in their headset).
2. SKYBOX official forum (Flarum, server-rendered HTML — extractable). Cited inline.
3. SKYBOX SPA's main JS bundle (`/js/app.*.js`) — UI string fragments confirming menu structure and component names.
4. Public web search results citing forum/Steam/Wayback snippets where available.

Where a claim has multiple sources, they are listed; where a claim comes from a single source, the source is cited inline. Items marked _(inferred)_ are assembled from indirect evidence rather than verbatim quotes.

---

## 1. Controller bindings during video playback (Quest / Oculus Touch)

Source: user-supplied screenshot of `https://skybox.xyz/support#Watch-Videos`, "During Video Playback" table.

| Button | Control | Effect |
|---|---|---|
| A / X / Trigger | Single click | Bring up / Hide the control panel |
| A / X / Trigger | Double click | Play / Pause |
| A / X / Trigger | Hold and move the controller | Move the screen |
| B / Y | Long-press | Reset your screen |
| Thumbstick | Move left / right | Rewind / Fast-forward |
| Thumbstick | Move up / down | Zoom in / out |
| Oculus button | Long-press | Recenter your headset view |

**Notes:**
- "Move the screen" = drag the virtual screen / cinema plane around the user's environment via raycast-anchored hold-drag.
- "Reset your screen" = restore the screen's default position/orientation/scale.
- "Recenter" = re-orient the headset's tracking origin so the user is facing forward.
- Controller-button mapping is **the same** for both controllers — SKYBOX explicitly lists "A/X/Trigger" together rather than separate mappings per hand.

These mappings drive the design of stash-vr's M3c (controller-mappings overhaul). The important behavioral signals:
- Single-click as panel toggle (NOT play/pause) is non-obvious — most VR players use single-click for play/pause. SKYBOX users have learned this convention; matching it removes their re-learning cost.
- Hold-and-move for screen position needs a 6-DOF drag mechanism, not a click.
- Long-press as reset/recenter is standard.

## 2. Format selector menu

Source: user-supplied screenshot, format-menu floating panel above the playback bar.

The menu is a 2D-style HUD that floats above the playback bar. Three rows of mutually-exclusive buttons + a Reset/Auto utility row:

```
┌───────────────────────────────────────────┐
│  ⊕ Normal     ⓘ FishEye     ⓘ YouTube      │   ← Type tabs (top row)
├───────────────────────────────────────────┤
│  [2D Single]  [3D SBS]      [3D TB]        │   ← Stereo row
│  [Cinema  ]   [VR 180°]     [VR 360°]      │   ← Geometry row
├───────────────────────────────────────────┤
│  ⤴ Reset                  💡 Auto           │   ← Utility row
└───────────────────────────────────────────┘
```

**Type tabs:**
- **Normal** = equirectangular projections (DOME 180°, SPHERE 360°, plus 2D flat).
- **FishEye** = circular fisheye projections. Sub-menu (per forum thread 1938, [SKYBOX dev confirmation 2023-07-31](https://forum.skybox.xyz/d/1938-filename-rule-for-fisheye-190)) exposes 180° / 190° / 200° (= MKX200) / RF52 / Canted fisheye sub-types when this tab is active. The sub-types are surfaced after tapping FishEye in the Type row, not as a flat list.
- **YouTube** = EAC (Equi-Angular Cubemap) — YouTube's 360° projection format. SKYBOX terms it "Youtube 360 EAC" or "Youtube 180 Fisheye" in their docs.

**Stereo row** (independent of Type):
- **2D Single** — mono.
- **3D SBS** — side-by-side stereo (left half = left eye).
- **3D TB** — top-bottom stereo (top half = left eye, per forum/dev convention).

**Geometry row** (independent of Type, but Cinema is mono-only):
- **Cinema** — flat 2D plane in 3D space (the "virtual cinema"). Stereo row is moot when Cinema is selected (forced 2D Single).
- **VR 180°** — half-sphere.
- **VR 360°** — full sphere.

**Reset** (per JS bundle string `'(3) If the video does not seem right at the moment, click "Reset" in the "Stereo Mode" menu.'`, `/js/app.*.js`):
- Reverts the manual override(s) back to the auto-detected stereo/geometry/type.

**Auto** _(inferred)_:
- Re-runs auto-detection from scratch (typically equivalent to Reset in practice).

**Notes on cross-product validity** (matches stash-vr's M3b spec):
- `Normal + 200°` not surfaced (no 200° equirectangular exists; SKYBOX hides it).
- `FishEye + Cinema` not surfaced (fisheye is inherently 3D-projected).
- `FishEye + 360°` not surfaced (SKYBOX uses the term for ≤200°).
- `Cinema + any stereo other than 2D` not honored.

## 3. Filename keyword auto-detection

### 3.1 Canonical rules from SKYBOX

Sources:

- **Forum thread 157**, [post by SKYBOX dev "Andy" 2018-04-27](https://forum.skybox.xyz/d/157-filename-rules-for-vr-format) — original ruleset.
- **Forum thread 1938**, [post by SKYBOX dev "Lucy" 2023-10-17](https://forum.skybox.xyz/d/1938-filename-rule-for-fisheye-190) — Fisheye 190° additions.
- The user-supplied screenshot of the support page reproduces the canonical table.

**3D format (stereo) keywords:**

| Keyword group | Maps to |
|---|---|
| `3dv`, `TB`, `Top+Bottom`, `OU`, `Over+Under`, `HOU`, `Half+OU`, `Half+Over+Under` | TB (top-bottom) |
| `3dh`, `LR`, `Left+Right`, `SBS`, `Side+By+Side`, `HSBS`, `Half+SBS`, `Half+Side+By+Side` | SBS (side-by-side) |

The `+` sign is filler — replaceable by `_`, space, hyphen, dot, or nothing. Capitalization and order within the filename are ignored.

**Geometry / FOV keywords** (from canonical post):

| Keyword | Maps to |
|---|---|
| `360`, `360°` | 360° equirectangular |
| `180x180`, `180°` | 180° equirectangular |
| `F180`, `VR180` | 180° fisheye (a.k.a. "Youtube 180") |
| `EAC360` | YouTube EAC 360° |

**Default (no keyword match):** 2D normal movie ("2d screen").

**Examples** (from Andy's 2018-04-27 post):

```
moviename.mp4                       → 2d screen
moviename_360.mp4                   → 2D 360
moviename_360_TB.mp4                → 3D top-bottom 360
moviename_180x180_Side-By-Side.mp4  → 3D side-by-side 360
moviename_EAC360.mp4                → 2d EAC
moviename_F180_OU.mp4               → 3D top-bottom fisheye
```

### 3.2 Per-format keyword tables shown on the support page

Source: user-supplied screenshot of the SKYBOX support `Watch-Videos` section. Verbatim from that screenshot:

| Video format | Keywords (case-insensitive) |
|---|---|
| Normal 2D | `2D_Screen`, `LR_Screen`, `TB_Screen` |
| YouTube EAC | `2D_EAC`, `3D_EAC` |
| VR180° | `2D_180`, `LR_180`, `TB_180` |
| VR360° | `2D_360`, `LR_360`, `TB_360` |
| FISHEYE 180° | `2D_180_FISHEYE`, `LR_180_FISHEYE`, `TB_180_FISHEYE` |
| FISHEYE 190° | `FISHEYE190`, `2D_190_FISHEYE`, `LR_190_FISHEYE`, `TB_190_FISHEYE` |
| FISHEYE 200° | `MKX200`, `Fisheye_200°`, `2D_200_FISHEYE`, `LR_200_FISHEYE`, `TB_200_FISHEYE` |
| Full SBS | `Full_SBS`, `fullsbs` |
| Full TB | `Full_TB`, `fulltb` |

> Example (per the screenshot): "you can rename a 180° Side-By-Side video as `movie_LR_180`."

### 3.3 Aspect-ratio fallback heuristic

Sources: web-search summary citing SKYBOX docs (`vrpupu.io` 2026-01 SKYBOX guide, plus other SKYBOX forum citations).

When neither a tag nor a filename keyword is present, SKYBOX falls back to aspect-ratio inspection:

- Aspect ratio **= 1.0** → mono.
- Aspect ratio **> 1.8** OR filename contains `_LR_` / `SbS` / similar SBS keyword → SBS stereo.
- Otherwise → manual setting required.

Geometry (DOME vs SPHERE vs FISHEYE) defaults are not derivable from aspect ratio alone; SKYBOX expects the user to manually set when AR is ambiguous.

### 3.4 stash-vr alignment

stash-vr's `internal/api/internal/projection.go::Detect` (M3a + M3b) implements a subset of these rules:

- Tag-pass first: matches stash-vr's own `VR_*` tag constants (M3b migration). Maps to {DOME, SPHERE, FISHEYE, MKX200, RF52} × {SBS, TB}.
- Filename-pass second: matches `MKX200`, `FISHEYE190`/`_190_FISHEYE`, `FISHEYE200`/`_200_FISHEYE`, `FISHEYE180`/`_180_FISHEYE`/bare `FISHEYE`, `RF52`, `_360`/`VR360`, `_180`/`VR180`, plus stereo tokens `_LR_`/`LR_`/`_LR.`, `_TB_`/`TB_`/`_TB.`, `_2D_`/`2D_`.
- Aspect-ratio heuristic NOT implemented (deferred — would require pulling the file's aspect ratio from Stash's GraphQL `Files[0].Width`/`Height` and is awkward as a tertiary fallback).
- Generic VR-substring fallback (`1407b52`): any tag whose name contains "VR" → DOME+SBS. Looser than SKYBOX, intentionally — stash-vr's M2 baseline used this and the user has scenes that depend on it.

## 4. Other in-VR UI elements

Sources: SKYBOX SPA JS bundle string fragments (`/js/app.*.js`) + forum threads + user screenshots.

### 4.1 Playback bar (the always-visible HUD when control panel is summoned)

From user's screenshot (format-menu screenshot also shows the playback bar below):

- Current time / duration display: `00:56:03 / 01:56:16`.
- Volume icon (left of left side).
- Subtitles / captions icon.
- Previous-track button.
- Play / Pause button (center).
- Next-track button.
- Settings (gear) icon — opens "Advanced Settings".
- Format toggle (the icon that opens the format menu shown in §2).
- Scrub bar with playhead and elapsed/remaining time.

### 4.2 Control panel

From JS bundle quotes:

- **Stereo Mode button** is on the **right side** of the control panel (`"(1) Click on the 'Stereo Mode' button on the right side of the control panel."`).
- **Subtitle and Track button** is on the **left side** (`"click on the 'Subtitle and Track' button on the left side of the control panel"`).
- **Advanced Setting button** is on the **right** (`"tap the 'Advanced Setting' button on the right of the control panel"`).
- **Heart icon** marks Favorite (`"call up the control panel, click on the 'heart' icon then the video is added to the 'Favorite' list."`).

### 4.3 Advanced Settings menu

Confirmed entries (from forum thread 157 post 1412 by SKYBOX dev, 2018-11-12):

- **3D offset** — adjust to increase/decrease perceived stereo depth. Range exemplified as `0` to `0.5`.
- **Tilt** — pitch adjustment of the virtual screen.
- **Scale** — size of the virtual screen.
- **Monoscopic** — force same image to both eyes ("see the same image in both eyes").

Unconfirmed but suggested by the JS-bundle string `"SKYBOX can play nearly all video format, and provides several advanced settings you may need."` and standard VR-player conventions:

- Brightness / saturation / contrast (typical VR-player advanced settings; not seen verbatim in extracted strings).
- IPD slider — _NOT confirmed_ in any extracted source. SKYBOX may rely on the headset's system-level IPD setting and not expose its own slider. The user's earlier reference to "IPD slider" came from their generic VR-player expectation, not from SKYBOX specifically.

### 4.4 Move-the-screen gesture

From JS bundle quotes:

- `"Then, point your cursor to the screen, move it by holding on the touchpad or trigger. And you can also slide up and down to adjust the screen size."`
- The control panel's Advanced Setting also exposes manual `Tilt` and `Scale` for the screen.

### 4.5 Stereo Mode menu sub-flow

From forum 1938 dev response (Lucy):

> "click Stereo Mode button on the right side of the control panel, select 'Fisheye' option, then you'll find the VR190° option."

So the menu structure is:
1. Tap "Stereo Mode" on the right of the control panel.
2. Pick a top-level option (Normal / FishEye / YouTube — matches §2's Type row).
3. If FishEye is picked, the FOV sub-options appear (180° / 190° / 200° / etc.).

### 4.6 FFR ("Fixed Foveated Rendering")

From forum 1412: `"This is because we are using FFR to optimize the performance of SKYBOX. In the coming v0.2.0, we will lower the FFR level. It should improve the image quality on the edges."`

SKYBOX uses Quest's foveated rendering to keep frame rate up on high-resolution videos at the cost of edge sharpness. Not a UI feature, but explains why SKYBOX video edges may look softer than competitors.

### 4.7 Control panel renders 2D when summoned

From forum 1412: `"when you call the control panel, the image is actually 2D (both eyes display the same image)."`

When the user opens the control panel, SKYBOX temporarily disables stereo and shows the same image to both eyes. This is why the image looks "much clearer" with the panel up — half the parallax-driven blur is gone. _Worth noting because users may report stash-vr's image looks different from SKYBOX's — the difference is partly this rendering quirk._

## 5. Feature names / vocabulary (for matching in stash-vr)

Use SKYBOX's terminology when labeling stash-vr UI to keep the user's mental model consistent:

| SKYBOX term | stash-vr equivalent |
|---|---|
| Stereo Mode | "Format" picker (M3b's vrFormatPicker — name kept generic since it covers Type+Degree+Stereo, not just stereo) |
| Control Panel | In-VR playback panel (`vrControls` entity) |
| Advanced Setting | TBD — IPD slider lives here in SKYBOX-followup if we add one |
| Auto | "Auto (re-detect)" button — same name |
| Reset | "Auto" button serves the same role in stash-vr (reverts to detected) — SKYBOX has both Reset (revert overrides) and Auto (rerun detection); stash-vr collapses these because the distinction is invisible to the user |
| Cinema | "Cinema" Degree option — same name |
| VR 180° / VR 360° | "180°" / "360°" Degree options — slightly shortened |
| Normal / FishEye | Same in stash-vr |
| YouTube | NOT exposed in stash-vr (CUBEMAP/EAC unsupported, deferred) |
| 2D Single / 3D SBS / 3D TB | "2D" / "SBS" / "TB" Stereo options — abbreviated |
| Fisheye 200° / MKX200 | "200°" Degree under FishEye Type — internal mapping to `VR_MKX200` tag |
| FFR | Not used (stash-vr has no foveated-rendering hook today) |
| Monoscopic | Mono/no-stereo in stash-vr's Stereo row (the "2D" button when geometry is VR) |

## 6. Sources

- User-supplied screenshots (this conversation, 2026-05-08): SKYBOX support page "During Video Playback" controller table, format menu floating panel.
- [Forum 1938 — Filename Rule for Fisheye 190](https://forum.skybox.xyz/d/1938-filename-rule-for-fisheye-190) — SKYBOX dev confirmation of FISHEYE190 keywords (verbatim extracted from page Flarum-JSON).
- [Forum 157 — Filename Rules for VR Format](https://forum.skybox.xyz/d/157-filename-rules-for-vr-format) — canonical Andy 2018 ruleset and follow-up dev replies (verbatim extracted from page Flarum-JSON).
- SKYBOX SPA main JS bundle (`https://skybox.xyz/js/app.a8a547b4.js`, hash valid as of 2026-05-08) — UI string fragments for control-panel layout and feature names.
- [vrpupu.io 2026-01 SKYBOX VR Player Ultimate Guide](https://vrpupu.io/2026/01/skybox-vr-player-guide/) — third-party guide citing aspect-ratio fallback heuristic and detection priorities.

The `https://skybox.xyz/support/...` SPA pages (Watch-Videos, Oculus-Touch-Controller-buttons, Stereo-Mode, etc.) could not be extracted directly — their content loads client-side via JS fetches that `WebFetch`/`curl`/Wayback don't see. If a future milestone needs richer details (e.g. the full Advanced Settings list), the path is: render the page in a real browser (Quest's Meta Browser counts) and screenshot, OR find the underlying JSON/REST endpoint the SPA fetches from.

## 7. How to use this doc

- **Before designing any new in-VR control surface** for stash-vr, check this doc for SKYBOX's name, layout, and behavior. Match unless there's a specific reason not to (e.g. stash-vr's flat-preset-grid was rejected in M3b's brainstorm in favor of SKYBOX's three-row layout).
- **When the user says "match SKYBOX"**, the linked memory `feedback_skybox_heresphere_parity.md` already says: don't drag the user through implementation tables in chat; just match the reference. Use this doc as the implementer's reference instead.
- **When updating this doc**: cite sources inline. Mark inferences as `_(inferred)_`. Don't speculate beyond what the cited source says.
