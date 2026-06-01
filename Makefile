.PHONY: all build run gotool clean fresh

BINARY="ceddit"

all: gotool build

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ${BINARY}

# run: one command to start everything.
#   - Ensures docker compose infra is up (MySQL + Redis + Kafka + Canal).
#   - Grants Canal replication privileges.
#   - Builds and starts the API server + Kafka consumer in one process.
run:
	@bash scripts/setup.sh
	@echo "[run] building..."
	@go build -o ${BINARY}
	@echo "[run] starting server (HTTP + Kafka consumer)..."
	@./${BINARY}

# fresh: full teardown, clean slate.
#   Stops containers, removes volumes, rebuilds everything from scratch.
fresh:
	@echo "[fresh] stopping containers and removing volumes..."
	@docker compose down -v 2>/dev/null || true
	@echo "[fresh] starting fresh infrastructure..."
	@docker compose up -d
	@bash scripts/setup.sh
	@echo "[fresh] building..."
	@go build -o ${BINARY}
	@echo "[fresh] starting server (HTTP + Kafka consumer)..."
	@./${BINARY}

gotool:
	go fmt ./...
	go vet ./...

clean:
	@if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi
