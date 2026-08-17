BINARY := bin/sailpoint-mcp-server

.PHONY: build run vet test smoke clean

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY)

vet:
	go vet ./...

test:
	go test ./...

# Handshake + tools/list + one live search_identities call. Needs credentials.
smoke: build
	go run ./cmd/smoke $(BINARY)

clean:
	rm -rf bin
