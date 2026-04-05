GOOGLEAPIS_PATH := $(shell go list -m -f '{{.Dir}}' github.com/googleapis/googleapis)

proto:
	@if [ -z "$(GOOGLEAPIS_PATH)" ]; then echo "googleapis not found. Run: go get github.com/googleapis/googleapis@latest"; exit 1; fi
	mkdir -p proto/authpb
	protoc \
		-I proto \
		-I $(GOOGLEAPIS_PATH) \
		--plugin=protoc-gen-go=$(shell go env GOPATH)/bin/protoc-gen-go \
		--plugin=protoc-gen-go-grpc=$(shell go env GOPATH)/bin/protoc-gen-go-grpc \
		--plugin=protoc-gen-grpc-gateway=$(shell go env GOPATH)/bin/protoc-gen-grpc-gateway \
		--go_out=proto/authpb \
		--go-grpc_out=proto/authpb \
		--grpc-gateway_out=proto/authpb \
		proto/auth.proto

run:
	go run main.go

build:
	go build -o bin/auth-service .

wire:
	cd wire && wire

.PHONY: wire proto run build