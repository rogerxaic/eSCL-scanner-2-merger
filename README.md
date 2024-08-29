# Scanning

## Development

```bash
npx nodemon --watch '*.go' --ext go --signal SIGTERM --exec 'go run example.go' -host=1.2.3.4
```

## Production

```bash
docker build . -t go-scanner:latest
docker run -it --rm -p 8080:8080 go-scanner:latest -host=host.docker.internal:1234
```
