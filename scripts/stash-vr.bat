@echo off
REM stash-vr launcher (reconstructed from handoff docs 2026-05-26)
REM Stash runs on the same Windows host at 10.0.0.19:9999.
REM Serves on :9666 (Caddy reverse-proxies https://stash-vr.duckdns.org -> localhost:9666).

cd /d "%~dp0"

stash-vr.exe ^
    --STASH_GRAPHQL_URL=http://10.0.0.19:9999/graphql ^
    --LISTEN_ADDRESS=:9666 ^
    --AUTO_SECTIONS_PERFORMERS=true ^
    --AUTO_SECTIONS_TAGS=true ^
    --AUTO_SECTIONS_AGGREGATES=true

echo.
echo stash-vr exited.
pause
