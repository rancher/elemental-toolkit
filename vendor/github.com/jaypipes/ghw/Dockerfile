FROM golang:1.21-buster as builder
WORKDIR /go/src/github.com/jaypipes/ghw

ENV GOPROXY=direct

# go.mod and go.sum go into their own layers.
COPY go.mod .
COPY go.sum .

# This ensures `go mod download` happens only when go.mod and go.sum change.
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o ghwc ./cmd/ghwc/

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
RUN apk add --no-cache ethtool

WORKDIR /bin

COPY --from=builder /go/src/github.com/jaypipes/ghw/ghwc /bin

CMD ghwc
