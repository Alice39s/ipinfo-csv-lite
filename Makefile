BINARY  := bin/ipinfo-lite
VERSION := 2.0.0
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := all

.PHONY: all build update process release mmdb xdb checksum generate fmt vet test clean

all: build
	$(BINARY) all
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

mmdb: build
	$(BINARY) mmdb

xdb: build
	$(BINARY) xdb

checksum: build
	$(BINARY) checksum

generate: build
	$(BINARY) generate

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf bin data/country_asn.csv data/country_asn.csv.gz \
		data/ipinfo-lite.csv data/ipinfo-lite.csv.gz data/ipinfo-lite.csv.xz \
		data/ipinfo-lite.csv.zst data/ipinfo-lite.mmdb \
		data/ipinfo-lite.ipv4.xdb data/ipinfo-lite.ipv6.xdb \
		data/checksums.txt data/ipinfo.version
