# catalog-service

Go + MongoDB. Cartas y producto sellado. Implementa `proto/catalog.proto` en el puerto gRPC 50051.

## Estructura

```
cmd/server/          servidor gRPC (con health check y reflection)
cmd/seed/            carga los productos de prueba (products.json)
internal/catalog/    handlers gRPC, acceso a MongoDB y conversión documento ↔ proto
internal/config/     variables de entorno y conexión a MongoDB
gen/catalogpb/       código generado desde el .proto (no se edita a mano)
```

El código generado **se sube al repo**: así el servicio compila sin instalar `protoc`.
Si cambia `proto/catalog.proto`, se regenera con `make proto` y se sube junto al cambio.

## Correr en local

Requisitos: Go, `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc` y Docker.

```bash
# desde la raíz del repo
docker compose up -d mongodb

# desde services/catalog-service
make seed    # carga los 17 productos de prueba (se puede repetir sin duplicar)
make run     # levanta el servidor en :50051
make test
```

O todo con Docker, desde la raíz:

```bash
docker compose up -d --build catalog-service
docker compose run --rm --entrypoint /app/seed catalog-service
```

## Probar con grpcurl

```bash
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{"id":"prd_mew_006_en_nm"}' \
  localhost:50051 tcgstore.catalog.CatalogService/GetProduct

# "carta de tipo fuego en inglés, Near Mint, bajo 30 dólares"
grpcurl -plaintext -d '{"type":"PRODUCT_TYPE_CARD","energy_type":"Fire","language":"en",
  "condition":"CARD_CONDITION_NEAR_MINT","max_price_cents":3000}' \
  localhost:50051 tcgstore.catalog.CatalogService/SearchProducts
```

## Variables de entorno

| Variable | Default | |
|---|---|---|
| `MONGO_URI` | — (obligatoria) | `mongodb://tcg:tcg@localhost:27017` en local |
| `MONGO_DB` | `catalog` | |
| `GRPC_PORT` | `50051` | |

## Notas para quien consume este servicio

- En MongoDB los enums se guardan cortos (`"CARD"`, `"NEAR_MINT"`); por gRPC viajan como el enum del proto.
- `price_cents` es `int64`: en JSON (grpcurl) aparece como string (`"2499"`), y en
  TypeScript con `@grpc/proto-loader` conviene usar `longs: Number`.
- Los ids de producto (`prd_mew_006_en_nm`) son los mismos que usará inventory-service.
- Los datos de prueba son de demo: los precios son inventados.
