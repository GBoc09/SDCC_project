# Service Registry distribuito

Progetto B1 — Sistemi Distribuiti e Cloud Computing, A.A. 2025/2026.
Registro di servizi sviluppato in Go, con replica tramite gossip e anti-entropy e persistenza locale.

## Installazione e avvio

Sono necessari Docker con il plugin Compose e `curl`. Go 1.23.4 o successivo serve solo per eseguire il programma o i test Go direttamente sull’host.

Scaricare il repository, aprire un terminale nella cartella del progetto e avviare il cluster:

```sh
docker compose up --build -d
docker compose ps
curl http://localhost:8080/health
```

La risposta attesa è `{"status":"ok"}`. I tre nodi sono accessibili su `localhost:8080`, `localhost:8081` e `localhost:8082`. Ogni nodo conserva i dati in un volume Docker separato. Il quarto nodo, sulla porta `8083`, viene avviato dal test di integrazione quando necessario.

Per visualizzare i log:

```sh
docker compose logs -f
```

## Configurazione

Con Docker, modificare le variabili nella sezione `environment` di ciascun nodo in `docker-compose.yml`, quindi rieseguire `docker compose up -d`.

| Variabile | Valore nel cluster Compose | Significato |
|---|---|---|
| `NODE_ID` | `registry-1`, `registry-2`, `registry-3` | Identificativo univoco del nodo |
| `HTTP_ADDRESS` | `:8080` | Indirizzo di ascolto nel container |
| `PEERS` | URL degli altri due nodi | Peer separati da virgola; vuoto disabilita la replica |
| `PEER_TIMEOUT` | `2s` | Timeout delle richieste ai peer |
| `SYNC_INTERVAL` | `5s` | Intervallo della sincronizzazione anti-entropy |
| `STATE_FILE` | `/data/registry-state.json` | File di persistenza; vuoto disabilita il salvataggio |
| `PERSIST_INTERVAL` | `1s` | Intervallo di salvataggio |

Le durate devono essere positive, ad esempio `500ms`, `2s` o `1m`. Le porte dell’host si possono cambiare con le variabili `REGISTRY_1_PORT`, `REGISTRY_2_PORT`, `REGISTRY_3_PORT` e `REGISTRY_4_PORT`, anche tramite un file `.env` nella cartella del progetto.

Per avviare un singolo nodo senza Docker:

```sh
go run .
```

Il nodo ascolta sulla porta `8080`, senza replica né persistenza. Per configurarlo, passare le variabili d’ambiente al comando, ad esempio `STATE_FILE=./data/state.json go run .`.

## Utilizzo delle API

Registrare un’istanza (ripetere il PUT con altri valori per aggiornarla):

```sh
curl -i -X PUT http://localhost:8080/services/payments/instances/payment-1 \
  -H 'Content-Type: application/json' \
  -d '{"address":"10.0.0.1","port":9001}'
```

Cercare le istanze attive da un altro nodo; la propagazione è asincrona:

```sh
curl http://localhost:8081/services/payments
```

Eliminare l’istanza oppure l’intero servizio:

```sh
curl -i -X DELETE http://localhost:8080/services/payments/instances/payment-1
curl -i -X DELETE http://localhost:8080/services/payments
```

## Esecuzione su Amazon EC2

Preparare un’istanza Linux con Docker e il plugin Compose, trasferirvi il progetto e lanciare gli stessi comandi di avvio tramite SSH. Tutti i container vengono eseguiti sulla stessa istanza EC2.

Gli esempi con `localhost` funzionano dal terminale SSH. Per accedere dal proprio computer, sostituire `localhost` con l’indirizzo pubblico dell’istanza e consentire nel Security Group le porte TCP `8080–8082` dal proprio IP. Le API pubbliche e interne condividono le stesse porte e non hanno autenticazione: limitare l’accesso agli indirizzi necessari.

## Test

Suite Go e controllo delle data race:

```sh
go test ./...
go test -race ./...
```

Il test di integrazione richiede Docker attivo, Bash, `curl` e `jq`. Verifica propagazione, conflitti, recupero dei nodi e persistenza. Usa un progetto Compose dedicato e ne elimina container e volumi al termine. Per eseguirlo anche con il cluster principale acceso, usare porte alternative libere:

```sh
REGISTRY_1_PORT=18080 REGISTRY_2_PORT=18081 \
REGISTRY_3_PORT=18082 REGISTRY_4_PORT=18083 \
bash scripts/integration-test.sh
```

Gli esperimenti di convergenza richiedono Go e il cluster principale avviato sulle porte predefinite `8080–8082`. Eseguono 20 campioni per scenario e sospendono temporaneamente `registry-3`:

```sh
RUN_CONVERGENCE_TESTS=1 go test -count=1 -v -timeout=5m ./tests/convergence
```

Senza questa variabile gli esperimenti vengono saltati da `go test ./...`.

## Arresto e pulizia

Arrestare il cluster conservando i dati:

```sh
docker compose --profile recovery-test down
```

Per eliminare anche i dati persistiti, aggiungere `-v`:

```sh
docker compose --profile recovery-test down -v
```
