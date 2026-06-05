# Runbook — router / LAN IP change

Use when the home router changes, the LAN subnet changes, or the host PC's
IP changes. Walks through every spot in the stash-vr deployment that hard-codes
the LAN IP and how to verify each layer end-to-end.

## Architecture recap

```
Quest 3 ──https──▶ stash-vr.duckdns.org
                     │
                     │ DNS resolves to LAN IP (split-horizon: same IP on LAN and WAN)
                     ▼
                  Router (10.0.0.1)
                     │
                     ▼
                  PC at 10.0.0.19
                     │
              ┌──────┴───────┐
              │              │
        Caddy :443      stash-vr :9666
        (reverse proxy)        │
              │                ▼
              └──────▶  Stash :9999  (graphql + media)
                     (auth enabled; uses STASH_API_KEY)
```

Caddy talks to stash-vr over `localhost`. stash-vr talks to Stash over the LAN
IP. Both are on the same PC. Quest hits the public hostname over HTTPS.

## What hard-codes the LAN IP

| Layer            | Where                                                | Value                |
| ---------------- | ---------------------------------------------------- | -------------------- |
| stash-vr exe     | `C:\Users\scw00\Downloads\stash-vr.bat` (running)    | `STASH_GRAPHQL_URL`  |
| stash-vr (repo)  | `scripts\stash-vr.bat` (no secrets — checked in)     | `STASH_GRAPHQL_URL`  |
| DuckDNS A-record | duckdns.org dashboard                                | IP for `stash-vr`    |
| PC LAN IP        | Router admin → DHCP reservation                      | bind MAC → `10.0.0.19` |
| Quest WiFi       | Quest settings (only if using static IP — see below) | gateway + DNS        |
| Caddy            | `C:\caddy\Caddyfile`                                 | **no IP** — talks to `localhost:9666` |
| Stash            | `~/.stash/config.yml`                                | **no IP** — binds `0.0.0.0` |

## Procedure

Run these in order. Each step has a verification check; don't skip them.

### 1. Pin the PC at a stable LAN IP

stash-vr's launcher contains the PC's own LAN IP — if DHCP later assigns a
different one, stash-vr → Stash breaks.

1. On the PC: `ipconfig` → note IPv4 address, default gateway, MAC of the
   active adapter.
2. Router admin (usually `http://<gateway>`) → DHCP Reservation → bind the
   PC's MAC to a chosen IP (default here: `10.0.0.19`).
3. `ipconfig /release && ipconfig /renew` → confirm the reserved IP came back.

### 2. Update the running stash-vr launcher

`C:\Users\scw00\Downloads\stash-vr.bat` is what actually launches the daemon.
The repo copy at `scripts\stash-vr.bat` is a clean reference without secrets.

Edit the running .bat:

```bat
stash-vr.exe ^
    --STASH_GRAPHQL_URL=http://<NEW_LAN_IP>:9999/graphql ^
    --STASH_API_KEY=<KEY_FROM_STASH_CONFIG> ^
    --LISTEN_ADDRESS=:9666 ^
    --AUTO_SECTIONS_PERFORMERS=true ^
    --AUTO_SECTIONS_TAGS=true ^
    --AUTO_SECTIONS_AGGREGATES=true
```

- `<NEW_LAN_IP>` — the address pinned in step 1 (e.g. `10.0.0.19`).
- `<KEY_FROM_STASH_CONFIG>` — read from `C:\Users\scw00\.stash\config.yml`
  field `api_key:`. Same value as Stash settings → Security → API Key.
- The repo copy at `scripts\stash-vr.bat` should be updated for `STASH_GRAPHQL_URL`
  only — never check the API key into git.

**Verify locally:**

```sh
curl -sS -o NUL -w "HTTP %{http_code}\n" http://localhost:9666/
```

Expect `HTTP 200`. Watch the stash-vr console — should print stash version
without 401 warnings.

### 3. Caddy — usually no change

The Caddyfile upstream is `localhost:9666` (same host), so a LAN IP change
doesn't affect it. Just confirm Caddy is running and serving HTTPS:

```sh
curl -sS -o NUL -w "HTTP %{http_code}\n" https://stash-vr.duckdns.org/
```

Expect `HTTP 200` from the PC. (DNS must already be correct for this — if
this fails, jump to step 4 first.)

Edit cases (rare):
- Caddy moved to a different host → change `reverse_proxy` to the new
  stash-vr IP/port.
- Caddy bound to a specific interface IP (not `:443`) → update that.

TLS cert auto-renewal uses the DuckDNS API token (DNS-01 challenge), which
is independent of any IP — keeps working as long as the token is intact.

### 4. Update DuckDNS A-record

```sh
nslookup stash-vr.duckdns.org
```

If it doesn't return the new LAN IP, log into <https://www.duckdns.org> and
set the `stash-vr` domain's IP to the new value. Or hit the update API:

```
https://www.duckdns.org/update?domains=stash-vr&token=<TOKEN>&ip=<NEW_LAN_IP>
```

(`<TOKEN>` lives in `C:\caddy\Caddyfile` under the `tls.dns duckdns ...`
directive — same token, dual purpose.)

After updating, flush local DNS: `ipconfig /flushdns`. Public resolvers
update within seconds; client-device caches can take longer.

**LAN-only vs. off-LAN deployments:** the current setup points DuckDNS at the
LAN IP (split-horizon — only works on the home network). If you need
off-network access, set DuckDNS to your public WAN IP and add a router
port-forward `443 → <NEW_LAN_IP>:443`. Also enable NAT hairpinning on the
router or LAN clients won't be able to use the DuckDNS hostname.

### 5. Quest WiFi

The Quest probably had a manual IP from the old subnet — that breaks when
the subnet changes.

1. Quest → Settings → Network → tap your network → **Forget**.
2. Reconnect (uses DHCP by default).
3. Open Meta Browser → `https://stash-vr.duckdns.org` → should load.

If it fails with DNS-resolution errors, the new router likely strips
private-IP DNS answers (DNS-rebinding protection). Workaround:

1. Quest → WiFi network → Advanced → **IP settings: Static**.
2. Fill in **all four** fields (Static mode disables auto):
   - IP: any unused `10.0.0.x` outside the router's DHCP pool (e.g. `10.0.0.50`)
   - Gateway: `10.0.0.1`
   - Network prefix length: `24`
   - DNS 1: `8.8.8.8`
   - DNS 2: `1.1.1.1`
3. Save → reconnect → retest.

DNS is only editable in Static mode on Quest — the field is greyed out under
DHCP.

### 6. Full smoke test

In order — stop at the first failure:

| Check | Where | Expect |
| --- | --- | --- |
| `nslookup stash-vr.duckdns.org` | PC | new LAN IP |
| `http://localhost:9666/` | PC browser | stash-vr index page, no 401 in log |
| `https://stash-vr.duckdns.org/` | PC browser | same page, HTTPS valid |
| `https://stash-vr.duckdns.org/` | Quest Meta Browser | same page |
| `https://stash-vr.duckdns.org/deovr` | Quest browser | JSON loads |
| Open library in DeoVR/HereSphere | Quest player | scenes appear |

## Common errors and what they mean

| Symptom | Layer that's failing | Fix |
| --- | --- | --- |
| `dial tcp <IP>: connectex` in stash-vr log | stash-vr → Stash, wrong IP | Step 2 (update `STASH_GRAPHQL_URL`) and restart |
| `returned error 401: ...` in stash-vr log | stash-vr → Stash, auth | Step 2 (`STASH_API_KEY` missing or wrong) |
| **502 Bad Gateway** from Caddy on Quest | Caddy → stash-vr | stash-vr isn't running on `:9666` or crashed |
| **connection refused / timeout** on Quest | Quest → Caddy | Wrong DuckDNS A-record, wrong Quest gateway, or router blocks |
| Quest browser: "DNS not resolved" | Quest → DNS | Switch to Static IP + DNS = `8.8.8.8` / `1.1.1.1` (Step 5) |
| TLS cert warning on Quest | Caddy cert renewal failing | Check Caddy logs; DuckDNS token may have rotated |

## Notes

- `scripts\stash-vr.bat` (this repo) is intentionally **secret-free**. Only
  `STASH_GRAPHQL_URL` and other public flags live there. The API key goes
  in the Downloads copy that's not checked in.
- The Stash API key is a JWT — it doesn't expire, but Stash will refuse it
  if the user it embeds is removed. Regenerate from Stash → Settings →
  Security → API Key if you ever rotate it (then update the launcher).
- `dangerous_allow_public_without_auth: "false"` in Stash config means *all*
  non-localhost requests need an API key — there is no LAN-trust shortcut.
  Don't flip this to `true` just to skip the API key; that opens Stash to
  anyone on the LAN (and anyone who breaches Caddy, if exposed to WAN).
