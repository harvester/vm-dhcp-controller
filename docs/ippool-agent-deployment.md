---
name: ippool-agent-deployment-reconcile
description: Plan to move IPPool agents from Pods to Deployments and reconcile via operator pattern.
---

# Plan

Obiettivo: passare da agent Pod a Deployment per ogni IPPool e riconciliare con pattern operator, aggiornando API, controller, RBAC e test.

## Requirements
- Ogni IPPool crea/gestisce un Deployment agent (repliche=1) con la stessa logica di rete/affinity/args attuale.
- Reconcile idempotente: crea/aggiorna/monitor/cleanup del Deployment.
- Upgrade immagine rispettando `hold-ippool-agent-upgrade`.

## Scope
- In: IPPool controller, API/CRD status, codegen, RBAC, test.
- Out: logica DHCP/IPAM/VMNetCfg non correlata.

## Files and entry points
- `pkg/controller/ippool/controller.go`
- `pkg/controller/ippool/common.go`
- `pkg/apis/network.harvesterhci.io/v1alpha1/ippool.go`
- `pkg/codegen/main.go`, `pkg/config/context.go`
- `pkg/util/fakeclient`, `pkg/controller/ippool/controller_test.go`
- `chart/templates/rbac.yaml`, `chart/crds/network.harvesterhci.io_ippools.yaml`
- `pkg/data/data.go`

## Data model / API changes
- Sostituire `status.agentPodRef` con `status.agentDeploymentRef` (nuovo `DeploymentReference`).

## Action items
[ ] Aggiungere client/controller apps/v1 Deployment (codegen + Management) e fakeclient.
[ ] Implementare `prepareAgentDeployment` e aggiornare Deploy/Monitor/Cleanup per Deployment.
[ ] Aggiornare watch `relatedresource` su Deployment con label ippool.
[ ] Aggiornare builder/status helpers e test per Deployment.
[ ] Aggiornare RBAC per `deployments` (get/list/watch/create/update/delete).
[ ] Rigenerare codegen/CRD/bindata.

## Testing and validation
- `go test ./...` (o `go test ./pkg/controller/ippool -run TestHandler_`).
- `go generate` per rigenerare CRD/client/bindata.

## Risks and edge cases
- Breaking change CRD status.
- Strategia Deployment (Recreate vs RollingUpdate) impatta continuita DHCP.
- Namespace agent != IPPool: relazione via label, non ownerRef.

## Open questions
- Posso usare `agentDeploymentRef` al posto di `agentPodRef` anche se breaking?
- Preferisci `Recreate` o `RollingUpdate` per i Deployment agent?

---

# IPPool agent su Deployment

Questo documento descrive logica e modifiche introdotte per spostare l'agent IPPool
da Pod singolo a Deployment, con riconciliazione di tipo operator e strategia
RollingUpdate.

## Obiettivo
- Ogni IPPool crea e gestisce un Deployment dedicato (repliche=1).
- Riconciliazione idempotente che crea o aggiorna il Deployment quando cambia la configurazione.
- Upgrade immagine controllato dall'annotazione `network.harvesterhci.io/hold-ippool-agent-upgrade`.
- Migrazione centrata sul nuovo riferimento di stato `agentDeploymentRef`.

## Flusso di riconciliazione
### DeployAgent
- Recupera ClusterNetwork dalla NetworkAttachmentDefinition (label `network.harvesterhci.io/clusternetwork`).
- Calcola l'immagine desiderata (rispetta l'annotazione di hold).
- Costruisce il Deployment desiderato via `prepareAgentDeployment`.
- Se `status.agentDeploymentRef` e valorizzato:
  - Verifica UID e che il Deployment non sia in deletion.
  - Richiede selector immutabile (errore se diverge).
  - Aggiorna labels, strategy, replicas, template e container se differiscono.
  - Aggiorna `agentDeploymentRef` con namespace/nome/UID.
- Se il Deployment non esiste o lo status e vuoto, crea il Deployment e registra lo status.

### MonitorAgent
- Se `noAgent` e true, non fa nulla; se IPPool e in pausa, ritorna errore.
- Verifica esistenza, UID e immagine del container rispetto allo status.
- Verifica readiness usando `ObservedGeneration` e repliche `Updated/Available`.
- In caso di mismatch o non readiness, ritorna errore (non cancella il Deployment).

### Cleanup
- Elimina il Deployment associato e pulisce IPAM/MAC/metriche.
- Usato su pause o rimozione IPPool.

## Dettagli implementativi
- Nome Deployment derivato da `util.SafeAgentConcatName`.
- Labels: `network.harvesterhci.io/vm-dhcp-controller=agent` + labels IPPool namespace/nome.
- Pod template con annotazione Multus `k8s.v1.cni.cncf.io/networks`.
- Init container `ip-setter` per configurare `eth1`; container `agent` con probes `/healthz` e `/readyz`.
- Container defaults (ImagePullPolicy e TerminationMessage*) esplicitati per evitare reconcile loop.
- Strategia RollingUpdate: `maxUnavailable=0`, `maxSurge=1`.

## Cambiamenti di stato/CRD
- `status.agentPodRef` sostituito da `status.agentDeploymentRef`.
- Nuova struct `DeploymentReference` con namespace, name, image, UID.
- CRD aggiornata e bindata rigenerata.

## Watch e relazione risorse
- Watch su Deployment con label `vm-dhcp-controller=agent`.
- Mapping IPPool tramite label `ippool-namespace` e `ippool-name`.

## RBAC
- Aggiunti permessi `deployments` per controller (get/list/watch).
- Role dedicato `*-deployment-manager` con create/update/delete.
- Binding aggiornato da `manage-pods` a `manage-deployments`.

## Codegen e generated
- Aggiunto gruppo apps/v1 alla codegen per controller/cache Deployment.
- Nuovi controller generati in `pkg/generated/controllers/apps`.
- Nuovo fake client per Deployment in `pkg/util/fakeclient/deployment.go`.

## Modifiche al codice (file principali)
- `pkg/controller/ippool/controller.go`: DeployAgent/MonitorAgent/cleanup aggiornati per Deployment.
- `pkg/controller/ippool/common.go`: `prepareAgentDeployment` e builder di supporto.
- `pkg/apis/network.harvesterhci.io/v1alpha1/ippool.go`: nuovo status field e tipo reference.
- `pkg/config/context.go`: AppsFactory aggiunta al bootstrap.
- `pkg/codegen/main.go`: codegen apps/v1 e rigenerazione dei controller.
- `chart/templates/rbac.yaml` e `chart/crds/network.harvesterhci.io_ippools.yaml`: RBAC/CRD aggiornate.

## Compatibilita e migrazione
- `status.agentPodRef` non e piu letto; eventuali Pod legacy non sono piu gestiti.
- RollingUpdate con `maxSurge=1` puo generare un secondo agent temporaneo.
- Se esiste un Deployment con selector differente, la reconcile fallisce per evitare adozioni errate.

## Test
- Eseguito `go test ./...`.
