#!/usr/bin/env bash

set -euo pipefail

OFFLINE_DURATION="${OFFLINE_DURATION:-30}" # durata down registry-3
INSTANCE_COUNT="${INSTANCE_COUNT:-100}" # numero di istanze da creare
RECOVERY_TIMEOUT="${RECOVERY_TIMEOUT:-60}" # tempo massimo di convergenza registry-3

COMPOSE=(
  docker compose
  -p sdcc-integration
)

cleanup() {
  echo "Pulizia dell'ambiente di test..."
  "${COMPOSE[@]}" --profile recovery-test \
    down -v --remove-orphans >/dev/null 2>&1 || true
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

wait_instance_count() {
  local url="$1"
  local expected="$2"
  local timeout="$3"
  local description="$4"

  for ((attempt = 0; attempt < timeout; attempt++)); do
    local body
    local count
    body="$(curl -fsS "$url" 2>/dev/null || true)"
    count="$(awk -F'\"id\":' '{print NF - 1}' <<<"$body")"

    if [[ "$count" -eq "$expected" ]]; then
      echo "OK: $description"
      return 0
    fi

    sleep 1
  done

  fail "$description (attese $expected istanze)"
}

[[ "$OFFLINE_DURATION" =~ ^[0-9]+$ ]] ||
  fail "OFFLINE_DURATION deve essere un intero non negativo"
[[ "$INSTANCE_COUNT" =~ ^[0-9]+$ ]] && ((INSTANCE_COUNT >= 2)) ||
  fail "INSTANCE_COUNT deve essere un intero maggiore o uguale a 2"
[[ "$RECOVERY_TIMEOUT" =~ ^[0-9]+$ ]] && ((RECOVERY_TIMEOUT >= 1)) ||
  fail "RECOVERY_TIMEOUT deve essere un intero maggiore o uguale a 1"

trap cleanup EXIT INT TERM

docker info >/dev/null 2>&1 ||
  fail "Docker daemon non disponibile"

echo "Preparazione dell'ambiente isolato..."
"${COMPOSE[@]}" --profile recovery-test \
  down -v --remove-orphans >/dev/null 2>&1 || true

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

echo "Avvio del test di recupero dopo inattività prolungata..."
echo "Arresto di registry-3..."
"${COMPOSE[@]}" stop registry-3 >/dev/null

echo "Avvio di registry-4..."
"${COMPOSE[@]}" up -d registry-4 >/dev/null

wait_contains \
  "http://localhost:8083/health" \
  '"status":"ok"' \
  "registry-4 è attivo"

echo "Creazione di $INSTANCE_COUNT istanze tramite registry-4..."
for ((index = 1; index <= INSTANCE_COUNT; index++)); do
  curl -fsS -X PUT \
    "http://localhost:8083/services/recovery-load/instances/load-$index" \
    -H "Content-Type: application/json" \
    -d "{\"address\":\"recovery-service-$index\",\"port\":9000}" \
    >/dev/null
done

echo "Aggiornamento della prima istanza..."
curl -fsS -X PUT \
  "http://localhost:8083/services/recovery-load/instances/load-1" \
  -H "Content-Type: application/json" \
  -d '{"address":"recovery-service-updated","port":9100}' \
  >/dev/null

echo "Eliminazione dell'ultima istanza..."
curl -fsS -X DELETE \
  "http://localhost:8083/services/recovery-load/instances/load-$INSTANCE_COUNT" \
  >/dev/null

expected_active=$((INSTANCE_COUNT - 1))

wait_instance_count \
  "http://localhost:8080/services/recovery-load" \
  "$expected_active" \
  "$RECOVERY_TIMEOUT" \
  "registry-1 riceve tutte le modifiche da registry-4"

wait_instance_count \
  "http://localhost:8081/services/recovery-load" \
  "$expected_active" \
  "$RECOVERY_TIMEOUT" \
  "registry-2 riceve tutte le modifiche da registry-4"

wait_contains \
  "http://localhost:8080/services/recovery-load" \
  '"address":"recovery-service-updated"' \
  "aggiornamento propagato a registry-1"

echo "Registry-3 rimane inattivo per altri $OFFLINE_DURATION secondi..."
sleep "$OFFLINE_DURATION"

echo "Riavvio di registry-3 e misurazione della convergenza..."
recovery_started=$SECONDS
"${COMPOSE[@]}" start registry-3 >/dev/null

wait_instance_count \
  "http://localhost:8082/services/recovery-load" \
  "$expected_active" \
  "$RECOVERY_TIMEOUT" \
  "registry-3 recupera tutte le istanze tramite anti-entropy"

wait_contains \
  "http://localhost:8082/services/recovery-load" \
  '"address":"recovery-service-updated"' \
  "registry-3 recupera l'aggiornamento"

wait_contains \
  "http://localhost:8082/internal/state" \
  "\"id\":\"load-$INSTANCE_COUNT\"" \
  "registry-3 recupera il tombstone dell'istanza eliminata"

recovery_elapsed=$((SECONDS - recovery_started))
echo "Tempo di convergenza di registry-3: ${recovery_elapsed}s"
echo "Istanze create durante l'inattività: $INSTANCE_COUNT"
echo "Istanze attive recuperate: $expected_active"

echo "Arresto di registry-4..."
"${COMPOSE[@]}" stop registry-4 >/dev/null

echo "Arresto e ricreazione dei container senza eliminare i volumi..."
"${COMPOSE[@]}" --profile recovery-test down >/dev/null

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

echo "Avvio del solo registry-4..."
"${COMPOSE[@]}" up -d registry-4 >/dev/null

wait_contains \
  "http://localhost:8083/health" \
  '"status":"ok"' \
  "registry-4 riparte dal proprio volume"

wait_instance_count \
  "http://localhost:8083/services/recovery-load" \
  "$expected_active" \
  "$RECOVERY_TIMEOUT" \
  "registry-4 conserva le istanze attive nel proprio volume"

wait_contains \
  "http://localhost:8083/internal/state" \
  "\"id\":\"load-$INSTANCE_COUNT\"" \
  "registry-4 conserva il tombstone nel proprio volume"

echo
echo "TUTTI I TEST DI INTEGRAZIONE SONO PASSATI"
