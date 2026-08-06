# eSCL scanner web UI

A small self-hosted web UI for network scanners that speak **eSCL** (also
known as AirScan/AirPrint scanning — most modern HP, Brother, Canon and Epson
network scanners do).

Its main trick is making **duplex scanning painless on scanners that only
have a single-sided document feeder**: scan the front sides, flip the stack,
scan the backs, and combine both PDFs with the pages interleaved into one
correctly-ordered document.

Beyond that, scans can be previewed, split into single pages, merged,
deleted, **emailed** (via [Resend](https://resend.com)) or pushed to one or
more [paperless-ngx](https://docs.paperless-ngx.com/) instances — all from a
phone-friendly web page.

## How it works

- Go backend ([gin](https://github.com/gin-gonic/gin)) that drives the
  scanner over eSCL using
  [stapelberg/airscan](https://github.com/stapelberg/airscan), and shells out
  to `pdftk` / ImageMagick for PDF splitting, merging and thumbnails
  (both are included in the container image).
- React frontend served by the same binary, with live updates over a
  websocket (scan status, results appearing, other browsers' changes).
- Everything configurable at runtime (retention, email recipients, paperless
  instances and credentials) lives in the **⚙️ settings view** and is
  persisted to `settings.json` in the data directory — no rebuild needed.

## Quick start

### 1. Find your scanner's IP

The scanner must be reachable on your network and support eSCL. Find its IP
from your router's client list or the scanner's own network menu, or discover
it via mDNS:

```bash
# macOS
dns-sd -B _uscan._tcp

# Linux
avahi-browse -rt _uscan._tcp
```

A static IP / DHCP reservation for the scanner is recommended.

### 2. Run the container

Images for linux/arm64 are published to GHCR on every push to `main`:

```bash
docker run -d --name scanner -p 8080:8080 \
  --mount type=volume,source=scanner-data,target=/scans \
  ghcr.io/rogerxaic/escl-scanner-2-merger:latest \
  -host=192.0.2.10 -dataDir=/scans
```

Or with compose:

```yaml
services:
  scanner:
    image: ghcr.io/rogerxaic/escl-scanner-2-merger:latest
    restart: always
    ports:
      - 8080:8080
    volumes:
      - ./data:/scans
    command:
      - -host=192.0.2.10        # your scanner's IP (or host[:port])
      - -dataDir=/scans
    environment:
      - GIN_MODE=release
      # optional, for emailing scans:
      - RESEND_API_KEY=re_...
      - EMAIL_FROM=Scanner <scan@example.com>
```

Open `http://<docker-host>:8080`, put a document in the feeder or on the
glass, and press **Scan**.

### 3. Configure (optional)

Open the **⚙️** button in the UI:

- **Retention** — scans are deleted after N days (daily cleanup at 02:00).
- **Email recipients** — the addresses offered in the send dialog.
  Requires `RESEND_API_KEY` and `EMAIL_FROM` (see below).
- **Paperless instances** — host + credentials per instance. Accounts with
  **MFA enabled must use an API token** (paperless-ngx → profile → API auth
  token) instead of a password; paperless rejects basic auth for them.

## Command-line flags

| Flag | Default | Purpose |
| --- | --- | --- |
| `-host` | `127.0.0.1` | Scanner address, `host` or `host:port` |
| `-dataDir` | `/scans` | Where scans, thumbnails and config are stored |

## Environment variables

| Variable | Purpose |
| --- | --- |
| `RESEND_API_KEY` | [Resend](https://resend.com) API key. Email is disabled without it |
| `EMAIL_FROM` | Sender, e.g. `Scanner <scan@example.com>`. The domain must be verified in Resend |
| `PAPERLESS_<HOST>_TOKEN` | Fallback API token for a paperless instance, when not set in the UI. `<HOST>` is the hostname uppercased with dots as underscores, e.g. `PAPERLESS_PAPERS_EXAMPLE_COM_TOKEN` |
| `PAPERLESS_<HOST>_USERNAME` / `_PASSWORD` | Fallback basic-auth credentials, same naming |
| `PAPERLESS_USERNAME` / `PAPERLESS_PASSWORD` | Generic fallback credentials for all instances |
| `GIN_MODE` | Set to `release` outside development |

Credentials entered in the settings UI take precedence over environment
variables. The API never returns stored secrets — only whether one is set.

## Scanning double-sided documents

1. Put the stack in the feeder front-side up, press **Scan**.
2. Flip the whole stack, press **Scan** again.
3. Select the two results (badges 1 and 2 show the order: rectos first).
4. Press **Combine duplex** — the pages are interleaved (fronts in order,
   backs in reverse) into a single PDF.

**Split PDF** does the opposite: bursts a multi-page scan into one result
per page, e.g. to drop a blank page or send pages to different places.

## Data layout

Everything lives in `-dataDir` (mount it as a volume):

```
/scans/
  <session-uuid>/scanned.pdf   # one directory per scan
  thumbnails/<session>.jpg
  settings.json                # UI-managed configuration
  scanresults.json             # scan list, survives restarts
```

Old sessions are garbage-collected daily according to the retention setting
(`GET /api/cleanup` triggers it manually).

## Development

```bash
# backend — auto-restarts on change; frontend is served from ./dist if built
npx nodemon --watch '*.go' --ext go --signal SIGTERM \
  --exec 'go run *.go' -host=<scanner-ip> -dataDir=/tmp/scans

# frontend dev server
cd frontend && npm install && npm start
```

Build the image locally:

```bash
docker build -t escl-scanner .
```

CI (`.github/workflows/main.yaml`) builds the arm64 image on every PR and
pushes `ghcr.io/rogerxaic/escl-scanner-2-merger:latest` plus a
`sha-<commit>` tag on pushes to `main`.

## API

The UI uses a small JSON API under `/api` (`/scan`, `/split`, `/merge`,
`/send`, `/delete`, `/settings`, `/ws`, ...). A
[Bruno](https://www.usebruno.com/) collection covering the main endpoints
lives in `bruno/`.
