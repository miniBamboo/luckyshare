# Build luckyshare in a stock Go builder container
FROM golang:alpine as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git


RUN git clone https://github.com/miniBamboo/luckyshare.git
WORKDIR  /go/luckyshare
RUN git checkout $(git describe --tags `git rev-list --tags --max-count=1`)
RUN make all

# Pull luckyshare into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /go/luckyshare/bin/luckyshare /usr/local/bin/
COPY --from=builder /go/luckyshare/bin/bootnode /usr/local/bin/

EXPOSE 51991 11235 11235/udp 55555/udp
ENTRYPOINT ["luckyshare"]