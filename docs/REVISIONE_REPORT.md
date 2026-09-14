# Revisione del report — Service Registry distribuito

## Valutazione complessiva

Sì: il report rappresenta bene l'impianto del progetto. Replica completa, API locali, versioni logiche, risoluzione dei conflitti, tombstone, gossip push, anti-entropia pull e persistenza JSON sono effettivamente presenti. Non serve riscrivere il progetto per renderlo coerente con la relazione: serve soprattutto correggere alcune affermazioni e migliorare la presentazione dei test.

Le priorità sono: correggere la definizione di convergenza, sostituire i percorsi API, descrivere correttamente l'isolamento nei test di conflitto e ridimensionare le conclusioni sperimentali. La parte linguistica più bisognosa di revisione è quella dei test.

Questa revisione confronta il testo allegato con il codice e gli script attualmente nel workspace, incluse le modifiche locali presenti. Non certifica la conformità alla traccia d'esame, che non è stata fornita. Non ho rieseguito gli esperimenti Docker/EC2 né verificato i valori numerici delle tabelle attraverso log originali. Le riscritture seguenti sono proposte da trasferire nel tuo documento, non modifiche già applicate al report.

**Come trovare i punti:** i numeri di riga si riferiscono al file `pasted-text.txt` allegato, prima di qualsiasi modifica. Sono accompagnati dalla sezione e da una citazione per consentire la ricerca anche in Overleaf. I riferimenti al codice indicano file e funzioni.

## 1. Correzioni tecniche necessarie

### 1.1 Introduzione — condizione di convergenza

**Punto:** frase «dopo la fine delle comunicazioni tra nodi, tutte le repliche convergono deterministicamente verso lo stesso stato».

**Problema:** inverti la condizione necessaria. Se la comunicazione termina prima che gli aggiornamenti siano stati scambiati, le repliche possono restare divergenti.

**Sostituzione:**

> Il sistema adotta un modello di consistenza finale: in assenza di nuovi aggiornamenti e a condizione che la comunicazione tra i nodi sia disponibile e consenta la propagazione dei record, le repliche convergono deterministicamente verso lo stesso stato.

Nella sottosezione «Modello di consistenza», sostituisci anche «in assenza di di nuovi aggiornamenti […] tutti i nodi convergono» con «in assenza di nuovi aggiornamenti […] tutti i nodi convergano». Evita «per un breve intervallo»: con un peer irraggiungibile la divergenza può durare a lungo. Scrivi «per un intervallo che dipende dalla comunicazione e dai tempi di sincronizzazione».

**Riscontro:** `internal/registry/merge.go`, `internal/peer/synchronizer.go`.

### 1.2 API HTTP — percorsi errati nella tabella

**Punto:** righe 121–136, soprattutto 127–130.

I percorsi implementati sono al plurale. Inoltre `instances` è un segmento fisso, non un parametro.

| Metodo | Percorso corretto | Funzione |
|---|---|---|
| GET | `/health` | Verifica che il processo risponda |
| PUT | `/services/{name}/instances/{id}` | Registrazione o aggiornamento di un'istanza |
| GET | `/services/{name}` | Discovery delle istanze attive |
| DELETE | `/services/{name}/instances/{id}` | Cancellazione di un'istanza |
| DELETE | `/services/{name}` | Cancellazione del servizio e delle istanze locali attive |
| GET | `/internal/state` | Lettura dello snapshot completo |
| PUT | `/internal/state` | Merge di uno snapshot remoto |

Sostituisci «Le API pubbliche implementate sono» con «Gli endpoint HTTP implementati sono», oppure separa le ultime due righe in una tabella delle API interne.

**Attenzione al LaTeX:** per stampare le parentesi dei parametri usa, per esempio, `\texttt{/services/\{name\}/instances/\{id\}}`. Le parentesi graffe non precedute da backslash raggruppano il testo e non vengono stampate.

**Riscontro:** `internal/api/handler.go`, `NewHandlerWithNotifier`.

### 1.3 Test di integrazione — conflitto aggiornamento/cancellazione

**Punto:** righe 177–178, «disabilitiamo nuovamente la registry-3».

**Problema:** lo script isola nuovamente tutti e tre i nodi, impostando `PEERS` a stringa vuota e ricreando i container con i volumi conservati. Non disabilita soltanto il terzo nodo. È proprio l'isolamento dei due scrittori a rendere riproducibile il conflitto.

**Sostituzione:**

> Per verificare il conflitto aggiornamento/cancellazione, il test isola nuovamente tutti e tre i nodi disabilitando la replica e conservando i volumi. Esegue quindi in parallelo un aggiornamento su `registry-1` e una cancellazione della stessa istanza su `registry-2`. Verifica che i record abbiano la stessa versione e origini diverse. In questo scenario prevale la cancellazione perché `registry-2` ha origine lessicograficamente maggiore. Ripristinata la configurazione dei peer, il test verifica l'uguaglianza degli stati completi e l'assenza dell'istanza nella discovery di tutti i nodi.

La tua precisazione secondo cui la cancellazione non ha priorità assoluta è corretta e va mantenuta.

**Riscontro:** `scripts/integration-test.sh`, `run_conflict`; `scripts/compose.integration-isolated.yml`.

### 1.4 Risultati — evitare conclusioni più forti delle misure

**Punto:** riga 262, intero paragrafo conclusivo.

Tre aspetti da correggere:

1. «Garantisce tempi […] significativamente inferiori» è troppo forte per 20 campioni e senza analisi statistica inferenziale. Scrivi «nei campioni raccolti mostra tempi inferiori».
2. Non hai isolato sperimentalmente le cause del divario locale/EC2. L'attribuzione certa a rete e virtualizzazione non è dimostrata. I tre container EC2 sono sullo stesso host: non stai misurando la latenza tra tre macchine EC2 distinte.
3. Le due misure comprendono operazioni diverse: la prima include la registrazione, la seconda `docker compose unpause` e il controllo di disponibilità. Non sono tempi puri dei due algoritmi.

**Sostituzione del paragrafo:**

> Nei 20 campioni raccolti per ciascuno scenario, la latenza media osservata per la propagazione dell'istanza è pari a 3,64 ms in locale e 9,81 ms su EC2; per il recupero del nodo sospeso è pari rispettivamente a 96,92 ms e 132,13 ms. Si tratta di misure end-to-end: lo scenario di propagazione comprende la registrazione e le verifiche HTTP, mentre quello di recupero comprende la riattivazione del container, il controllo di disponibilità e l'osservazione dell'istanza recuperata. I risultati descrivono quindi il comportamento dei due scenari nella configurazione sperimentale adottata. Le differenze tra l'ambiente locale e quello EC2 non consentono, da sole, di identificare il contributo dei singoli fattori infrastrutturali.

**Prima di usare questa riscrittura:** conferma che i numeri provengano dalla versione dei test descritta nel report. Non ho trovato log originali che permettano di verificarli.

### 1.5 Risultati anti-entropia — spiegare il rapporto con i 5 secondi

**Punto:** righe 76, 204–217, 237 e 254.

Nel Compose l'intervallo è 5 s, mentre riporti recuperi medi di circa 97 e 132 ms. Questo non dimostra automaticamente che i dati siano errati: il tempo osservato dopo `unpause` dipende dalla fase del timer e da eventuali sincronizzazioni già avviate o pronte a riprendere. Non è però corretto dedurne che un nodo arbitrariamente disconnesso recuperi sempre in circa 100 ms.

**Aggiunta dopo la descrizione dell'esperimento:**

> Il tempo misurato parte immediatamente prima del comando di riattivazione e non comprende il periodo di sospensione. Non coincide con l'intervallo di sincronizzazione configurato, poiché dipende anche dalla fase del timer al momento della ripresa e dalle attività eventualmente già in corso. I risultati si riferiscono pertanto a questo specifico protocollo di pausa e riattivazione.

Per rendere i risultati verificabili, aggiungi versione del codice, comando eseguito, configurazione effettiva dei container e log dei 20 campioni. Non modificare i numeri senza risalire alla loro origine.

**Riscontro:** `tests/convergence/recovery_test.go`, `config_test.go`, `internal/peer/synchronizer.go`.

## 2. Precisazioni che rendono il report più fedele al codice

### 2.1 Strategia di replicazione — specificare cosa invii e a chi

**Punto:** righe 70–78.

Il `Gossiper` invia snapshot completi a tutti i peer configurati, uno dopo l'altro. Non seleziona casualmente un sottoinsieme dei nodi. Ha una coda di capacità uno e può sostituire una notifica in attesa quando arriva un altro snapshot. Il merge di uno stato remoto non genera automaticamente un nuovo gossip. `Synchronizer.Run` esegue un primo tentativo all'avvio, poi usa il timer.

**Testo da integrare:**

> La propagazione push invia lo snapshot completo ai peer configurati, contattati in sequenza. Il componente Gossiper utilizza una coda limitata, nella quale una nuova notifica può sostituire quella in attesa. La risposta al client non attende il completamento degli invii di rete. Il Synchronizer acquisisce gli snapshot completi dei peer tramite richieste GET e ne integra i record; esegue un tentativo iniziale all'avvio e successivamente si attiva con intervallo configurato di 5 s. La lista dei peer è fornita tramite configurazione.

La parola «gossip» può rimanere, essendo il nome adottato nel progetto, ma precisare questa implementazione evita di suggerire un protocollo probabilistico che qui non c'è. «Propagazione immediata» indica l'attivazione dopo la modifica, non una consegna istantanea garantita.

**Riscontro:** `internal/peer/gossiper.go`, `synchronizer.go`, `internal/api/handler.go`.

### 2.2 Rappresentazione dello stato — identità e versione

**Punto:** righe 81–92 e 109–116.

**Riscrittura della distinzione servizio/istanza:**

> Il servizio rappresenta un'entità logica identificata dal nome; ciascuna istanza rappresenta un endpoint concreto che offre quel servizio ed è identificata dalla coppia `(ServiceName, ID)`.

**Riscrittura dei due paragrafi sui campi:**

> `Version` consente di confrontare aggiornamenti dello stesso record. `OriginNode` identifica il nodo che ha generato l'aggiornamento e risolve i conflitti quando due record hanno la stessa versione. Aggiornamenti concorrenti possono avere la stessa versione, ma la concorrenza non implica necessariamente l'uguaglianza delle versioni.

**Aggiunta dopo il versioning:**

> Il contatore logico è locale al nodo ed è condiviso dalle operazioni sui diversi record. La versione cresce quando viene generato un nuovo record aggiornato; una singola richiesta può quindi consumare più versioni, per esempio quando crea sia il servizio sia la sua prima istanza. Una cancellazione ripetuta su un record già eliminato non incrementa nuovamente la versione.

Usa `Version` e `OriginNode` quando parli dei campi Go, oppure `version` e `originNode` quando parli dei campi JSON. `OrigineNode` non esiste.

La frase «se entrambi coincidono, il record rappresenta lo stesso aggiornamento» assume identificativi dei nodi univoci e assenza di riutilizzo delle versioni per aggiornamenti distinti. Più aderente al codice: «se entrambi coincidono, il record remoto viene ignorato». Il merge non confronta gli altri campi per sciogliere un eventuale pareggio completo.

**Riscontro:** `internal/registry/model.go`, `registry.go`, `merge.go`.

### 2.3 Cancellazioni — chiarire limiti e riattivazione

**Punto:** righe 118–119.

**Riscrittura:**

> La cancellazione è rappresentata da un tombstone: il record viene mantenuto con stato `deleted` e una nuova versione. Questo impedisce a una copia attiva precedente di prevalere durante il merge. La cancellazione di un'istanza lascia attivo il servizio; quella di un servizio marca come eliminate anche le sue istanze localmente attive. La discovery restituisce istanze soltanto se sia il servizio sia l'istanza risultano attivi. Una successiva registrazione può riattivare il servizio o l'istanza con una nuova versione.

Aggiungi che non è implementata la rimozione definitiva dei tombstone. Il merge tratta i record di servizio e istanza separatamente: non presentare la cancellazione come una transazione atomica distribuita né come una regola di priorità assoluta su tutte le scritture concorrenti.

**Riscontro:** `internal/registry/registry.go`, `state.go`, `merge.go`.

### 2.4 Concorrenza — nome della primitiva e granularità

**Punto:** righe 138–140.

**Sostituzione:**

> Lo stato condiviso del registry è protetto da un `sync.RWMutex`. Le operazioni di lettura, come discovery e snapshot, acquisiscono un lock condiviso; gli aggiornamenti acquisiscono un lock esclusivo. Il merge di uno snapshot applica i record separatamente, quindi l'intero merge non costituisce un'unica operazione atomica.

La frase sul reporting degli errori può rimanere: è previsto un componente dedicato con sincronizzazione. Non confondere assenza di data race con atomicità dell'intero snapshot ricevuto.

### 2.5 Persistenza — componente, validazione e durabilità

**Punto:** righe 145–152.

Sostituisci «dal componente Persister» con «da `FileStore.Save`, invocato dal componente `Persister`». Il procedimento temporaneo → sincronizzazione → rename è descritto correttamente.

Sostituisci «Un file persistente ma non valido impedisce l'avvio, evitando quindi i dati corrotti» con:

> Se il file esiste ma il caricamento o la decodifica JSON falliscono, il nodo interrompe l'avvio. Il decoder rifiuta anche campi sconosciuti e valori JSON aggiuntivi; non è però prevista una validazione semantica completa dei record caricati.

**Aggiunta consigliata:**

> La persistenza è periodica e utilizza un volume distinto per ciascun nodo. Sono previsti salvataggi anche durante l'arresto; la risposta a una scrittura non attende tuttavia la persistenza su disco. Un arresto improvviso può quindi perdere gli aggiornamenti locali non ancora salvati, recuperabili dai peer soltanto se già replicati.

Il test con `docker compose stop/down` verifica un arresto controllato, non la resistenza a un'interruzione improvvisa dell'alimentazione.

**Riscontro:** `internal/persistence/file.go`, `persister.go`, `main.go`, `docker-compose.yml`.

### 2.6 API interne e health check — intenzione e comportamento

**Punto:** riga 136 e tabella API.

**Aggiunta:**

> Gli endpoint interni sono destinati alla replica, ma il codice li espone sullo stesso server HTTP delle API pubbliche e non implementa autenticazione. L'eventuale restrizione dell'accesso dipende quindi dalla configurazione del deployment. L'endpoint `/health` segnala che il processo risponde; non verifica la raggiungibilità degli endpoint dei servizi registrati.

Non sono implementati heartbeat o TTL delle istanze: un'istanza registrata non viene rimossa automaticamente se il processo che rappresenta si arresta. Basta dichiararlo fra i limiti; non è necessario implementarlo per correggere la relazione.

### 2.7 AWS EC2 — ambito della tolleranza ai guasti

**Punto:** subito dopo riga 155.

**Aggiunta:**

> I tre nodi sono processi separati eseguiti in container sulla stessa macchina virtuale. Questa configurazione consente di sperimentare l'indisponibilità dei singoli nodi del registry, ma non offre ridondanza rispetto al guasto dell'host EC2, che ospita tutte le repliche.

Indica tipo dell'istanza, vCPU, memoria, sistema operativo, versioni degli strumenti e caratteristiche della macchina locale realmente usate. Sono informazioni da recuperare dall'ambiente sperimentale, non da dedurre dal repository.

## 3. Riscritture della parte sui test

### 3.1 Introduzione al test di integrazione — righe 159–163

> Lo script verifica il ciclo operativo del cluster in un ambiente Docker Compose dedicato. Esegue registrazioni, aggiornamenti e cancellazioni, controlla la propagazione dello stato, introduce conflitti riproducibili mediante isolamento dei nodi e verifica il recupero dopo l'indisponibilità di un peer. Controlla inoltre la conservazione dei tombstone e dello stato completo dopo la ricreazione dei container con i volumi mantenuti.

**Precisazione:** nello script l'anti-entropia rimane abilitata durante i normali controlli di propagazione. Questi dimostrano la propagazione, ma non escludono da soli un contributo dell'anti-entropia. Anche il test chiamato `TestEndToEndGossipConvergence` non disabilita la sincronizzazione periodica. Se vuoi dimostrare in modo esclusivo il percorso gossip, occorre un esperimento che isoli i due meccanismi.

### 3.2 Punto 4, recupero del nodo — riga 168

> Il test riavvia `registry-3` e interroga il suo endpoint `GET /internal/state` per verificare la presenza del tombstone. Il nodo recupera lo stato attraverso il Synchronizer, che contatta i peer anche all'avvio.

Così distingui la richiesta del test, che osserva, dalle richieste del nodo ai peer, che recuperano.

### 3.3 Punto 5, stato comune — riga 169

> Il test registra l'istanza `shared` del servizio `conflict` e attende che gli stati completi dei tre nodi coincidano. Questa condizione permette di riprodurre scritture concorrenti a parità di versione.

### 3.4 Punto 6, aggiornamento/aggiornamento — righe 170–176

> Per creare un conflitto riproducibile, il test ricrea i tre container conservando i volumi e impostando a vuota la lista dei peer. Le API restano disponibili, mentre la replica è disabilitata. Vengono eseguiti in parallelo due aggiornamenti della stessa istanza, rispettivamente su `registry-1` e `registry-2`. Il test verifica che entrambe le richieste riescano e che i record risultanti abbiano la stessa versione e origini diverse. Ripristinata la comunicazione, verifica che gli stati completi dei tre nodi coincidano con quello atteso: a parità di versione prevale l'aggiornamento di `registry-2`, la cui origine è lessicograficamente maggiore.

### 3.5 Punto 8, quarto nodo — righe 179–188

Sostituisci «Al suo posto viene avviato» con «Viene inoltre avviato». `registry-4` è un nodo aggiuntivo con identità e volume propri, non una sostituzione dell'identità di `registry-3`.

**Riscrittura:**

> Il test arresta nuovamente `registry-3` e avvia `registry-4`, configurato con i soli peer `registry-1` e `registry-2`. Con i parametri predefiniti, registra 100 istanze del servizio `recovery-load`, aggiorna `load-1` ed elimina `load-100`. Per questo servizio lo stato atteso comprende 99 istanze attive e il tombstone dell'istanza eliminata. Dopo la propagazione ai nodi disponibili e un ulteriore periodo di inattività, `registry-3` viene riavviato: deve recuperare dai peer conosciuti anche aggiornamenti originati da `registry-4`, che non compare nella sua lista dei peer.

Il numero 100 è configurabile tramite `INSTANCE_COUNT`; le 99 istanze non rappresentano tutto il registry, che contiene anche i record degli scenari precedenti.

### 3.6 Punti 9–11, misurazione e persistenza — righe 189–192

> Il tempo di recupero è misurato dall'avvio del comando che riavvia `registry-3` fino al completamento dei primi controlli sullo stato recuperato. Comprende l'avvio del processo, la sincronizzazione e le verifiche HTTP; lo script lo riporta in secondi. Successivamente viene verificata l'uguaglianza degli stati completi dei quattro nodi, confrontando tutti i campi, inclusi versioni, origini, timestamp e tombstone.

> Per verificare la persistenza, il test conserva lo stato atteso, rimuove i container mantenendo i volumi e avvia soltanto `registry-3`. Confronta lo stato caricato con quello atteso e verifica che l'istanza cancellata non ricompaia nella discovery. Arresta quindi `registry-3` e ripete il controllo con il solo `registry-4`. Poiché i peer non sono disponibili, il controllo verifica il recupero dal volume locale.

> I controlli vengono ripetuti con pause di un secondo fino al limite di tentativi previsto. Se una condizione non è soddisfatta, il test termina con un errore. Una procedura di pulizia rimuove container e volumi dell'ambiente di test anche in caso di errore.

Il confronto completo dei quattro stati avviene dopo la misurazione del tempo nello script: evita di includerlo nella definizione del valore cronometrato.

### 3.7 Test di convergenza gossip — righe 202–203

> Il test registra una nuova istanza su `registry-1` e misura il tempo fino alla sua osservazione su `registry-2` e `registry-3`. La misura comprende la richiesta iniziale e le verifiche HTTP dei due destinatari, interrogati in sequenza. Riguarda la visibilità dell'istanza considerata, non il confronto di tutti i record dell'intero registry. Dopo ogni controllo fallito il test attende 10 ms prima di riprovare; il primo controllo è immediato, quindi sono possibili misure inferiori a 10 ms. Ogni destinatario ha un timeout di attesa configurato di 5 s.

Il polling non determina un campionamento esatto ogni 10 ms: vanno aggiunti i tempi delle richieste e di esecuzione. I timeout sono controllati tra richieste; non sono un limite assoluto preciso al millisecondo.

### 3.8 Test di recupero — righe 205–217

> Il test sospende `registry-3` mediante `docker compose pause`, registra una nuova istanza su `registry-1` e attende che sia visibile su `registry-2`. Mantiene poi il terzo nodo sospeso per ulteriori 2,5 s, pari al timeout dei peer di 2 s più un margine di 500 ms, per lasciar scadere il tentativo di gossip. Avvia il cronometro immediatamente prima di `docker compose unpause`, attende che il nodo risponda al controllo di disponibilità e verifica che l'istanza compaia nella discovery. Il tempo di sospensione precedente alla riattivazione è escluso dalla misura; il timeout configurato per l'attesa dell'istanza recuperata è 10 s.

La pausa del container conserva la memoria del processo: questo scenario è diverso dall'arresto e dal riavvio con caricamento dal disco usati nel test di integrazione.

## 4. Correzioni linguistiche puntuali

Per i passaggi lunghi già riscritti sopra conviene sostituire il paragrafo intero. Per il resto, queste correzioni possono essere applicate con la ricerca testuale.

| Dove | Testo attuale | Correzione |
|---|---|---|
| Abstract | «di registrar e gestire» | «di registrare e gestire» |
| Introduzione | «interamente in GO» | «interamente in Go» |
| Introduzione | «il discovery» | «la discovery» |
| Introduzione, Concorrenza | «anty-entropy» | «anti-entropia» |
| Architettura | «una copia del locale del registry» | «una copia locale del registry» |
| Modello di consistenza | «in assenza di di» | «in assenza di» |
| Strategia | «Quando eseguiamo un modifica» | «Quando viene eseguita una modifica» |
| Strategia | «durante una interruzione» | «durante un'interruzione» |
| Strategia | «è stato settato» | «è stato impostato» |
| Listing istanza | «Implementazione del Instance Record» | «Struttura InstanceRecord» |
| Stato | «di una istanza» | «di un'istanza» |
| Versioning | «OrigineNode» | «OriginNode» |
| Versioning | «lessico grafico» | «lessicografico» |
| Cancellazioni | «tombostone» | «tombstone» |
| Concorrenza | «snaphot» | «snapshot» |
| Concorrenza | «RWmutex» | «sync.RWMutex» |
| Integrazione | «ciclo operativo complete» | «ciclo operativo completo» |
| Integrazione | «dopo la creazione del container» | «dopo la ricreazione dei container» |
| Punto 3 | «Verfichiamo» | «Verifichiamo» |
| Punto 3 | «possiamo essere certa» | «possiamo verificare» |
| Punto 3 | «il registri» | «il registry» |
| Punto 6 | «Pear» | «peer» |
| Punto 6 | «L'PAI restano disponibili» | «Le API restano disponibili» |
| Punto 6 | «temporaneamente disabilita» | «temporaneamente disabilitata» |
| Punto 6 | «Lanciamo un parallelo» | «Eseguiamo in parallelo» |
| Punto 6 | «l'origine lessicale lessico graficamente maggiore» | «l'origine lessicograficamente maggiore» |
| Punto 7 | «tre noti» | «tre nodi» |
| Punto 8 | «viene firmato» | «viene fermato» |
| Punto 8 | «Registry-4» | «registry-4» |
| Punto 8 | «registry-3 Deve» | «registry-3 deve» |
| Punto 9 | «il tempo necessario i controlli» | «il tempo necessario ai controlli» |
| Punto 9 | «Successivamente confronta» | «Successivamente il test confronta» |
| Punto 10 | «non hanno più disponibili dei quali recuperare» | «non hanno peer disponibili dai quali recuperare» |
| Punto 11 | «avvengono ripetuti» | «vengono ripetuti» |
| Punto 11 | «asincrona.se» | «asincrona. Se» |
| Convergenza | «La suite tests» | «La suite di test» |
| Convergenza | «affinchè», «affichè» | «affinché» |
| Convergenza | «a un Timeout dei 5s» | «ha un timeout di 5 s» |
| Recupero | «non-disponibilità» | «indisponibilità» |
| Recupero, punto 6 | «abbiamo il cronometro e riattiamo» | «avviamo il cronometro e riattiviamo» |
| Recupero | «il tema out» | «il timeout» |
| Recupero | «500 ms di immagine» | «500 ms di margine» |
| Recupero | «Il tempo trascorso off line, non entra» | «Il tempo trascorso offline non entra» |
| Risultati | «anti-entropy.In» | «anti-entropia. In» |

Uniforma inoltre «stato» e «stati» con iniziale minuscola all'interno delle frasi; usa «anti-entropia» nel testo italiano e conserva i nomi del codice. Scegli una forma narrativa coerente: «il test verifica», «il sistema mantiene», «il nodo recupera» rendono la relazione più uniforme rispetto all'alternanza fra «eseguiamo», «verifichiamo» e «confronta».

**Abstract proposto:**

> Il progetto realizza in Go un Service Registry distribuito e decentralizzato, nel quale ogni nodo mantiene una replica completa dello stato e gestisce autonomamente le richieste di registrazione, aggiornamento, discovery e cancellazione delle istanze. La propagazione degli aggiornamenti combina notifiche push e sincronizzazione periodica anti-entropia. Versioni logiche, risoluzione deterministica dei conflitti e tombstone consentono la convergenza delle repliche al ripristino della comunicazione. Il sistema include persistenza locale su file JSON ed è valutato mediante test di integrazione e misure end-to-end in ambiente locale e su una singola istanza EC2.

## 5. Integrazioni utili, senza aggiungere funzionalità al progetto

### 5.1 Breve sottosezione sui test unitari

**Dove:** all'inizio di «Testing», prima dei test di integrazione.

> Il progetto comprende test dei singoli componenti per il ciclo di vita delle istanze, la validazione degli input, il merge dei record e dei tombstone, l'avanzamento delle versioni, le API HTTP, i client di replica e la persistenza. Sono presenti anche verifiche sulle operazioni concorrenti. La suite ordinaria può essere eseguita con `go test ./...`; il comando `go test -race ./...` abilita il rilevamento delle data race durante l'esecuzione dei test. Gli esperimenti di convergenza richiedono l'abilitazione esplicita tramite `RUN_CONVERGENCE_TESTS=1` e un ambiente Docker predisposto.

Riporta «test superati» o un valore di copertura solo disponendo dell'output della relativa esecuzione. La presenza di un test non dimostra che sia stato eseguito con successo.

### 5.2 Esempio API

**Dove:** dopo la tabella HTTP.

Mostra un corpo di registrazione, per esempio `{"address":"10.0.0.1","port":9001}`, e descrivi sinteticamente gli esiti: `201` per creazione/riattivazione di un'istanza, `200` per aggiornamento e discovery, `204` per cancellazione riuscita, `400` per input non valido e `404` per cancellazione di un elemento assente. Una discovery senza risultati restituisce `200` con `[]`.

### 5.3 Limiti e sviluppi futuri

**Dove:** breve sezione finale.

> L'implementazione utilizza peer configurati staticamente e scambia snapshot completi, con un costo di comunicazione crescente al crescere dello stato. I tombstone sono conservati senza una procedura di eliminazione definitiva. Non sono previsti heartbeat o TTL per rilevare automaticamente l'indisponibilità delle istanze registrate. La persistenza periodica non garantisce il salvataggio immediato di ogni scrittura confermata. Il deployment su un unico host consente di testare guasti dei singoli processi, ma non la perdita dell'intera macchina.

Questi sono limiti da dichiarare, non richieste di implementazione. Per decidere se una funzionalità mancante sia obbligatoria servirebbe la traccia del progetto.

Se vuoi migliorare davvero la valutazione sperimentale, le priorità sono conservare i campioni grezzi, ripetere le misure in condizioni documentate e isolare i meccanismi di replica. Per valutare genericamente il tempo di recupero, varia anche il momento della riattivazione rispetto al timer invece di affidarti soltanto a una sequenza ripetitiva.

## 6. Impaginazione e LaTeX

1. **Tabella API, righe 122–135:** contiene percorsi lunghi e tre colonne centrate. Nel formato IEEE a due colonne può oltrepassare la larghezza disponibile. Valuta una tabella su due colonne (`table*`) o una struttura con testo a capo. Aggiungi una `\caption{Endpoint HTTP del registry}` prima di `\label{tab:API}`.
2. **Tabella EC2, riga 249:** hai sette colonne dichiarate (`{lrrrrrr}`), ma sei celle per riga. Usa `{lrrrrr}`, come nella tabella locale.
3. **Tabelle dei risultati:** usa «P95» o «95° percentile» in modo coerente. Arrotonda, per esempio, a tre cifre decimali in millisecondi: molte cifre non corrispondono alla precisione effettiva dell'osservazione. Conserva i valori originali nei log.
4. **Definizione P95:** il codice usa il metodo del rango superiore (`ceil(0,95 × n)`); con 20 campioni il P95 è il diciannovesimo valore ordinato. Puoi specificarlo in una frase metodologica. La mediana è la media dei due campioni centrali.
5. **Interruzioni di riga:** riduci `\\` e `\vspace{...}` usati per separare i paragrafi. Nel testo ordinario preferisci una riga vuota nel sorgente. La sequenza di frecce della replica è molto lunga per una colonna: dividila su più righe o trasformala in un piccolo schema.
6. **Listati:** porta la parentesi finale di `ServiceRecord` su una riga separata, uniforma l'indentazione e indica che i tag JSON sono omessi per brevità. Verifica che la configurazione `listings` del tuo ambiente riconosca `language=Go`; in caso di errore del compilatore occorre definirlo o usare un linguaggio supportato. Non ho compilato il sorgente allegato.
7. **Preambolo:** per un testo italiano valuta `\usepackage[italian]{babel}`. Rimuovi il ritorno a capo superfluo nel titolo e le parti residue del template che non usi, come il comando relativo ai finanziamenti se non pertinente.

## Ordine consigliato per applicare le modifiche

1. Correggi convergenza, tabella API e test aggiornamento/cancellazione.
2. Verifica provenienza e configurazione dei risultati; sostituisci il commento conclusivo.
3. Integra le precisazioni su replica, persistenza e singolo host EC2.
4. Sostituisci i paragrafi dei test con le riscritture e correggi i refusi rimanenti.
5. Aggiungi test unitari e limiti; controlla il PDF compilato per tabelle e listati.

Il nucleo del report è valido. Le modifiche proposte servono a rendere distinguibili ciò che il codice implementa, ciò che i test verificano e ciò che le misure permettono effettivamente di concludere.
