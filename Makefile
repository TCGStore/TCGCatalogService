# Local development defaults; they match .env.example and docker-compose.yml.
MONGO_URI ?= mongodb://tcg:tcg@localhost:27017
export MONGO_URI

.PHONY: proto run seed test

# Regenerate gen/catalogpb after changing proto/catalog.proto.
proto:
	protoc -I proto \
		--go_out=gen/catalogpb --go_opt=paths=source_relative \
		--go-grpc_out=gen/catalogpb --go-grpc_opt=paths=source_relative \
		proto/catalog.proto

run:
	go run ./cmd/server

seed:
	go run ./cmd/seed

test:
	go test ./...
