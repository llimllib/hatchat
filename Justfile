lint:
    go vet ./...
    staticcheck ./...

build:
    go build -o tinychat ./cmd/server.go

run: build
    ./tinychat
