# DeoVR `/deovr` JSON shape — research notes (2026-05-08)

Research only. No code changes. All raw responses saved alongside this file.

## Per-source results

### 1. `https://www.sexlikereal.com/deovr` (anonymous GET, default UA)
- HTTP: 302 to `/all-age` (set-cookie `s_did=...`), then 200 from `/all-age`.
- `Content-Type: text/html` — **not JSON**. 184,632 bytes of HTML, the standard SLR landing page.
- Same result with `User-Agent: Mozilla/5.0 (DeoVR)` and `Accept: application/json` (`slr-deovr-jsonaccept-*`). SLR does not serve a JSON document at this path for anonymous clients.
- Saved: `slr-deovr-response.html`, `slr-deovr-headers.txt`, `slr-deovr-jsonaccept-response.raw`, `slr-deovr-jsonaccept-headers.txt`.
- Grep for `deovr|/api/|application/json|videoData` inside the HTML: zero meaningful matches (only an unrelated `/vr-porn-...` blog link). The HTML contains no embedded `/deovr` JSON, no JSON endpoint hint, no `videoData` reference. SLR does not expose a `/deovr` JSON contract publicly.

### 2. `https://deovr.com/deovr` (anonymous GET)
- HTTP 200, `Content-Type: application/json`, **10,851,368 bytes (~10.4 MB)**, no redirect.
- Saved: `deovr-com-deovr-response.json`, `deovr-com-deovr-headers.txt`.
- Top-level shape: `{ "scenes": [ ... ] }` — only `scenes`. No `authorized`, no pagination cursor, no metadata. 
- `scenes` is an array of length **1**. The single section is `{ "name": "Library", "list": [...] }` with **41,957 items**.
- All 41,957 items have **the exact same 4-key shape** (verified by enumerating every key signature):
  ```json
  { "title": "...", "videoLength": 1252, "video_url": "https://deovr.com/deovr/video/id/118489", "thumbnailUrl": "https://s3.deovr.com/images/.../...-cover-desktop.jpg" }
  ```
  No `id`, no `encodings`, no `screenType`, no `stereoMode`, no `is3d`, no `description`, no `date`, no `actors`/`paysite`/`tags`/`isFavorite` at the index level. The full per-video JSON lives behind `video_url` (e.g. `https://deovr.com/deovr/video/id/118489`), which DeoVR fetches lazily when a tile is selected.
- Sections homogeneous (only one). No nested `sections`, no `subscenes`, no `more`/`next`/`offset`/`page`/`url` field on the section.

### 3. `https://www.sexlikereal.com/deovr/` (trailing slash)
- HTTP 301 -> `http://www.sexlikereal.com/deovr` (note: drops https), then identical to #1 (HTML, 200).

### 4. DeoVR JSON Schema repo `hzrd149/deovr-json-schema`
- Saved: `deovr-json-schema-readme.md`, `deovr-json-schema-index.ts`, `deovr-json-schema-package.json`.
- `index.ts` is the entire schema (TypeScript). Relevant types verbatim:
  - `Scene = { name: string; list: (FullVideo | Picture | VideoLink)[] }`
  - `MultiVideoJson = AuthMetadata & { scenes?: Scene[] }`
  - `VideoLink = { title; thumbnailUrl; videoLength?; video_url }` (the index-level shape — what deovr.com emits)
  - `FullVideo` (per-video drill target) adds `encodings`, `description`, `videoThumbnail`, `videoPreview`, `timeStamps`, `skipIntro`, `fps`, `date`, plus screen metadata.
  - `AuthMetadata.authorized: -1 | 0 | 1`.
- **Hierarchy / pagination / drill-down support in the schema:** none. `Scene` is a single flat array of items. There is no `sections[].url`, no `cursor`, no `next`, no `children`, no `more`. The only documented "drill-down" mechanism is `VideoLink.video_url`: a per-item URL the player fetches when the tile is opened — that fetch returns a `FullVideo` JSON. The schema explicitly does not model section-level navigation.

### 5. Official DeoVR doc `https://deovr.com/app/doc`
- Confirms the same shape as the schema. Documents `path` (string image/stream URL alternative to `encodings`), `passthrough_settings`, `encodings_spatial`, the resolution ladder (1080–3840), `corrections`, `timeStamps`, `skipIntro`, `authorized` ("0"/"1"/"-1"), and the file-naming convention (`_180`, `_SBS`, `_TB`, `_FB360`, etc.).
- Selection-Scene format documented as exactly `{"scenes":[{"name":"...","list":[...]}], "authorized":"0"}`. No browse / search / paginate / nested-section primitives are documented.
- Important quote: "Browser access to domain (`www.yoursite.com`) requests `http://www.yoursite.com/deovr`. Specific URLs (`www.yoursite.com/video/test`) request unchanged endpoint."

### 6. Forum / third-party servers (saved as `deovr-forum-*.json`)
- **Thread 7896 ("Create website compliant with DeoVR"), post #2 (verbatim, key passage):** "Afaik DeoVR can operate in four modes, **SLR, DeoVR, Json and JillVR**. All but Json modes are determined by the URL and are **hardcoded**. Json mode is used when there is a hostname/deovr endpoint... You can on a plain browser just have links to videos and those open in the player... **Json mode is clearly not a first class citizen.**"
- **Thread 2659 ("Quest 3 DeoVR app never makes call to /deovr json file"):** the OP confirms recent DeoVR Quest builds do not invoke `/deovr` automatically anymore; the app's own docs label JSON files "legacy". The DeoVR mod's only reply: "post the URLs you're trying to open" — no fix in the public thread.
- **Thread 853 ("Problems with DeoVR JSON interface, documentation"):** confirms thumbnail in different directory breaks selection display, and asks what the difference is between `videoThumbnail`, `videoPreview`, `thumbnailUrl` (DeoVR mod replies "we'll re-visit docs", no spec answers given). Confirms that subdirectory-hosted `/deovr` is unreliable.
- **Thread 1362 ("New Customized Stash Server v0.20.2 for DeoVR"):** the maintainer (philpw99) abandoned the JSON path and instead patched Stash to expose an **"Open in External Player"** button that emits a `deovr://` deeplink to the direct stream URL. He explicitly states JSON-mode glitches kept reappearing with each DeoVR update. Repo: `philpw99/stash4deovr`.
- **`Tweeticoats/stash-deovr` plugin source** (`stash-deovr-plugin.py` lines 195-313, saved):
  - Only emits scenes that have the `export_deovr` tag (a curated whitelist).
  - Builds **exactly three fixed sections** plus optional pinned sections: `[{name:"Recent",list:recent},{name:"VR",list:vr},{name:"2D",list:flat}]`, then appends one section per name in user-configured `pinned_studio` and one per name in `pinned_performers`. There is no per-tag or per-studio explosion.
  - Per-item index entries use the same 4-key shape as deovr.com (`title`, `videoLength`, `thumbnailUrl`, `video_url`), with `video_url` pointing at a per-scene `/custom/deovr/<id>.json`.
  - Per-scene file is the full DeoVR video document (`encodings`, `screenType`, `stereoMode`, `is3d`, `description`, `actors`, `paysite`, `isFavorite`, `isScripted`, `isWatchlist`, `fullVideoReady`, `fullAccess`).

## Synthesis

**Does the DeoVR JSON spec support hierarchy / drill-down / pagination?**

No. The spec (schema + official doc + 10MB live deovr.com payload) defines exactly one level of grouping (`scenes[].name + .list`) and one drill-down primitive: per-item `video_url`. There is no documented field for section-level URLs, cursors, "load more", nested sub-sections, or per-section pagination. The schema's only optional richness is at the per-video level (the `FullVideo` fields fetched after click).

**What pattern does SLR use?**

SLR does **not** serve a JSON `/deovr` document at all. `https://www.sexlikereal.com/deovr` returns the regular SLR website HTML (after a 302 to `/all-age`). DeoVR has a hardcoded **"SLR mode"** (forum 7896 post #2: "All but Json modes are determined by the URL and are hardcoded"). When the headset visits `sexlikereal.com`, DeoVR renders SLR's normal web UI inside its webview and uses SLR-specific deeplinks for playback. Their thousands-of-scenes navigation is the SLR website's own React/HTML UI — not a JSON shape stash-vr can mimic.

**What does deovr.com itself do at scale?** One section (`Library`) with the entire catalog (41,957 items) crammed into a single flat `list`. They literally do not split by tag/performer/studio at all in the JSON; they ship a 10MB blob and rely on the headset's local search/scroll. Per-section sprawl is avoided by having only one section.

**Practical recommendation for stash-vr `/deovr`:**

1. Stop emitting one section per performer/tag with auto-sections. The DeoVR JSON contract has no concept that scales beyond "a small number of named buckets". 400+ sections is a stash-vr-side decision, not a DeoVR-supported pattern.
2. Match the proven shapes:
   - **deovr.com pattern** — one section called `"Library"` (or similar) with every scene flat. Simplest, demonstrably working at 40k+ scenes.
   - **stash-deovr plugin pattern** — a tiny fixed set of curated sections: `Recent`, `VR`, `2D`, plus optional user-pinned studios/performers from `config.json`. This is the closest existing precedent and aligns with stash-vr's existing user-config concept.
3. Per-item index entries should stay at the **4-key minimum** (`title`, `videoLength`, `thumbnailUrl`, `video_url`) — that's exactly what deovr.com emits and what the schema specifies as `VideoLink`. Anything richer should live behind the per-scene `video_url` document, not at the index.
4. Treat JSON mode as best-effort. Multiple forum threads (2659, 7896, 853, 1362) confirm DeoVR's JSON path is "legacy / not first class", with breaking changes per release. Do not over-invest in JSON-side hierarchy that the spec does not actually support.
5. If users want richer browse-by-tag in DeoVR, the only path that scales is "host a real HTML UI and let DeoVR webview render it" — same approach SLR uses. Out of scope for /deovr JSON.

## Files in this directory

- `deovr-com-deovr-response.json` — full 10.4 MB live deovr.com payload.
- `deovr-com-deovr-headers.txt` — response headers.
- `slr-deovr-response.html` / `slr-deovr-headers.txt` — SLR HTML page.
- `slr-deovr-jsonaccept-response.raw` / `-headers.txt` — same SLR endpoint with DeoVR UA + JSON Accept (also HTML).
- `deovr-json-schema-index.ts` — full TypeScript schema.
- `deovr-json-schema-readme.md`, `deovr-json-schema-package.json` — schema repo metadata.
- `stash-deovr-plugin.py` — Tweeticoats stash plugin source (the existing third-party reference).
- `stash-deovr-README.md`, `stash-deovr-tree.json` — plugin README + repo file list.
- `deovr-forum-{1362,853,7896,2521,2659}.json` — Flarum API responses for relevant forum threads.
- `deovr-forum-2659.html`, `deovr-forum-2659-post6216.json` — supporting captures.
- `notes.md` — this file.
