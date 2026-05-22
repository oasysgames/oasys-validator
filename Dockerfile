# Build Geth in a stock Go builder container
# FROM golang:1.24.10-bookworm as base

# Build Geth in Amazon Linux 2 to support the following distros:
# - Amazon Linux 2
# - Rocky Linux 8
# These distros use older versions of glibc, so binaries built with `golang:1.24.10-bookworm` fail to run.
# As such, Geth is built in an Amazon Linux 2 environment.
FROM amazonlinux:2 as base

# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

RUN yum update -y && yum install -y git gcc gcc-c++ make wget tar gzip ca-certificates && yum clean all && \
    update-ca-trust && \
    mkdir -p /usr/share/ca-certificates && \
    cp -a /etc/pki/ca-trust/extracted/pem/. /usr/share/ca-certificates/ && \
    rm -f /etc/ssl/certs && mkdir -p /etc/ssl/certs && \
    cp -a /etc/pki/tls/certs/. /etc/ssl/certs/

ARG TARGETARCH
RUN wget -q https://go.dev/dl/go1.24.10.linux-${TARGETARCH}.tar.gz && \
    rm -rf /usr/local/go && tar -C /usr/local -xzf go1.24.10.linux-${TARGETARCH}.tar.gz && \
    rm -f go1.24.10.linux-${TARGETARCH}.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

# Get dependencies - will also be cached if we won't change go.mod/go.sum
COPY go.mod /go-ethereum/
COPY go.sum /go-ethereum/
RUN cd /go-ethereum && go mod download

ADD . /go-ethereum

# For blst
ENV CGO_CFLAGS="-O -D__BLST_PORTABLE__"
ENV CGO_CFLAGS_ALLOW="-O -D__BLST_PORTABLE__"

FROM base as geth-builder
# NOTE: -static is removed because Go's plugin.Open() uses dlopen() internally,
# which does not work in statically-linked binaries. The final image uses
# distroless/base-debian12 (glibc included), so dynamic linking works fine.
# RUN cd /go-ethereum && go run build/ci.go install -static ./cmd/geth
RUN cd /go-ethereum && go run build/ci.go install ./cmd/geth

FROM base as plugin-builder
ARG PLUGIN_VERSION=""
RUN cd /go-ethereum && go run build/ci.go plugin \
    ${PLUGIN_VERSION:+-version "$PLUGIN_VERSION"}

# Binary extraction stages
FROM scratch as binaries
COPY --from=geth-builder /go-ethereum/build/bin/geth /geth

FROM scratch as plugin-binaries
COPY --from=plugin-builder /go-ethereum/build/bin/suspicious_txfilter.so /suspicious_txfilter.so
COPY --from=plugin-builder /go-ethereum/build/bin/build_fingerprint.txt /build_fingerprint.txt

# Final stage
FROM gcr.io/distroless/base-debian12
COPY --from=geth-builder /go-ethereum/build/bin/geth /usr/local/bin/geth
COPY --from=geth-builder /etc/ssl /etc/ssl
COPY --from=geth-builder /usr/share/ca-certificates /usr/share/ca-certificates

EXPOSE 8545 8546 30303 30303/udp
ENTRYPOINT ["geth"]

# Add some metadata labels to help programatic image consumption
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

LABEL commit="$COMMIT" version="$VERSION" buildnum="$BUILDNUM"
