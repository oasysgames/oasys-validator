# Build Geth in a stock Go builder container
FROM golang:1.21.13-bookworm as builder

# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

RUN apt update && apt install -y git ca-certificates

# Get dependencies - will also be cached if we won't change go.mod/go.sum
COPY go.mod /go-ethereum/
COPY go.sum /go-ethereum/
RUN cd /go-ethereum && go mod download

ADD . /go-ethereum
RUN cd /go-ethereum && go run build/ci.go install -static ./cmd/geth

# Binary extraction stage
FROM scratch as binaries
COPY --from=builder /go-ethereum/build/bin/geth /geth

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
