#!/usr/bin/make -f

VERSION=$(shell git describe --tags --always)
IMAGE=previousnext/github-keys

release: build push

build:
	docker build -t ${IMAGE}:${VERSION} .

push:
	docker push ${IMAGE}:${VERSION}
