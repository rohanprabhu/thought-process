BINARY = thought-process
TMP_DIR = tmp
GOBIN = $(shell go env GOPATH)/bin
AIR = $(GOBIN)/air

.PHONY: build run dev clean

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY)

$(AIR):
	go install github.com/air-verse/air@latest

dev: $(AIR)
	$(AIR)

clean:
	rm -f $(BINARY)
	rm -rf $(TMP_DIR)
