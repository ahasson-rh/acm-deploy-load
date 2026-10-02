.PHONY: build test clean install lint

GO := go
BINARY := workload-image-curator
INSTALL_DIR := ..

build:
	$(GO) build -o $(BINARY) .

test:
	$(GO) test -v -race ./...

test-unit:
	$(GO) test -v -race ./models ./pyxis ./registry ./output ./config

clean:
	rm -f $(BINARY)
	$(GO) clean

install: build
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	chmod +x $(INSTALL_DIR)/$(BINARY)

lint:
	$(GO) fmt ./...
	$(GO) vet ./...

dev: build
	./$(BINARY) --help
