.PHONY: api
api:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=./pb --go-grpc_opt=paths=source_relative grpc/hello.proto