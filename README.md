# eSCL scanner web UI

A small web UI for eSCL (AirScan) network scanners, aimed at making duplex
scanning painless on scanners that only have a single-sided ADF: scan the
rectos, flip the stack, scan the versos, and combine the two PDFs with the
pages interleaved. Scans can be previewed, split, merged, deleted, emailed
(via [Resend](https://resend.com)) or pushed to one or more
[paperless-ngx](https://docs.paperless-ngx.com/) instances.

- Go backend (gin) talking eSCL via [stapelberg/airscan](https://github.com/stapelberg/airscan),
  using `pdftk` and ImageMagick for PDF handling
- React frontend, live updates over a websocket
- Configuration (retention days, email recipients, paperless instances and
  credentials) is managed in the UI and persisted to `settings.json` in the
  data directory

## Running

The container image is published to GHCR (linux/arm64):

```bash
docker run -d -p 8080:8080 \
  --mount type=volume,source=scanner-data,target=/scans \
  -e RESEND_API_KEY=re_... \
  -e "EMAIL_FROM=Scanner <scan@example.com>" \
  ghcr.io/rogerxaic/escl-scanner-2-merger:latest \
  -host=<scanner-ip> -dataDir=/scans
```

### Environment variables

| Variable | Purpose |
| --- | --- |
| `RESEND_API_KEY` | Resend API key for emailing scans (optional — email is disabled without it) |
| `EMAIL_FROM` | Sender, e.g. `Scanner <scan@example.com>`; the domain must be verified in Resend |
| `PAPERLESS_<HOST>_TOKEN` | Fallback paperless API token for `<HOST>` (dots as underscores, uppercase) |
| `PAPERLESS_<HOST>_USERNAME` / `_PASSWORD` | Fallback paperless basic-auth credentials |

Paperless credentials set in the settings UI take precedence over environment
variables. Accounts with MFA enabled must use an API token.

## Development

```bash
# backend (frontend served from ./dist if built)
npx nodemon --watch '*.go' --ext go --signal SIGTERM --exec 'go run *.go' -host=<scanner-ip> -dataDir=/tmp

# frontend
cd frontend && npm start
```

### Build the image locally

```bash
docker build -t escl-scanner .
```

## API

See `bruno/` for a [Bruno](https://www.usebruno.com/) collection covering the
main endpoints (`/api/scan`, `/api/split`, `/api/merge`, `/api/send`,
`/api/delete`, `/api/settings`, ...).
