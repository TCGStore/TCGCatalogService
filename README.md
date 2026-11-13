# catalog-service

Go + MongoDB. Cartas y producto sellado. Implementa `proto/catalog.proto` en el puerto gRPC 50051.

## De dónde salen los productos y sus precios

| | Datos y precio | Cómo se cargan |
|---|---|---|
| **Cartas individuales** | De la API pública [pokemontcg.io](https://docs.pokemontcg.io): nombre, set, número, rareza, tipos, imagen y el precio de mercado de TCGplayer (USD) | `cmd/import` |
| **Producto sellado** (sobres, booster boxes, ETBs) | La API no los tiene. Se crean con el seed **sin precio** y el staff (admin) les pone precio desde el BFF | `cmd/seed` + RPC `SetProductPrice` |

**Un producto sin precio no lo ve ningún cliente.** `GetProduct`, `GetProducts` y
`SearchProducts` lo ocultan salvo que la petición traiga `include_unpriced: true`, y eso
solo lo hace el BFF cuando quien pregunta es admin. Como orders-service no lo manda,
tampoco se puede agregar al carrito ni comprar.

`SetProductPrice` rechaza las cartas (`FAILED_PRECONDITION`): el siguiente import pisaría
el precio puesto a mano. Si la API no tiene precio para una carta, esa carta queda oculta
hasta un import que sí lo traiga.

### El import de cartas

```bash
make import                     # sets por defecto: sv3pt5 (151), sv3 (Obsidian Flames), sv1 (Scarlet & Violet)
make import SETS=sv3pt5,sv4     # otros sets (ids de pokemontcg.io)
```

- Cada carta se guarda como **un producto en inglés, Near Mint**, porque la API trae un
  solo precio por carta. El id sigue el mismo formato de siempre: `prd_mew_006_en_nm`
  (código del set impreso en la carta + número), así que coincide con inventory-service.
- Se puede repetir: actualiza datos y precios. Solo escribe cartas; nunca toca el
  producto sellado.
- La API falla seguido (500/502, a veces 2 de cada 3 peticiones). El cliente reintenta solo
  hasta 10 veces por petición; si aun así un set falla, el import sigue con los demás y al
  final dice cuáles faltaron, para volver a correrlo con `-sets`.
  Sin API key funciona con un límite de 1000 peticiones al día; con `POKEMONTCG_API_KEY`
  el límite es mayor.

## Estructura

```
cmd/server/          servidor gRPC (con health check y reflection)
cmd/seed/            carga el producto sellado, sin precio (products.json)
cmd/import/          carga las cartas y sus precios desde pokemontcg.io
internal/catalog/    handlers gRPC, acceso a MongoDB, conversión documento ↔ proto y carta de la API → producto
internal/pokemontcg/ cliente HTTP de la API de pokemontcg.io
internal/config/     variables de entorno y conexión a MongoDB
gen/catalogpb/       código generado desde el .proto (no se edita a mano)
```

El código generado **se sube al repo**: así el servicio compila sin instalar `protoc`.
Si cambia `proto/catalog.proto`, se regenera con `make proto` y se sube junto al cambio.

## Correr en local

Requisitos: Go, `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc` y Docker.

```bash
# desde TCGBackend
docker compose up -d mongodb

# desde este repo
make seed    # carga el producto sellado (se puede repetir: no pisa los precios ya puestos)
make import  # carga las cartas desde pokemontcg.io (necesita internet)
make run     # levanta el servidor en :50051
make test
```

O todo con Docker, desde TCGBackend:

```bash
docker compose up -d --build catalog-service
docker compose run --rm --entrypoint /app/seed catalog-service
docker compose run --rm --entrypoint /app/import catalog-service
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

# vista del staff: producto sellado que espera precio, y ponérselo
grpcurl -plaintext -d '{"unpriced_only":true}' \
  localhost:50051 tcgstore.catalog.CatalogService/SearchProducts
grpcurl -plaintext -d '{"id":"prd_obf_box_en","price_cents":14999}' \
  localhost:50051 tcgstore.catalog.CatalogService/SetProductPrice
```

## Variables de entorno

| Variable | Default | |
|---|---|---|
| `MONGO_URI` | — (obligatoria) | `mongodb://tcg:tcg@localhost:27017` en local |
| `MONGO_DB` | `catalog` | |
| `GRPC_PORT` | `50051` | |
| `POKEMONTCG_API_KEY` | — (opcional) | Solo la usa `cmd/import` |

## Notas para quien consume este servicio

- En MongoDB los enums se guardan cortos (`"CARD"`, `"NEAR_MINT"`); por gRPC viajan como el enum del proto.
- `price_cents` es `int64`: en JSON (grpcurl) aparece como string (`"2499"`), y en
  TypeScript con `@grpc/proto-loader` conviene usar `longs: Number`.
- `price_cents` es `optional`: un producto sin precio llega sin el campo (en TypeScript,
  `undefined`). Solo pasa con `include_unpriced: true`.
- Los ids de producto (`prd_mew_006_en_nm`) son los mismos que usa inventory-service.

## Contratos

Los archivos de `proto/` son una **copia** de los de `TCGBackend/proto`, que es la fuente
de verdad. No se editan aquí: el cambio se acuerda con el equipo, se hace en `TCGBackend` y
se copia con `./scripts/sync-proto.sh` (desde TCGBackend).
