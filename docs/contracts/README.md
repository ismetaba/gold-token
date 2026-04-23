# GOLD Akıllı Sözleşme Spesifikasyonu

**v0.1 · Nisan 2026 · GİZLİ**

Bu doküman, [`system-design.md`](../system-design.md) Bölüm 4'ün derinlemesine teknik spesifikasyonudur. Beş çekirdek sözleşmenin — GoldToken, ComplianceRegistry, MintController, BurnController, ReserveOracle — davranışını, invaryantlarını ve etkileşimini tanımlar.

---

## İçindekiler

1. [Tasarım İlkeleri](#1-tasarım-İlkeleri)
2. [Sözleşme Topolojisi](#2-sözleşme-topolojisi)
3. [GoldToken](#3-goldtoken)
4. [ComplianceRegistry](#4-complianceregistry)
5. [ReserveOracle](#5-reserveoracle)
6. [MintController](#6-mintcontroller)
7. [BurnController](#7-burncontroller)
8. [Roller ve İzinler](#8-roller-ve-İzinler)
9. [Upgrade Akışı](#9-upgrade-akışı)
10. [Kritik İnvaryantlar](#10-kritik-İnvaryantlar)
11. [Test Planı](#11-test-planı)
12. [Güvenlik Konuları](#12-güvenlik-konuları)
13. [Açık Kararlar](#13-açık-kararlar)

---

## 1. Tasarım İlkeleri

1. **Güvenin ana hat üzerinde olması**: Her transfer, mint ve burn en az bir uyum kapısından geçer.
2. **Değişmez denetim geçmişi**: `ReserveOracle` upgradable DEĞİL. Denetim geçmişini değiştirme vektörü yok.
3. **En az yetki**: Her rol tek bir sorumluluğa sahip. Tek bir anahtar ele geçirilmesi sistemi batırmaz.
4. **PoR-kapılı basım**: `MintController.executeMint()` rezerv tazeliği (maxAge) ve invaryant (supply ≤ attested) kontrolü yapmadan token basmaz.
5. **Checks-Effects-Interactions**: Durum güncellemeleri dış çağrılardan önce yapılır. ReentrancyGuard tüm mutatif dış çağrılarda.
6. **Açıkça okunabilir hatalar**: `Errors.sol` içinde custom error'lar, parametreli. "Neden başarısız?" sorusu her zaman net cevaplanır.
7. **Chain-agnostic token, chain-specific denetim**: GOLD birçok zincirde yaşar ama PoR tek zincirde (Ethereum mainnet); diğer zincirler köprü kilidi ile sınırlandırılır.

---

## 2. Sözleşme Topolojisi

```
┌──────────────────────────────────────────────────────────────────────┐
│                      Treasury Safe (Gnosis 3/5)                      │
│           DEFAULT_ADMIN_ROLE + TREASURY_ROLE + UPGRADER_ROLE         │
└───────────┬─────────────┬─────────────┬─────────────┬─────────────┬──┘
            │             │             │             │             │
            ▼             ▼             ▼             ▼             ▼
    ┌──────────────┐ ┌────────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
    │  GoldToken   │ │ Compliance │ │  Reserve │ │  Mint    │ │  Burn    │
    │  (UUPS)      │ │  Registry  │ │  Oracle  │ │  Ctrl    │ │  Ctrl    │
    │              │ │  (UUPS)    │ │ (immut.) │ │  (UUPS)  │ │  (UUPS)  │
    └──────┬───────┘ └─────┬──────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘
           │               ▲              ▲            │            │
           │ _update hook  │              │            │            │
           └───────────────┘              │            │            │
                                          │            │            │
                                          │  read      │  mint call │  burnFrom call
                                          └────────────┤            │
                                                       │            │
           ┌───────────────────────────────────────────┴────────────┘
           │
           ▼
    ┌──────────────┐
    │   GoldToken  │  (call graph: Mint/Burn Ctrl → Token.mint/burnFrom)
    └──────────────┘
```

---

## 3. GoldToken

**Dosya:** [`src/GoldToken.sol`](../../contracts/src/GoldToken.sol)
**Kind:** UUPS Upgradable ERC-20 + Permit + Pausable + AccessControl
**Symbol:** `GOLD`
**Decimals:** 18
**Birim:** 1 GOLD = 1 gram %99.99 altın

### 3.1 Temel davranış

| Operasyon | Kim yapabilir | Yan etki |
|---|---|---|
| `transfer` / `transferFrom` | KYC'li her cüzdan | Compliance check → state update |
| `mint` | Sadece `mintController` | Token arzını artırır |
| `burnFrom` | Sadece `burnController` | Arzı azaltır (allowance tüketir) |
| `pause` | `PAUSER_ROLE` | Tüm transfer/mint/burn durur |
| `unpause` | `TREASURY_ROLE` | Operasyonu sürdürür |
| Registry/Controller değişikliği | `TREASURY_ROLE` | Compliance veya controller swap |

### 3.2 `_update` hook'u

OZ v5'teki tek giriş noktası. Mint (from=0) ve burn (to=0) durumlarında compliance atlanır — bu kontroller `canMint` / `canBurn` üzerinden controller'larda yapılır. Normal transfer için `canTransfer` çağrılır; başarısızlık durumunda spesifik custom error döndürülür.

### 3.3 Storage layout (ERC-7201)

Namespaced storage — upgrade sırasında çarpışma yok. Mevcut slot placeholder; mainnet öncesi `cast keccak "gold.token.storage"` ile hesaplanacak.

### 3.4 Permit (EIP-2612)

DEX ve aggregator entegrasyonu için tek imza approve. Domain separator `"GOLD Token" / "1"`.

---

## 4. ComplianceRegistry

**Dosya:** [`src/ComplianceRegistry.sol`](../../contracts/src/ComplianceRegistry.sol)
**Kind:** UUPS Upgradable

### 4.1 Durum

Her cüzdan için `WalletProfile`:

```
tier:            NONE | BASIC | ENHANCED | INSTITUTIONAL
jurisdiction:    bytes2 (ISO-3166)
kycApprovedAt:   timestamp
kycExpiresAt:    timestamp
frozen:          bool
sanctioned:      bool
```

Ek durum:
- `jurisdictionBlocked[bytes2]` — örn. "US" bloklanabilir
- `travelRuleApproved[hash(from,to,amount)]` — eşik üstü transferler için
- `travelRuleThreshold` — varsayılan 1000 GOLD (wei cinsinden)

### 4.2 `canTransfer` akışı

```
1. Her iki taraf da frozen değilse
2. Her iki taraf da sanctioned değilse
3. Her iki taraf da izinli jurisdiction'sa
4. Her iki taraf da geçerli KYC'liyse
5. Amount < travelRuleThreshold VEYA counterparty onayı kayıtlıysa
→ true
```

### 4.3 Travel Rule

Tutar eşiği aşıyorsa `recordTravelRuleApproval` ile counterparty VASP'ten gelen IVMS 101.202 mesajının hash'i zincir üstünde kayıt altına alınır. Ancak mesajın KENDİSİ off-chain TRP protokolü ile VASP'ler arasında iletilir. On-chain sadece "var ve doğrulandı" kanıtı.

**Sorumluluk:** KYC backend, TRP gateway (Notabene) üzerinden mesaj alıp doğruladıktan sonra bu kaydı yazar.

### 4.4 Güvenlik notları

- Profil yazımı yalnızca `KYC_WRITER_ROLE` — bu rol KYC Service backend'in kısa ömürlü anahtarında.
- Freeze/unfreeze yalnızca `COMPLIANCE_OFFICER_ROLE` — insan denetçi, ayrı hardware wallet.
- Batch güncelleme fonksiyonu eklenebilir (gas tasarrufu); skeleton'da tekil.

---

## 5. ReserveOracle

**Dosya:** [`src/ReserveOracle.sol`](../../contracts/src/ReserveOracle.sol)
**Kind:** Immutable (UPGRADABLE DEĞİL)

### 5.1 Neden immutable?

Denetim geçmişinin değişmezliği, sistemin güven anlatısının **temel taşıdır**. Bir upgradable denetim kontratı, teoride yanlış bir geçmiş yazmanın kapısını açar. Bunun yerine:

- `ReserveOracle` immutable deploy edilir
- Bug varsa yeni `ReserveOracleV2` deploy edilir
- `MintController.setOracle()` ile swap
- Eski oracle hala zincirde, eski atestasyonlar hala okunabilir (veri sürekliliği)

### 5.2 Attestation struct

```solidity
struct Attestation {
    uint64 timestamp;        // zincirde yayın anı
    uint64 asOf;             // denetimin referans tarihi
    uint256 totalGrams;      // wei cinsinden (gram * 1e18)
    bytes32 merkleRoot;      // çubuk bazında ağaç
    bytes32 ipfsCid;         // tam rapor paketi
    address auditor;         // imzalayan denetçi firma adresi
}
```

### 5.3 Merkle çubuk ağacı

Her leaf:
```
keccak256(abi.encode(
    barSerial: bytes32,      // rafineri seri no hash
    weightGrams: uint256,    // wei
    purity: uint16,          // 9999 = %99.99
    vaultCode: bytes4,       // "TRCH","TRIS","CHZH","AEDU"
    refinerLBMAId: bytes32   // LBMA Good Delivery listesinde
))
```

Kullanıcı `verifyBarInclusion(index, leaf, proof)` ile zincirde doğrulayabilir. Frontend bu proof'u otomatik üretir.

### 5.4 EIP-712 imzası

Atestasyonlar denetçinin hardware wallet'ından EIP-712 ile imzalanır. Hem on-chain doğrulama hem de **hukuki delil** sağlar — İsviçre hukuku altında.

Domain: `"GOLD ReserveOracle" / "1"`.

### 5.5 Monotonicity

- `timestamp` ve `asOf` önceki atestasyondan büyük olmalı
- `timestamp` ≤ `block.timestamp + 1 saat` (gelecek saldırı engeli)

### 5.6 Stale detection

```
timeSinceLatest() → MintController maxReserveAge ile karşılaştırır
```

Denetçi ayda bir yayınlamazsa `maxReserveAge=35d` geçtikten sonra mint durur. **Bu bir feature.**

---

## 6. MintController

**Dosya:** [`src/MintController.sol`](../../contracts/src/MintController.sol)
**Kind:** UUPS Upgradable

### 6.1 Akış

```
┌─────────┐      ┌─────────┐      ┌─────────┐      ┌─────────┐
│ Propose │ ───▶ │ Approve │ ───▶ │ Execute │ ───▶ │  Mint   │
│ (role:  │      │ (3/5    │      │ (role:  │      │ (Token) │
│Proposer)│      │ approv.)│      │Executor)│      │         │
└─────────┘      └─────────┘      └─────────┘      └─────────┘
                                       │
                                       ▼
                              ┌────────────────┐
                              │ Invaryant      │
                              │ Check: supply  │
                              │ + amount ≤     │
                              │ attestedGrams  │
                              └────────────────┘
```

### 6.2 Rol ayrımı — 3 ayrı aktör

| Rol | Kim | Sayı | Anahtar |
|---|---|---|---|
| PROPOSER | Mint/Burn Service backend | 1 | Backend HSM |
| APPROVER | İnsan imzacılar | 5 | Ayrı hardware wallet'lar |
| EXECUTOR | Relayer (veya insan) | 1 | Ops HSM |

Bir aktörün 2+ rolü alamaması **yazılı politika**; kontrat teknik olarak engellemiyor. Treasury Safe bu ilkeyi uygulamalı.

### 6.3 `proposalId = allocationId`

Off-chain mint/burn service her talep için UUID üretir. Bu UUID proposal ID olur. Avantaj:
- Çifte mint engellenir (allocationUsed kaydıyla)
- Off-chain izlenebilirlik (order_id ile doğrudan eşleme)

### 6.4 Invaryant yakalama anı

Execute sırasında. Propose veya approve anında DEĞİL. Nedeni: arada PoR güncellenmiş olabilir, supply değişmiş olabilir. Gerçek kontrol mint'in tam çalıştırılacağı an yapılmalı.

### 6.5 Rezerv tazeliği

`maxReserveAge` varsayılan 35 gün — aylık denetim + 5 gün tolerans. Denetçi "geç kalırsa" mint otomatik durur. Treasury tolerance'ı ayarlayabilir ama 90 gün gibi yüksek değer büyük kırmızı bayrak.

### 6.6 Cancel

İki yol:
- Proposer kendi teklifini iptal edebilir (ops hatası)
- `COMPLIANCE_OFFICER_ROLE` veya `TREASURY_ROLE` istediği teklifi iptal edebilir (uyum itirazı)

---

## 7. BurnController

**Dosya:** [`src/BurnController.sol`](../../contracts/src/BurnController.sol)
**Kind:** UUPS Upgradable

### 7.1 İki akış

**A) Kullanıcı itfa yakımı**

```
1. Kullanıcı platform üzerinden itfa talebi açar (fiat veya fiziksel)
2. Off-chain onay (KYC doğrulaması, risk skorlaması)
3. Kullanıcı GoldToken.approve(burnController, amount) yapar (UI imza ile)
4. Backend `requestRedemption()` çağırır
5. Kontrat burnFrom çalıştırır
6. Off-chain: kasada çubuk allocation release, fiat/fiziksel teslim başlar
```

Fiziksel itfa için min 1kg (configurable).

**B) Operatör burn**

Operasyon hatalarını geri almak için. Örneğin yanlış mint olmuş ve kullanıcı henüz çekmemişse. `BURN_OPERATOR_ROLE` + `COMPLIANCE_OFFICER_ROLE` imzası (EIP-712) gerekir — dual control.

### 7.2 Meta-transactions (gelecek)

Kullanıcı gas ödememeli (UX). EIP-2771 forwarder + paymaster entegrasyonu Faz 3'te.

---

## 8. Roller ve İzinler

| Role | Kontrat | Aksiyon | Önerilen adres türü |
|---|---|---|---|
| `DEFAULT_ADMIN_ROLE` | Hepsi | Rol grant/revoke | Treasury Safe 3/5 |
| `TREASURY_ROLE` | Hepsi | Parametre değişikliği, unpause | Treasury Safe 3/5 |
| `UPGRADER_ROLE` | UUPS'li hepsi | Implementation upgrade | Treasury Safe + 7d Timelock |
| `PAUSER_ROLE` | Token | Acil pause | 2/3 Ops Safe (hızlı) |
| `KYC_WRITER_ROLE` | Registry | setProfile | KYC Service HSM |
| `COMPLIANCE_OFFICER_ROLE` | Registry | freeze, Travel Rule, op burn imza | İnsan denetçi hardware |
| `MINT_PROPOSER_ROLE` | MintCtrl | proposeMint | Mint Service HSM |
| `MINT_APPROVER_ROLE` | MintCtrl | approveMint | 5 ayrı hardware wallet |
| `MINT_EXECUTOR_ROLE` | MintCtrl | executeMint | Relayer HSM |
| `BURN_OPERATOR_ROLE` | BurnCtrl | requestRedemption, operatorBurn | Burn Service HSM |
| `AUDITOR_ROLE` | Oracle | publish | Denetçi firma hardware |

### 8.1 Rol çatışmaları

Aynı adres 2+ rol alamaz — üretim politikası:
- Proposer ≠ Approver ≠ Executor
- KYC Writer ≠ Compliance Officer
- Auditor ≠ herhangi başka rol

Yazılı kural + deployment script'i assert'leri ile zorlanır.

---

## 9. Upgrade Akışı

```
1. Treasury Safe → yeni implementation deploy
2. 7-day TimelockController'a upgrade tx gönder
3. 7 gün boyunca public gözlem (OpenZeppelin Defender alert)
4. Timelock süresi dolunca → Treasury Safe execute
5. UUPS proxy yeni impl'e yönelir
```

**İsviçre hukuku:** upgrade'ler FINMA bildirimi gerektirir. 7 gün buffer FINMA inceleme penceresi.

**Geri alma:** yeni sürüm bozuksa — acil `pause` + hotfix yolu (ayrı akıllı runbook).

---

## 10. Kritik İnvaryantlar

Bu invaryantlar formal verification (Certora/Halmos) ile kanıtlanacak:

### INV-1: Rezerv İnvaryantı
```
token.totalSupply() ≤ oracle.latestAttestedGrams()
```
Her zaman geçerli olmalı. Sadece mint bu oranı bozabilir; MintController bunu check eder.

### INV-2: Tek-Kullanımlı Tahsis
```
∀ allocationId: mintedCount(allocationId) ≤ 1
```
Çifte mint imkansız.

### INV-3: Dondurulmuş Cüzdandan Transfer Yok
```
wallet.frozen → ¬ ∃ transfer from wallet
```

### INV-4: Pause Geçerli
```
paused = true → ¬ ∃ herhangi bir transfer/mint/burn
```

### INV-5: Atestasyon Monotonicity
```
attestations[i].timestamp < attestations[i+1].timestamp
attestations[i].asOf < attestations[i+1].asOf
```

### INV-6: Onay Eşiği
```
∀ executed mint: |approvers| ≥ approvalThreshold
```

### INV-7: Yetkili İmza
```
∀ published attestation: AUDITOR_ROLE(attestation.auditor)
   ∧ validSignature(attestation, signature, attestation.auditor)
```

---

## 11. Test Planı

### 11.1 Kapsam hedefi
- Line coverage > %95
- Branch coverage > %90
- Mutation test > %85 (foundry mutation testing)

### 11.2 Test kategorileri

**Birim testleri** (`test/{Contract}.t.sol`):
- Happy path
- Rol yetkisiz çağrılar
- Sıfır/sınır değerler
- Custom error doğrulama

**Integration testleri** (`test/integration/`):
- Uçtan uca mint akışı
- Uçtan uca burn/redemption akışı
- Registry → Token → Controller zinciri
- Pause/unpause koreografi

**Invariant testleri** (`test/invariant/`):
- Foundry invariant campaign — INV-1 ... INV-7
- Random call sequences

**Fuzz testleri**:
- Amount değerleri (overflow'a kadar)
- Timestamp manipülasyonu
- Multi-approver sıralama

**Formal verification** (Certora):
- INV-1 rezerv invaryantı (kritik)
- INV-2 tek-kullanımlı allocation
- İmza manipülasyonuna dayanıklılık

### 11.3 Güvenlik özel testleri

- Reentrancy (ERC777 hook'lu token deneysel)
- Storage collision (upgrade simulation)
- Selfdestruct ve delegatecall yok kontrolü
- Initialize doubling (`initializer` guard)
- Role renounce senaryoları

---

## 12. Güvenlik Konuları

### 12.1 Tehdit modeli özet

| Vektör | Etki | Azaltma |
|---|---|---|
| Özel anahtar sızıntısı (Proposer) | Sahte proposal | Approver eşiği (3/5) bunu yakalar |
| Proposer+1 Approver anlaşması | Hala 2 bağımsız approver gerekir | Ek ops denetimi |
| 3 Approver anlaşması | Sahte mint gerçekleşir | Invaryant check EXECUTE anında — attested altını aşamaz |
| Denetçi özel anahtarı | Sahte atestasyon | Multi-auditor rotasyonu (yıl 2+); çoklu-firma imzası (ileride) |
| Oracle hijack | Atestasyon manipülasyonu | Immutable kontrat + ayrı hardware wallet |
| Governance attack (DAO olursa) | N/A | DAO yok; Treasury Safe merkezi |
| Frontend compromise | Kullanıcı cüzdanı imzalar zararlı tx | EIP-712 tipli imza, bilinen domain, phishing eğitim |

### 12.2 Bilinen kısıtlamalar (v0.1)

1. **Storage slot placeholder'ları** — ERC-7201 hash'leri henüz hesaplanmadı (kodda `0xa1a1...` vb.)
2. **BurnController deadline kontrolü** eksik (EIP-712 imzada nonce var ama deadline yok) → upgrade ile eklenecek
3. **Multi-chain PoR** henüz yok — Ethereum mainnet'e özel
4. **Oracle rotate** fonksiyonu yok (Treasury `deauthorizeAuditor` ile vurur)
5. **Rate limiting** yok (DoS): çok sayıda proposeMint çağrısı storage şişirebilir — üretim öncesi cap eklenecek

### 12.3 Denetim kontrol listesi

- [ ] OpenZeppelin audit
- [ ] Trail of Bits audit
- [ ] Spearbit audit
- [ ] Certora formal verification — INV-1 için zorunlu
- [ ] Halmos symbolic execution
- [ ] Slither static analysis temiz
- [ ] Mythril deep analysis temiz
- [ ] Echidna invariant campaign > 24 saat
- [ ] Immunefi public bounty canlı

---

## 13. Açık Kararlar

Sistem tasarım dokümanındaki 10 karar maddesi, akıllı sözleşme tarafında şöyle yansır. ✅ kesinleşmiş, ⏳ hala açık.

| # | Karar | Durum | Sözleşme etkisi |
|---|---|---|---|
| 1 | Decimals 18 | ✅ | `GoldToken.decimals() = 18` |
| 2 | Self-custody jurisdiction bazlı | ✅ | `ComplianceRegistry.jurisdiction` — backend API'de enforce |
| 3 | Min 1g operasyonel | ✅ | Token seviyesinde hayır; 1 wei altı iş yok. Backend gate. |
| 4 | Burn onayı | ⏳ | v0.2'de `BurnController` threshold routing eklenecek |
| 5 | Fiziksel itfa 1kg | ✅ | `BurnController.minPhysicalGrams = 1000 * 1e18` |
| 6 | Bridge modeli | ✅ (erteleme) | Faz 4 RFC. Sözleşmeler bridge-agnostic — etki yok |
| 7 | TR custodial-only | ✅ | `ComplianceRegistry.isJurisdictionBlocked` ile toggle edilebilir |
| 8 | KVKK veri ikametgâhı | ⏳ | On-chain sadece tag/hash; PII çözümü backend'de |
| 9 | Gas ödeme | ⏳ | v1: EIP-2612 permit var; v2: EIP-2771 forwarder eklenir |
| 10 | L2 stratejisi | ⏳ | Mainnet sadece v1'de; aynı bytecode L2'lere taşınabilir |

---

## Sonraki Adımlar

1. **`forge install` ile OZ + Solady + forge-std eklemek** — sonra `forge build` ile derlemek ve ilk test'leri çalıştırmak.
2. **ERC-7201 storage slot'larını hesapla** — 5 kontrat için `cast keccak`.
3. **MintController'a rate-limit + DoS koruması** — storage şişme vektörü.
4. **Storage layout'un upgrade güvenli olduğunu test et** — `forge test --fork-url` veya OpenZeppelin Upgrades plugin.
5. **Certora / Halmos formal verification spec'i** yaz — en az INV-1 ve INV-2 için.
6. **Backend iskelesini tasarla** — Bu doküman artık "en soldaki" sorumluluk. Backend Bölüm 6'dan devam edebilir.

---

**BELGE SONU — v0.1**
