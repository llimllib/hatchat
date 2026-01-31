run: build
    ./hatchat

lint:
    golangci-lint run & (cd client && npx biome check src)

test: lint
    cd client && npm test
    go test ./...

models:
    bash tools/models.sh

# Generate JSON Schema from Go protocol types
schema:
    go run ./tools/schemagen > schema/protocol.json

# Generate TypeScript types from JSON Schema
client-types: schema
    cd client && node gen-types.mjs && npx biome check --fix src/protocol.generated.ts

build-js:
    cd client && npx tsc --noEmit && node esbuild.config.mjs

build-go:
    go build -o hatchat ./cmd/server.go

build:
    (cd client && node esbuild.config.mjs) & go build -o hatchat ./cmd/server.go

browse-db:
    # Maybe use datasette on the command line instead for broader applicability?
    open /Applications/Datasette.app chat.db
