.PHONY: build clean test

# Build static binary for Linux
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s -extldflags '-static'" -o sp_wrapper .

# Build for current platform
build-local:
	go build -o sp_wrapper .

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f sp_wrapper

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run
run:
	go run .
