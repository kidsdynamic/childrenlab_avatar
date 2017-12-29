.PHONY: deps vet test dev cp build clean buildFront

PACKAGES = $(shell glide novendor)
DOCKER_REPO_URL = jack08300/childrenlab_avatar

deps:
	dep ensure

vet:
	go vet $(PACKAGES)

build:
	GOOS=linux go build -o ./build/main *.go

clean:
	rm -rf build/*
	find . -name '*.test' -delete

push-image: clean build build-image
	docker tag childrenlab_avatar $(DOCKER_REPO_URL):latest
	docker push $(DOCKER_REPO_URL):latest

build-image:
	docker build --rm -t childrenlab_avatar:latest .