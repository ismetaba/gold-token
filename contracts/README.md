# GOLD Contracts

GOLD altın token için akıllı sözleşme iskelet kodu (v0.1).

> **Uyarı:** Bu kod denetimden geçmemiş, üretim için hazır değil. Testnet lansmanından önce en az 3 bağımsız güvenlik denetimi (OpenZeppelin, Trail of Bits, Spearbit) gerekli.

## Kurulum

```bash
# Foundry kurulu değilse
curl -L https://foundry.paradigm.xyz | bash && foundryup

# Bağımlılıklar
forge install OpenZeppelin/openzeppelin-contracts@v5.1.0 --no-commit
forge install OpenZeppelin/openzeppelin-contracts-upgradeable@v5.1.0 --no-commit
forge install Vectorized/solady --no-commit
forge install foundry-rs/forge-std --no-commit

# Derleme
forge build

# Test
forge test -vvv

# Kapsam
forge coverage
```

## Sözleşme Haritası

```
GoldToken           — ERC-20 token (UUPS proxy). Tüm transferler ComplianceRegistry'den geçer.
ComplianceRegistry  — KYC durumu, dondurma listesi, jurisdiction etiketi, Travel Rule kaydı.
MintController      — Çoklu imza + PoR-kapılı token basımı.
BurnController      — Kullanıcı itfa yakımı + operatör ops yakımı.
ReserveOracle       — Değişmez, append-only denetim atestasyonları.
```

## Dağıtım

Tüm upgradable sözleşmeler UUPS proxy arkasında. Proxy admin = Treasury (Gnosis Safe 3/5 + 7 gün timelock).

```bash
forge script script/Deploy.s.sol --rpc-url sepolia --broadcast
```

## Üretim Öncesi Zorunlu Liste

- [ ] ERC-7201 storage slot'ları gerçek keccak değerleriyle doldur
- [ ] 3 bağımsız güvenlik denetimi
- [ ] Formal verification (Certora veya Halmos) — mint invaryantı için
- [ ] Fuzz test > 10M run
- [ ] Invariant test > 1M depth
- [ ] Immunefi bug bounty aktif ($500k kritik)
- [ ] Treasury Safe çoklu-imza konfigürasyonu
- [ ] OpenZeppelin Defender izleme + otomatik pause kuralları
- [ ] Mainnet dağıtım runbook + acil durum playbook

Detaylı spec: [`../docs/contracts/README.md`](../docs/contracts/README.md)
