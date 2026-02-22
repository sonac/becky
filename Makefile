APP_NAME=mongo-backup

.PHONY: build build-linux-amd64 test vet tidy

build:
	go build -o bin/$(APP_NAME) ./cmd/mongo-backup

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/$(APP_NAME)-linux-amd64 ./cmd/mongo-backup

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy
