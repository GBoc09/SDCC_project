# SDCC Project — Distributed Service Registry

Progetto B1 del corso di Sistemi Distribuiti e Cloud Computing, A.A. 2025/2026.

Il progetto implementa in Go un Service Registry decentralizzato, replicato e tollerante all'indisponibilità temporanea dei nodi. Ogni nodo mantiene una copia del registro, accetta richieste client e converge con i peer tramite gossip immediato e sincronizzazione anti-entropy periodica.

## Funzionalità

- registrazione e aggiornamento dinamico delle istanze;
- discovery delle sole istanze attive;
- rimozione di una singola istanza o di un intero servizio;
- tombstone per impedire la ricomparsa di dati obsoleti;
- registry thread-safe in memoria;
- replica bidirezionale tra più nodi;
- gossip push immediato dopo le modifiche locali;
- anti-entropy pull periodica per recuperare aggiornamenti persi;
- risoluzione deterministica dei conflitti;
- persistenza atomica su file JSON;
- arresto controllato tramite `SIGINT` e `SIGTERM`;
- esecuzione di un cluster a tre nodi tramite Docker Compose;
- test unitari, race detector e test d'integrazione automatico.

## Architettura

Ogni nodo espone le API del registry e mantiene una copia completa dei record.

```text
                       gossip push
              +--------------------------+
              |                          v
Client ---> Registry 1 <-----------> Registry 2
              ^       anti-entropy       |
              |                          |
              +-------- Registry 3 <-----+

Ogni nodo:
  - conserva servizi, istanze e tombstone;
  - salva periodicamente uno snapshot su disco;
  - continua a servire richieste se un peer non è disponibile.
```

Il gossip riduce il tempo necessario per propagare una modifica. L'anti-entropy rimane attiva come meccanismo di recupero: quando un nodo torna disponibile, scarica gli snapshot dei peer e applica solamente i record più recenti.

Gli snapshot completi vengono scambiati attraverso gli endpoint interni `GET /internal/state` e `PUT /internal/state`.

## Modello dei dati

Il sistema utilizza due tipi di record:

1. `ServiceRecord`, identificato dal nome del servizio;
2. `InstanceRecord`, identificato dalla coppia `(serviceName, ID)`.

Entrambi contengono:

- stato `active` o `deleted`;
- versione logica;
- identificativo del nodo che ha prodotto l'ultima modifica;
- data dell'ultimo aggiornamento.

Le istanze contengono inoltre indirizzo e porta dell'endpoint.

### Risoluzione dei conflitti

La precedenza tra due copie dello stesso record viene determinata dalla coppia:

```text
(version, originNode)
```

Le regole sono:

1. vince la versione maggiore;
2. a parità di versione, vince l'`originNode` maggiore in ordine lessicografico;
3. se entrambi coincidono, il merge è idempotente e il record viene ignorato.

Ogni modifica locale, incluse le cancellazioni, incrementa la versione logica. Quando un nodo riceve una versione remota più alta, fa avanzare anche il proprio contatore locale.

### Tombstone

Una cancellazione non rimuove fisicamente il record, ma imposta lo stato a `deleted` e incrementa la versione. Il tombstone viene replicato e impedisce a un nodo rimasto offline di reintrodurre una vecchia copia attiva.

La cancellazione di un'istanza non elimina automaticamente il servizio. La cancellazione dell'intero servizio genera invece il tombstone del servizio e rende eliminate le sue istanze.

## API HTTP

### API pubbliche

| Metodo | Percorso | Descrizione |
|---|---|---|
| `GET` | `/health` | Stato del nodo |
| `PUT` | `/services/{name}/instances/{id}` | Registra o aggiorna un'istanza |
| `GET` | `/services/{name}` | Restituisce le istanze attive |
| `DELETE` | `/services/{name}/instances/{id}` | Elimina un'istanza |
| `DELETE` | `/services/{name}` | Elimina un servizio e le sue istanze |

Esempio di registrazione:

```sh
curl -i -X PUT \
  http://localhost:8080/services/payments/instances/payment-1 \
  -H 'Content-Type: application/json' \
  -d '{"address":"10.0.0.1","port":9001}'
```

Discovery:

```sh
curl http://localhost:8080/services/payments
```

Cancellazione dell'istanza:

```sh
curl -i -X DELETE \
  http://localhost:8080/services/payments/instances/payment-1
```

### API interne

| Metodo | Percorso | Descrizione |
|---|---|---|
| `GET` | `/internal/state` | Restituisce lo snapshot completo |
| `PUT` | `/internal/state` | Esegue il merge di uno snapshot remoto |

Gli endpoint interni includono anche i tombstone e sono destinati esclusivamente alla comunicazione tra nodi. Nel deployment devono essere protetti dalla configurazione di rete e non esposti direttamente a Internet.

## Configurazione

Il nodo viene configurato tramite variabili d'ambiente.

| Variabile | Default | Descrizione |
|---|---:|---|
| `NODE_ID` | `registry-1` | Identificativo univoco del nodo |
| `HTTP_ADDRESS` | `:8080` | Indirizzo HTTP di ascolto |
| `PEERS` | vuoto | URL dei peer separati da virgola |
| `PEER_TIMEOUT` | `2s` | Timeout delle richieste tra peer |
| `SYNC_INTERVAL` | `5s` | Intervallo della sincronizzazione anti-entropy |
| `STATE_FILE` | vuoto | File JSON di persistenza; se vuoto la persistenza è disabilitata |
| `PERSIST_INTERVAL` | `1s` | Intervallo di salvataggio dello snapshot |

Le durate utilizzano il formato di Go, per esempio `500ms`, `2s` o `1m`.

## Esecuzione locale

Requisiti:

- Go 1.23 o successivo;
- `curl` per le verifiche manuali.

Avvio di un singolo nodo in memoria:

```sh
go run .
```

Avvio con persistenza:

```sh
STATE_FILE=./data/registry-state.json \
PERSIST_INTERVAL=1s \
go run .
```

Esempio con due nodi, in due terminali differenti:

```sh
NODE_ID=registry-1 \
HTTP_ADDRESS=:8080 \
PEERS=http://localhost:8081 \
go run .
```

```sh
NODE_ID=registry-2 \
HTTP_ADDRESS=:8081 \
PEERS=http://localhost:8080 \
go run .
```

## Docker Compose

Il file `docker-compose.yml` crea tre nodi completamente connessi. Un quarto
nodo opzionale viene utilizzato dal test di recupero dopo inattività
prolungata:

| Nodo | Porta host | Peer interni |
|---|---:|---|
| `registry-1` | `8080` | `registry-2`, `registry-3` |
| `registry-2` | `8081` | `registry-1`, `registry-3` |
| `registry-3` | `8082` | `registry-1`, `registry-2` |
| `registry-4` | `8083` | `registry-1`, `registry-2` |

Ogni nodo utilizza un volume Docker separato per `/data/registry-state.json`.
`registry-4` appartiene al profilo Compose `recovery-test`, quindi non viene
avviato insieme al cluster standard.

Avvio:

```sh
docker compose up --build -d
```

Avvio esplicito del quarto nodo:

```sh
docker compose up -d registry-4
```

Stato e log:

```sh
docker compose ps
docker compose logs -f
```

Arresto del cluster standard conservando i dati:

```sh
docker compose down
```

Se è stato avviato anche `registry-4`, il profilo deve essere incluso nella
pulizia:

```sh
docker compose --profile recovery-test down --remove-orphans
```

Arresto di tutti i nodi eliminando anche i volumi:

```sh
docker compose --profile recovery-test down -v --remove-orphans
```

Durante l'avvio possono comparire brevemente errori `connection refused`: ciascun nodo avvia subito la prima sincronizzazione e alcuni peer potrebbero non essere ancora in ascolto. Le sincronizzazioni successive vengono ritentate automaticamente.

## Test

Test completi:

```sh
go test ./...
```

Controllo delle data race:

```sh
go test -race ./...
```

### Test d'integrazione Docker

Lo script automatico verifica:

- health check dei tre nodi;
- gossip immediato;
- indisponibilità temporanea di un peer;
- recupero tramite anti-entropy;
- propagazione dei tombstone;
- avvio di `registry-4` mentre `registry-3` è inattivo;
- creazione, aggiornamento e cancellazione di molte istanze tramite
  `registry-4`;
- misurazione del tempo impiegato da `registry-3` per recuperare le modifiche
  accumulate durante l'inattività;
- persistenza dopo la ricreazione di un container.

Assicurarsi che Docker sia attivo e che le porte `8080`, `8081`, `8082` e
`8083` siano libere, quindi eseguire:

```sh
./scripts/integration-test.sh
```

Il nuovo scenario usa questi valori predefiniti:

| Variabile | Default | Descrizione |
|---|---:|---|
| `OFFLINE_DURATION` | `30` | Secondi aggiuntivi di inattività di `registry-3` |
| `INSTANCE_COUNT` | `100` | Istanze create tramite `registry-4` |
| `RECOVERY_TIMEOUT` | `60` | Secondi massimi concessi alla convergenza |

I valori possono essere modificati al momento dell'esecuzione:

```sh
OFFLINE_DURATION=120 \
INSTANCE_COUNT=500 \
RECOVERY_TIMEOUT=90 \
./scripts/integration-test.sh
```

Lo script riporta il tempo trascorso dal riavvio di `registry-3` fino al
recupero delle istanze tramite anti-entropy. Verifica inoltre che aggiornamenti,
tombstone e dati persistiti da `registry-4` siano conservati correttamente.

Lo script usa un progetto Compose isolato e rimuove automaticamente le risorse create per il test.

### Test di convergenza

I test di convergenza misurano separatamente:

- la latenza end-to-end del gossip, dalla registrazione su `registry-1` alla visibilità su `registry-2` e `registry-3`;
- la latenza end-to-end del recupero anti-entropy dopo una temporanea sospensione di `registry-3`.

I risultati riportano minimo, media, mediana, percentile p95 e massimo su 20 campioni. Poiché richiedono un cluster Docker già attivo e manipolano temporaneamente `registry-3`, vengono eseguiti soltanto su richiesta:

```sh
RUN_CONVERGENCE_TESTS=1 \
go test -count=1 -v -timeout=5m ./tests/convergence
```

Il polling delle API avviene ogni 10 ms; questo intervallo rappresenta anche la risoluzione approssimativa delle misurazioni.

## Persistenza

Ogni salvataggio viene effettuato in modo atomico:

1. lo snapshot viene scritto in un file temporaneo;
2. il contenuto viene sincronizzato su disco;
3. il file temporaneo sostituisce quello precedente tramite rename.

All'avvio il nodo carica lo stato persistito ed esegue il merge, ripristinando anche il contatore logico. Se il file non esiste, il nodo parte con uno stato vuoto. Un file presente ma non valido impedisce l'avvio, evitando di ignorare silenziosamente dati corrotti.

## Struttura del progetto

```text
.
├── main.go
├── Dockerfile
├── docker-compose.yml
├── internal
│   ├── api           # handler HTTP pubblici e interni
│   ├── peer          # client, gossip e anti-entropy
│   ├── persistence   # salvataggio atomico e worker periodico
│   ├── reporting     # gestione condivisa degli error handler
│   └── registry      # modello, operazioni locali e merge
├── scripts
│   └── integration-test.sh
└── tests
    └── convergence   # misure di gossip e recovery anti-entropy
```

## Stato del progetto

Sono completati registry locale, replica multi-nodo, gossip, anti-entropy, tombstone, persistenza, Docker Compose e test automatici.
Il sistema è stato inoltre distribuito e verificato su un'istanza Amazon EC2, dove il test d'integrazione è stato eseguito con successo.
