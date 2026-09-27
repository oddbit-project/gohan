CYCLONEDX_GOMOD_VERSION ?= v1.12.0
TRIVY_VERSION ?= 0.74.0
SBOM_FILE ?= sbom.json

.PHONY: help test examples sbom scan sbom-clean

help:
	@echo "make test       - vet, format check, race tests and examples"
	@echo "make examples   - vet and build the examples module, run the SQLite example"
	@echo "make sbom       - generate a CycloneDX SBOM ($(SBOM_FILE))"
	@echo "make scan       - Trivy scan of the SBOM and the repository (uses a local trivy, else Docker)"
	@echo "make sbom-clean - remove generated SBOM/scan files"

test:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l .; echo "gofmt: files need formatting"; exit 1)
	go test -race ./...
	$(MAKE) examples

# The examples are a separate module (examples/go.mod) so their drivers are not
# dependencies of gohan; the PostgreSQL and ClickHouse examples need a server.
examples:
	cd examples && go vet ./... && go build ./... && go run ./sqlite

sbom:
	go run github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@$(CYCLONEDX_GOMOD_VERSION) mod -licenses -json -output $(SBOM_FILE)
	@echo "SBOM written to $(SBOM_FILE)"

scan: sbom
	@if command -v trivy >/dev/null 2>&1; then \
		trivy sbom $(SBOM_FILE) --severity CRITICAL,HIGH --exit-code 1 && \
		trivy fs . --scanners vuln,secret,misconfig --severity CRITICAL,HIGH --exit-code 1; \
	else \
		docker run --rm -v "$$(pwd)":/src -w /src aquasec/trivy:$(TRIVY_VERSION) sbom $(SBOM_FILE) --severity CRITICAL,HIGH --exit-code 1 && \
		docker run --rm -v "$$(pwd)":/src -w /src aquasec/trivy:$(TRIVY_VERSION) fs . --scanners vuln,secret,misconfig --severity CRITICAL,HIGH --exit-code 1; \
	fi

sbom-clean:
	rm -f $(SBOM_FILE) trivy-results.sarif
