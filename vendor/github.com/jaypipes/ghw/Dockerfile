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

FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc
RUN apk add --no-cache ethtool

WORKDIR /bin

COPY --from=builder /go/src/github.com/jaypipes/ghw/ghwc /bin

CMD ghwc
