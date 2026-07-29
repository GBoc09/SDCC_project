# SDCC_project
Progetto B1 di Sistemi Distribuiti e Cloud Computing A.A 2025/2026. 

### Descrizione 
Il progetto ha l'obiettivo di realizzare un sistema distribuito in **Go** che implementi un **Service Registry decentralizzato e Tollerante ai guasti**. 

Il sistema permetterà ai servizi di registrare, aggiornare, rimuovere e ricercare i propri endpoint senza dipendere da un sistema centrale. 

Ogni nodo del registry manterrà una copia delle informazioni sui servizi disponibili e collaborerà con gli altri nodi per propagare gli aggiornamenti e garantire la convergenza dei dati. 

## Obiettivi principali 
Il sistema deve soddisfare i seguenti requisiti: 
- decentralizzazione del service registry
- eliminazione dello SPOF 
- registrazione dinamica delle istanze dei servizi 
- rimozione dinamica delle istanze 
- discovery degli endpoint disponibili
- tolleranza ai guasti o all'irragiungibilità temporanea di uno o più nodi
- consistenza finale delle informazioni replicate
- esecuzione del sistema tramite Docker Compose 
deployment su un'istanza EC2 di Amazon

