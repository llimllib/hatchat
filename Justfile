run: build
    ./hatchat

lint:
    golangci-lint run & (cd client && npx eslint src)

test: lint
    go test ./...

models:
    bash models.sh
build-js:
    cd client && npx tsc --noEmit && node esbuild.config.mjs

build-go:
    go build -o hatchat ./cmd/server.go

build:
    (cd client && node esbuild.config.js) & go build -o hatchat ./cmd/server.go
