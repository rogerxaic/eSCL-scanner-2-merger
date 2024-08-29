FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY . .

RUN go build -o main .

# # # # # # # # # # # # # 
# # # # # # # # # # # # # 

FROM minidocks/pdftk

COPY --from=builder /app/main /usr/bin/main

ENTRYPOINT ["main"]
