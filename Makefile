BINARY     := sirtopham
BIN_DIR    := bin
CMD_PKG    := ./cmd/sirtopham
FRONTEND   := frontend

.PHONY: build test dev-backend dev-frontend dev frontend-deps frontend-build clean

build: frontend-build
	CGO_ENABLED=1 go build -o $(BIN_DIR)/$(BINARY) $(CMD_PKG)

test:
	CGO_ENABLED=1 go test ./...

dev-backend:
	CGO_ENABLED=1 go run $(CMD_PKG) serve --dev

dev-frontend:
	@echo "Frontend not yet implemented"

dev: dev-backend

frontend-deps:
	@echo "Frontend not yet implemented"

frontend-build:
	@echo "Frontend not yet implemented — skipping"

clean:
	rm -rf $(BIN_DIR)
	rm -rf $(FRONTEND)/dist
