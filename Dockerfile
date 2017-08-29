FROM golang:1.8
ADD workspace /go
RUN go get github.com/mitchellh/gox
RUN make build

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=0 /go/bin/sshd-github /usr/local/bin/sshd-github
CMD ["sshd-github"]
