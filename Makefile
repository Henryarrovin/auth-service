GOOGLEAPIS_PATH := $(shell go list -m -f "{{.Dir}}" github.com/googleapis/googleapis)

PROTOC_GEN_GO := $(shell where protoc-gen-go)
PROTOC_GEN_GO_GRPC := $(shell where protoc-gen-go-grpc)
PROTOC_GEN_GRPC_GATEWAY := $(shell where protoc-gen-grpc-gateway)

proto:
	mkdir -p proto/authpb
	protoc \
		-I proto \
		-I "$(GOOGLEAPIS_PATH)" \
		--plugin=protoc-gen-go="$(PROTOC_GEN_GO)" \
		--plugin=protoc-gen-go-grpc="$(PROTOC_GEN_GO_GRPC)" \
		--plugin=protoc-gen-grpc-gateway="$(PROTOC_GEN_GRPC_GATEWAY)" \
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
	-p 80:80 \
	-p 8080:8080 \
	-p 50051:50051 \
	-v auth-server-docker:/var/lib/docker \
	-v auth-minikube:/root/.minikube \
	-v auth-kube:/root/.kube \
	-v C:/Users/henry/Desktop/auth-service/kubernetes:/workspace/kubernetes \
	auth-server

.PHONY: wire proto run build docker-run