#!/usr/bin/env bash

set -euo pipefail

COMPOSE=(
  docker compose
  -p sdcc-integration
)

cleanup() {
  echo "Pulizia dell'ambiente di test..."
  "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}

fail() {
  echo "ERRORE: $1" >&2
  exit 1
}

wait_contains() {
  local url="$1"
  local expected="$2"
  local description="$3"

  for _ in {1..30}; do
    local body
    body="$(curl -fsS "$url" 2>/dev/null || true)"

    if [[ "$body" == *"$expected"* ]]; then
      echo "OK: $description"
      return 0
    fi

    sleep 1
  done

  fail "$description"
}

wait_empty_discovery() {
  local url="$1"
  local description="$2"

  for _ in {1..30}; do
    local body
    body="$(curl -fsS "$url" 2>/dev/null || true)"

    if [[ "$body" == "[]" ]]; then
      echo "OK: $description"
      return 0
    fi

    sleep 1
  done

  fail "$description"
}

trap cleanup EXIT INT TERM

docker info >/dev/null 2>&1 ||
  fail "Docker daemon non disponibile"

echo "Preparazione dell'ambiente isolato..."
"${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true

echo "Avvio del cluster..."
"${COMPOSE[@]}" up --build -d

wait_contains \
  "http://localhost:8080/health" \
  '"status":"ok"' \
  "registry-1 è attivo"

wait_contains \
  "http://localhost:8081/health" \
  '"status":"ok"' \
  "registry-2 è attivo"

wait_contains \
  "http://localhost:8082/health" \
  '"status":"ok"' \
  "registry-3 è attivo"

echo "Registrazione su registry-1..."
curl -fsS -X PUT \
  "http://localhost:8080/services/integration/instances/integration-1" \
  -H "Content-Type: application/json" \
  -d '{"address":"integration-service","port":9000}' \
  >/dev/null

wait_contains \
  "http://localhost:8081/services/integration" \
  '"id":"integration-1"' \
  "gossip da registry-1 a registry-2"

wait_contains \
  "http://localhost:8082/services/integration" \
  '"id":"integration-1"' \
  "gossip da registry-1 a registry-3"

echo "Arresto temporaneo di registry-3..."
"${COMPOSE[@]}" stop registry-3 >/dev/null

echo "Eliminazione tramite registry-2..."
curl -fsS -X DELETE \
  "http://localhost:8081/services/integration/instances/integration-1" \
  >/dev/null

wait_empty_discovery \
  "http://localhost:8080/services/integration" \
  "tombstone propagato a registry-1"

echo "Riavvio di registry-3..."
"${COMPOSE[@]}" start registry-3 >/dev/null

wait_contains \
  "http://localhost:8082/internal/state" \
  '"status":"deleted"' \
  "registry-3 recupera il tombstone tramite anti-entropy"

echo "Arresto e ricreazione dei container senza eliminare i volumi..."
"${COMPOSE[@]}" down >/dev/null

echo "Avvio del solo registry-3..."
"${COMPOSE[@]}" up -d registry-3 >/dev/null

wait_contains \
  "http://localhost:8082/health" \
  '"status":"ok"' \
  "registry-3 riparte dal proprio volume"

wait_contains \
  "http://localhost:8082/internal/state" \
  '"status":"deleted"' \
  "tombstone conservato dopo la ricreazione del container"

wait_empty_discovery \
  "http://localhost:8082/services/integration" \
  "istanza eliminata non ricompare"

echo
echo "TUTTI I TEST DI INTEGRAZIONE SONO PASSATI"