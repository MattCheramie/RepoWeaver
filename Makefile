.PHONY: run build desktop dist test vet tidy clean

# Run the local web server (defaults: PORT=8080, LLM_PROVIDER=mock).
run:
	go run .

# Build the binary into ./bin/repoweaver.
build:
	go build -o bin/repoweaver .

# Build the native desktop app (requires CGO + a system webview: on Linux
# install libwebkit2gtk-4.1-dev/4.0 and GTK). Produces ./bin/repoweaver-desktop.
desktop:
	CGO_ENABLED=1 go build -tags desktop -o bin/repoweaver-desktop .

# Build cross-platform release archives into ./dist (pure-Go, no CGO).
# Override the version: make dist VERSION=v1.2.3
dist:
	scripts/dist.sh $(VERSION)

# Run the full test suite (uses the keyless mock LLM provider).
test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin dist
	rm -f repoweaver.db
