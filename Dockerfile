# Multi-stage build for the patched GeeseFS (encoding-type=url listing fix, see
# DeepBits ADR-0007; symlinks surviving a remount, ADR-0008). The final image is
# a tool image: other Dockerfiles
# COPY the binary out of it, mirroring the ghcr.io/deepbitstechnology/rizin:release
# convention. The binary lives at BOTH /geesefs and /usr/bin/geesefs.

FROM golang:1.25.9-alpine AS builder

# git is needed because go.mod pulls dependencies; bash for shell parity with
# upstream's own Dockerfile.build.
RUN apk add --no-cache git bash

WORKDIR /build

# Copy the whole source. The module has a `replace github.com/aws/aws-sdk-go =>
# ./s3ext` directive, so the vendored SDK must be present before `go mod download`.
COPY . .

RUN go mod download

# Static, stripped, linux/amd64 build (matches the k8s nodes and the Daytona
# snapshot). Version records the base tag + patch so the running binary is
# identifiable.
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
RUN go build -ldflags "-X main.Version=0.43.8-deepbits.2 -s -w" -o /geesefs .

# Minimal final stage: just the patched binary at both COPY-able paths.
FROM debian:stable-slim AS final
COPY --from=builder /geesefs /geesefs
COPY --from=builder /geesefs /usr/bin/geesefs
ENTRYPOINT ["/geesefs"]
