# Build Geth in a stock Go builder container
FROM golang:1.23.7-bookworm as builder

# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

<<<<<<< HEAD
RUN apt update && apt install -y git ca-certificates
=======
# Build Geth in a stock Go builder container
FROM golang:1.24-alpine AS builder
>>>>>>> fca6a6bee850b226938d2f2a990afab3246efc1e

# Get dependencies - will also be cached if we won't change go.mod/go.sum
COPY go.mod /go-ethereum/
COPY go.sum /go-ethereum/
RUN cd /go-ethereum && go mod download

ADD . /go-ethereum

# For blst
ENV CGO_CFLAGS="-O -D__BLST_PORTABLE__"
ENV CGO_CFLAGS_ALLOW="-O -D__BLST_PORTABLE__"
RUN cd /go-ethereum && go run build/ci.go install -static ./cmd/geth

<<<<<<< HEAD
# Binary extraction stage
FROM scratch as binaries
COPY --from=builder /go-ethereum/build/bin/geth /geth
=======
# Pull Geth into a second stage deploy alpine container
FROM alpine:3.21
>>>>>>> fca6a6bee850b226938d2f2a990afab3246efc1e

# Final stage
FROM gcr.io/distroless/base-debian12
COPY --from=builder /go-ethereum/build/bin/geth /usr/local/bin/geth
COPY --from=builder /etc/ssl /etc/ssl
COPY --from=builder /usr/share/ca-certificates /usr/share/ca-certificates

EXPOSE 8545 8546 30303 30303/udp
ENTRYPOINT ["geth"]

# Add some metadata labels to help programatic image consumption
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

LABEL commit="$COMMIT" version="$VERSION" buildnum="$BUILDNUM"
