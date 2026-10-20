.PHONY: build test vet install

build:
	go build -o bin/devnode ./cmd/devnode

test:
	go test ./...

vet:
	go vet ./...

install:
	./scripts/install.sh

