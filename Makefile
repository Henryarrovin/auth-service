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

docker-run:
	docker run -d \
	--name auth-server \
	--privileged \
	-p 2222:22 \
	-p 8080:8080 \
	-p 50051:50051 \
	-v auth-server-docker:/var/lib/docker \
	-v auth-minikube:/root/.minikube \
	-v auth-kube:/root/.kube \
	-v C:/Users/henry/Desktop/auth-service/kubernetes:/workspace/kubernetes \
	auth-server

.PHONY: wire proto run build docker-run