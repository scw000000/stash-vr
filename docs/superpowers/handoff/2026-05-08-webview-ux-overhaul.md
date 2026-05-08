# Handoff: stash-vr DeoVR webview UX overhaul

**Date:** 2026-05-08
**Status:** Stuck. Multiple paths attempted. Two specific UX issues remain.
**User platform:** Meta Quest 3 (Horizon OS, stock). Stash + stash-vr on Windows 11. LAN-only network.

---

## TL;DR for the next agent

The previous session did a lot of exploratory work on three separate
problems (broken `/browse` thumbnails on the headset, broken DeoVR
playback, broken `/browse` Play button). The first two are fixed and
committed. The third is fundamentally **not fixable** the way it was
designed (DeoVR's in-VR browser cannot launch DeoVR's player; SLR /
deovr.com don't try to either).

The user has **pivoted to webview-mode-only** (DeoVR's library
renderer consuming `/deovr` JSON) as the play surface. That works
mechanically — playback fires, thumbnails appear (because Caddy gives
them publicly-trusted HTTPS now). But two *cosmetic but blocking* UX
problems remain in webview, both surfaced after enabling
`AUTO_SECTIONS_PERFORMERS`/`AUTO_SECTIONS_TAGS`:

1. **Tofu glyphs** — performer/tag names with non-Latin characters
   (CJK in particular) render as `□□□□` boxes in DeoVR's library
   renderer. The renderer evidently lacks fonts with CJK coverage.
2. **400+ sections in one packed row** — with auto-sections enabled
   the user has hundreds of per-performer sections. DeoVR's library
   renderer apparently lays them out in a way that's unusable at that
   scale ("a long string like performer=333 performer=334
   performer=335... like hundreds of them").

The user's instruction for the next session: **if we stick with
webview, the UI must match SLR's pattern.** SLR has thousands of
scenes and works fine in DeoVR webview, so they have solved this
problem. We need to copy their structure.

---

## What is committed (work to preserve)

Five commits added on top of `master`, in order:

```
65de10c browse: route thumbnails via /cover proxy and fix Play URL
ae5c6f2 deovr,heresphere: route thumbnails via /cover proxy and present DeoVR direct stream first
11fa6b4 server: optional built-in HTTPS with auto self-signed cert and /ca.crt download
d8035a4 library: batch-resolve auto-section performer/tag names
c56c7ba scripts: add Windows build helper
```

What each does:

| Commit | What it fixes |
|---|---|
| `65de10c` | `/browse` thumbnails in headset's in-VR browser (was failing on cross-port HTTP fetch from Stash:9999) — now thumbnails go through stash-vr's `/cover/{id}` proxy, same-origin. Also fixed `/browse` Play link URL: was `/deovr/videoData/{id}` (404), now `/deovr/{id}`. |
| `ae5c6f2` | DeoVR/HereSphere thumbnails routed through `/cover/{id}` proxy (same reason). DeoVR `encodings[]` order swapped: direct first, transcoding second. Stash's transcoder hangs on large 8K MP4s; direct-first lets DeoVR play immediately via byte-range. |
| `11fa6b4` | Optional `--HTTPS_LISTEN_ADDRESS` flag spawns an HTTPS listener with auto-generated self-signed cert (PEM at `%APPDATA%/stash-vr/cert.pem`). `/ca.crt` route serves the cert for download. UI banner on `/` auto-detects trust status and offers download. **Default empty (off)** because Quest 3 won't accept user CAs anyway — feature is dormant; only useful on platforms that allow CA install. |
| `d8035a4` | Auto-section names: batch-resolve performer/tag names via `FindPerformersByIDs`/`FindTagsByIDs` instead of falling back to `"Performer 333"` placeholders. Materializer threads resolved names from `getSectionsByFilters`. |
| `c56c7ba` | `scripts/build-windows.bat` — convenience build helper that mirrors Dockerfile flags. |

**The ROOT cause of the recurring "broken thumbnails" is the same in
all the commits above**: media URLs originating from Stash (port 9999)
fail to load from inside DeoVR / Quest browsers when they're a
different origin than the page or use long apikey query strings. The
universal fix is `/cover/{id}` — a stash-vr-side proxy that serves
images same-origin with no apikey leak.

---

## Working state of the user's environment

- **stash-vr.exe** at `C:\Users\scw00\Downloads\stash-vr.exe`, launch flags include both
  `--STASH_GRAPHQL_URL=http://192.168.1.183:9999/graphql` (the LAN IP version is what wins)
  and `--LISTEN_ADDRESS=:9666`. Build helper at `scripts/build-windows.bat`.
- **Stash** runs on the same Windows host at `192.168.1.183:9999`.
- **Caddy** (custom build with `dns.providers.duckdns` plugin) reverse-proxies
  `https://stash-vr.duckdns.org/...` → `http://localhost:9666/...`. Cert auto-renews via
  Let's Encrypt DNS-01 challenge using a DuckDNS token. Caddyfile uses
  `propagation_timeout -1` to skip the DNS-propagation pre-check (the user's network blocks
  outbound DNS to non-default servers).
- **DuckDNS** subdomain `stash-vr.duckdns.org` points at LAN IP `192.168.1.183`.
- **Router** (Google Fiber Network Box) does DNS-rebinding-protection at the resolver layer
  with no UI to disable. Workaround: client devices set custom DNS directly to bypass
  the router's DNS proxy.
  - PC: Manual IPv4=8.8.8.8/1.1.1.1 + IPv6=2001:4860:4860::8888 set on the active adapter.
  - Quest 3: Wi-Fi → Manual IP (keep its existing IP/gateway/prefix) + DNS=8.8.8.8/1.1.1.1.
- The user's **DeoVR library URL** is now `https://stash-vr.duckdns.org/deovr` (HTTPS).
  Webview-mode playback works end-to-end through this URL.

---

## The two open issues

### 1. Tofu glyphs on non-Latin performer/tag names

**Symptom:** in DeoVR webview, performer/tag section names containing CJK
characters render as boxes / "tofu" — `□`. Latin names render fine.

**Cause:** DeoVR's library renderer is using a font without CJK coverage.
The JSON we send is correct UTF-8 with proper names; the issue is purely
on the rendering side, inside DeoVR's app.

**Possible fixes (all need investigation):**

- **A. Transliterate CJK names to Latin in stash-vr.** Lossy but
  pragmatic. Use a romanization library (e.g. `mozillazg/go-pinyin` for
  Chinese, `gojp/kana` for Japanese) at section-build time. Need to pick
  per-language mappings and accept some name distortion. Could expose a
  `--TRANSLITERATE_NAMES=true` flag.
- **B. See if DeoVR webview accepts CSS / @font-face declarations in
  any field.** Probably not — JSON-rendered sections aren't HTML — but
  worth confirming.
- **C. Drop CJK-named entities from auto-sections (filter out anything
  outside ASCII)**. Loses information but keeps the UI usable. Easy.
- **D. Stop relying on auto-sections; use a different navigation
  pattern entirely (see issue 2).** This is the path the user prefers.

**Don't propose** asking the user to install DeoVR fonts — there's no
mechanism for that on Quest 3.

### 2. 400+ sections is unusable in DeoVR webview

**Symptom:** with `AUTO_SECTIONS_PERFORMERS=true` (and similar for tags),
the user has hundreds of per-performer sections. DeoVR webview lays them
out in a packed strip that's unusable to navigate. User said: "all I can
see in webview is a long string like performer=333 performer=334
performer=335... like hundreds of them".

**Cause:** the per-performer-as-section approach was the natural way to
work around DeoVR's flat-library API limitation, but it doesn't scale.

**The user's verbatim instruction:** *"if we want to go for webview
mode, that's fine, but must use same ui as SLR".*

So the next step is **investigate SLR's library JSON shape** and copy
their navigation pattern. SLR has thousands of scenes and the user
reports their library UX works fine. Whatever they're doing is the
target.

---

## What to investigate first (priority for next session)

### Step 1: Reverse-engineer SLR's library JSON

The previous session downloaded SLR pages but did NOT specifically capture
their library JSON (the equivalent of our `/deovr`). Get it.

- SLR's library endpoint is likely `https://www.sexlikereal.com/deovr` or
  similar — check their API docs / forum, or sniff a request from the
  DeoVR app. (`HereSphere-JSON-Version` is set; SLR may have an
  equivalent header.) The previous SLR research artifacts are in
  `scripts/slr_*.{html,js,css}` (untracked) — `slr_videoplayer.js` had
  the only "DeoVR" string but only as a tooltip.
- Fetch the actual JSON. Compare its top-level structure to what
  stash-vr currently emits (`internal/api/deovr/index.go` `indexDto`).
- Specifically look for:
  - How many sections do they emit? (Probably small — 5-20.)
  - What's in each section? (Probably curated sets, e.g., "New This
    Week", "Top Rated", or genre buckets — not 400 per-performer
    sections.)
  - Do they expose any pagination or "load more" mechanism within a
    section?
  - Do they have a nested or hierarchical structure DeoVR's renderer
    handles (e.g., a "browse all performers" entrypoint)?
  - Any custom keys outside the documented schema?

### Step 2: Look at deovr.com's own library

Same exercise, same idea. Contrast SLR's approach with deovr.com's. The
DeoVR-format-JSON spec hasn't been formally published — the most
authoritative public schema is at https://github.com/hzrd149/deovr-json-schema
(referenced by the prior research agent). Read that schema to know
what fields are even valid.

### Step 3: Decide an /deovr structure that matches the SLR pattern

Given what you find, propose a new `/deovr` structure. Likely shape:

- A handful of top-level sections (e.g. "Recent", "Highly Rated",
  "Unwatched", and maybe a single combined "Browse" section that
  somehow hands off to filtered views).
- Drop the per-performer section explosion entirely.
- Whatever entrypoint SLR uses to "drill down into a performer" —
  replicate that.

Keep `/browse` (the HTML page) as the rate/edit surface; don't try to
make its Play button trigger DeoVR's player — that's been proven
impossible.

---

## Constraints, hard-earned (don't relitigate)

These were each tested in the previous session. Don't waste time
retrying them:

| Hypothesis | Reality |
|---|---|
| `deovr://https://...` URL scheme launches DeoVR from inside its in-VR browser | **No.** Inside DeoVR's own integrated browser, the scheme is a no-op (Chromium falls back to fragment). It works only from EXTERNAL browsers (Quest's Meta Browser → DeoVR app via OS intent). Verified: SLR/deovr.com don't use it either. |
| `window.vrPlayerSettings = { videoData: ... }` inlined on a page triggers the DeoVR player auto-launch in the in-VR browser | **No.** Tested. DeoVR's in-VR browser is plain Chromium and doesn't watch for that global. |
| Custom HTTP header (e.g. `DeoVR-JSON-Version`) on the JSON response triggers auto-launch | **No.** Not in any documented behavior. |
| Self-signed HTTPS cert can be installed on Quest 3 | **No.** Meta removed the user-CA install UI from Settings; the Files app filters out `.crt` files; `adbd cannot run as root`, blocking system-store push. |
| Self-signed HTTPS cert can be opened/installed via Quest's Meta Browser | **No.** Browser downloads it but the OS provides no installer activity. |
| DeoVR's webview will load HTTP-served thumbnails | **No.** Documented HTTPS-only restriction in the project README. Confirmed via testing. |
| Stash's transcoding endpoint (`/scene/{id}/stream.mp4`) works for large files | **No.** For an 8K 11.9GB MP4 it hangs indefinitely (60s+ timeout, no bytes returned). DeoVR sits on a black screen waiting. **Direct stream is the workaround** (commit `ae5c6f2`). |
| Caddy DNS-01 challenge will succeed by default on Google Fiber's network | **No.** ISP/router blocks outbound DNS to authoritative nameservers, breaking Caddy's propagation pre-check. Workaround: `propagation_timeout -1` in Caddyfile. |
| Google Fiber router's "Use custom DNS servers" toggle bypasses its rebinding filter | **No.** The toggle changes upstream resolver but the router's DNS proxy still applies the filter on the way back to clients. Bypass requires per-device manual DNS. |
| DeoVR's in-VR browser has any custom JS API or window-level extension | **No.** It's plain Chromium 144. UA: `Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.7559.236 Safari/537.36`. No DeoVR identifier. |

---

## Two architectural facts that drive everything

These are the two pieces of mental model the new agent needs to keep
straight or they'll repeat the same dead ends:

1. **DeoVR has TWO completely separate UIs inside the app, both reachable from the same URL:**
   - **In-VR browser** (Chromium, renders HTML pages — what the user calls "VR view")
   - **Library renderer / webview** (DeoVR's native UI rendering tile grids
     from `/deovr`-format JSON)

   The same URL behaves differently in each. `/browse` is HTML — only
   meaningful in the in-VR browser. `/deovr` is JSON — only meaningful
   in the library renderer. Switching is a manual user action; we
   cannot programmatically trigger it from a web page.

2. **Playback only fires from the library renderer.** Tiles in the
   library renderer hand off to DeoVR's player. There is no documented
   or tested way to trigger playback from a web page in the in-VR
   browser. This is why **all the work this session on `/browse` Play
   button was futile** and why the user has now correctly pivoted to
   webview-mode-only.

---

## Specific files that matter

When picking up the next session, these are the files to read or
modify:

```
internal/api/deovr/index.go            <- /deovr (library list) JSON shape
internal/api/deovr/videodata.go        <- /deovr/{id} (single scene) JSON
internal/api/deovr/router.go           <- routes
internal/library/sections.go           <- section assembly + name resolution
internal/library/autosection*.go       <- auto-section logic
internal/library/library.go            <- service + scene cache
internal/api/browse/                   <- HTML browse surface (filter+rate UI)
internal/static/index.gohtml           <- root status page (+ HTTPS banner)
internal/static/browse.gohtml          <- /browse HTML
internal/static/browse_scene.gohtml    <- /browse/scene/{id} HTML
internal/api/heatmap/http.go           <- /cover/{id} proxy (already used)
```

Ignore the HTTPS scaffolding (`internal/cert/`, `internal/server/server.go`'s
HTTPS branch, `/ca.crt` route) unless the next environment supports cert
install. It's dormant and out of the way.

---

## Things the user has explicitly said they don't want

- "this is a nightmare" / "this is not a good solution" — referring to
  the cert install gauntlet on Quest 3. They burned a lot of time on
  it. Don't push them through it again unless there's a new path.
- "I don't get it, why sexlikereal.com works fine?" — they expect us
  to figure out and replicate SLR's working pattern. **This is the
  request to honor for the next session.**
- They tried auto-sections. Tofu glyphs + flat list of 400 = unusable.
  They want a different shape, not more tuning of `MIN_SCENES_PER_PERFORMER`.

---

## First actions for the next session

1. **Read this file end-to-end before doing anything else.**
2. **Read `CLAUDE.md`** in the repo root for project conventions.
3. **Fetch SLR's library JSON** (whatever URL serves it) and dump
   contents. If you can't find the URL, ask the user — they have an
   active SLR session.
4. **Compare SLR's JSON shape to ours** at `internal/api/deovr/index.go`.
5. **Write a short design doc** (`docs/superpowers/specs/2026-05-08-webview-slr-pattern.md`)
   proposing the structural change. Get user buy-in before implementing.
6. Implement, ship, verify on the headset.

The user is software-savvy but exhausted by this. Show them you have a
plan before touching code; they will reject vibe-coded changes again.
