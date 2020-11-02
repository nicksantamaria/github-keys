#!/usr/bin/make -f

VERSION=$(shell git describe --tags --always)
IMAGE=nicksantamaria/github-keys
NAME=github-keys
PACKAGE=github.com/nicksantamaria/$(NAME)
LGFLAGS="-extldflags "-static""

# Build binaries for linux/amd64 and darwin/amd64
build:
	gox -os='linux darwin' -arch='amd64' -output='bin/{{.Arch}}/{{.OS}}/$(NAME)' -ldflags=$(LGFLAGS) $(PACKAGE)


# Run all lint checking with exit codes for CI
lint:
	golint -set_exit_status .

# Run tests with coverage reporting
test:
	go test -cover ./...

release: docker-build docker-push

docker-build:
	docker build -t ${IMAGE}:${VERSION} -t ${IMAGE}:latest .

docker-push:
	docker push ${IMAGE}:${VERSION}
	docker push ${IMAGE}:latest
