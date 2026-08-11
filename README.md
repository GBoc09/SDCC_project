# SDCC_project
Progetto B1 di Sistemi Distribuiti e Cloud Computing A.A 2025/2026. 

### Descrizione 
Il progetto ha l'obiettivo di realizzare un sistema distribuito in **Go** che implementi un **Service Registry decentralizzato e Tollerante ai guasti**. 

Il sistema permetterà ai servizi di registrare, aggiornare, rimuovere e ricercare i propri endpoint senza dipendere da un sistema centrale. 

Ogni nodo del registry manterrà una copia delle informazioni sui servizi disponibili e collaborerà con gli altri nodi per propagare gli aggiornamenti e garantire la convergenza dei dati. 

### Obiettivi principali 
Il sistema deve soddisfare i seguenti requisiti: 
- decentralizzazione del service registry
- eliminazione dello SPOF 
- registrazione dinamica delle istanze dei servizi 
- rimozione dinamica delle istanze 
- discovery degli endpoint disponibili
- tolleranza ai guasti o all'irragiungibilità temporanea di uno o più nodi
- consistenza finale delle informazioni replicate
- esecuzione del sistema tramite Docker Compose 
- deployment su un'istanza EC2 di Amazon

### Architettura del sistema 
Il sistema è distribuito: ogni nodo può essere contattato da un client e gli aggiornamenti eseguiti all'interno di un nodo vengono propagati agli altri peer tramite gossip.

Un insieme di nodi crea il Service Registry; ciascun nodo contiene una copia del registro dei servizi. I nodi del registry comunicano con le istanze dei servizi applicativi, che forniscono le funzionalità vere e proprie.
I client comunicano con i nodi del SR in modo che possano ricevere gli indirizzi delle istanze che gli interessano. 

Esistono due tipi di record all'interno del sistema: 
1. **record del servizio** --> (nome, stato, versione, nodoOrigine)
2. **record dell'istanza** --> (nomeServizio, ID, endpoint, stato, versione, nodoOrigine)

#### Registrazione
Il servizio comunica: 
- nome del servizio; 
- ID univoco dell'istanza; 
- indirizzo; 
- porta; 

#### Discovery 
Dato il nome di un servizio, il registry restituisce tutte le sue istanze attive. 

#### Rimozione 
Il sistema consente di rimuovere sia una singola istanza sia un intero servizio. La rimozione di un’istanza genera un tombstone relativo al suo identificativo. La rimozione di un servizio genera un tombstone associato al nome del servizio e rende inattive tutte le sue istanze. I tombstone vengono propagati tra i nodi del registry e impediscono che informazioni obsolete possedute da nodi temporaneamente offline vengano reintrodotte. Una registrazione successiva, dotata di una versione più recente, può riattivare il servizio.

### Modello dei dati 
I servizi sono definiti tramite i seguenti parametri: 
- ID istanza 
- nome servizio 
- endpoint (address, port)
- stato (active, deleted)
- versione
- nodo che ha generato la modifica più recente
- data dell'ultimo aggiornamento 

I servizi vengono identificati tramite la chiave univoca composta dalla coppia *(ID, nomeServizio)* 

**Risoluzione Conflitti**
Se si verificano conflitti, la loro risoluzione avviene tramite la verifica dei seguenti parametri *(versione, IDnodo)*. 
La regola stabilisce che: 
- prima si confronta la versione --> la versione maggiore vince 
- se le versioni sono uguali si confronta ID del nodo
- anche le rimozioni incrementano la versione 

Ogni record possiede una propria versione. Il nodo incrementa la versione quando modifica il record.


**Peer non raggiungibile** 
Quando un nodo del registry riceve una modifica, la applica alla propria copia locale e tenta di propagarla ai peer. Se un peer è temporaneamente irraggiungibile, l’operazione locale non viene bloccata. Quando il peer torna disponibile, recupera gli aggiornamenti mancanti mediante una sincronizzazione anti-entropy.

### API 
**Registrazione/Aggiornamento** --> PUT /services/{name}/instances/{ID} --> Istanza registrata / aggiornata 

**Discovery** --> GET /services/{name} --> istanze attive con quel nome 

**DELETE** --> DELETE /services/{name}/instances/{ID} --> rimozione di una singola istanza

**DELETE** --> DELETE /services/{name} --> rimozione dell'intero servizio con quel nome

**Stato del nodo** --> GET /health --> ritorna se il nodo è attivo


### Test 
1. registrazione nuova istanza 
2. aggiornamento istanza esistente 
3. registrazione di due istanze nello stesso servizio 
4. discovery di un servizio inesistente 
5. rimozione di un'istanza 
6. rimozione ripetuta
7. registrazione successiva alla rimozione 
8. richieste contemporanee 
9. richiesta con indirizzo o porta non validi 
10. rimozione di un servizio 
11. registrazione di un nuovo servizio


Per avviare il nodo:

```sh
go run .
```

Per impostazione predefinita il nodo usa l'identificativo `registry-1` e ascolta sulla porta `8080`. I valori possono essere configurati tramite `NODE_ID` e `HTTP_ADDRESS`.

Per eseguire i test:

```sh
go test ./...
go test -race ./...
```
