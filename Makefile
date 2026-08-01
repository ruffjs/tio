.DEFAULT_GOAL := help

APP := tio
VERSION ?= dev
GIT_COMMIT := $(shell git rev-parse --short HEAD)

.PHONY: help run build test test-race fmt fix mod-tidy mod-check vet lint check
.PHONY: test-integration test-protocols test-mtls
.PHONY: preflight-demo-light test-demo-light preflight-demo-mtls test-demo-mtls
.PHONY: web-install web-build web-dev docker-build
.PHONY: nats-cluster-up nats-cluster-down

help:
	@echo "Common targets:"
	@echo "  make run               Start Tio"
	@echo "  make build             Build Tio into dist/"
	@echo "  make test              Run normal tests"
	@echo "  make test-race         Run normal tests with the race detector"
	@echo "  make fix               Apply standard Go source fixes"
	@echo "  make lint              Run non-mutating Go checks"
	@echo "  make check             Run lint and normal tests"
	@echo "  make test-integration  Run legacy/JSON integration tests"
	@echo "  make test-protocols    Test all protocol/encoding combinations"
	@echo "  make test-mtls         Run MQTT mTLS integration tests"
	@echo "  make test-demo-light   Run the external light demo test"
	@echo "                         Requires legacy/JSON Tio on API 9000 and MQTT 1883"
	@echo "  make test-demo-mtls    Run the external mTLS demo test"
	@echo "                         Requires Tio on API 9000 with MQTT TLS on 8883"
	@echo "  make web-build         Build the web UI"
	@echo "  make docker-build      Build the Docker image"

run:
	go run cmd/tio/main.go

build:
	mkdir -p dist
	CGO_ENABLED=1 go build \
		-ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT)" \
		-o dist/$(APP) \
		cmd/tio/main.go

test:
	go test $$(go list ./... | grep -v '/demos/')

test-race:
	go test -race $$(go list ./... | grep -v '/demos/')

fmt:
	go fmt ./...

fix:
	go fix ./...

mod-tidy:
	go mod tidy

mod-check:
	go mod tidy -diff

vet:
	go vet ./...

lint: vet mod-check

check: lint test

test-integration:
	TIO_TEST_PROTOCOL=legacy TIO_TEST_ENCODING=json \
		go test -tags=integration ./integration_tests -count=1

test-protocols:
	TIO_TEST_PROTOCOL=legacy TIO_TEST_ENCODING=json \
		go test -tags=integration ./integration_tests -run TestLegacy -count=1
	TIO_TEST_PROTOCOL=legacy TIO_TEST_ENCODING=cbor \
		go test -tags=integration ./integration_tests -run TestLegacy -count=1
	TIO_TEST_PROTOCOL=simple TIO_TEST_ENCODING=json \
		go test -tags=integration ./integration_tests -run TestSimple -count=1
	TIO_TEST_PROTOCOL=simple TIO_TEST_ENCODING=cbor \
		go test -tags=integration ./integration_tests -run TestSimple -count=1

test-mtls:
	TIO_TEST_MQTT_TLS=1 TIO_TEST_PROTOCOL=legacy TIO_TEST_ENCODING=json \
		go test -tags=integration ./integration_tests \
		-run 'TestThingConnectWithCertificateAndPasswordCoexist|TestThingRejectsInvalidAuthenticationPaths|TestThingRejectsInvalidClientCertificates|TestDeviceRejectsInvalidServerCertificate|TestCertificateConnectionUsesAuthenticatedThingIDForPresenceAndACL' \
		-count=1

preflight-demo-light:
	@command -v curl >/dev/null || { echo "error: curl is required"; exit 1; }
	@command -v nc >/dev/null || { echo "error: nc is required"; exit 1; }
	@curl --fail --silent --show-error --max-time 3 \
		--user admin:public 'http://127.0.0.1:9000/api/v1/things/?pageSize=1' >/dev/null || { \
		echo "error: Tio API is not ready at http://127.0.0.1:9000 with admin/public"; \
		echo "hint: start Tio with protocol.mode=legacy and protocol.encoding=json"; \
		exit 1; \
	}
	@nc -z -w 3 127.0.0.1 1883 || { \
		echo "error: plaintext MQTT is not ready at 127.0.0.1:1883"; \
		echo "hint: start Tio with the default legacy/JSON configuration"; \
		exit 1; \
	}
	@echo "Light demo prerequisites passed; Tio must be using legacy/JSON."

test-demo-light: preflight-demo-light
	go test -tags=demo ./demos/light -count=1

preflight-demo-mtls:
	@command -v curl >/dev/null || { echo "error: curl is required"; exit 1; }
	@command -v nc >/dev/null || { echo "error: nc is required"; exit 1; }
	@test -f demos/mtls/certs/ca.pem \
		-a -f demos/mtls/certs/client-cert.pem \
		-a -f demos/mtls/certs/client-key.pem || { \
		echo "error: mTLS demo certificates are missing"; \
		echo "hint: run demos/mtls/certs/generate-certs.sh"; \
		exit 1; \
	}
	@curl --fail --silent --show-error --max-time 3 \
		--user admin:public 'http://127.0.0.1:9000/api/v1/things/?pageSize=1' >/dev/null || { \
		echo "error: Tio API is not ready at http://127.0.0.1:9000 with admin/public"; \
		echo "hint: start Tio before running the mTLS demo test"; \
		exit 1; \
	}
	@nc -z -w 3 localhost 8883 || { \
		echo "error: MQTT TLS port is not ready at localhost:8883"; \
		echo "hint: config.yaml must enable mqttTls and mqttPort 8883 before make run"; \
		exit 1; \
	}
	@echo "mTLS demo prerequisites passed; certificate authentication will be verified by the test."

test-demo-mtls: preflight-demo-mtls
	go test ./demos/mtls/device -count=1

web-install:
	cd web && yarn

web-build:
	cd web && yarn build

web-dev:
	cd web && yarn dev

docker-build:
	./build/docker/build.sh

nats-cluster-up:
	docker compose -f deployments/nats-cluster/docker-compose.yml up -d

nats-cluster-down:
	docker compose -f deployments/nats-cluster/docker-compose.yml down
