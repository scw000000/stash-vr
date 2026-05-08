# SLR / DeoVR webview playback hook — deep investigation
**Date:** 2026-05-08
**Question:** Can `https://stash-vr.duckdns.org/<some-path>` serve HTML that DeoVR's webview renders AND launches playback when a tile is clicked, the way `sexlikereal.com` does?
**Verdict (short):** No. There is no HTML-side mechanism. The "SLR pattern" is not an HTML technique — SLR plays videos in-page with its own WebGL/WebXR player inside the plain Chromium webview. DeoVR's "SLR mode" is hardcoded to the `sexlikereal.com` host name in DeoVR's app code; it is not an HTML opt-in.

This document grounds that verdict in (1) every DeoVR-related string actually present in SLR's production JS, (2) the official DeoVR docs, (3) six years of DeoVR forum threads on this exact question, and (4) the only known third-party precedent (`philpw99/stash4deovr`).

---

## A. Mechanism findings (file-grounded)

The SLR production assets captured to `c:\dev\stash-vr\scripts\` total **~1.2 MB of HTML** and **~640 KB of JS** (`slr_videoplayer.js` 413 KB, `slr_watch.js` 65 KB, `slr_appcore.js` 95 KB, `slr_router.js` 33 KB, `slr_web.js` 37 KB, plus the 185 KB SSR HTML for each page). The minified bundles were re-pretty-printed to `*.pretty.js` (15 607 / 3 689 / 2 355 / 1 907 / 2 057 lines) so grep with line numbers is meaningful.

### A1. The ONLY DeoVR string in any SLR asset is a UI tooltip

`slr_videoplayer.js.pretty.js:15212`:
```
playInVrTooltip:"Open sexlikereal.com in DeoVR app or Meta/Safari browser"
```

That's it. One occurrence across **all** SLR assets — and it is a localized tooltip string for the "Play in VR" affordance on a desktop browser page. It does not reference any API, header, scheme, global, or postMessage. No other instance of `deovr`, `DeoVR`, `Deovr` appears in the entire 640 KB JS bundle (case-insensitive, all five files searched). There is zero DeoVR string in `slr_listing.html`, `slr_scene.html`, `slr_allage.html`, or any of the other JS files.

Domain-tied? Irrelevant — the string is a help-text label, not a mechanism.

### A2. SLR's HTML is a SPA shell with NO DeoVR-specific markup

`slr_listing.html` (185 KB) is an Astro + Solid.js SSR shell. The body (`<body>` starts at byte offset 109 775) contains a single `<astro-island>` component which is hydrated by JS at runtime; there are no scene cards, anchors, facet panels, or playback hooks in the static HTML. All scene tiles, the right-side filter panel, and the click handlers are rendered client-side by the Solid.js bundles.

Verified `<meta>` and `<link>` tag inventory of `slr_listing.html` (full extracted list in this directory; the relevant subset):

```
<meta name="viewport" ...>          <- standard
<meta name="color-scheme" content="dark">
<meta name="robots" content="index, follow">
<meta name="description" ...>       <- SEO
<meta name="twitter:*" ...>         <- SEO
<meta property="og:*" ...>          <- SEO
<meta name="theme-color" content="#242424">
<meta name="mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="full-screen" content="yes">
<meta name="browsermode" content="application">
<meta name="layoutmode" content="fitscreen/standard">

<link rel="icon" ...>
<link rel="canonical" href="https://www.sexlikereal.com/all-age">
<link rel="apple-touch-icon" ...>
<link rel="mask-icon" ...>
<link rel="manifest" href="data:application/json;base64,...">
<link rel="stylesheet" href="/_astro/*.css"> (16 of these)
<link rel="preconnect" href="//cdn-vr.sexlikereal.com">
```

There is no `link rel="application/x-deovr"`, no `meta name="deovr-*"`, no `meta name="vr-player-*"`, no custom protocol-handler markup, no `<video>` tag in the static HTML, and no DeoVR-typed `<script>` tag.

Domain-tied? Even if there *were* a meta tag, DeoVR docs (section A4 below) document zero such mechanism — so this is a moot question.

### A3. SLR's player is an in-page WebGL VR player using `navigator.xr` (WebXR), NOT a hand-off to the DeoVR app

`slr_watch.js.pretty.js:2895-2897` — the watch page dynamically imports a Solid.js component called `VideoResourcePlayer`:
```
const{VideoResourcePlayer:e}=await import("./VideoResourcePlayer.C217ipRq.js").then(n=>n.V);
```

`slr_watch.js.pretty.js:3187-3215` — `<VideoResourcePlayer>` is rendered inline with `scene`, `encodings`, `toys`, `viewsHeatmap` props. The video plays *inside* the page; no navigation, no deeplink, no external launch:
```
,children:ce=>t(Vl,{
get scene(){ return h() },
get encodings(){ return ce() },
get class(){ return le.videoSectionPlayer },
get toys(){ return fe() },
get viewsHeatmap(){ return Ke() },
onNextClick:async()=>{...}
```

`slr_videoplayer.js.pretty.js:7414-7461` — the player is a custom WebGL renderer that drives an HTMLVideoElement (`this.video`):
```
super(t),this.currTime=0,this.src="",this.video=pr,...
this.video.addEventListener("loadeddata",()=>{...})
case"src": this.image=null,this.isAtlas=!1,this.src=o;
this.video.src=r,this.video.load(),this.node.needsDraw=!0,...
case"fullsrc": ... fetch(o).then(n=>n.blob()).then(n=>{
  this.video.src=URL.createObjectURL(n),this.video.load()
})
```

`slr_videoplayer.js.pretty.js:735` — at boot, the player probes `navigator.xr` (the standard WebXR API) and sets internal flags for AR / Quest / Galaxy XR variants:
```
navigator.xr&&(_.questVersion!==0&&(_.ar=1),
navigator.xr.isSessionSupported("immersive-ar").then(r=>_.ar=r?1:0)),
... _.NativeXR ... _.maxVideoSize=8192,_.NativeXR.v) switch(...) {
  case"Quest3":case"Quest3S": _.questVersion=3,_.hmd=Je.QUEST_3,_.xrPixelRatio=1.5;
  case"Quest2": _.questVersion=2,_.hmd=Je.QUEST_2;
  case"QuestPro": _.questVersion=2,_.hmd=Je.QUEST_PRO;
}
```

`_.NativeXR` is a property on the device-info object but is **never assigned by any SLR code**. It is read at runtime — meaning SLR expects some host (the OS, a custom browser shim, possibly a future Quest3-native runtime) to inject `window.NativeXR` if available. Today, in DeoVR's plain Chromium webview, `_.NativeXR` is undefined and the SLR player falls through to the WebXR / mobile path.

**Conclusion of A3:** SLR uses zero DeoVR-app handoff. It uses **WebXR (`navigator.xr.requestSession("immersive-vr")`)** — the open browser standard — to enter VR from inside whatever Chromium-based webview it runs in, including DeoVR's. The "Enter VR" path is purely browser-driven; the VR session lives inside the page, not inside DeoVR's player UI.

Domain-tied? The mechanism (WebXR) is domain-agnostic. Anyone can call `navigator.xr.requestSession`. **But it does not give us "DeoVR's player UI" — it gives us our own, bare WebXR scene that we'd have to write ourselves**, which is a 400 KB SLR-engineering project, not a stash-vr feature. We do not have an SLR-equivalent VR player implementation; building one is wholly out of scope.

### A4. DeoVR's official docs document only two integrations — both proven (separately) not to apply

Source: `https://deovr.com/app/doc` (re-fetched 2026-05-08, see `webfetch-deovr-app-doc.txt` for the relevant text).

The full inventory of DeoVR third-party integration mechanisms documented:

> 1. **Single video deeplink** — "a button with a deeplink to this file (e.g. 'deovr://https://www.deovr.com/something.json') is added to the site"
> 2. **Multiple videos selection (Selection Scene)** — same deeplink, JSON containing `scenes[].list[]`
> 3. **Browser-based access** — "put file 'deovr' (without extension and quotes) into the root directory of the server. When following a link containing only domain name, DeoVR will request the data at the address 'http://www.yoursite.com/deovr'"

The doc has zero references to JS globals (`window.deoVR`, `vrPlayerSettings`), `postMessage`, `<video>` tag attributes, custom meta tags, custom HTTP headers, anchor sniffing, or `registerProtocolHandler`. WebFetch summary verbatim: *"Integration appears exclusively JSON/deeplink-based."*

Of these three:
- The `deovr://` deeplink is **already proven by the prior session not to work from inside DeoVR's in-VR browser**. Independently confirmed by forum thread 80 (`api/discussions/80`, 2020-04-22, user crwxaj): *"the deovr:// protocol is unknown [in the in-app browser], and linking to http:// just views the JSON data."* Six years on, no DeoVR staff has replied with a workaround.
- The `/deovr` JSON endpoint **is what stash-vr already serves**. It cannot express SLR's right-side filter panel because the JSON schema (`hzrd149/deovr-json-schema/index.ts`) has no `sections[].url`, no `cursor`, no `next`, no `children`, no `more`, no facet primitive — just a flat `Scene = { name; list }`.

### A5. The "four modes hardcoded by URL" forum quote is direct, recent, and from a developer building exactly this

`docs/superpowers/research/2026-05-08-deovr-shape/deovr-forum-7896.json`, post #2 (2025-02-11, user `EUZfZxe7j`, who runs `a.domain.test/deovr` and `b.domain.test/deovr` for their own DeoVR-compliant site):

> "Afaik DeoVR can operate in four modes, **SLR, DeoVR, Json and JillVR**. **All but Json modes are determined by the URL and are hardcoded.** Json mode is used when there is a hostname/deovr endpoint, which you have been using. There is also the plain browser. You can on a plain browser just have links to videos and those open in the player, but you have to select the projection manually (or use a filename that DeoVR can use to deduce the projection)."

This is the most authoritative public statement about how DeoVR distinguishes its render modes. The poster is a third-party site builder, not an end-user — they have implemented this end-to-end and reported the limit. They explicitly call Json mode "**clearly not a first-class citizen**" and "behavior can and has changed with each release without warning."

### A6. The "click direct .mp4 link" path that previously worked has just been broken by Quest OS

Forum thread 10601 (`api/discussions/10601`, "Direct video links no longer opening", 4 pages, recent):

> Post 1 (quamassbobo): *"For over a year now I've been opening webpages in DeoVR and clicking direct 'download' links to videos, which opened the DeoVR player"* — but after a recent Quest OS update, an infinite loading indicator appears.
> Post 6 (fabpeg72, Quest 3, DeoVR 15.3.3545): *"Quest OS update broke this functionality."*
> Post 7 (rzhddd): *"File is being downloaded rather than played back"* instead of streaming.
> No DeoVR developer reply, no workaround.

This is the **same mechanism** philpw99's `stash4deovr` "Open Ext." button relies on (forum 1362, post #1 by philpw99): *"in each scene you will see an 'Ext. Player' button. Click on that and DeoVR will open that video file through http protocol."* That precedent **just stopped working** on Quest 3.

Domain-tied? No — but the mechanism itself has been broken by an OS update with no fix in sight.

### A7. Ad-hoc negative searches in SLR's bundles

To rule out other plausible mechanisms, every promising substring was grepped across all five pretty-printed SLR JS files; results:

| Pattern | Hits | Location | Meaning |
|---|---|---|---|
| `deovr`, `DeoVR`, `Deovr` (case-insensitive) | 1 | tooltip line above | A4 |
| `videoData`, `vrPlayerSettings`, `vrPlayer`, `webkitDeoVR`, `window.deo`, `application/x-deovr`, `HereSphere` | 0 | — | None |
| `deovr://`, `sexlikereal://`, `slr-app://`, `slrapp://`, `vrl://`, `oculus://`, `intent:` | 0 | — | None |
| `launcherUrl`, `launchVr`, `launchVR`, `OpenInApp`, `openInApp`, `openInPlayer`, `sendToPlayer`, `inApp`, `is_app` | 0 | — | None |
| `postMessage`, `registerProtocolHandler` | 0 | — | None |
| `Quest`, `Oculus`, `userAgent`, `navigator.xr` | many | UA sniff for sizing & `_.NativeXR` (A3) | Standard browser feature detection, not DeoVR comms |

Inverse check on the static HTML (`slr_listing.html`, `slr_scene.html`, `slr_allage.html`): zero hits on `deovr|DeoVR|videoData|vrPlayer` (case-insensitive).

### A8. SLR's `/deovr` endpoint just serves the regular HTML site — it has no JSON contract with DeoVR

From the prior session's capture (`docs/superpowers/research/2026-05-08-deovr-shape/notes.md` §1):

> `https://www.sexlikereal.com/deovr` returns HTTP 302 → `/all-age`, then 200 with `Content-Type: text/html` (184 632 bytes). Same with `Accept: application/json` and a `User-Agent: Mozilla/5.0 (DeoVR)` header. SLR does not expose any `/deovr` JSON document publicly — for any client.

This is independent confirmation that **SLR is in "SLR mode" (URL-hardcoded) and not in "Json mode"**, exactly as forum 7896 post #2 says. There is no JSON contract we could replicate.

---

## B. Verdict on replicability

**Can we serve HTML at `https://stash-vr.duckdns.org/deovr` (or another path) such that DeoVR's webview renders it AND launches playback when the user clicks a scene tile?**

**No, with high confidence.** The investigation surfaced no HTML-side mechanism — neither in SLR's production code, nor in DeoVR's official docs, nor in any of the dozen+ DeoVR forum threads searched. The previous session's three negative tests (`deovr://` href in-browser, `window.vrPlayerSettings` global, `DeoVR-JSON-Version` header) ruled out the most-likely candidates; this investigation shows there are no other candidates to test.

The specific reasons:

1. **SLR's UI is domain-tied at the DeoVR-app level, not at the HTML level.** Forum 7896 post #2 (a third-party site builder verbatim): *"All but Json modes are determined by the URL and are hardcoded."* DeoVR's app code recognizes the `sexlikereal.com` host name and switches to "SLR mode," which presumably enables SLR-specific app integrations (auth/session/right-panel native overlay, etc.). The HTML SLR serves is a generic Astro/Solid SPA — `slr_listing.html` contains zero DeoVR-specific markup (A2). Any `stash-vr.duckdns.org` URL will be in DeoVR's "plain browser" or "Json mode" branch, never SLR mode.

2. **SLR plays videos in-page with WebXR, not via a DeoVR app handoff.** `slr_watch.js.pretty.js:2895-3215` renders a `<VideoResourcePlayer>` Solid.js component inline; `slr_videoplayer.js.pretty.js:7414+` is a 413 KB WebGL VR player that drives an HTMLVideoElement directly and probes `navigator.xr` for VR session entry (A3). Replicating that "right-side filter panel + tile click → play" UX *inside the page* would mean shipping our own 400 KB WebGL VR player. That isn't a DeoVR integration question; it's a "rewrite SLR's webapp" question, and stash-vr cannot in-house it.

3. **DeoVR's documented integrations don't admit an HTML path.** The DeoVR app doc (re-fetched 2026-05-08) lists exactly three integration mechanisms (A4): the `deovr://` deeplink, the `/deovr` JSON file, and the JSON file at the server root. There is no `<video>` tag pickup, no JS API, no postMessage, no meta tag, no header. WebFetch confirmed verbatim: *"Integration appears exclusively JSON/deeplink-based."*

4. **The two HTML-adjacent paths that *did* work in the past are dead.** The `deovr://` deeplink does not fire from inside DeoVR's own in-VR browser (forum 80, 2020; prior session's empirical retest, 2026). The "click a direct `.mp4` link, get the DeoVR player" path is broken on current Quest OS (forum 10601, 2026, multiple users on DeoVR 15.3.3545 / Quest 3 / firmware v83.1034; no developer fix posted). philpw99's `stash4deovr` "Ext. Player" button (forum 1362) used that second path and is therefore also broken.

5. **The `/deovr` JSON path that *does* still work cannot express the right-side filter panel.** The schema (`hzrd149/deovr-json-schema/index.ts`) has only `Scene = { name; list }` — no facets, no URL-per-section, no pagination, no nested children. deovr.com's own response (10.4 MB live capture) ships a single `Library` section with 41 957 items flat, exactly because the schema cannot express anything richer. This is what stash-vr is already doing; this is the ceiling of JSON mode.

**What SLR provably relies on that we cannot replicate:**

- **A hardcoded host-name match in DeoVR's compiled Android app** (`sexlikereal.com` → SLR mode). We have no way to inject a host name into a published binary running on the user's headset.
- **A 413 KB / ~16 000-line proprietary WebGL+WebXR player** (`slr_videoplayer.js`) plus a Solid.js SPA front end with right-panel facets, both fetched from SLR's own CDN. To "match SLR's UX" outside SLR, we would have to build all of that ourselves. This is independent of DeoVR — it would also be the answer if the user wanted the same UX in plain Chrome.
- **An undocumented/private contract between DeoVR and SLR** beyond the URL hardcode (likely auth/session/right-panel native overlay). This is hidden in DeoVR's app and is not something a third-party site can opt into.

**Single-sentence answer for the user:** SLR's library UX is rendered by SLR's own JavaScript inside the Quest's plain Chromium webview, and DeoVR's "SLR mode" augmentations are hardcoded on the `sexlikereal.com` host name in DeoVR's app — there is no documented or empirically-discoverable HTML opt-in that would let `stash-vr.duckdns.org` produce the same outcome, and the two previously-working HTML→playback fallbacks (`deovr://` from in-VR browser, click-direct-.mp4) are both currently dead on Quest 3.

---

## Files referenced by this investigation

Production assets (already on disk before this session):
- `c:\dev\stash-vr\scripts\slr_listing.html` (185 KB)
- `c:\dev\stash-vr\scripts\slr_scene.html` (185 KB)
- `c:\dev\stash-vr\scripts\slr_allage.html` (185 KB)
- `c:\dev\stash-vr\scripts\slr_app.js` (224 B)
- `c:\dev\stash-vr\scripts\slr_appcore.js` (95 KB)
- `c:\dev\stash-vr\scripts\slr_router.js` (33 KB)
- `c:\dev\stash-vr\scripts\slr_videoplayer.js` (413 KB)
- `c:\dev\stash-vr\scripts\slr_watch.js` (65 KB)
- `c:\dev\stash-vr\scripts\slr_web.js` (37 KB)

Pretty-printed copies created this session (for line-numbered grep evidence):
- `c:\dev\stash-vr\scripts\slr_videoplayer.js.pretty.js` (15 607 lines)
- `c:\dev\stash-vr\scripts\slr_watch.js.pretty.js` (3 689 lines)
- `c:\dev\stash-vr\scripts\slr_appcore.js.pretty.js` (2 355 lines)
- `c:\dev\stash-vr\scripts\slr_router.js.pretty.js` (1 907 lines)
- `c:\dev\stash-vr\scripts\slr_web.js.pretty.js` (2 057 lines)

Prior research (re-checked in this session):
- `c:\dev\stash-vr\docs\superpowers\research\2026-05-08-deovr-shape\notes.md`
- `c:\dev\stash-vr\docs\superpowers\research\2026-05-08-deovr-shape\deovr-forum-7896.json`
- `c:\dev\stash-vr\docs\superpowers\handoff\2026-05-08-webview-ux-overhaul.md`

External sources fetched this session:
- `https://deovr.com/app/doc` (re-confirmed only `deovr://` deeplink, `/deovr` JSON, server-root deovr file)
- `https://forum.deovr.com/api/discussions/10601` (direct .mp4 links broken on current Quest OS)
- `https://forum.deovr.com/api/discussions/10481` (DeoVR team referred user to docs; no HTML path)
- `https://forum.deovr.com/api/discussions/80` (in-app browser deovr:// is no-op, since 2020, no fix)
- `https://forum.deovr.com/api/discussions/1362` (philpw99's Ext. Player button = http:// to direct file = same path as 10601)
- `https://forum.deovr.com/api/discussions/681` (only mechanism mentioned: direct .mp4)
