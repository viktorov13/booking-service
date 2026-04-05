GO_ENV=GOCACHE=$(CURDIR)/.gocache GOPATH=$(CURDIR)/.gopath

.PHONY: up down test build seed swagger lint

up:
	docker-compose up --build

down:
	docker-compose down -v

build:
	$(GO_ENV) go build ./cmd/app

test:
	$(GO_ENV) go test ./...

seed:
	$(GO_ENV) go run ./cmd/seed

swagger:
	$(GO_ENV) go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/app/main.go -o docs

lint:
	$(GO_ENV) go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run
