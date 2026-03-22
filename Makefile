proto:
	cd proto && protoc --go_out=. --go-grpc_out=. auth.proto

run:
	go run main.go

build:
	go build -o bin/auth-service .