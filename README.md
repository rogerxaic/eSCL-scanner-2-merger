# Scanning

## Running

### Development

```bash
npx nodemon --watch '*.go' --ext go --signal SIGTERM --exec 'go run *.go' -host=1.2.3.4 -dataDir=/tmp
```

### Production

```bash
docker build . -t go-scanner:latest
docker run -it --rm -p 8080:8080 \
  --mount type=volume,source=scanner-data,target=/scans/ \
  go-scanner:latest -host=host.docker.internal:1234 -dataDir=/scans
```

### Using pdftk

```bash
docker run -it --rm -p 8080:8080 --mount type=volume,source=scanner-data,target=/scans/ -w /scans --entrypoint=pdftk go-scanner:dev A=recto.pdf B=verso.pdf shuffle A Bend-1 output merged.pdf
```

## Deploying

```bash
tsh ssh pi@target rm -rf /home/pi/eSCL-scanner-2-merger/go-scanner/.git
tsh scp -r . pi@target:/home/pi/eSCL-scanner-2-merger/go-scanner

tsh scp -r *.go pi@target:/home/pi/eSCL-scanner-2-merger/go-scanner
tsh scp -r go.sum go.mod Dockerfile .Dockerignore pi@target:/home/pi/eSCL-scanner-2-merger/go-scanner
tsh scp -r frontend/pa* frontend/ts* frontend/src frontend/public pi@target:/home/pi/eSCL-scanner-2-merger/go-scanner/frontend

tsh ssh pi@target "docker compose -f /home/pi/eSCL-scanner-2-merger/compose.yaml up -d --build"
```
