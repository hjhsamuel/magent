FROM golang:1.27.3 AS builder

ARG VERSION=DEV
ARG COMMITID=unknown

WORKDIR /src

COPY . .

RUN go env -w GOPROXY=https://goproxy.cn,direct && \
    go mod download && \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X 'main.Version=${VERSION}' -X 'main.CommitID=${COMMITID}' -X 'main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" -o /out/main ./cmd/

FROM golang:1.27.3

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -m 777 -p /app/logs /app/workspace

WORKDIR /app

COPY --from=builder /out/main main

EXPOSE 8080

ENTRYPOINT ["./main"]