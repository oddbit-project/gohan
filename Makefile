CYCLONEDX_GOMOD_VERSION ?= v1.12.0
TRIVY_VERSION ?= 0.74.0
SBOM_FILE ?= sbom.json
FUZZTIME ?= 30s
FUZZ_TARGETS := FuzzIdentRoundTrip FuzzValueNeverInlined FuzzRawNoOrdinal

.PHONY: help test examples integration fuzz sbom scan sbom-clean

help:
	@echo "make test        - vet, format check, race tests and examples"
	@echo "make examples    - vet and build the examples module, run the SQLite example"
	@echo "make integration - run the integration module against real PostgreSQL and ClickHouse"
	@echo "make fuzz        - run each fuzz target for $(FUZZTIME) (override with FUZZTIME=5m)"
	@echo "make sbom        - generate a CycloneDX SBOM ($(SBOM_FILE))"
	@echo "make scan        - Trivy scan of the SBOM and the repository (uses a local trivy, else Docker)"
	@echo "make sbom-clean  - remove generated SBOM/scan files"

test:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l .; echo "gofmt: files need formatting"; exit 1)
	go test -race ./...
	$(MAKE) examples

# The examples are a separate module (examples/go.mod) so their drivers are not
# dependencies of gohan; the PostgreSQL and ClickHouse examples need a server.
examples:
	cd examples && go vet ./... && go build ./... && go run ./sqlite

# integration runs the integration module (integration/go.mod) against real
# PostgreSQL and ClickHouse servers started in Docker; SQLite runs in-memory.
# The containers are stopped even if the tests fail.
integration:
	docker run -d --rm --name gohan-pg -p 55432:5432 \
		-e POSTGRES_USER=gohan -e POSTGRES_PASSWORD=gohan -e POSTGRES_DB=gohan \
		postgres:17
	docker run -d --rm --name gohan-ch -p 59000:9000 -p 58123:8123 \
		-e CLICKHOUSE_USER=gohan -e CLICKHOUSE_PASSWORD=gohan -e CLICKHOUSE_DB=gohan \
		clickhouse/clickhouse-server:26.7
	@until docker exec gohan-pg pg_isready -U gohan >/dev/null 2>&1; do sleep 1; done
	@until curl -sf http://localhost:58123/ping >/dev/null 2>&1; do sleep 1; done
	cd integration && GOHAN_PG_DSN='postgres://gohan:gohan@localhost:55432/gohan?sslmode=disable' \
		GOHAN_CH_DSN='clickhouse://gohan:gohan@localhost:59000/gohan' \
		GOHAN_REQUIRE_ENGINES=postgres,sqlite,clickhouse \
		go test -race -count=1 ./... ; \
		status=$$?; \
		docker stop gohan-pg gohan-ch >/dev/null; \
		exit $$status

fuzz:
	@for t in $(FUZZ_TARGETS); do \
		go test -run '^$$' -fuzz "^$$t$$" -fuzztime $(FUZZTIME) . || exit 1; \
	done

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
