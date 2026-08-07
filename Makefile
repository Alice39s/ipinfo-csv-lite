BINARY  := bin/ipinfo-lite
VERSION := 2.0.0
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := all

.PHONY: all build update process release fmt vet clean

all: update process release
	@echo "All steps executed successfully."

build:
	@mkdir -p bin
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) .

update: build
	$(BINARY) update

process: build
	$(BINARY) process

release: build
	$(BINARY) release

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -rf bin data/country_asn.csv data/country_asn.csv.gz \
		data/ipinfo-lite.csv data/ipinfo-lite.csv.gz data/ipinfo-lite.csv.xz \
		data/ipinfo.version
