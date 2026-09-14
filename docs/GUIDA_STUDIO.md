# Guida completa allo studio del Distributed Service Registry

Questa dispensa descrive l'implementazione reale del progetto, le motivazioni delle scelte progettuali, le configurazioni importanti e i limiti da conoscere. È pensata sia per comprendere il codice sia per preparare la discussione orale.

## 1. Obiettivo e proprietà del sistema

Il sistema è un **service registry distribuito** scritto in Go. Permette di registrare, aggiornare, cercare e cancellare istanze di servizi. Un'istanza è identificata dalla coppia `(serviceName, instanceID)` e possiede un endpoint `(address, port)`.

Ogni nodo conserva una copia completa dello stato e può ricevere richieste. Non esistono leader, consenso o scritture sincrone a maggioranza. Una scrittura è accettata localmente anche se gli altri nodi sono offline; le repliche possono quindi divergere per un breve periodo, ma convergono dopo il ripristino della comunicazione.

Questa proprietà è detta **consistenza eventuale**. Non è garantito che una lettura eseguita immediatamente su un altro nodo veda l'ultima scrittura.

> **Scelta progettuale:** evitare leader e quorum rende ogni nodo autonomo e disponibile durante una partizione temporanea. Il prezzo è una finestra di inconsistenza e la necessità di risolvere deterministicamente gli aggiornamenti concorrenti.

## 2. Architettura generale

```text
                         snapshot completi
                gossip push / anti-entropy pull
              +-------------------------------+
              |                               |
Client ---> Registry 1 <-----> Registry 2 <---+
              ^                   |
              |                   v
              +------------> Registry 3

Ogni nodo: API HTTP -> Registry in memoria -> replica verso peer
                              |
                              +-> persistenza JSON
```

| Percorso | Responsabilità |
|---|---|
| `main.go` | Configurazione, composizione e ciclo di vita |
| `internal/registry` | Modello, CRUD, versioni, snapshot e merge |
| `internal/api` | Endpoint HTTP pubblici e interni |
| `internal/peer` | Client HTTP, gossip e anti-entropy |
| `internal/persistence` | Caricamento e salvataggio atomico |
| `internal/reporting` | Gestione thread-safe degli errori asincroni |
| `tests/convergence` | Misure end-to-end della convergenza |
| `scripts/integration-test.sh` | Verifica del cluster e dei guasti |

> **Scelta progettuale:** dominio, trasporto e persistenza sono separati. Il registry non conosce HTTP o Docker e può essere testato direttamente in memoria.

## 3. Avvio del programma (`main.go`)

### 3.1 Sequenza di avvio

`main()` esegue nell'ordine:

1. legge le variabili d'ambiente;
2. valida le durate;
3. crea il registry in memoria;
4. carica l'eventuale stato persistito;
5. crea un contesto cancellabile tramite segnali;
6. costruisce l'handler HTTP;
7. avvia l'eventuale persister;
8. costruisce client, synchronizer e gossiper se sono presenti peer;
9. avvia le goroutine di background;
10. avvia il server HTTP;
11. esegue lo shutdown su `SIGINT` o `SIGTERM`.

### 3.2 Parsing della configurazione

- `valueOrDefault` usa il fallback quando la variabile è vuota.
- `parsePeers` divide su virgola, elimina spazi e ignora voci vuote.
- `parseDuration` usa il formato Go e rifiuta valori nulli o negativi.

Durate valide: `500ms`, `2s`, `1m`. Valori come `5`, `0s` o `-1s` impediscono l'avvio con `log.Fatalf`.

> **Configurazione particolare:** `PERSIST_INTERVAL` viene letto solo quando `STATE_FILE` non è vuoto. Senza file di stato, la persistenza è disabilitata e quell'intervallo non ha effetto.

### 3.3 Composizione condizionale

Se `STATE_FILE` è configurato, `FileStore.Load()` legge lo snapshot, `MergeState()` lo applica, parte il salvataggio periodico e viene registrato un salvataggio finale con `defer`. Se è vuoto, il nodo lavora soltanto in memoria.

Se `PEERS` è vuoto, il nodo funziona standalone. Altrimenti vengono avviati gossip e anti-entropy e l'handler riceve il gossiper come notificatore.

> **Scelta progettuale:** l'iniezione delle dipendenze tramite costruttori permette alla stessa API di funzionare con o senza replica e consente ai test di usare un notifier finto.

### 3.4 Shutdown controllato

`signal.NotifyContext` cancella un contesto condiviso su `SIGINT` e `SIGTERM`. Persister, synchronizer e gossiper osservano quel contesto. `shutdownServer` concede cinque secondi al server HTTP per completare le richieste attive.

Docker invia normalmente `SIGTERM` con `docker stop`: il nodo può quindi arrestare i worker, chiudere HTTP e salvare lo stato. `http.ErrServerClosed` non è trattato come errore perché è il risultato normale dello shutdown.

## 4. Modello dati (`internal/registry/model.go`)

### 4.1 Record

`ServiceRecord` contiene nome, stato, versione, nodo di origine e timestamp. `InstanceRecord` aggiunge ID, nome del servizio, indirizzo e porta.

| Campo | Significato |
|---|---|
| `Status` | `active` oppure `deleted` |
| `Version` | ordine logico dell'ultima modifica |
| `OriginNode` | nodo che ha generato quella versione |
| `UpdatedAt` | istante UTC informativo |

`InstanceInput` espone al client soltanto `Address` e `Port`. Stato, versione, origine e timestamp sono assegnati dal server.

> **Scelta progettuale:** separare input pubblico e record interno impedisce ai client di scegliere versioni o origine e interferire con il protocollo di conflitto.

### 4.2 Struttura in memoria

```text
mu        sync.RWMutex
nodeID    string
version   uint64
services  map[serviceName]ServiceRecord
instances map[serviceName]map[instanceID]InstanceRecord
```

La doppia mappa rende diretta la ricerca per servizio e istanza. `RWMutex` protegge mappe e contatore: le modifiche usano `Lock`, discovery e snapshot usano `RLock`.

> **Dettaglio di codice:** `nextVersion()` non acquisisce autonomamente il lock perché viene chiamata soltanto da sezioni che possiedono già il lock esclusivo. Chiamarla senza lock in futuro introdurrebbe una data race.

## 5. Operazioni locali

### 5.1 Registrazione e aggiornamento

`InsertUpdateInstance` normalizza le stringhe, valida i dati, acquisisce il lock, crea o riattiva il servizio se necessario e salva l'istanza attiva con una nuova versione.

Restituisce `created=true` se l'istanza era assente o cancellata. L'API usa questo valore per restituire `201 Created`; un aggiornamento restituisce `200 OK`.

La creazione del servizio consuma una versione e quella dell'istanza ne consuma un'altra: sono record indipendenti, replicati e confrontati separatamente.

### 5.2 Validazione

Sono rifiutati servizio vuoto, ID vuoto, indirizzo vuoto e porta fuori da `1..65535`. Gli errori sono sentinelle verificabili con `errors.Is`.

### 5.3 Discovery

`Discover` restituisce `[]` se il servizio è assente o cancellato. Per un servizio attivo filtra le istanze cancellate e ordina le attive per ID.

> **Scelta progettuale:** un array vuoto invece di `null` mantiene stabile il contratto JSON. L'ordinamento elimina la casualità delle mappe Go e rende risultati e test riproducibili.

### 5.4 Cancellazioni

`DeleteInstance` non rimuove il record: imposta `deleted`, incrementa la versione e aggiorna origine e timestamp. Una seconda cancellazione è idempotente; un ID mai esistito restituisce `ErrInstanceNotFound`.

`DeleteService` crea il tombstone del servizio e tombstona tutte le sue istanze attive, assegnando una versione distinta a ogni record. Tombstonare anche le istanze evita che una futura riattivazione renda visibili dati locali vecchi.

## 6. Versioni logiche e conflitti

Ogni modifica locale incrementa `Registry.version`. Quando viene accettato un record remoto con versione superiore, il contatore locale avanza a quel valore: la prossima modifica locale sarà più recente. È un orologio logico simile a un Lamport clock semplificato.

`isNewerRecord` confronta la coppia:

```text
(Version, OriginNode)
```

1. vince la versione maggiore;
2. a parità vince `OriginNode` lessicograficamente maggiore;
3. stessa versione e stessa origine indicano un record già noto.

Esempio: `(7, registry-3)` prevale su `(7, registry-1)`, mentre qualsiasi record con versione `8` prevale su entrambi.

> **Configurazione critica:** i `NODE_ID` devono essere univoci e stabili. Due nodi con lo stesso ID potrebbero produrre record indistinguibili a parità di versione.

> **Dettaglio essenziale:** `UpdatedAt` non decide il conflitto. È soltanto metadato. Usare l'orologio fisico esporrebbe il sistema agli scostamenti temporali tra macchine.

`MergeService` e `MergeInstance` accettano record sconosciuti o più recenti, rifiutano quelli obsoleti e aggiornano il contatore. `MergeState` applica prima i servizi e poi le istanze, perché un'istanza è scopribile solo insieme al servizio attivo.

Le proprietà sono determinismo, idempotenza, protezione dallo stato obsoleto e convergenza indipendente dall'ordine di consegna.

> **Limite:** il modello è last-writer-wins logico. Una scrittura concorrente perde; non vengono mantenute entrambe come avverrebbe con vector clock o CRDT multi-value.

## 7. Tombstone

Un tombstone è un record conservato con `status: deleted`. La rimozione fisica sarebbe pericolosa: un nodo offline potrebbe tornare con una copia attiva vecchia e reintrodurla.

```text
A e B possiedono v1 active
B va offline
A elimina -> v2 deleted
B ritorna con v1 active
merge: v2 deleted prevale -> il dato non ricompare
```

Snapshot e persistenza includono i tombstone; la discovery li nasconde.

> **Limite noto:** non esiste garbage collection. Memoria, snapshot e file possono crescere. Una rimozione sicura richiederebbe sapere che tutte le repliche hanno osservato la cancellazione o introdurre retention e membership più sofisticate.

## 8. API HTTP (`internal/api`)

Il routing usa i pattern del `ServeMux` moderno e `request.PathValue`. Il modulo
dichiara Go 1.23.4 e il Dockerfile usa la toolchain Go 1.23.

### 8.1 Endpoint

| Metodo | Percorso | Risposta principale |
|---|---|---|
| `GET` | `/health` | `200 {"status":"ok"}` |
| `PUT` | `/services/{name}/instances/{id}` | `201` creazione, `200` aggiornamento |
| `GET` | `/services/{name}` | `200` con istanze attive |
| `DELETE` | `/services/{name}/instances/{id}` | `204` o `404` |
| `DELETE` | `/services/{name}` | `204` o `404` |
| `GET` | `/internal/state` | snapshot completo |
| `PUT` | `/internal/state` | merge e `{"applied":N}` |

Il `GET` interno è usato dall'anti-entropy, il `PUT` dal gossip.

> **Configurazione di sicurezza:** gli endpoint interni non hanno autenticazione. Devono essere protetti con rete privata, firewall o reverse proxy. Se la porta è pubblica, anche `/internal/state` è raggiungibile.

### 8.2 Parsing JSON difensivo

Gli handler limitano il body a `1 << 20` byte (1 MiB), rifiutano campi sconosciuti con `DisallowUnknownFields` e verificano che esista un solo valore JSON. Questo impedisce input enormi, errori di battitura e corpi come `{} {}`.

### 8.3 Notifier e propagazione

Dopo una modifica pubblica riuscita, l'handler crea uno snapshot e invoca `Notify`. `StateNotifier` ha un solo metodo e rende semplice sostituire il gossiper con un fake nei test.

> **Dettaglio importante:** `PUT /internal/state` esegue il merge ma non genera altro gossip, evitando cicli e tempeste. La configurazione prevista è completamente connessa; in topologie non completamente connesse questo limita la propagazione transitiva degli aggiornamenti remoti.

`/health` è un controllo di liveness: non verifica peer, volume o convergenza. Un nodo isolato può risultare sano, coerentemente con la disponibilità locale.

## 9. Comunicazione tra peer (`internal/peer`)

### 9.1 Client

`Client.Sync` esegue `GET /internal/state`, decodifica e chiama `MergeState`. `Client.Push` serializza lo snapshot, esegue `PUT /internal/state` e legge il numero di record applicati. Gli URL perdono eventuali slash finali; richieste e risposte usano context, timeout e limite di 1 MiB.

> **Configurazione importante:** `PEER_TIMEOUT` limita quanto un peer lento blocca un tentativo. Troppo alto rallenta il ciclo; troppo basso causa falsi errori su rete lenta o snapshot grandi.

### 9.2 Gossip

Dopo ogni modifica locale il gossiper invia lo snapshot completo a tutti i peer. Il canale `updates` ha capacità uno e `Notify` non blocca:

- se libero, inserisce lo snapshot;
- se pieno, elimina quello pendente e conserva il più recente.

È un **coalescing latest-wins**. Poiché il messaggio è uno snapshot completo, quello più recente contiene anche le modifiche precedenti. `notifyMu` rende atomica questa sostituzione fra chiamate concorrenti.

> **Scelta progettuale:** il buffer limitato evita che richieste veloci accumulino snapshot obsoleti in memoria. Gli stati intermedi possono non essere inviati, ma lo stato finale viene preservato.

`Publish` contatta i peer in sequenza. Un peer lento può ritardare i successivi fino al timeout; invii paralleli sarebbero un possibile miglioramento.

### 9.3 Anti-entropy

`Synchronizer.Run` sincronizza immediatamente all'avvio e poi a ogni `SYNC_INTERVAL`. Tenta tutti i peer e raccoglie gli errori separatamente. Il fallimento di un peer non ferma il nodo né impedisce il tentativo verso gli altri.

> **Scelta progettuale:** il gossip riduce la latenza normale; l'anti-entropy ripara i push persi durante disconnessioni. Sono complementari.

### 9.4 Costo degli snapshot completi

Inviare lo stato completo semplifica il protocollo: non servono log, cursori o acknowledgement per evento e il merge idempotente rende sicure le ripetizioni. Il costo cresce però con record e peer. Per larga scala sarebbero utili delta, Merkle tree, paginazione o log incrementali.

## 10. Errori asincroni (`internal/reporting`)

`ErrorHandler` conserva una callback protetta da `RWMutex`. `Report` copia la callback sotto read lock e la invoca dopo averlo rilasciato, evitando di trattenere il lock durante codice esterno potenzialmente lento. Senza callback è un no-op sicuro. In `main`, gli errori di replica e persistenza vengono scritti nei log senza arrestare il servizio.

## 11. Persistenza (`internal/persistence`)

### 11.1 Load e strategia fail-fast

`Load` restituisce stato vuoto se il file non esiste, ma fallisce per file illeggibile, JSON corrotto, campi sconosciuti o valori multipli. All'avvio lo stato viene applicato con `MergeState`, ripristinando anche il massimo contatore osservato.

> **Scelta progettuale:** se un file esistente è corrotto, partire vuoti potrebbe propagare perdita di dati. Il processo si ferma per rendere evidente il problema.

### 11.2 Scrittura atomica

`Save`:

1. crea la directory con permessi `0750`;
2. crea un temporaneo nella stessa directory;
3. scrive JSON indentato;
4. esegue `Sync` e chiude;
5. sostituisce il file definitivo con `Rename`;
6. rimuove eventuali temporanei residui.

Il temporaneo sta sullo stesso filesystem per rendere atomico il rename. Un crash durante la scrittura non lascia mezzo JSON al percorso definitivo.

> **Limite tecnico:** viene sincronizzato il file ma non esplicitamente la directory dopo il rename. Per durabilità estrema servirebbe anche il `fsync` della directory.

### 11.3 Persister

`Persister.Run` salva subito, a ogni `PERSIST_INTERVAL` e alla cancellazione del context. `main` effettua anche un salvataggio finale: è una ridondanza prudente.

> **Configurazione importante:** un intervallo breve riduce la finestra di dati non persistiti ma aumenta le scritture. Nel Compose è `1s`.

## 12. Docker

### 12.1 Dockerfile multi-stage

Il builder usa `golang:1.23-alpine` e compila con:

```text
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w"
```

- `CGO_ENABLED=0`: binario statico;
- `GOOS=linux`: destinazione Linux;
- `-trimpath`: elimina percorsi locali;
- `-s -w`: riduce il binario eliminando simboli e debug info.

Lo stage finale `alpine:3.21` contiene solo il binario e avvia il processo come utente non root `registry`. Il multi-stage riduce dimensione e superficie d'attacco; l'utente non root limita i privilegi.

### 12.2 Compose e porte

| Nodo | Porta host | Porta container | Peer |
|---|---:|---:|---|
| registry-1 | 8080 | 8080 | registry-2, registry-3 |
| registry-2 | 8081 | 8080 | registry-1, registry-3 |
| registry-3 | 8082 | 8080 | registry-1, registry-2 |

I nomi dei servizi sono risolti dal DNS interno Compose. Ogni nodo ha un volume separato: condividerne uno violerebbe l'indipendenza delle repliche e produrrebbe scritture concorrenti sul file.

`restart: unless-stopped` riavvia il container dopo crash o reboot salvo arresto esplicito. L'health check usa `wget` ogni 5 secondi, timeout 2 secondi, 5 tentativi e start period di 2 secondi.

`registry-4` appartiene al profilo `recovery-test` e serve per il test di recupero prolungato. Non parte nel cluster standard.

> **Comando particolare:** per eliminare anche registry-4 e tutti i volumi:
>
> ```sh
> docker compose --profile recovery-test down -v --remove-orphans
> ```

## 13. Configurazione completa

| Variabile | Default | Ruolo | Nota critica |
|---|---:|---|---|
| `NODE_ID` | `registry-1` | Identità e tie-break | Deve essere unica e stabile |
| `HTTP_ADDRESS` | `:8080` | Binding HTTP | Ascolta su tutte le interfacce |
| `PEERS` | vuoto | URL separati da virgola | Vuoto disabilita replica |
| `PEER_TIMEOUT` | `2s` | Timeout peer | Bilancia velocità e rete lenta |
| `SYNC_INTERVAL` | `5s` | Periodo anti-entropy | Influenza il recovery time |
| `STATE_FILE` | vuoto | Snapshot JSON | Vuoto disabilita persistenza |
| `PERSIST_INTERVAL` | `1s` | Periodo salvataggio | Vale solo con `STATE_FILE` |

Configurazione tipica:

```yaml
environment:
  NODE_ID: registry-1
  HTTP_ADDRESS: ":8080"
  PEERS: "http://registry-2:8080,http://registry-3:8080"
  PEER_TIMEOUT: "2s"
  SYNC_INTERVAL: "5s"
  STATE_FILE: "/data/registry-state.json"
  PERSIST_INTERVAL: "1s"
```

Relazioni da ricordare:

- `SYNC_INTERVAL` basso accelera il recupero ma aumenta traffico;
- `PEER_TIMEOUT` si applica a peer contattati in sequenza;
- il recovery test aspetta oltre il timeout per escludere il vecchio gossip;
- snapshot oltre 1 MiB non sono supportati dal protocollo attuale.

## 14. Flussi end-to-end

### 14.1 Registrazione

```text
Client PUT
 -> handler valida JSON
 -> InsertUpdateInstance acquisisce Lock
 -> crea/riattiva servizio e salva istanza
 -> risposta 201 o 200
 -> Snapshot e Notify
 -> Gossiper.Push ai peer
 -> PUT /internal/state
 -> MergeState applica solo record più recenti
```

La replica è asincrona: il successo locale non dipende dai peer.

### 14.2 Recupero di un peer

```text
registry-3 offline
 -> registry-1 accetta una modifica
 -> gossip a registry-2 riesce
 -> gossip a registry-3 scade e viene loggato
 -> registry-3 ritorna
 -> Synchronizer esegue GET sui peer
 -> MergeState applica le versioni mancanti
 -> convergenza
```

### 14.3 Cancellazione

```text
DELETE -> tombstone con versione maggiore
 -> snapshot include deleted
 -> replica lo propaga
 -> discovery lo filtra
 -> persistenza lo conserva al riavvio
```

## 15. Strategia di test

### 15.1 Test unitari e race detector

I test coprono configurazione, CRUD, validazione, accessi concorrenti, conflitti, avanzamento del contatore, tombstone, API, notifier, client peer, gossip, synchronizer, persistenza e callback di errore.

```sh
go test ./...
go test -race ./...
```

Il race detector è fondamentale perché handler, gossip, anti-entropy e persister condividono il registry.

### 15.2 Test d'integrazione

`scripts/integration-test.sh` usa `set -euo pipefail`, un progetto Compose isolato e un `trap` di pulizia. Verifica:

1. health dei tre nodi;
2. gossip iniziale;
3. cancellazione con registry-3 offline;
4. recupero del tombstone;
5. molte modifiche prodotte da registry-4;
6. recupero di creazione, update e cancellazione;
7. tempo approssimato di convergenza;
8. ricreazione dei container mantenendo i volumi;
9. corretto recupero persistente.

| Variabile test | Default | Vincolo |
|---|---:|---|
| `OFFLINE_DURATION` | 30 | intero non negativo |
| `INSTANCE_COUNT` | 100 | almeno 2 |
| `RECOVERY_TIMEOUT` | 60 | almeno 1 |

Le funzioni `wait_*` usano polling: il test termina appena la condizione è vera ed è robusto su macchine di velocità diversa.

### 15.3 Test di convergenza

```sh
RUN_CONVERGENCE_TESTS=1 \
go test -count=1 -v -timeout=5m ./tests/convergence
```

Sono disabilitati di default perché richiedono Docker e manipolano registry-3. `-count=1` disabilita la cache. Su 20 campioni calcolano minimo, media, mediana, p95 e massimo con polling ogni 10 ms.

Il test gossip misura dal PUT alla visibilità su registry-2 e registry-3. Il test recovery mette in pausa registry-3, aspetta `PEER_TIMEOUT + 500ms` affinché il gossip scada, lo riattiva e misura l'anti-entropy.

> **Interpretazione:** la risoluzione è circa 10 ms e la misura include HTTP, scheduling, Docker e polling; è end-to-end.

## 16. EC2 e sicurezza

Il cluster usa su EC2 gli stessi container Compose. Il test d'integrazione è stato eseguito con successo. Comandi di controllo:

```sh
docker compose up --build -d
docker compose ps
curl http://localhost:8080/health
./scripts/integration-test.sh
```

Aspetti importanti:

- limitare SSH al proprio IP;
- non esporre pubblicamente tutte le porte 8080-8083;
- proteggere `/internal/*` tramite rete privata o reverse proxy;
- non inserire segreti AWS nel repository o nell'immagine;
- `docker compose down` conserva i volumi;
- `docker compose down -v` elimina lo stato persistito.

## 17. Complessità e prestazioni

Con `N` record totali, `k` istanze del servizio e `P` peer:

- ricerca/upsert: mediamente `O(1)`;
- discovery: `O(k log k)` per l'ordinamento;
- snapshot: `O(N log N)` per gli ordinamenti;
- merge: mediamente `O(N)`;
- replica completa: dati dell'ordine di `O(P*N)`.

La soluzione è adatta a cluster piccoli. Su larga scala snapshot completo e lock durante copia/merge diventano punti da ottimizzare.

## 18. Garanzie e limiti

### Garantisce

- operazioni thread-safe;
- disponibilità locale con peer offline;
- gossip rapido e riparazione periodica;
- conflitti deterministici e merge idempotente;
- protezione delle cancellazioni;
- persistenza atomica;
- shutdown coordinato.

### Non garantisce

- consistenza forte o quorum;
- conservazione di entrambe le scritture concorrenti;
- autenticazione o TLS;
- propagazione transitiva completa in topologie arbitrarie;
- garbage collection dei tombstone;
- replica efficiente per stati molto grandi;
- health check approfondito;
- membership dinamica dei nodi.

Conoscere questi limiti dimostra quali proprietà derivano dalle scelte effettuate e quali richiederebbero meccanismi aggiuntivi.

## 19. Miglioramenti possibili

1. autenticazione e TLS;
2. reverse proxy per separare API pubbliche e interne;
3. gossip parallelo verso i peer;
4. delta replication o Merkle tree;
5. garbage collection sicura dei tombstone;
6. metriche di latenza, errori e dimensione dello stato;
7. readiness check oltre alla liveness;
8. membership dinamica;
9. validazione semantica degli snapshot remoti;
10. `fsync` della directory dopo il rename.

## 20. Domande da discussione

### Perché gossip e anti-entropy insieme?

Il gossip propaga velocemente. Se il push viene perso, l'anti-entropy recupera periodicamente lo stato.

### Perché non eliminare fisicamente?

Una replica isolata potrebbe reintrodurre il dato. Il tombstone conserva una cancellazione più recente.

### Perché il timestamp non decide il conflitto?

Gli orologi dei nodi possono essere disallineati. Versione logica e origine producono un ordine deterministico.

### Perché serve `OriginNode`?

Due nodi possono produrre la stessa versione. L'origine risolve la parità in modo uguale ovunque.

### Perché ordinare gli snapshot?

Le mappe Go non hanno ordine stabile. L'ordinamento rende output e test riproducibili.

### Perché inviare snapshot e non eventi?

Gli snapshot semplificano protocollo, idempotenza e coalescing. Il costo è maggiore per registri grandi.

### Perché gli errori peer non fermano il nodo?

Un peer offline è previsto: il nodo deve restare disponibile e riprovare con l'anti-entropy.

### Replica e persistenza sono la stessa cosa?

No. La replica mantiene copie su processi differenti; la persistenza conserva la copia dello stesso nodo dopo il riavvio.

### Perché il canale gossip ha capacità uno?

Evita di bloccare le richieste e accumulare snapshot obsoleti. L'ultimo snapshot contiene lo stato precedente.

### Cosa accade al riavvio?

Il nodo carica il JSON, applica il merge, recupera il massimo contatore, avvia subito l'anti-entropy e poi i cicli periodici.

## 21. Percorso di studio consigliato

1. `main.go`: ciclo di vita e dipendenze.
2. `model.go`, `registry.go`, `state.go`, `merge.go`.
3. Simulazione manuale di conflitto e tombstone.
4. `handler.go`: dal router al dominio.
5. Confronto tra `gossiper.go` e `synchronizer.go`.
6. `file.go`: persistenza atomica.
7. Collegamento tra Compose e variabili di `main.go`.
8. Script d'integrazione come scenario di guasto.
9. Test unitari come specifica eseguibile.
10. Ripetizione orale di garanzie, compromessi e limiti.

## 22. Checklist di autoverifica

Prima della discussione bisogna saper spiegare:

- percorso completo di registrazione e cancellazione;
- generazione delle versioni e tie-break;
- necessità dei tombstone;
- differenza tra gossip e anti-entropy;
- recupero di un peer offline;
- caricamento e salvataggio atomico;
- ruolo del context nello shutdown;
- significato di ogni variabile;
- mapping porte host/container;
- integration test e convergence test;
- consistenza eventuale contro consistenza forte;
- limiti di sicurezza e scalabilità;
- miglioramenti possibili e relativo costo.
