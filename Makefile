.PHONY: all build test web-build run-redact run-scan run-certify clean

BINARY_NAME=videoblur
BIN_DIR=bin

all: test build

build: web-build
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/videoblur

test:
	go test -v ./...

web-build:
	cd web && npm install && npm run build
	@mkdir -p internal/server/dist
	cp -r web/dist/* internal/server/dist/

run-scan: build
	./$(BIN_DIR)/$(BINARY_NAME) scan -i examples/sample_media/bodycam_traffic_stop.mp4

run-redact: build
	./$(BIN_DIR)/$(BINARY_NAME) redact \
		-i examples/sample_media/bodycam_traffic_stop.mp4 \
		-o examples/sample_media/redacted_bodycam.mp4 \
		-a examples/sample_media/audit_bodycam.json \
		--mode gaussian \
		--standard CJIS

run-certify: build
	./$(BIN_DIR)/$(BINARY_NAME) certify -m examples/sample_media/audit_bodycam.json

clean:
	rm -rf $(BIN_DIR) internal/server/dist web/dist
