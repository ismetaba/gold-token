# Mint/Burn Service — Spesifikasyon

**v0.1 · Nisan 2026 · GİZLİ**

Bu doküman, [`backend/README.md`](../README.md) §5.5 + §8'i (Kritik Yol: Mint Saga) detaylandırır. Servisin iç yapısı, saga durum makinesi, veritabanı şeması ve operasyonel endişeler burada.

Kod iskeleti: [`backend/services/mint-burn/`](../../../backend/services/mint-burn/)

---

## İçindekiler

1. [Sorumluluk](#1-sorumluluk)
2. [Mimari](#2-mimari)
3. [Saga Durum Makinesi](#3-saga-durum-makinesi)
4. [Veritabanı Şeması](#4-veritabanı-şeması)
5. [Event Arayüzü](#5-event-arayüzü)
6. [HTTP API](#6-http-api)
7. [Konkurans ve Tutarlılık](#7-konkurans-ve-tutarlılık)
8. [Zincir Etkileşimi](#8-zincir-etkileşimi)
9. [Compensation ve Hata Akışları](#9-compensation-ve-hata-akışları)
10. [Gözlemlenebilirlik](#10-gözlemlenebilirlik)
11. [Test Stratejisi](#11-test-stratejisi)
12. [Dağıtım](#12-dağıtım)
13. [Yol Haritası](#13-yol-haritası)

---

## 1. Sorumluluk

**Tek görevi:** Order Service'in tamamladığı fiat ödeme → zincir üstü GOLD mint'i güvenli biçimde tetiklemek. Ve tersi: kullanıcı itfa talebi → zincir üstü burn + kasadan çubuk serbest bırakma.

**Yapmadığı:** Fiat alma, KYC, fiyat beslemesi, bar fiziksel kargo. Bunlar diğer servisler.

---

## 2. Mimari

```
                ┌──────────────────┐
                │  Order Service   │
                └─────────┬────────┘
                          │ gold.order.ready_to_mint
                          ▼
┌────────────────────────────────────────────────────────────┐
│                     mintburnd                              │
│ ┌────────────┐  ┌───────────┐  ┌─────────┐  ┌────────────┐ │
│ │  Consumer  │→ │ Orchestr. │→ │  Chain  │  │   Bars     │ │
│ │ (NATS sub) │  │  (saga)   │  │ Client  │  │   Repo     │ │
│ └────────────┘  └─────┬─────┘  └────┬────┘  └─────┬──────┘ │
│                       │             │             │        │
│                       ▼             ▼             ▼        │
│                  ┌──────────┐  ┌───────────┐ ┌──────────┐  │
│                  │ Saga Repo│  │ Ethereum  │ │ PostgreSQL│ │
│                  └──────────┘  │ via       │ │ (mint.*)  │ │
│                                │ Fireblocks│ └──────────┘  │
│                                └───────────┘               │
│ ┌────────────┐                                             │
│ │ HTTP (admin)│  GET /health, GET /admin/sagas/{id}        │
│ └────────────┘                                             │
└────────────────────────────────────────────────────────────┘
                 │
                 │ gold.mint.executed / gold.mint.failed
                 ▼
           (NATS → Order Service, Notification, Compliance)
```

Tek process; iki goroutine (HTTP server + saga worker). Yatay ölçekleme: N replika, DB satır kilidi ile koordinasyon.

---

## 3. Saga Durum Makinesi

### 3.1 Mint akışı

```
CREATED
  ↓ (compliance preflight ok)
RESERVING_BARS
  ↓ (bars allocated)
PROPOSING
  ↓ (MintController.proposeMint tx confirmed)
AWAITING_APPROVALS
  ↓ (on-chain approval count ≥ 3)
EXECUTING
  ↓ (MintController.executeMint tx confirmed)
COMPLETED  ← terminal

Hata yolları (her terminaldir):
CREATED/PROPOSING/EXECUTING   → FAILED
RESERVING_BARS                → FAILED_NO_STOCK
AWAITING_APPROVALS (timeout)  → FAILED_APPROVAL_TIMEOUT
EXECUTING (invariant revert)  → FAILED_RESERVE_INVARIANT
```

Her geçiş `domain.MintStateTransition()` tarafından doğrulanır — yasadışı geçişte programmatic panic (bug).

### 3.2 Kod

- State tanımı: [`domain/saga.go`](../../../backend/services/mint-burn/internal/domain/saga.go)
- Orchestrator: [`saga/orchestrator.go`](../../../backend/services/mint-burn/internal/saga/orchestrator.go)

### 3.3 Kritik invaryantlar

| # | Invaryant | Nerede kontrol |
|---|---|---|
| I-1 | `∑ bar_allocations.allocated_wei ≤ bar.weight_wei` | DB CHECK constraint + repo |
| I-2 | `saga.allocation_id` benzersiz | DB UNIQUE index |
| I-3 | `totalSupply + amount ≤ attestedGrams` | On-chain MintController (savunma derin) |
| I-4 | Son PoR ≤ 35 gün | On-chain MintController |
| I-5 | `saga.state` terminalse `completed_at != NULL` | UpdateState'te |
| I-6 | Bir saga için bir worker | DB `FOR UPDATE SKIP LOCKED` |

---

## 4. Veritabanı Şeması

Migration: [`migrations/0001_mintburn_initial.sql`](../../../backend/migrations/0001_mintburn_initial.sql)

### 4.1 Tablolar

| Tablo | Rol | Partition |
|---|---|---|
| `mint.saga_instances` | Saga durumu + context | yok (düşük hacim — günde ~10k) |
| `mint.vaults` | Kasa konumları | yok (düşük cardinality) |
| `mint.gold_bars` | Ana envanter (her çubuk bir satır) | vault_id ile liste partition (büyürse) |
| `mint.bar_allocations` | Çubuk ↔ saga ilişkisi | allocated_at ay partition (>10M satırdan sonra) |
| `mint.outbox` | Event publish outbox | created_at günlük partition |

### 4.2 Kritik index'ler

```sql
-- Bar seçimi için kritik (mint saga'da en pahalı sorgu)
CREATE INDEX idx_bars_available ON mint.gold_bars(vault_id)
WHERE status = 'in_vault' AND allocated_sum_wei < weight_grams_wei;

-- Saga polling
CREATE INDEX idx_saga_last_step ON mint.saga_instances(last_step_at)
WHERE completed_at IS NULL;
```

### 4.3 Bar rezervasyonu sorgusu (pseudo)

```sql
BEGIN;

WITH selected AS (
    SELECT serial_no, weight_grams_wei, allocated_sum_wei,
           LEAST(weight_grams_wei - allocated_sum_wei, $remaining) AS take_wei
    FROM mint.gold_bars
    WHERE vault_id = $vault AND status = 'in_vault'
      AND allocated_sum_wei < weight_grams_wei
    ORDER BY cast_date ASC  -- FIFO
    FOR UPDATE SKIP LOCKED
    LIMIT 50  -- safety cap
)
INSERT INTO mint.bar_allocations (allocation_id, saga_id, bar_serial, allocated_wei)
SELECT $alloc, $saga, serial_no, take_wei FROM selected
WHERE take_wei > 0;

UPDATE mint.gold_bars SET allocated_sum_wei = allocated_sum_wei + a.take_wei
FROM selected a WHERE gold_bars.serial_no = a.serial_no;

COMMIT;
```

**Not:** Gerçek implementation'da tek transaction içinde remaining amount azaltılarak yeter miktara ulaşılana kadar döngü — Faz 1 `sqlc` ile yazılacak.

---

## 5. Event Arayüzü

### 5.1 Tüketilen

| Subject | Kaynak | Tetiklenen aksiyon |
|---|---|---|
| `gold.order.ready_to_mint.v1` | Order Service | Yeni mint saga oluştur |
| `gold.burn.requested.v1` | Order Service / User | Yeni burn saga oluştur (Faz 1 tamamlanacak) |
| `gold.order.cancelled.v1` | Order Service | Devam eden saga varsa compensate (iade öncesi) |

### 5.2 Yayınlanan

| Subject | Payload | Tüketiciler |
|---|---|---|
| `gold.mint.proposed.v1` | `{saga_id, proposal_id, tx_hash}` | Notification, Compliance |
| `gold.mint.executed.v1` | `{saga_id, order_id, amount_wei, tx_hash, allocation_id}` | Order (→ user completion), Notification, Reporting |
| `gold.mint.failed.v1` | `{saga_id, order_id, error_code, message}` | Order (→ refund), Compliance (alert) |
| `gold.burn.executed.v1` | `{saga_id, amount_wei, tx_hash}` | Redemption, Reporting |

### 5.3 Envelope

Ortak zarf: [`pkg/events/envelope.go`](../../../backend/pkg/events/envelope.go).
Msg-Id = EventID → NATS JetStream dedup.

---

## 6. HTTP API

Minimal — ops/admin için.

```
GET  /health                    liveness
GET  /readyz                    readiness (DB + NATS)
GET  /admin/sagas/{id}          saga detay (COMPLIANCE_OFFICER scope)
POST /admin/sagas/{id}/cancel   (Faz 1) — compliance officer imzası ile
POST /admin/sagas/{id}/retry    (Faz 1) — terminal-olmayan saga için
```

Auth: internal mTLS + JWT. Sadece ops VPN'inden erişilebilir.

---

## 7. Konkurans ve Tutarlılık

### 7.1 Worker dağıtımı

N adet `mintburnd` replika aynı anda çalışır. Her worker her `StepPollInterval` (2s) bir sorar:
```sql
SELECT ... FROM mint.saga_instances
WHERE completed_at IS NULL AND last_step_at < now() - interval '5s'
ORDER BY last_step_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED
```

`FOR UPDATE SKIP LOCKED` ile bir saga'yı aynı anda sadece bir worker işler. Failure'da başka worker pickup eder (saga'nın `last_step_at`'i eski kalır).

### 7.2 Idempotency

- **Saga düzeyinde**: `saga.allocation_id` UNIQUE. Aynı order için iki saga oluşmaz.
- **On-chain**: `MintController.proposeMint` allocation_id'yi reject eder (zaten kullanılmış).
- **Chain tx**: Her bir propose/execute'ta nonce yönetimi Fireblocks tarafında; bizden idempotent olmayan ikinci tx gelmez.

### 7.3 Zaman damgası ve sıralama

- Saga içi state geçişleri `last_step_at` ile takip edilir
- On-chain tx'in block timestamp'i ile DB'deki advance timestamp'i ~farklı olabilir; reconciliation job zincir tarafını kanonik kabul eder

---

## 8. Zincir Etkileşimi

### 8.1 Client interface

```go
type MintControllerClient interface {
    ProposeMint(ctx, req MintRequest) (txHash string, err error)
    ExecuteMint(ctx, proposalID [32]byte) (txHash string, err error)
    ProposalStatus(ctx, proposalID [32]byte) (ProposalStatus, error)
    ApprovalCount(ctx, proposalID [32]byte) (uint8, error)
}
```

### 8.2 Production implementation

- **Client**: `go-ethereum/ethclient` — RPC bağlantısı
- **Bindings**: `abigen` ile `contracts/out/*.abi.json` → Go struct'lar
- **Signer**: Fireblocks MPC API. `PROPOSE_MINT` işlemi için özel policy — otomatik onay sadece ≤ 100g için; üstü insan onayı
- **Tx onay stratejisi**:
  - Submit
  - `WaitForReceipt(ctx, 12 blocks)` — 12 blok derinliğinde kabul
  - Receipt.Status != 1 ise `ErrTxReverted`
- **Fallback**: Fireblocks kesintisinde — kritik mint'ler duracak; alarm. Anchorage ikincil bekleme için değerlendirilebilir (Faz 3+).

### 8.3 Stub implementation

Test ve local dev için: [`chain/mint_controller.go`](../../../backend/services/mint-burn/internal/chain/mint_controller.go) — `StubClient`. Testlerde `AddApproval` helper'ı ile çoklu-imza simüle edilir.

---

## 9. Compensation ve Hata Akışları

| Adım | Başarısızlık | Saga state | Compensation | Kullanıcıya |
|---|---|---|---|---|
| RESERVING_BARS | Yeterli çubuk yok | FAILED_NO_STOCK | - | Order iade; bar tahsisi zaten yok |
| PROPOSING | Propose revert | FAILED | Bar allocation release | Order iade |
| PROPOSING | RPC timeout | (retry) | - | bekleme bildirimi |
| AWAITING_APPROVALS | 4h timeout | FAILED_APPROVAL_TIMEOUT | Bar release + on-chain `cancelMint` | Order iade |
| EXECUTING | Invariant revert | FAILED_RESERVE_INVARIANT | Bar release + **kritik alarm** | Order iade + soruşturma |
| EXECUTING | Tx submit failure | (retry 5x) | - | bekleme |

Kritik `FAILED_RESERVE_INVARIANT` — PoR verisi ile envanter uyuşmuyor demek. P0 alert → manual investigation + possibly pause mint'i for all sagas.

Compensation fonksiyonu: [`orchestrator.go#compensate`](../../../backend/services/mint-burn/internal/saga/orchestrator.go).

---

## 10. Gözlemlenebilirlik

### 10.1 Metrikler (Prometheus)

```
gold_saga_total{type,state}                           counter
gold_saga_duration_seconds{type,outcome}              histogram
gold_saga_step_duration_seconds{type,state}           histogram
gold_saga_pending_gauge{state}                        gauge
gold_bar_allocations_wei_total{vault}                 counter
gold_chain_tx_total{op,status}                        counter (op=propose|execute)
gold_chain_tx_duration_seconds{op}                    histogram
```

### 10.2 Traces

OpenTelemetry span'ı her saga adımında:
- Root span: `saga.tick` (per worker tick)
- Child: `saga.step.{state}`
- Grandchild: `chain.propose_mint`, `db.reserve_bars`

Trace ID her log ve event envelope'unda (`correlation_id`).

### 10.3 Kritik alarmlar

| Alarm | Eşik | Şiddet |
|---|---|---|
| Saga stuck (>30dk single state) | 1 | P1 |
| Mint saga failure rate | >%5/15dk | P1 |
| Invariant revert | 1 | **P0** |
| Chain RPC error rate | >%10/5dk | P1 |
| Approval timeout | 1 | P2 |
| DB connection fail | 1 | P0 |

---

## 11. Test Stratejisi

### 11.1 Unit

Saga state machine geçişleri — state transition rule testleri.
Her step handler — stub client + fake repo ile.

### 11.2 Integration

`testcontainers-go` ile PostgreSQL + NATS. Full saga lifecycle — from event to mint.executed.

### 11.3 Chaos / Fault injection

- Stub chain client'a random revert ekle (failure mode tester)
- DB connection kesinti (repo timeout)
- NATS bağlantı kopma (consumer reconnect)

### 11.4 Foundry forkundan gerçek kontratla

Staging'de Sepolia fork üzerinde Anvil + gerçek `MintController` deploy → end-to-end test.

### 11.5 Invariant property

`gopter` ile: rastgele saga sequence + step interleaving → invaryantları doğrula (özellikle bar allocation conservation).

---

## 12. Dağıtım

### 12.1 Container

Multi-stage Dockerfile. Base: `gcr.io/distroless/static:nonroot` (en küçük yüzey).

```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./ && go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/mintburnd ./services/mint-burn/cmd/mintburnd

FROM gcr.io/distroless/static:nonroot
COPY --from=build /bin/mintburnd /mintburnd
EXPOSE 8081
USER nonroot
ENTRYPOINT ["/mintburnd"]
```

### 12.2 Kubernetes

- Deployment replicas: min 3 (HA)
- Liveness: `/health` her 10s
- Readiness: `/readyz` her 5s
- PDB: minAvailable 2
- HPA: CPU %70 veya custom `gold_saga_pending_gauge > 50`
- Resource: request 200m/256Mi, limit 500m/512Mi

### 12.3 Secret

- `DATABASE_URL` — External Secrets Operator (AWS Secrets Manager)
- Fireblocks API key — CSI mount (tmpfs)
- NATS creds — k8s secret (short rotation)

### 12.4 Per-arena deployment

TR/CH/AE/EU arenaları ayrı namespace'ler; her namespace kendi DB (veri ikametgâhı), kendi Fireblocks vault account'u. Tek binary — farklı env.

---

## 13. Yol Haritası

### Faz 0 (mevcut — iskelet)

- [x] Domain tipleri + saga state machine
- [x] Orchestrator skeleton
- [x] Stub chain client
- [x] Compileable Go kod
- [x] Migration (mint şeması)
- [ ] Unit test setup (`go test ./...` geçer ama henüz test yok)

### Faz 1 (ilk 6 ay, MVP)

- [ ] Tam `repo.ReserveBars` implementation (+ testler)
- [ ] Tam burn saga (`stepBurn` + BurnController client)
- [ ] Real chain client (`go-ethereum` + Fireblocks)
- [ ] Outbox worker (`pg_notify` trigger)
- [ ] NATS JetStream stream + consumer setup
- [ ] Compliance gRPC client
- [ ] `testcontainers-go` ile integration test suite
- [ ] Prometheus metrics + OpenTelemetry tracing
- [ ] Sepolia deployment + end-to-end smoke test

### Faz 2

- [ ] Invariant property tests (gopter)
- [ ] Chaos engineering (dedicated suite)
- [ ] Per-arena deployment (K8s + ArgoCD)
- [ ] Saga retry policies per failure type
- [ ] Admin UI (ops paneli — ayrı frontend)

### Faz 3+

- [ ] Formal verification hint'leri (saga state machine — TLA+?)
- [ ] Ek zincir desteği (L2, cross-chain mint)

---

**BELGE SONU — v0.1**
