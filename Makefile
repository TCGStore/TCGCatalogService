# Local development defaults; they match .env.example and docker-compose.yml.
MONGO_URI ?= mongodb://tcg:tcg@localhost:27017
export MONGO_URI

.PHONY: proto run seed import test

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

# Cards and prices from pokemontcg.io (needs internet). Other sets: make import SETS=sv3pt5,sv1
import:
	go run ./cmd/import $(if $(SETS),-sets $(SETS))

test:
	go test ./...
