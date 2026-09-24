FROM golang:1.27

WORKDIR /go/src/github.com/djherbis/times
COPY . .

RUN GO111MODULE=auto go test -covermode=count -coverprofile=profile.cov
