lint:
    go vet ./...
    staticcheck ./...

test: lint
    go test ./...

build:
    go build -o hatchat ./cmd/server.go

run: build
    ./hatchat
