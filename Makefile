APP_NAME := model-registry-service
BUILD_DIR := bin
DOCKER_IMAGE := $(APP_NAME)

.PHONY: build run test clean docker-build docker-run lint

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server/

run: build
	$(BUILD_DIR)/$(APP_NAME)

test:
	go test ./... -v

clean:
	rm -rf $(BUILD_DIR)

docker-build:
	docker build -f deployments/docker/Dockerfile -t $(DOCKER_IMAGE):latest .

docker-run:
	docker compose up --build

lint:
	golangci-lint run ./...

tidy:
	go mod tidy
