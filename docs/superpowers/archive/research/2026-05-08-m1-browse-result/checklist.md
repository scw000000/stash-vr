# M1 /browse — Quest 3 Meta Browser checklist

**Run on:** Quest 3 hardware, Meta Browser (NOT DeoVR's in-VR browser; that's M2 territory and known not to support this stack).

**URL to open:** `https://stash-vr.duckdns.org/browse`

For each criterion: PASS / FAIL / PARTIAL + one-line note.

## Browse / search

- [x] Open `/browse` directly (no `?q=` in URL). Full grid renders with all scenes. (Baseline — confirms empty `q` is treated as no filter.)
  - Result: ___ — note: ___

- [x] /browse loads. Cards visible. Thumbnails visible. Search input visible above the grid.
  - Result: ___ — note: ___

- [x] Type a known title fragment, press Enter. URL shows `?q=<fragment>`. Grid filters.
  - Result: ___ — note: ___

- [x] Click "Clear" → all scenes back.
  - Result: ___ — note: ___

- [x] Click into a sidebar performer / studio / tag → entity-filtered grid loads.
  - Result: ___ — note: ___

- [x] On entity-filtered route, type a query → grid scopes to entity + query.
  - Result: ___ — note: ___

- [x] Pagination Next/Prev works AND preserves `?q=...` if present.
  - Result: ___ — note: ___

- [x] No `▶` overlay on tiles (only thumbnail + duration).
  - Result: ___ — note: ___

## Scene detail / playback

- [x] Click any scene tile → detail page loads.
  - Result: ___ — note: ___

- [x] Video element visible at top of page. Player controls work.
  - Result: ___ — note: ___

- [x] Click play (or unmute) — audio audible, frames visible.
  - Result: ___ — note: ___

- [x] Drag the seek scrubber to a different position — playback resumes at the dragged position within ~2 s. (Validates byte-range. If playback restarts from 0:00 or hangs, byte-range is broken → FAIL.)
  - Result: ___ — note: ___

- [x] No "Play in DeoVR" button anywhere on scene detail.
  - Result: ___ — note: ___

## Existing mutations regression

- [x] Click a star → rating updates (visible after page reload).
  - Result: ___ — note: ___

- [] Toggle favorite → state persists.
  - Result: ___ — note: ___

- [ ] Add a tag via the input → tag appears as a chip.
  - Result: ___ — note: ___

- [ ] Remove a tag → chip disappears.
  - Result: ___ — note: ___

- [ ] O-counter +/- → number updates.
  - Result: ___ — note: ___

- [ ] Organized toggle → button state changes.
  - Result: ___ — note: ___

## Overall

- [x] All checks PASS → proceed to M2 design.
- [ ] At least one FAIL → write up in result.md and surface to user.
