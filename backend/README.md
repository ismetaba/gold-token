# GOLD Backend

Off-chain servisler. Go 1.22+.

> v0.1 iskelet — Mint/Burn Service scaffold'u ile başladı. Diğer servisler Faz 1 yol haritasında.

## Kurulum

```bash
go mod tidy
make build
make test
```

## Servisler

- **services/mint-burn/cmd/mintburnd** — Mint/Burn Service. [Spec](../docs/backend/services/mint-burn.md)

## Dizin yapısı

```
backend/
├── pkg/                         # shared paketler (public API, cross-service)
│   ├── obs/                     # logger + tracer
│   ├── errors/                  # coded errors
│   ├── chain/                   # Ethereum client wrapper (interface)
│   └── events/                  # NATS wrapper + event envelope
├── services/
│   └── mint-burn/
│       ├── cmd/mintburnd/main.go        # entrypoint
│       └── internal/                    # servis-iç kod (Go internal rule)
│           ├── domain/                  # entity + saga state machine
│           ├── repo/                    # PostgreSQL data access
│           ├── saga/                    # orchestrator (kritik yol)
│           ├── chain/                   # MintController client
│           ├── http/                    # admin API
│           ├── events/                  # event pub/sub
│           └── config/
└── migrations/                  # goose migrations
```

## Yerel geliştirme

PostgreSQL 16 + NATS JetStream + Anvil (Foundry) gerekir. `docker-compose.yml` Faz 1 start sırasında eklenecek.
