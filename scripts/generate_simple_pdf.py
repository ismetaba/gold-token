"""GOLD Token projesinin basit anlatımlı PDF özetini üretir.

Çalıştırma:
    python3 scripts/generate_simple_pdf.py

Çıktı: docs/GOLD_Token_Basit_Anlatim.pdf
"""

from pathlib import Path

from reportlab.lib.colors import HexColor
from reportlab.lib.enums import TA_JUSTIFY, TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import cm
from reportlab.platypus import (
    ListFlowable,
    ListItem,
    PageBreak,
    Paragraph,
    SimpleDocTemplate,
    Spacer,
    Table,
    TableStyle,
)

OUT = Path(__file__).resolve().parent.parent / "docs" / "GOLD_Token_Basit_Anlatim.pdf"

GOLD = HexColor("#B8860B")
DARK = HexColor("#1f1f1f")
LIGHT = HexColor("#f5e9c4")
GREY = HexColor("#555555")


def build_styles():
    base = getSampleStyleSheet()
    styles = {
        "title": ParagraphStyle(
            "title",
            parent=base["Title"],
            fontName="Helvetica-Bold",
            fontSize=26,
            textColor=GOLD,
            alignment=TA_LEFT,
            spaceAfter=6,
        ),
        "subtitle": ParagraphStyle(
            "subtitle",
            parent=base["Normal"],
            fontName="Helvetica",
            fontSize=12,
            textColor=GREY,
            spaceAfter=20,
        ),
        "h1": ParagraphStyle(
            "h1",
            parent=base["Heading1"],
            fontName="Helvetica-Bold",
            fontSize=18,
            textColor=GOLD,
            spaceBefore=18,
            spaceAfter=10,
        ),
        "h2": ParagraphStyle(
            "h2",
            parent=base["Heading2"],
            fontName="Helvetica-Bold",
            fontSize=13,
            textColor=DARK,
            spaceBefore=12,
            spaceAfter=6,
        ),
        "body": ParagraphStyle(
            "body",
            parent=base["BodyText"],
            fontName="Helvetica",
            fontSize=10.5,
            leading=15,
            textColor=DARK,
            alignment=TA_JUSTIFY,
            spaceAfter=8,
        ),
        "bullet": ParagraphStyle(
            "bullet",
            parent=base["BodyText"],
            fontName="Helvetica",
            fontSize=10.5,
            leading=14,
            textColor=DARK,
            alignment=TA_LEFT,
        ),
        "note": ParagraphStyle(
            "note",
            parent=base["BodyText"],
            fontName="Helvetica-Oblique",
            fontSize=9.5,
            leading=13,
            textColor=GREY,
        ),
    }
    return styles


def bullets(items, style):
    flows = [
        ListItem(Paragraph(text, style), leftIndent=10, value="circle")
        for text in items
    ]
    return ListFlowable(
        flows,
        bulletType="bullet",
        bulletColor=GOLD,
        leftIndent=14,
        bulletFontSize=8,
        spaceBefore=2,
        spaceAfter=8,
    )


def info_box(title, body_paragraphs, styles):
    inner = [Paragraph(f"<b>{title}</b>", styles["h2"])]
    for p in body_paragraphs:
        inner.append(Paragraph(p, styles["body"]))
    tbl = Table([[inner]], colWidths=[16 * cm])
    tbl.setStyle(
        TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, -1), LIGHT),
                ("BOX", (0, 0), (-1, -1), 0.5, GOLD),
                ("LEFTPADDING", (0, 0), (-1, -1), 12),
                ("RIGHTPADDING", (0, 0), (-1, -1), 12),
                ("TOPPADDING", (0, 0), (-1, -1), 10),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 10),
            ]
        )
    )
    return tbl


def styled_table(data, col_widths):
    tbl = Table(data, colWidths=col_widths, repeatRows=1)
    tbl.setStyle(
        TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, 0), GOLD),
                ("TEXTCOLOR", (0, 0), (-1, 0), HexColor("#ffffff")),
                ("FONTNAME", (0, 0), (-1, 0), "Helvetica-Bold"),
                ("FONTSIZE", (0, 0), (-1, -1), 9.5),
                ("ALIGN", (0, 0), (-1, -1), "LEFT"),
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ("ROWBACKGROUNDS", (0, 1), (-1, -1), [HexColor("#fffaf0"), HexColor("#ffffff")]),
                ("GRID", (0, 0), (-1, -1), 0.25, HexColor("#dddddd")),
                ("LEFTPADDING", (0, 0), (-1, -1), 6),
                ("RIGHTPADDING", (0, 0), (-1, -1), 6),
                ("TOPPADDING", (0, 0), (-1, -1), 5),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 5),
            ]
        )
    )
    return tbl


def footer(canvas, doc):
    canvas.saveState()
    canvas.setFont("Helvetica", 8)
    canvas.setFillColor(GREY)
    canvas.drawString(2 * cm, 1.2 * cm, "GOLD Token — Basit Anlatım v0.1 — GİZLİ")
    canvas.drawRightString(
        A4[0] - 2 * cm, 1.2 * cm, f"Sayfa {canvas.getPageNumber()}"
    )
    canvas.setStrokeColor(GOLD)
    canvas.setLineWidth(0.5)
    canvas.line(2 * cm, 1.6 * cm, A4[0] - 2 * cm, 1.6 * cm)
    canvas.restoreState()


def build_story(styles):
    story = []

    # Kapak
    story.append(Spacer(1, 4 * cm))
    story.append(Paragraph("GOLD Token", styles["title"]))
    story.append(
        Paragraph(
            "Altın destekli dijital token platformu<br/>"
            "Projenin basit anlatımı: nedir, nasıl çalışır, nasıl yapılır",
            styles["subtitle"],
        )
    )
    story.append(Spacer(1, 1 * cm))
    story.append(
        info_box(
            "Tek cümleyle proje",
            [
                "1 GOLD token = 1 gram %99.99 saflıkta gerçek altın. "
                "Dört yetki alanında (Türkiye, İsviçre, BAE, Liechtenstein) "
                "lisanslı olarak çalışan, her ay Big Four tarafından denetlenen "
                "ve denetim sonucunu blokzincire yazan bir tokenize altın sistemi."
            ],
            styles,
        )
    )
    story.append(Spacer(1, 1 * cm))
    story.append(
        Paragraph(
            "Bu doküman teknik olmayan okuyucu için yazıldı. "
            "Detaylı teknik tasarım için <b>docs/system-design.md</b> dosyasına bakın.",
            styles["note"],
        )
    )
    story.append(PageBreak())

    # 1. Proje nedir
    story.append(Paragraph("1. Proje Nedir?", styles["h1"]))
    story.append(
        Paragraph(
            "GOLD Token, fiziksel altını dijital paraya çeviren bir platformdur. "
            "Yatırımcı 100 dolar ile 100 dolar tutarında altın alabilir, ama "
            "bunu fiziksel kasada saklamak yerine cüzdanında bir token olarak "
            "tutar. Token, gerçek bir altın çubuğa kayıtlıdır — istediği zaman "
            "satabilir, başkasına transfer edebilir veya (1 kilogramın üstünde) "
            "fiziksel teslim alabilir.",
            styles["body"],
        )
    )

    story.append(Paragraph("Neden var?", styles["h2"]))
    story.append(
        bullets(
            [
                "<b>Erişilebilirlik:</b> Banka kasasına gitmeden, gram bazında altın alıp satabilirsiniz.",
                "<b>Şeffaflık:</b> Her ay bağımsız denetçi kasaları sayar, sonuç herkesin görebileceği şekilde blokzincire yazılır.",
                "<b>Güven:</b> Her token gerçek bir altın çubuğa bağlı — kesirli rezerv yok, %100 karşılıklı.",
                "<b>Küresel kullanım:</b> Türkiye, İsviçre, Dubai ve Avrupa'da yasal — her ülkenin kuralına uyumlu.",
            ],
            styles["bullet"],
        )
    )

    story.append(Paragraph("Rakiplerden farkı", styles["h2"]))
    story.append(
        styled_table(
            [
                ["Özellik", "GOLD", "PAXG / XAUT"],
                ["Asgari miktar", "1 gram", "1 ons (~31 gram)"],
                ["Yetki alanı sayısı", "4 (TR/CH/AE/LI)", "1"],
                ["Rafineri", "Kendi rafinerisi (Çorum)", "Üçüncü taraf"],
                ["Denetim", "Aylık Big Four + Merkle proof", "Aylık üçüncü taraf"],
                ["Çubuk doğrulama", "Cüzdandan zincire kadar izlenebilir", "Sadece toplam"],
            ],
            [5 * cm, 6 * cm, 5.5 * cm],
        )
    )

    story.append(PageBreak())

    # 2. Nasıl çalışır
    story.append(Paragraph("2. Sistem Nasıl Çalışır?", styles["h1"]))

    story.append(Paragraph("Kullanıcı bakış açısı", styles["h2"]))
    story.append(
        Paragraph(
            "Bir yatırımcının yapacağı işlem altı basit adımda anlatılabilir:",
            styles["body"],
        )
    )
    story.append(
        bullets(
            [
                "<b>Kayıt + KYC:</b> Kullanıcı uygulamaya giriş yapar, kimliğini doğrular (Türkiye'de TC kimlik, İsviçre'de pasaport, vs.).",
                "<b>Para yatırma:</b> TL, USD, EUR veya AED ile banka transferi yapar.",
                "<b>Sipariş:</b> Mesela 50 gram GOLD almak ister. Sistem güncel altın fiyatından siparişi alır.",
                "<b>Tahsis:</b> Kasadan boş bir altın çubuk bulunur, 50 gramı bu kullanıcıya tahsis edilir.",
                "<b>Mint (basım):</b> Blokzincirde 50 GOLD token üretilir ve kullanıcının cüzdanına gönderilir.",
                "<b>Kullanım:</b> Kullanıcı token'ı tutar, başkasına transfer eder, satar veya (1kg+) fiziksel teslim alır.",
            ],
            styles["bullet"],
        )
    )

    story.append(Paragraph("Sistem bakış açısı (akış)", styles["h2"]))
    story.append(
        info_box(
            "Para → Altın → Token zinciri",
            [
                "<b>1. Fiat geldi:</b> Banka, ödeme servisi sağlayıcısı üzerinden para platforma ulaşır.",
                "<b>2. Kasa tahsisi:</b> Mint/Burn Service müsait altın çubuğa kullanıcının siparişini bağlar.",
                "<b>3. Çoklu imza onayı:</b> 5 yetkilinin 3'ü mint işlemini onaylar (Hazine, Uyum, Denetçi, Teknik, CIO).",
                "<b>4. PoR kontrolü:</b> Son denetim 35 günden eski değilse mint izni verilir. Aksi halde bloklanır.",
                "<b>5. On-chain mint:</b> MintController akıllı sözleşmesi token'ı basıp kullanıcı cüzdanına gönderir.",
                "<b>6. Kayıt:</b> Tahsisat veritabanına ve event bus'a yazılır; sonraki aylık denetimde Merkle ağacına dahil olur.",
            ],
            styles,
        )
    )

    story.append(PageBreak())

    # 3. Bileşenler
    story.append(Paragraph("3. Sistem Bileşenleri (Lego Parçaları)", styles["h1"]))

    story.append(
        Paragraph(
            "Sistemi dört ana katmanda düşünebiliriz. Her katman bağımsız çalışır "
            "ama hep birlikte bir altın platformu oluşturur:",
            styles["body"],
        )
    )

    story.append(Paragraph("A. Akıllı Sözleşmeler (blokzincirde)", styles["h2"]))
    story.append(
        bullets(
            [
                "<b>GoldToken:</b> ERC-20 token; transfer, bakiye, onay işlemleri.",
                "<b>ComplianceRegistry:</b> Kimin transfer yapabileceğini denetler (KYC, dondurma listesi).",
                "<b>MintController:</b> Yeni token basımı; çoklu imza ve PoR kontrolü ile.",
                "<b>BurnController:</b> Token yakımı; satış veya fiziksel teslim için.",
                "<b>ReserveOracle:</b> Aylık denetim sonucunun değiştirilemez kaydı.",
                "<b>PriceOracle:</b> Chainlink üzerinden anlık altın fiyatı.",
                "<b>Treasury Safe (3/5):</b> Sözleşme yöneticisi — sadece çoklu imza ile değişiklik.",
            ],
            styles["bullet"],
        )
    )

    story.append(Paragraph("B. Backend Servisleri (sunucularda)", styles["h2"]))
    story.append(
        bullets(
            [
                "<b>Auth:</b> Giriş, 2FA, oturum yönetimi.",
                "<b>KYC/AML:</b> Kimlik doğrulama, yaptırım listesi taraması.",
                "<b>Wallet:</b> Saklamalı cüzdan (custodial) ve harici cüzdan bağlama.",
                "<b>Order:</b> Alım/satım/itfa siparişleri.",
                "<b>Mint/Burn:</b> Kasa tahsisinden zincir üstü mint'e kadar olan saga.",
                "<b>Price Oracle:</b> Birden fazla kaynaktan altın fiyatı toplama.",
                "<b>Proof-of-Reserve:</b> Aylık Merkle ağacı + IPFS + on-chain yayın.",
                "<b>Compliance Engine:</b> Şüpheli işlem kuralları, Travel Rule.",
                "<b>Notification + Reporting:</b> E-posta/SMS + düzenleyici raporları.",
            ],
            styles["bullet"],
        )
    )

    story.append(Paragraph("C. Fiziksel Altyapı", styles["h2"]))
    story.append(
        styled_table(
            [
                ["Lokasyon", "Görev", "Yetki Alanı"],
                ["Çorum — Rafineri", "Altın üretimi ve birincil kasa", "TR"],
                ["İstanbul — BIST KMP", "Yerel saklama", "TR"],
                ["Zürih — Brink's / Loomis", "Avrupa kasası", "CH / EU"],
                ["Dubai — DMCC / Brink's", "Körfez kasası", "AE"],
            ],
            [5 * cm, 7.5 * cm, 4 * cm],
        )
    )

    story.append(Paragraph("D. İstemci Uygulamaları", styles["h2"]))
    story.append(
        bullets(
            [
                "<b>Web (Next.js):</b> Tam fonksiyonlu — onboarding, alım/satım, portföy.",
                "<b>iOS + Android:</b> Mobil odaklı kullanım.",
                "<b>Kurumsal API:</b> Banka ve piyasa yapıcı entegrasyonu (OpenAPI 3.1).",
                "<b>verify.gold.example:</b> Kamuya açık — çubuk/cüzdan sorgulama portalı.",
            ],
            styles["bullet"],
        )
    )

    story.append(PageBreak())

    # 4. Güven nasıl sağlanır
    story.append(Paragraph("4. Güven Nasıl Sağlanır?", styles["h1"]))

    story.append(
        Paragraph(
            "Tokenize altının en zayıf noktası şudur: token'ın gerçekten karşılığı "
            "olduğunu kim garanti eder? GOLD üç katmanlı bir doğrulama ile bu "
            "soruyu cevaplar:",
            styles["body"],
        )
    )

    story.append(Paragraph("1. Tahsisli rezerv — her token bir çubuğa bağlı", styles["h2"]))
    story.append(
        Paragraph(
            "Klasik bankalar gibi 'kesirli rezerv' yoktur. Sistem 1.000 token "
            "basmışsa, kasada o kullanıcılara tahsis edilmiş 1.000 gram altın "
            "vardır — seri numarasıyla. <b>bar_allocations</b> tablosu bu "
            "bağı tutar: hangi çubuğun ne kadarı kime tahsisli.",
            styles["body"],
        )
    )

    story.append(Paragraph("2. Aylık bağımsız denetim", styles["h2"]))
    story.append(
        bullets(
            [
                "Her ayın 1'inde Big Four firması (PwC, Deloitte, EY veya KPMG) dört kasayı fiziksel olarak sayar.",
                "Her çubuğun seri no, ağırlık, saflık, rafineri LBMA kimliği kayda alınır.",
                "Bu liste Merkle ağacına dönüştürülür ve denetçi EIP-712 ile imzalar.",
                "Tam rapor IPFS'e yüklenir (kalıcı, değiştirilemez); özet hash blokzincire yazılır.",
                "<b>Sonuç:</b> Sistem 35 günden eski denetimle mint yapamaz — kontrat bloklar.",
            ],
            styles["bullet"],
        )
    )

    story.append(Paragraph("3. Çubuk bazında kullanıcı doğrulaması", styles["h2"]))
    story.append(
        Paragraph(
            "Kullanıcı <b>verify.gold.example</b> sitesine cüzdan adresini girer. "
            "Sistem cevaplar: 'Sizin 50 gramınız, Çorum kasasındaki "
            "TR-2026-00428 seri numaralı çubuktan tahsisli.' Kullanıcı bu bilgiyi "
            "ReserveOracle akıllı sözleşmesinde Merkle proof ile bağımsız "
            "doğrulayabilir — platforma güvenmek zorunda değil.",
            styles["body"],
        )
    )

    story.append(PageBreak())

    # 5. Nasıl yapılacak - yol haritası
    story.append(Paragraph("5. Nasıl Yapılacak? — Yol Haritası", styles["h1"]))

    story.append(
        Paragraph(
            "Proje altı fazda inşa edilir. Her faz öncekinin üstüne bir "
            "yetenek ekler; aceleci lansman yerine 'gerçek altın güvenliği' "
            "öncelikli adım adım büyüme.",
            styles["body"],
        )
    )

    phases = [
        (
            "Faz 0 — Temel (Ay 0–2)",
            [
                "Tech lead + 2 smart contract + 2 backend + 1 SRE işe alımı",
                "Ethereum testnet'te GoldToken + ComplianceRegistry + MintController iskeleti",
                "Yerel geliştirme ortamı (docker-compose) + CI/CD kurulumu",
                "Güvenlik tehdit modeli atölyesi",
                "KYC ve kasa tedarikçi POC (Sumsub + Fireblocks)",
            ],
        ),
        (
            "Faz 1 — MVP Türkiye Arenası (Ay 2–6)",
            [
                "Türkiye uçtan uca: kayıt → TL yatırma → mint → custodial cüzdan → satış → TL çekme",
                "Tek kasa: Çorum rafinerisi",
                "Manuel PoR (aylık tablo + manuel imza)",
                "Iç alpha test (çalışanlar)",
                "CMB (Sermaye Piyasası Kurulu) ön başvuru",
            ],
        ),
        (
            "Faz 2 — PoR Otomasyon + İsviçre (Ay 6–10)",
            [
                "Otomatik PoR: ReserveOracle deploy, Merkle ağacı, IPFS yayını",
                "Zürih kasa entegrasyonu",
                "USD/EUR/CHF para giriş-çıkış",
                "FINMA SRO başvurusu",
                "İlk dış güvenlik denetimi (OpenZeppelin)",
                "Sepolia testnet üzerinde public beta",
            ],
        ),
        (
            "Faz 3 — Mainnet Lansmanı (Ay 10–14)",
            [
                "Ethereum mainnet dağıtımı + Treasury Safe devri",
                "İkinci + üçüncü güvenlik denetimi (Trail of Bits, Spearbit)",
                "10 milyon dolar başlangıç rezervi → ilk mint",
                "TR + CH arenaları canlı",
                "1–2 borsa listesi + 1 piyasa yapıcı",
                "verify.gold.example halka açık",
            ],
        ),
        (
            "Faz 4 — Küresel Ölçek (Ay 14–24)",
            [
                "Dubai VARA lisansı + BAE arenası",
                "Liechtenstein / MiCA Avrupa pasaportu",
                "Avalanche + BNB Chain köprüleri (LayerZero OFT)",
                "DEX likidite havuzları",
                "Kurumsal API + banka ortaklıkları",
                "Şeriat uyumlu varyant",
            ],
        ),
        (
            "Faz 5 — Olgunluk (Ay 24+)",
            [
                "Layer 2 (Base / Arbitrum) genişleme",
                "Tokenize altın ETF köprüsü",
                "Altın leasing / yield varyantları",
                "Kimlik doğrulamalı açık finansal API",
            ],
        ),
    ]

    for title, items in phases:
        story.append(Paragraph(title, styles["h2"]))
        story.append(bullets(items, styles["bullet"]))

    story.append(PageBreak())

    # 6. Teknoloji seçimleri
    story.append(Paragraph("6. Teknoloji Seçimleri (Kısaca)", styles["h1"]))

    story.append(
        Paragraph(
            "Her seçim 'olgunluk + denetim havuzu + operasyonel kolaylık' "
            "ekseninde yapıldı. Hipster teknolojilerden kaçınıldı:",
            styles["body"],
        )
    )

    story.append(
        styled_table(
            [
                ["Katman", "Seçim", "Neden"],
                ["Akıllı sözleşme dili", "Solidity 0.8.24", "Audit havuzu en geniş"],
                ["SC framework", "Foundry", "Hızlı test, Rust native"],
                ["SC kütüphanesi", "OpenZeppelin + Solady", "Denetlenmiş standart"],
                ["Oracle", "Chainlink", "En olgun PoR desteği"],
                ["Köprü (Faz 4)", "LayerZero OFT", "Merkezi havuz riski yok"],
                ["Custody", "Fireblocks (MPC)", "Policy engine + HSM"],
                ["Backend dili", "Go (servisler) + TypeScript (BFF)", "Operasyon kolay"],
                ["Veritabanı", "PostgreSQL 16", "ACID + zengin özellik"],
                ["Event bus", "NATS JetStream", "Hafif, streaming"],
                ["Web frontend", "Next.js 15 + Tailwind + shadcn", "SEO + hız"],
                ["Mobile", "Swift + Kotlin + KMP shared", "Native performans"],
                ["KYC", "Sumsub + Jumio (çift)", "Arena başına en iyi"],
                ["Cloud", "AWS primary + GCP DR", "Çoklu region"],
                ["Container", "Kubernetes (EKS)", "Çoklu cloud taşınabilir"],
            ],
            [4.5 * cm, 6 * cm, 6 * cm],
        )
    )

    story.append(PageBreak())

    # 7. Güvenlik
    story.append(Paragraph("7. Güvenlik Nasıl Korunuyor?", styles["h1"]))

    story.append(
        Paragraph(
            "Tokenize altında güvenlik = kontrat güvenliği + anahtar güvenliği "
            "+ fiziksel güvenlik + uyum. Hepsinin aynı anda çalışması gerekiyor.",
            styles["body"],
        )
    )

    story.append(
        styled_table(
            [
                ["Tehdit", "Karşı Önlem"],
                ["Akıllı sözleşme exploit", "Çoklu imza + PoR gated + formal verification + 3 bağımsız denetim"],
                ["Kasa iç tehdidi", "4-göz kuralı, CCTV, dual control, çubuk-başı tag, aylık denetim"],
                ["Özel anahtar sızıntısı", "AWS CloudHSM L3 + Fireblocks MPC + cold storage"],
                ["Kullanıcı phishing", "Hardware wallet desteği, EIP-712 domain bağlama"],
                ["Oracle manipülasyonu", "Chainlink + 3+ beslemeden medyan + sapma koruyucu"],
                ["Reentrancy saldırısı", "OpenZeppelin ReentrancyGuard + checks-effects-interactions"],
                ["KYC by-pass (sybil)", "Biyometrik + belge ağı analizi + çift vendor"],
                ["API DoS", "CloudFlare + rate limit + WAF"],
                ["Regülatör baskını", "Çoklu yetki alanı + İsviçre yedek"],
                ["Tedarik zinciri (npm/go)", "Pinned deps, SBOM, Snyk, iç artifact registry"],
            ],
            [4.5 * cm, 12 * cm],
        )
    )

    story.append(Paragraph("Operasyonel kurallar", styles["h2"]))
    story.append(
        bullets(
            [
                "<b>Sıcak cüzdan:</b> Toplam arzın en fazla %1'i. Günlük operasyon için.",
                "<b>Soğuk cüzdan:</b> Kalan tüm bakiye. Çok imzalı, coğrafi dağıtılmış, geofencing alarmlı.",
                "<b>Treasury Safe imzacıları:</b> 5 kişi, 3 farklı lokasyon, 2 farklı jurisdiction. Hepsi Ledger hardware wallet.",
                "<b>Anahtar rotasyonu:</b> Yıllık + her güvenlik olayından sonra.",
                "<b>Bug bounty:</b> Immunefi üzerinden — kritik 500K USD, yüksek 100K USD.",
                "<b>İncident response:</b> RTO 1 saat, RPO 5 dakika.",
            ],
            styles["bullet"],
        )
    )

    story.append(PageBreak())

    # 8. Kim ne yapacak
    story.append(Paragraph("8. Kim Ne Yapacak? — Başlangıç Ekibi", styles["h1"]))

    story.append(
        Paragraph(
            "Faz 0–1 için önerilen başlangıç takımı (toplam 13 kişi):",
            styles["body"],
        )
    )

    story.append(
        styled_table(
            [
                ["Rol", "Adet", "Görev"],
                ["Tech Lead / Chief Architect", "1", "Genel mimari ve teknik yön"],
                ["Smart Contract Engineer", "2", "Solidity + Foundry, kontrat tasarımı ve testleri"],
                ["Backend Engineer (Go)", "3", "Mikroservisler, mint/burn, compliance"],
                ["Frontend Lead (Next.js)", "1", "Web uygulaması ve verify portalı"],
                ["Mobile Engineer", "2", "iOS (Swift) + Android (Kotlin)"],
                ["SRE / DevOps", "1", "Kubernetes, CI/CD, gözlemlenebilirlik"],
                ["Security Engineer", "1", "Tehdit modeli, audit hazırlığı, key management"],
                ["QA / Test Engineer", "1", "E2E testleri, regresyon"],
                ["Product Manager", "1", "Sipariş akışları, KYC akışları"],
                ["Compliance Tech Liaison", "1", "Hukuk + ürün arasındaki köprü"],
            ],
            [6 * cm, 1.5 * cm, 9 * cm],
        )
    )

    story.append(
        Paragraph(
            "Faz 2'den sonra eklenecek: KYC ops, müşteri destek, business "
            "development, data analyst, üç ek arenanın yerel temsilcileri.",
            styles["body"],
        )
    )

    story.append(PageBreak())

    # 9. Açık sorular
    story.append(Paragraph("9. Hala Açık Olan Sorular", styles["h1"]))

    story.append(
        Paragraph(
            "Nisan 2026 toplantısında bazı kararlar kesinleşti, bazıları "
            "düzenleyici görüşüne bağlı:",
            styles["body"],
        )
    )

    story.append(Paragraph("✅ Karara bağlananlar", styles["h2"]))
    story.append(
        bullets(
            [
                "<b>Hassasiyet:</b> 18 decimals (DeFi uyumu + alt-gram fraksiyonu).",
                "<b>Self-custody:</b> Jurisdiction bazlı — TR'de custodial-only, CH/AE/LI'de Enhanced KYC sonrası serbest.",
                "<b>Minimum alım:</b> 1 gram (sipariş bazında). DEX'te fraksiyon serbest.",
                "<b>Fiziksel teslim minimumu:</b> 1 kg (LBMA çubuk bölme maliyeti).",
                "<b>Köprü modeli:</b> Faz 4'te yeniden değerlendirilecek; default LayerZero OFT.",
            ],
            styles["bullet"],
        )
    )

    story.append(Paragraph("⏳ Hala açık", styles["h2"]))
    story.append(
        bullets(
            [
                "<b>Yakım onayı:</b> &lt;100gr otomatik, üstü onaylı (öneri, henüz onaysız).",
                "<b>KVKK veri ikametgâhı:</b> Türk kullanıcı KYC verisi yurt dışı çıkabilir mi? Hukuki görüş bekleniyor.",
                "<b>Gas ödeme modeli:</b> Kullanıcı ETH mi öder, meta-tx mi? Önerilen yol: v1 EIP-2612 permit, v2 meta-tx + paymaster.",
                "<b>L2 stratejisi:</b> Day-1 sadece Ethereum mainnet mi, Base/Arbitrum day-1 mi? Önerilen: sadece mainnet, odak.",
            ],
            styles["bullet"],
        )
    )

    story.append(PageBreak())

    # 10. Özet
    story.append(Paragraph("10. Tek Sayfada Özet", styles["h1"]))

    story.append(
        info_box(
            "GOLD nedir?",
            [
                "Her token gerçek bir gram altına eşit, dört yetki alanında "
                "lisanslı, her ay Big Four denetimli bir tokenize altın platformu."
            ],
            styles,
        )
    )
    story.append(Spacer(1, 0.3 * cm))

    story.append(
        info_box(
            "Nasıl çalışıyor?",
            [
                "Kullanıcı para yatırır → kasadaki çubuğun bir kısmı ona tahsis "
                "edilir → çoklu imza ile mint onaylanır → token cüzdana gider. "
                "Satarken / fiziksel teslimde tam tersi: token yakılır, çubuk serbest kalır."
            ],
            styles,
        )
    )
    story.append(Spacer(1, 0.3 * cm))

    story.append(
        info_box(
            "Neden farklı?",
            [
                "Çoklu yetki alanı (TR/CH/AE/LI), kendi rafinerisi, gram bazlı "
                "minimum, çubuk bazında izlenebilirlik, Merkle proof ile bağımsız doğrulama."
            ],
            styles,
        )
    )
    story.append(Spacer(1, 0.3 * cm))

    story.append(
        info_box(
            "Nasıl yapılacak?",
            [
                "6 fazlık, 24+ aylık yol haritası: önce Türkiye MVP, sonra "
                "İsviçre + otomatik PoR, sonra mainnet lansman, sonra global "
                "ölçek ve olgunluk. Her faz öncekinin üstüne bina yapar."
            ],
            styles,
        )
    )
    story.append(Spacer(1, 0.3 * cm))

    story.append(
        info_box(
            "Ne kadar sürer?",
            [
                "İlk MVP (TR arena, manuel PoR): 6 ay. Mainnet lansman "
                "(otomatik PoR + 3 güvenlik denetimi): 14 ay. Global ölçek "
                "(4 arena + çoklu zincir): 24 ay."
            ],
            styles,
        )
    )

    story.append(Spacer(1, 1 * cm))
    story.append(
        Paragraph(
            "Daha derin teknik detay için repo içindeki dokümanlara bakın: "
            "<b>docs/system-design.md</b> (972 satır tam mimari), "
            "<b>docs/contracts/README.md</b> (kontrat spesifikasyonu), "
            "<b>docs/backend/README.md</b> (backend servisi spesifikasyonu).",
            styles["note"],
        )
    )

    return story


def main():
    OUT.parent.mkdir(parents=True, exist_ok=True)
    styles = build_styles()
    doc = SimpleDocTemplate(
        str(OUT),
        pagesize=A4,
        leftMargin=2 * cm,
        rightMargin=2 * cm,
        topMargin=2 * cm,
        bottomMargin=2 * cm,
        title="GOLD Token - Basit Anlatim",
        author="GOLD Token Ekibi",
    )
    doc.build(build_story(styles), onFirstPage=footer, onLaterPages=footer)
    print(f"OK: {OUT}")


if __name__ == "__main__":
    main()
