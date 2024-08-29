# Scanning

## Development

```bash
npx nodemon --watch '*.go' --ext go --signal SIGTERM --exec 'go run example.go
```

## Production

```bash
docker build . -t go-scanner:latest
docker run -p 8080:8080 go-scanner:latest
```
