# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.1

FROM golang:${GO_VERSION}-bookworm AS build

WORKDIR /src

ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/askl-golang-indexer ./cmd/askl-golang-indexer

FROM golang:${GO_VERSION}-bookworm

RUN groupadd --gid 65532 nonroot \
    && useradd --uid 65532 --gid 65532 --home-dir /home/nonroot --create-home --shell /usr/sbin/nologin nonroot \
    && mkdir -p /workspace /out \
    && chown -R 65532:65532 /workspace /out /home/nonroot

ENV HOME=/tmp \
    GOPATH=/tmp/go \
    GOCACHE=/tmp/go-build

WORKDIR /workspace

COPY --from=build /out/askl-golang-indexer /usr/local/bin/askl-golang-indexer

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/askl-golang-indexer"]
CMD ["--help"]
