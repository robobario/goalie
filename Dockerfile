FROM golang:1.26@sha256:3aff6657219a4d9c14e27fb1d8976c49c29fddb70ba835014f477e1c70636647 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN VERSION=$(cat VERSION) && \
    mkdir -p /dist && \
    GOOS=linux  GOARCH=amd64 go build -ldflags="-s -w -X main.version=${VERSION}" -o /dist/goalie-linux-amd64  ./cmd/goalie && \
    GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.version=${VERSION}" -o /dist/goalie-darwin-amd64 ./cmd/goalie && \
    GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.version=${VERSION}" -o /dist/goalie-darwin-arm64 ./cmd/goalie

FROM scratch AS export
COPY --from=builder /dist /
