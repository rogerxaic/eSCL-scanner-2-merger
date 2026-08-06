FROM node:20-alpine AS frontend

WORKDIR /app

COPY frontend/package*.json ./

RUN npm install

COPY frontend/ ./

RUN npm run build

# # # # # # # # # # # # # 
# # # # # # # # # # # # # 

FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN go build -o scanner .

# # # # # # # # # # # # # 
# # # # # # # # # # # # # 

FROM ubuntu:24.04

LABEL org.opencontainers.image.source="https://github.com/rogerxaic/eSCL-scanner-2-merger"

WORKDIR /app

RUN apt update && \
    apt install -y pdftk imagemagick && \
    apt clean && \
    apt autoremove -y

COPY --from=builder /app/scanner /usr/bin/scanner

COPY --from=frontend /app/build ./dist

VOLUME /scans

ENTRYPOINT ["scanner"]
