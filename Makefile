export
APP_NAME := fs-go-moby
POSTGRES_IMAGE := postgres:14.6-alpine

.PHONY: test
test: 
	@go test ./...
