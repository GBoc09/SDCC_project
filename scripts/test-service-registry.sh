#!/usr/bin/env bash

set -euo pipefail

OFFLINE_DURATION="${OFFLINE_DURATION:-30}" # durata down registry-3
INSTANCE_COUNT="${INSTANCE_COUNT:-100}" # numero di istanze da creare
RECOVERY_TIMEOUT="${RECOVERY_TIMEOUT:-60}" # tempo massimo di convergenza registry-3

cd "$(dirname "${BASH_SOURCE[0]}")/.."

export REGISTRY_1_PORT="${REGISTRY_1_PORT:-8080}"
export REGISTRY_2_PORT="${REGISTRY_2_PORT:-8081}"
export REGISTRY_3_PORT="${REGISTRY_3_PORT:-8082}"
export REGISTRY_4_PORT="${REGISTRY_4_PORT:-8083}"

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

# Confronta tutti i campi, inclusi versioni, origini, timestamp e tombstone.
fetch_state() {
  curl --connect-timeout 2 --max-time 5 -fsS "$1/internal/state" |
    jq -ceS '
      if (.services | type) != "array" or (.instances | type) != "array"
      then error("stato non valido")
      else .services |= sort_by(.name) |
           .instances |= sort_by(.serviceName, .id)
      end'
}

wait_state_equals() {
  local expected="$1" description="$2"
  shift 2
  local attempt url actual matched
  for ((attempt = 0; attempt < RECOVERY_TIMEOUT; attempt++)); do
    matched=true
    for url in "$@"; do
      if ! actual="$(fetch_state "$url" 2>/dev/null)" || [[ "$actual" != "$expected" ]]; then
        matched=false
      fi
    done
    if [[ "$matched" == true ]]; then
      echo "OK: $description"
      return 0
    fi
    sleep 1
  done
  echo "Stato atteso: $expected" >&2
  for url in "$@"; do
    echo "$url: $(fetch_state "$url" 2>/dev/null || true)" >&2
  done
  fail "$description"
}

run_conflict() {
  local scenario="$1" pid1 pid2 left right expected
  echo "Conflitto controllato: $scenario..."
  # I volumi sono conservati. Nessun nodo può replicare durante le scritture.
  "${COMPOSE[@]}" -f docker-compose.yml -f scripts/compose.integration-isolated.yml \
    up -d --force-recreate registry-1 registry-2 registry-3 >/dev/null
  for port in "$REGISTRY_1_PORT" "$REGISTRY_2_PORT" "$REGISTRY_3_PORT"; do
    wait_contains "http://localhost:$port/health" '"status":"ok"' "nodo $port isolato pronto"
  done

  curl --max-time 10 -fsS -X PUT \
    "http://localhost:${REGISTRY_1_PORT}/services/conflict/instances/shared" \
    -H "Content-Type: application/json" \
    -d '{"address":"writer-1","port":9101}' >/dev/null &
  pid1=$!
  if [[ "$scenario" == update-update ]]; then
    curl --max-time 10 -fsS -X PUT \
      "http://localhost:${REGISTRY_2_PORT}/services/conflict/instances/shared" \
      -H "Content-Type: application/json" \
      -d '{"address":"writer-2","port":9102}' >/dev/null &
  else
    curl --max-time 10 -fsS -X DELETE \
      "http://localhost:${REGISTRY_2_PORT}/services/conflict/instances/shared" >/dev/null &
  fi
  pid2=$!
  local writes_ok=true
  wait "$pid1" || writes_ok=false
  wait "$pid2" || writes_ok=false
  [[ "$writes_ok" == true ]] || fail "scritture concorrenti fallite"

  left="$(fetch_state http://localhost:${REGISTRY_1_PORT} | jq -ce '.instances[] | select(.serviceName == "conflict" and .id == "shared")')"
  right="$(fetch_state http://localhost:${REGISTRY_2_PORT} | jq -ce '.instances[] | select(.serviceName == "conflict" and .id == "shared")')"
  jq -en --argjson left "$left" --argjson right "$right" --arg scenario "$scenario" '
    $left.version == $right.version and
    $left.originNode == "registry-1" and $right.originNode == "registry-2" and
    $left.status == "active" and $left.address == "writer-1" and $left.port == 9101 and
    (if $scenario == "update-update"
     then $right.status == "active" and $right.address == "writer-2" and $right.port == 9102
     else $right.status == "deleted" end)
  ' >/dev/null || fail "conflitto non riprodotto: versioni, origini o valori inattesi"

  # A parità di versione prevale l'origine lessicograficamente maggiore.
  expected="$(fetch_state http://localhost:${REGISTRY_2_PORT})"
  "${COMPOSE[@]}" up -d --force-recreate registry-1 registry-2 registry-3 >/dev/null
  wait_state_equals "$expected" "convergenza completa $scenario: prevale registry-2" \
    http://localhost:${REGISTRY_1_PORT} http://localhost:${REGISTRY_2_PORT} http://localhost:${REGISTRY_3_PORT}
  if [[ "$scenario" == update-delete ]]; then
    for port in "$REGISTRY_1_PORT" "$REGISTRY_2_PORT" "$REGISTRY_3_PORT"; do
      wait_empty_discovery "http://localhost:$port/services/conflict" "nessuna resurrezione su $port"
    done
  fi
}

command -v jq >/dev/null || fail "jq non disponibile: installarlo per confrontare gli stati JSON"

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
  "http://localhost:${REGISTRY_1_PORT}/health" \
  '"status":"ok"' \
  "registry-1 è attivo"

wait_contains \
  "http://localhost:${REGISTRY_2_PORT}/health" \
  '"status":"ok"' \
  "registry-2 è attivo"

wait_contains \
  "http://localhost:${REGISTRY_3_PORT}/health" \
  '"status":"ok"' \
  "registry-3 è attivo"

echo "Registrazione su registry-1..."
curl -fsS -X PUT \
  "http://localhost:${REGISTRY_1_PORT}/services/payment/instances/payment-1" \
  -H "Content-Type: application/json" \
  -d '{"address":"payment-service","port":9000}' \
  >/dev/null

wait_contains \
  "http://localhost:${REGISTRY_2_PORT}/services/payment" \
  '"id":"payment-1"' \
  "gossip da registry-1 a registry-2"

wait_contains \
  "http://localhost:${REGISTRY_3_PORT}/services/payment" \
  '"id":"payment-1"' \
  "gossip da registry-1 a registry-3"

echo "Arresto temporaneo di registry-3..."
"${COMPOSE[@]}" stop registry-3 >/dev/null

echo "Eliminazione tramite registry-2..."
curl -fsS -X DELETE \
  "http://localhost:${REGISTRY_2_PORT}/services/payment/instances/payment-1" \
  >/dev/null

wait_empty_discovery \
  "http://localhost:${REGISTRY_1_PORT}/services/payment" \
  "tombstone propagato a registry-1"

echo "Riavvio di registry-3..."
"${COMPOSE[@]}" start registry-3 >/dev/null

wait_contains \
  "http://localhost:${REGISTRY_3_PORT}/internal/state" \
  '"status":"deleted"' \
  "registry-3 recupera il tombstone tramite anti-entropy"

baseline="$(fetch_state http://localhost:${REGISTRY_1_PORT})"
wait_state_equals "$baseline" "stati completi uguali dopo il recupero del tombstone" \
  http://localhost:${REGISTRY_1_PORT} http://localhost:${REGISTRY_2_PORT} http://localhost:${REGISTRY_3_PORT}

curl -fsS -X PUT "http://localhost:${REGISTRY_1_PORT}/services/conflict/instances/shared" \
  -H "Content-Type: application/json" -d '{"address":"initial","port":9000}' >/dev/null
baseline="$(fetch_state http://localhost:${REGISTRY_1_PORT})"
wait_state_equals "$baseline" "stato iniziale comune per i conflitti" \
  http://localhost:${REGISTRY_1_PORT} http://localhost:${REGISTRY_2_PORT} http://localhost:${REGISTRY_3_PORT}
run_conflict update-update
run_conflict update-delete

echo "Avvio del test di recupero dopo inattività prolungata..."
echo "Arresto di registry-3..."
"${COMPOSE[@]}" stop registry-3 >/dev/null

echo "Avvio di registry-4..."
"${COMPOSE[@]}" up -d registry-4 >/dev/null

wait_contains \
  "http://localhost:${REGISTRY_4_PORT}/health" \
  '"status":"ok"' \
  "registry-4 è attivo"

echo "Creazione di $INSTANCE_COUNT istanze tramite registry-4..."
for ((index = 1; index <= INSTANCE_COUNT; index++)); do
  curl -fsS -X PUT \
    "http://localhost:${REGISTRY_4_PORT}/services/recovery-load/instances/load-$index" \
    -H "Content-Type: application/json" \
    -d "{\"address\":\"recovery-service-$index\",\"port\":9000}" \
    >/dev/null
done

echo "Aggiornamento della prima istanza..."
curl -fsS -X PUT \
  "http://localhost:${REGISTRY_4_PORT}/services/recovery-load/instances/load-1" \
  -H "Content-Type: application/json" \
  -d '{"address":"recovery-service-updated","port":9100}' \
  >/dev/null

echo "Eliminazione dell'ultima istanza..."
curl -fsS -X DELETE \
  "http://localhost:${REGISTRY_4_PORT}/services/recovery-load/instances/load-$INSTANCE_COUNT" \
  >/dev/null

expected_active=$((INSTANCE_COUNT - 1))

wait_instance_count \
  "http://localhost:${REGISTRY_1_PORT}/services/recovery-load" \
  "$expected_active" \
  "$RECOVERY_TIMEOUT" \
  "registry-1 riceve tutte le modifiche da registry-4"

wait_instance_count \
  "http://localhost:${REGISTRY_2_PORT}/services/recovery-load" \
  "$expected_active" \
  "$RECOVERY_TIMEOUT" \
  "registry-2 riceve tutte le modifiche da registry-4"

wait_contains \
  "http://localhost:${REGISTRY_1_PORT}/services/recovery-load" \
  '"address":"recovery-service-updated"' \
  "aggiornamento propagato a registry-1"

echo "Registry-3 rimane inattivo per altri $OFFLINE_DURATION secondi..."
sleep "$OFFLINE_DURATION"

echo "Riavvio di registry-3 e misurazione della convergenza..."
recovery_started=$SECONDS
"${COMPOSE[@]}" start registry-3 >/dev/null

wait_instance_count \
  "http://localhost:${REGISTRY_3_PORT}/services/recovery-load" \
  "$expected_active" \
  "$RECOVERY_TIMEOUT" \
  "registry-3 recupera tutte le istanze tramite anti-entropy"

wait_contains \
  "http://localhost:${REGISTRY_3_PORT}/services/recovery-load" \
  '"address":"recovery-service-updated"' \
  "registry-3 recupera l'aggiornamento"

wait_contains \
  "http://localhost:${REGISTRY_3_PORT}/internal/state" \
  "\"id\":\"load-$INSTANCE_COUNT\"" \
  "registry-3 recupera il tombstone dell'istanza eliminata"

recovery_elapsed=$((SECONDS - recovery_started))
echo "Tempo di convergenza di registry-3: ${recovery_elapsed}s"
echo "Istanze create durante l'inattività: $INSTANCE_COUNT"
echo "Istanze attive recuperate: $expected_active"

recovered_state="$(fetch_state http://localhost:${REGISTRY_4_PORT})"
wait_state_equals "$recovered_state" "stati completi uguali sui quattro nodi dopo il recovery" \
  http://localhost:${REGISTRY_1_PORT} http://localhost:${REGISTRY_2_PORT} http://localhost:${REGISTRY_3_PORT} http://localhost:${REGISTRY_4_PORT}

echo "Arresto di registry-4..."
"${COMPOSE[@]}" stop registry-4 >/dev/null

echo "Arresto e ricreazione dei container senza eliminare i volumi..."
"${COMPOSE[@]}" --profile recovery-test down >/dev/null

echo "Avvio del solo registry-3..."
"${COMPOSE[@]}" up -d registry-3 >/dev/null

wait_contains \
  "http://localhost:${REGISTRY_3_PORT}/health" \
  '"status":"ok"' \
  "registry-3 riparte dal proprio volume"

wait_contains \
  "http://localhost:${REGISTRY_3_PORT}/internal/state" \
  '"status":"deleted"' \
  "tombstone conservato dopo la ricreazione del container"

wait_empty_discovery \
  "http://localhost:${REGISTRY_3_PORT}/services/payment" \
  "istanza eliminata non ricompare"

wait_state_equals "$recovered_state" "stato completo di registry-3 conservato nel volume" \
  http://localhost:${REGISTRY_3_PORT}
"${COMPOSE[@]}" stop registry-3 >/dev/null

echo "Avvio del solo registry-4..."
"${COMPOSE[@]}" up -d registry-4 >/dev/null

wait_contains \
  "http://localhost:${REGISTRY_4_PORT}/health" \
  '"status":"ok"' \
  "registry-4 riparte dal proprio volume"

wait_instance_count \
  "http://localhost:${REGISTRY_4_PORT}/services/recovery-load" \
  "$expected_active" \
  "$RECOVERY_TIMEOUT" \
  "registry-4 conserva le istanze attive nel proprio volume"

wait_contains \
  "http://localhost:${REGISTRY_4_PORT}/internal/state" \
  "\"id\":\"load-$INSTANCE_COUNT\"" \
  "registry-4 conserva il tombstone nel proprio volume"

echo
wait_state_equals "$recovered_state" "stato completo di registry-4 conservato nel volume" \
  http://localhost:${REGISTRY_4_PORT}

echo "TUTTI I TEST DI INTEGRAZIONE SONO PASSATI"
