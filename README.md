# Scanning

## Running

### Development

```bash
npx nodemon --watch '*.go' --ext go --signal SIGTERM --exec 'go run *.go' -host=1.2.3.4 -dataDir=/tmp
```

### Production

```bash
docker build . -t go-scanner:latest
docker run -it --rm -p 8080:8080 --mount type=volume,source=scanner-data,target=/scans/ go-scanner:latest -host=host.docker.internal:1234 -dataDir=/scans
```

### Using pdftk

```bash
docker run -it --rm -p 8080:8080 --mount type=volume,source=scanner-data,target=/scans/ -w /scans --entrypoint=pdftk go-scanner:dev A=recto.pdf B=verso.pdf shuffle A Bend-1 output merged.pdf
```
