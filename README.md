# GOLD Token

Altın destekli dijital token platformu — çok katmanlı yetki alanı stratejisi ile küresel tokenize altın sistemi.

> **Gizlilik:** Bu repo yönetim kurulu seviyesinde gizli. Dağıtmayın.

## Monorepo yapısı

```
gold-token/
├── contracts/          Solidity + Foundry — ERC-20, ComplianceRegistry, Mint/Burn, ReserveOracle
├── backend/            Go — off-chain servisler (mint-burn iskeleti)
└── docs/               Sistem tasarımı + sözleşme + backend spec'leri
```

## Hızlı bakış

| Bölüm | Durum | Dok |
|---|---|---|
| Sistem tasarımı | v0.1 ✅ | [docs/system-design.md](docs/system-design.md) |
| Akıllı sözleşmeler | v0.1 — compile + 19/19 test ✅ | [docs/contracts/README.md](docs/contracts/README.md) |
| Backend spesifikasyonu | v0.1 ✅ | [docs/backend/README.md](docs/backend/README.md) |
| Mint/Burn Service iskeleti | v0.1 — build temiz ✅ | [docs/backend/services/mint-burn.md](docs/backend/services/mint-burn.md) |

## Kurulum

```bash
# Foundry (sözleşmeler için)
curl -L https://foundry.paradigm.xyz | bash && foundryup
cd contracts && forge install && forge build && forge test

# Go (backend)
cd ../backend && go build ./...
```

## Misyon

1 token = 1 gram %99.99 altın. 4 yetki alanı (TR + CH + AE + LI). Dikey entegre rafineri + kasa → yatırımcıya kadar tam zincir kontrolü.

Rakiplere (PAXG, XAUT) karşı: daha erişilebilir (gram bazlı), daha güvenilir (çoklu jurisdiction + kendi rafineri), daha şeffaf (aylık Big Four + Merkle çubuk proof'u).
