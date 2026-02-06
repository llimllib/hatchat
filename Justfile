run: build
    ./hatchat

# Install client npm dependencies if needed
[private]
npm-deps:
    @test -d client/node_modules || (cd client && npm install)

lint: npm-deps
    mise exec -- golangci-lint run & (cd client && npx biome check src *.mjs) && wait

test: lint
    cd client && npm test
    go test -tags fts5 ./...

# Run all tests including e2e (slower but comprehensive)
test-all: test e2e

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
    go build -tags fts5 -o hatchat ./cmd/server.go

build: npm-deps
    (cd client && node esbuild.config.mjs) & go build -tags fts5 -o hatchat ./cmd/server.go

browse-db:
    # Maybe use datasette on the command line instead for broader applicability?
    open /Applications/Datasette.app chat.db

# Run the server with dev users seeded (alice/alice, bob/bob)
run-dev: build
    SEED_DEVELOPMENT_DB=1 ./hatchat

# Run the server for e2e tests (fresh database each run)
run-e2e: build
    rm -f e2e-test.db
    ./hatchat -db file:e2e-test.db

# Install e2e test dependencies
[private]
e2e-deps:
    @test -d e2e/node_modules || (cd e2e && npm install && npx playwright install chromium)

# Run e2e tests with Playwright
e2e: build e2e-deps
    cd e2e && npm test

# Run e2e tests in headed mode (visible browser)
e2e-headed: e2e-deps
    cd e2e && npm run test:headed

# Run e2e tests in debug mode
e2e-debug: e2e-deps
    cd e2e && npm run test:debug

# Run e2e tests with UI
e2e-ui: e2e-deps
    cd e2e && npm run test:ui
