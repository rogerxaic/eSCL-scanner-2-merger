FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN go build -o scanner .

# # # # # # # # # # # # # 
# # # # # # # # # # # # # 

FROM ubuntu:24.04

RUN apt update && \
    apt install -y pdftk && \
    apt clean && \
    apt autoremove -y

COPY --from=builder /app/scanner /usr/bin/scanner

VOLUME /scans

ENTRYPOINT ["scanner"]
