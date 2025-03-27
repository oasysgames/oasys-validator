# Build Geth in a stock Go builder container
FROM golang:1.21.13-bookworm as builder

# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

<<<<<<< HEAD
RUN apt update && apt install -y git ca-certificates
=======
# Build Geth in a stock Go builder container
FROM golang:1.23-alpine AS builder
>>>>>>> 294c7321ab439545b2ab1bb7eea74a44d83e94a1

RUN apk add --no-cache make cmake gcc musl-dev linux-headers git bash build-base libc-dev
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

# Final stage
FROM gcr.io/distroless/base-debian12
COPY --from=builder /go-ethereum/build/bin/geth /usr/local/bin/geth
COPY --from=builder /etc/ssl /etc/ssl
COPY --from=builder /usr/share/ca-certificates /usr/share/ca-certificates
=======
# Pull Geth into a second stage deploy alpine container
FROM alpine:3.17

ARG BSC_USER=bsc
ARG BSC_USER_UID=1000
ARG BSC_USER_GID=1000

ENV BSC_HOME=/bsc
ENV HOME=${BSC_HOME}
ENV DATA_DIR=/data

ENV PACKAGES ca-certificates jq \
  bash bind-tools tini \
  grep curl sed gcc

RUN apk add --no-cache $PACKAGES \
  && rm -rf /var/cache/apk/* \
  && addgroup -g ${BSC_USER_GID} ${BSC_USER} \
  && adduser -u ${BSC_USER_UID} -G ${BSC_USER} --shell /sbin/nologin --no-create-home -D ${BSC_USER} \
  && addgroup ${BSC_USER} tty \
  && sed -i -e "s/bin\/sh/bin\/bash/" /etc/passwd  

RUN echo "[ ! -z \"\$TERM\" -a -r /etc/motd ] && cat /etc/motd" >> /etc/bash/bashrc

WORKDIR ${BSC_HOME}

COPY --from=builder /go-ethereum/build/bin/geth /usr/local/bin/
>>>>>>> 294c7321ab439545b2ab1bb7eea74a44d83e94a1

COPY docker-entrypoint.sh ./

RUN chmod +x docker-entrypoint.sh \
    && mkdir -p ${DATA_DIR} \
    && chown -R ${BSC_USER_UID}:${BSC_USER_GID} ${BSC_HOME} ${DATA_DIR}

VOLUME ${DATA_DIR}

USER ${BSC_USER_UID}:${BSC_USER_GID}

# rpc ws graphql
EXPOSE 8545 8546 8547 30303 30303/udp

ENTRYPOINT ["/sbin/tini", "--", "./docker-entrypoint.sh"]