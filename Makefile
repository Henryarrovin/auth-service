proto:
	cd proto && protoc --go_out=. --go-grpc_out=. auth.proto

run:
	source .env && go run main.go

build:
	source .env && go build -o bin/auth-service .