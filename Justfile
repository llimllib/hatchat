lint:
    go vet ./...
    golangci-lint run

test: lint
    go test ./...

models:
    rm -f xo.db && \
        sqlite3 'xo.db' < schema.sql && \
        xo -v schema sqlite://xo.db -o server/xomodels && \
        rm xo.db

build:
    go build -o hatchat ./cmd/server.go

run: build
    ./hatchat
