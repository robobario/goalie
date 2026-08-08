FROM docker.io/goreleaser/goreleaser:v2.17.1@sha256:ad46dcae6d92cf1f39ef12e7a9aa7dcb094ac1da26d972d45da4cbfe69f341d2 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN goreleaser release --snapshot --clean

FROM scratch AS export
COPY --from=builder /src/dist /
