FROM golang:1.13
ADD . /go/src/github.com/previousnext/github-keys
WORKDIR /go/src/github.com/previousnext/github-keys
RUN go get github.com/mitchellh/gox
RUN make build

FROM alpine:latest
RUN apk --no-cache add ca-certificates
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
COPY --from=0 /go/src/github.com/previousnext/github-keys/bin/amd64/linux/github-keys /usr/local/bin/github-keys
CMD ["github-keys"]
