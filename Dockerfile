FROM golang:1.23 AS builder

ENV GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64

WORKDIR /src
COPY . .

RUN go build -ldflags "-s -w -extldflags '-static'" -o /bin/app . 

FROM gruebel/upx:latest AS upx
COPY --from=builder /bin/app  /app.org
RUN upx --best --lzma -o /app /app.org

FROM scratch
COPY --from=upx /app /app
ENTRYPOINT ["/app"]