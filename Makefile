.PHONY: run build test vet tidy clean

# Run the local web server (defaults: PORT=8080, LLM_PROVIDER=mock).
run:
	go run .

# Build the binary into ./bin/repoweaver.
build:
	go build -o bin/repoweaver .

# Run the full test suite (uses the keyless mock LLM provider).
test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f bin/repoweaver repoweaver.db
