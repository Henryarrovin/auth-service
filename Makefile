proto:
	mkdir -p proto/authpb
	protoc \
  		--go_out=proto/authpb \
  		--go-grpc_out=proto/authpb \
  		proto/auth.proto

run:
	go run main.go

build:
	go build -o bin/auth-service .