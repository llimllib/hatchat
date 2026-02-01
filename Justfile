run: build
    ./hatchat

# Install client npm dependencies if needed
[private]
npm-deps:
    @test -d client/node_modules || (cd client && npm install)

lint: npm-deps
    golangci-lint run & (cd client && npx biome check src *.mjs)

test: lint
    cd client && npm test
    go test ./...

models:
    bash tools/models.sh

# Generate JSON Schema from Go protocol types
schema:
    go run ./tools/schemagen > schema/protocol.json

# Generate TypeScript types from JSON Schema
client-types: schema npm-deps
    cd client && node gen-types.mjs && npx biome check --fix src/protocol.generated.ts

# Build the documentation website (includes protocol schema docs)
site: schema
    bash tools/build-site.sh

build-js: npm-deps
    cd client && npx tsgo --noEmit && node esbuild.config.mjs

build-go:
    go build -o hatchat ./cmd/server.go

build: npm-deps
    (cd client && node esbuild.config.mjs) & go build -o hatchat ./cmd/server.go

browse-db:
    # Maybe use datasette on the command line instead for broader applicability?
    open /Applications/Datasette.app chat.db

# Run the server with dev users seeded (alice/alice, bob/bob)
run-dev: build
    SEED_DEVELOPMENT_DB=1 ./hatchat
