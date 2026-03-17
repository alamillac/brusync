APP_NAME := brusync
BUILD_DIR := bin
BIN := $(BUILD_DIR)/$(APP_NAME)
GO := go
MAIN := .

.PHONY: help fmt tidy test build run clean install

help:
	@printf "Targets disponibles:\n"
	@printf "  make fmt      - Formatea el codigo Go\n"
	@printf "  make tidy     - Ordena dependencias (go mod tidy)\n"
	@printf "  make test     - Ejecuta tests\n"
	@printf "  make build    - Compila el binario en $(BIN)\n"
	@printf "  make run      - Ejecuta la app (usa ARGS='...')\n"
	@printf "  make install  - Instala binario en GOPATH/bin\n"
	@printf "  make clean    - Borra artefactos de build\n"

format:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./...

build:
	mkdir -p "$(BUILD_DIR)"
	$(GO) build -o "$(BIN)" $(MAIN)

run:
	$(GO) run $(MAIN) $(ARGS)

install:
	$(GO) install $(MAIN)

clean:
	rm -rf "$(BUILD_DIR)"
