.PHONY: dev run build test clean

# Run with hot reloading using Air (auto-installs if not in PATH)
dev:
	@which air > /dev/null 2>&1 || (echo "Air not found in PATH, running via go run..." && go run github.com/air-verse/air@latest)
	@which air > /dev/null 2>&1 && air

# Run standard Go server without hot reload
run:
	go run ./cmd/server

# Build standalone production binary
build:
	go build -o server ./cmd/server

# Run test suite
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf tmp server
