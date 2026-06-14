"""GOLD Token projesinin sohbet diliyle anlatımlı PDF özetini üretir.

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
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
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

FONT_DIR = Path("/usr/share/fonts/truetype/dejavu")
pdfmetrics.registerFont(TTFont("DejaVu", str(FONT_DIR / "DejaVuSans.ttf")))
pdfmetrics.registerFont(TTFont("DejaVu-Bold", str(FONT_DIR / "DejaVuSans-Bold.ttf")))
pdfmetrics.registerFont(TTFont("DejaVu-Mono", str(FONT_DIR / "DejaVuSansMono.ttf")))

from reportlab.pdfbase.pdfmetrics import registerFontFamily
registerFontFamily(
    "DejaVu",
    normal="DejaVu",
    bold="DejaVu-Bold",
    italic="DejaVu",
    boldItalic="DejaVu-Bold",
)

GOLD = HexColor("#B8860B")
DARK = HexColor("#1f1f1f")
LIGHT = HexColor("#f5e9c4")
GREY = HexColor("#555555")


def build_styles():
    base = getSampleStyleSheet()
    return {
        "title": ParagraphStyle(
            "title", parent=base["Title"], fontName="DejaVu-Bold",
            fontSize=26, textColor=GOLD, alignment=TA_LEFT, spaceAfter=6,
        ),
        "subtitle": ParagraphStyle(
            "subtitle", parent=base["Normal"], fontName="DejaVu",
            fontSize=12, textColor=GREY, spaceAfter=20, leading=16,
        ),
        "h1": ParagraphStyle(
            "h1", parent=base["Heading1"], fontName="DejaVu-Bold",
            fontSize=18, textColor=GOLD, spaceBefore=18, spaceAfter=10,
        ),
        "h2": ParagraphStyle(
            "h2", parent=base["Heading2"], fontName="DejaVu-Bold",
            fontSize=13, textColor=DARK, spaceBefore=12, spaceAfter=6,
        ),
        "body": ParagraphStyle(
            "body", parent=base["BodyText"], fontName="DejaVu",
            fontSize=10.5, leading=16, textColor=DARK,
            alignment=TA_JUSTIFY, spaceAfter=8,
        ),
        "bullet": ParagraphStyle(
            "bullet", parent=base["BodyText"], fontName="DejaVu",
            fontSize=10.5, leading=15, textColor=DARK, alignment=TA_LEFT,
        ),
        "note": ParagraphStyle(
            "note", parent=base["BodyText"], fontName="DejaVu",
            fontSize=9.5, leading=14, textColor=GREY,
        ),
    }


def bullets(items, style):
    flows = [ListItem(Paragraph(t, style), leftIndent=10) for t in items]
    return ListFlowable(
        flows, bulletType="bullet", bulletColor=GOLD,
        leftIndent=14, bulletFontSize=8, spaceBefore=2, spaceAfter=8,
    )


def info_box(title, body_paragraphs, styles):
    inner = [Paragraph(f"<b>{title}</b>", styles["h2"])]
    for p in body_paragraphs:
        inner.append(Paragraph(p, styles["body"]))
    tbl = Table([[inner]], colWidths=[16 * cm])
    tbl.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, -1), LIGHT),
        ("BOX", (0, 0), (-1, -1), 0.5, GOLD),
        ("LEFTPADDING", (0, 0), (-1, -1), 12),
        ("RIGHTPADDING", (0, 0), (-1, -1), 12),
        ("TOPPADDING", (0, 0), (-1, -1), 10),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 10),
    ]))
    return tbl


def styled_table(data, col_widths):
    tbl = Table(data, colWidths=col_widths, repeatRows=1)
    tbl.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), GOLD),
        ("TEXTCOLOR", (0, 0), (-1, 0), HexColor("#ffffff")),
        ("FONTNAME", (0, 0), (-1, 0), "DejaVu-Bold"),
        ("FONTNAME", (0, 1), (-1, -1), "DejaVu"),
        ("FONTSIZE", (0, 0), (-1, -1), 9.5),
        ("ALIGN", (0, 0), (-1, -1), "LEFT"),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [HexColor("#fffaf0"), HexColor("#ffffff")]),
        ("GRID", (0, 0), (-1, -1), 0.25, HexColor("#dddddd")),
        ("LEFTPADDING", (0, 0), (-1, -1), 6),
        ("RIGHTPADDING", (0, 0), (-1, -1), 6),
        ("TOPPADDING", (0, 0), (-1, -1), 5),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 5),
    ]))
    return tbl


def footer(canvas, doc):
    canvas.saveState()
    canvas.setFont("DejaVu", 8)
    canvas.setFillColor(GREY)
    canvas.drawString(2 * cm, 1.2 * cm, "GOLD Token — Basit Anlatım v0.4 — GİZLİ")
    canvas.drawRightString(A4[0] - 2 * cm, 1.2 * cm, f"Sayfa {canvas.getPageNumber()}")
    canvas.setStrokeColor(GOLD)
    canvas.setLineWidth(0.5)
    canvas.line(2 * cm, 1.6 * cm, A4[0] - 2 * cm, 1.6 * cm)
    canvas.restoreState()


def build_story(styles):
    story = []

    # ---------- Kapak ----------
    story.append(Spacer(1, 4 * cm))
    story.append(Paragraph("GOLD Token", styles["title"]))
    story.append(Paragraph(
        "Altın destekli dijital token projesi<br/>"
        "Hiç teknik bilmeyen biri de anlasın diye yazıldı",
        styles["subtitle"],
    ))
    story.append(Spacer(1, 1 * cm))
    story.append(info_box(
        "İki cümleyle bu proje ne?",
        [
            "Bir GOLD = bir gram gerçek altın. Sahip olduğun token'ın "
            "karşılığı kasada bekliyor, istediğin zaman satarsın, "
            "transfer edersin, 1 kiloyu geçtiğinde de fiziksel olarak "
            "teslim alırsın. O kadar."
        ],
        styles,
    ))
    story.append(Spacer(1, 1 * cm))
    story.append(Paragraph(
        "Not: Burada işin özünü konuşacağız. Detaylı teknik tasarıma "
        "ihtiyacın olursa repo içindeki <b>docs/system-design.md</b> "
        "var, tam 972 satır.",
        styles["note"],
    ))
    story.append(PageBreak())

    # ---------- 1. Hikaye ----------
    story.append(Paragraph("1. Önce hikaye: bu proje neden var?", styles["h1"]))
    story.append(Paragraph(
        "Diyelim ki elinde bir miktar paran var ve altın almak istiyorsun. "
        "Bugün ne yapıyorsun? Ya kuyumcuya gidiyorsun, ya bankaya. "
        "Kuyumcuda makas var, bankada saklama ücreti var, ikisinde de "
        "fiziksel altını saklamak ayrı bir dert.",
        styles["body"],
    ))
    story.append(Paragraph(
        "GOLD bu işi şöyle yapıyor: kasada gerçek altın çubuklar duruyor, "
        "biz bu çubukları senin adına bir miktar 'tahsis' ediyoruz, "
        "karşılığında telefonundaki cüzdana 'GOLD' adında bir token "
        "geliyor. Bu token bir kağıt parçası değil, gerçek bir gram "
        "altına bağlı. Hatta hangi çubuğa bağlı olduğunu seri "
        "numarasıyla bile görebilirsin.",
        styles["body"],
    ))
    story.append(Paragraph(
        "Avantajı? Telefondan alıyorsun, telefondan satıyorsun. İstersen "
        "arkadaşına gönderiyorsun. Gerçekten elinde tutmak istersen "
        "(en az 1 kilo) kargolatıyorsun. Kasa, sigorta, denetim — "
        "hepsi arka planda hallediliyor.",
        styles["body"],
    ))

    story.append(Paragraph("Peki PAXG, XAUT zaten var, sen ne farklısın?", styles["h2"]))
    story.append(Paragraph(
        "Üç şey farklı: <b>(1)</b> Onlar 1 ons üzerinden çalışıyor, biz "
        "1 gram. Yani 100 dolarlık altın almak isteyen Türk öğrenci de "
        "girebiliyor. <b>(2)</b> Onlar tek ülkede, biz dört ülkede "
        "lisanslıyız (Türkiye, İsviçre, BAE, Liechtenstein) — bir tanesi "
        "kapanırsa diğeri ayakta. <b>(3)</b> Onların altını üçüncü taraf "
        "rafineriden geliyor, bizim Çorum'da kendi rafinerimiz var. "
        "Yani altın bizim, kasa bizim, token bizim. Baştan sona kontrol.",
        styles["body"],
    ))

    story.append(PageBreak())

    # ---------- 2. Kullanıcı ne yaşıyor ----------
    story.append(Paragraph("2. Kullanıcı tarafından ne yaşanıyor?", styles["h1"]))
    story.append(Paragraph(
        "En sade haliyle anlatayım. Sen uygulamaya giriyorsun, ben de "
        "sistem tarafında ne olduğunu yan yana söyleyeyim:",
        styles["body"],
    ))
    story.append(bullets([
        "<b>Önce kayıt:</b> E-posta, telefon, sonra kimlik fotoğrafı + selfie. Türkiye'deysen MERNİS'e bakılıyor, İsviçre'deysen pasaport okunuyor. Bu işlem 'KYC' deniyor — yani 'müşterini tanı'. Bir kere yapıyorsun, bitiyor.",
        "<b>Sonra para yatırma:</b> Banka havalesi, kart, ne kolayına geliyorsa. TL, USD, EUR, AED — hangi arenadansan o para birimi.",
        "<b>Sipariş veriyorsun:</b> 'Bana 50 gram GOLD ver' diyorsun. Sistem o anki altın fiyatını gösteriyor, onaylıyorsun.",
        "<b>Arka planda kasaya gidiliyor:</b> Müsait bir çubuk seçiliyor, 50 gramı senin adına işaretleniyor. Bu kayda 'tahsisat' diyoruz.",
        "<b>5 kişiden 3'ü 'tamam' diyor:</b> Hazine, uyum müdürü, denetçi, teknik, CIO — bunlardan üçü onaylamadan token basılmıyor. Tek kişinin yetkisi yok, kötü niyetli birinin sistemi çalması imkânsız.",
        "<b>Token cüzdanına geliyor:</b> Ethereum üzerinde mint işlemi yapılıyor, 50 GOLD senin cüzdanına düşüyor. Tüm bu süre 2–3 dakika.",
        "<b>İstediğini yap:</b> Sat, gönder, beklet, ya da 1 kiloyu aştığında 'fiziksel istiyorum' de — Brink's ile evine kargolatalım.",
    ], styles["bullet"]))

    story.append(Paragraph("Satarken ne oluyor?", styles["h2"]))
    story.append(Paragraph(
        "Tersini yapıyoruz. Sen 'satıyorum' diyorsun, token cüzdanından "
        "alınıp 'yakılıyor' (burn). Yakılma kelimesinden korkma — "
        "Ethereum'da bir token'ı sonsuza kadar yok etmek demek bu. "
        "Karşılığında kasada o gramlık çubuk tekrar 'boşa çıkıyor' ve "
        "sana hesabına paran yatıyor.",
        styles["body"],
    ))

    story.append(PageBreak())

    # ---------- 3. Sistemin parçaları ----------
    story.append(Paragraph("3. Sistemin parçaları neler?", styles["h1"]))
    story.append(Paragraph(
        "Sistemi 4 katmana bölelim. Her birinin işi farklı ama "
        "hepsi birbiriyle konuşuyor:",
        styles["body"],
    ))

    story.append(Paragraph("A. Blokzincirdeki akıllı sözleşmeler", styles["h2"]))
    story.append(Paragraph(
        "Bunlar Ethereum'da çalışan, Solidity diliyle yazılmış kod "
        "parçaları. Bir kere yazıldıktan sonra kimse müdahale "
        "edemiyor, herkes okuyabiliyor:",
        styles["body"],
    ))
    story.append(bullets([
        "<b>GoldToken:</b> Asıl token. Transfer et, bakiye gör, onay ver — temel ERC-20 işleri.",
        "<b>ComplianceRegistry:</b> Kimin alıp satabileceğini tutan defter. KYC yapmamış birine token gitmez.",
        "<b>MintController:</b> Yeni token basımı. Çoklu imza olmadan çalışmaz, denetim 35 günden eskiyse de durur.",
        "<b>BurnController:</b> Token yakımı. Satış ya da fiziksel teslim olduğunda devreye girer.",
        "<b>ReserveOracle:</b> Aylık denetim sonucunu blokzincire yazan defter. Bir kez yazıldı mı silinmiyor.",
        "<b>PriceOracle:</b> Chainlink üzerinden anlık altın fiyatı.",
        "<b>Treasury Safe:</b> Sistemin patronu, ama tek patron değil — 5 kişiden 3'ünün imzası gerek.",
    ], styles["bullet"]))

    story.append(Paragraph("B. Backend (yani sunucu) servisleri", styles["h2"]))
    story.append(Paragraph(
        "Blokzincir her şeyi yapamıyor — kimlik doğrulamak, bankayla "
        "konuşmak, kasaya komut göndermek gibi işler için klasik "
        "sunucu servisleri lazım. Go diliyle yazılmış mikroservisler "
        "var, her biri tek bir işi yapıyor:",
        styles["body"],
    ))
    story.append(bullets([
        "<b>Auth:</b> Giriş, 2 faktörlü doğrulama, oturum yönetimi.",
        "<b>KYC/AML:</b> Kimliğini doğrular, yaptırım listelerine bakar.",
        "<b>Wallet:</b> Sana saklamalı cüzdan açar ya da kendi cüzdanını bağlatır.",
        "<b>Order:</b> Alış/satış/itfa siparişlerini takip eder.",
        "<b>Mint/Burn:</b> Kasa ile blokzincir arasındaki köprü. En kritik servis.",
        "<b>Price Oracle:</b> Birden fazla kaynaktan altın fiyatı toplar, manipülasyonu yakalar.",
        "<b>Proof-of-Reserve:</b> Aylık denetim verisini toplar, Merkle ağacı kurar, IPFS'e atar.",
        "<b>Compliance Engine:</b> Şüpheli işlemleri tarar, büyük transferlerde 'Travel Rule' uygular.",
        "<b>Notification + Reporting:</b> E-posta/SMS gönderir, devlete rapor üretir.",
    ], styles["bullet"]))

    story.append(Paragraph("C. Fiziksel altyapı — kasalar", styles["h2"]))
    story.append(Paragraph(
        "Bu kısım çoğu kişinin atladığı yer: altın gerçek bir yerde "
        "duruyor. Tek kasa yok, dört yerde dağıtık:",
        styles["body"],
    ))
    story.append(styled_table([
        ["Lokasyon", "Görev", "Arena"],
        ["Çorum — kendi rafinerimiz", "Üretim + birincil kasa", "TR"],
        ["İstanbul — BIST Kıymetli Madenler", "Yerel saklama", "TR"],
        ["Zürih — Brink's / Loomis", "Avrupa kasası", "CH / EU"],
        ["Dubai — DMCC / Brink's", "Körfez kasası", "AE"],
    ], [5.5 * cm, 7 * cm, 4 * cm]))

    story.append(Paragraph("D. Kullanıcının gördüğü kısım", styles["h2"]))
    story.append(bullets([
        "<b>Web uygulaması (Next.js):</b> En tam fonksiyonlu kanal — onboarding, alış/satış, portföy, denetim raporları.",
        "<b>iOS ve Android uygulamaları:</b> Mobil odaklı, native (Swift + Kotlin).",
        "<b>Kurumsal API:</b> Bankaların ve piyasa yapıcıların kullanması için.",
        "<b>verify.gold.example:</b> Halka açık doğrulama portalı. Kimse üye olmadan girip 'şu cüzdanın altını gerçekten var mı' diye sorabilir.",
    ], styles["bullet"]))

    story.append(PageBreak())

    # ---------- 4. Güven ----------
    story.append(Paragraph("4. Sana neden güveneyim?", styles["h1"]))
    story.append(Paragraph(
        "İşin can damarı bu soru. Türkiye'de FTX olduğunu görmüş, "
        "Celsius olduğunu görmüş bir insan haklı olarak soruyor. "
        "Üç katmanlı bir cevabımız var:",
        styles["body"],
    ))

    story.append(Paragraph("Birinci katman: Tahsisli rezerv — kesirli yok", styles["h2"]))
    story.append(Paragraph(
        "Bankalar genelde 'kesirli rezerv' ile çalışır: 100 lira mevduat "
        "topladığında belki 10 lirayı tutar, gerisini kredi olarak verir. "
        "Bizde böyle bir şey yok. Sistemde 1.000 token varsa, kasada "
        "1.000 gram altın var. Hangi çubuğun kime ait olduğu da "
        "<b>bar_allocations</b> diye bir tabloda kayıtlı.",
        styles["body"],
    ))

    story.append(Paragraph("İkinci katman: Aylık Big Four denetimi", styles["h2"]))
    story.append(Paragraph(
        "Her ayın 1'inde Big Four diye bilinen dört büyük denetim "
        "firmasından biri (PwC, Deloitte, EY veya KPMG) dört kasayı "
        "fiziksel olarak sayıyor. Her çubuğun seri numarası, ağırlığı, "
        "saflığı kayda giriyor.",
        styles["body"],
    ))
    story.append(bullets([
        "Tüm liste 'Merkle ağacı' diye bir veri yapısına sokuluyor — bunun özelliği şu: tek bir çubuk değişse ağacın kökü değişir, anlarsın.",
        "Denetçi bu kökü EIP-712 standardıyla imzalıyor (sahte imza atılamaz).",
        "Tam rapor IPFS'e atılıyor — IPFS'in özelliği şu: bir kez yüklendi mi dosya hep aynı 'adres' altında, değiştiremezsin.",
        "Özet hash blokzincire yazılıyor — geri dönülmez.",
        "Denetim 35 günden eski olursa MintController yeni token basmayı reddediyor. Yani 'denetim atlayalım' gibi bir oyun yok.",
    ], styles["bullet"]))

    story.append(Paragraph("Üçüncü katman: Çubuk bazında doğrulama", styles["h2"]))
    story.append(Paragraph(
        "Sen verify.gold.example'a cüzdan adresini yazıyorsun. Sistem "
        "diyor ki: 'Senin 50 gramın Çorum kasasında, TR-2026-00428 "
        "seri numaralı çubukta tahsisli.' Hatta sana Merkle proof "
        "veriyor ki, sen bunu blokzincirdeki ReserveOracle "
        "sözleşmesinde bağımsız olarak doğrulayabilirsin. Bize "
        "güvenmek zorunda bile değilsin — matematik söylüyor.",
        styles["body"],
    ))

    story.append(PageBreak())

    # ---------- 5. Yol haritası ----------
    story.append(Paragraph("5. Bunu nasıl yapacağız? — Yol haritası", styles["h1"]))
    story.append(Paragraph(
        "İlk günden 'her şeyi yapalım' demek tehlikeli. Onun yerine "
        "altı faza böldük. Her faz öncekinin üstüne bir özellik "
        "ekliyor, hiçbir faz aceleye gelmiyor:",
        styles["body"],
    ))

    phases = [
        ("Faz 0 — Temeli atalım (Ay 0–2)", [
            "Ekibi topla: tech lead + 2 smart contract + 2 backend + 1 SRE.",
            "Ethereum testnet'te ilk kontratları kur — GoldToken, ComplianceRegistry, MintController iskeletleri.",
            "Yerel geliştirme ortamı (docker-compose ile her şey ayakta) ve CI/CD.",
            "Güvenlik tehdit modeli atölyesi — neyin nereden saldırı yiyebileceğini konuşalım.",
            "Tedarikçilerle pilot: Sumsub (KYC), Fireblocks (cüzdan saklama).",
        ]),
        ("Faz 1 — Türkiye MVP'si (Ay 2–6)", [
            "Türkiye için baştan sona iş akışı: kayıt → TL yatırma → mint → cüzdan → satış → TL çekme.",
            "Tek kasa: Çorum.",
            "PoR manuel — aylık tablo ve manuel imza yetiyor şimdilik.",
            "Çalışanlar arası alfa testi (önce kendimiz yiyoruz).",
            "CMB'ye (Sermaye Piyasası Kurulu) ön başvuru.",
        ]),
        ("Faz 2 — Denetim otomasyonu + İsviçre (Ay 6–10)", [
            "Otomatik PoR: ReserveOracle deploy, Merkle ağacı, IPFS.",
            "Zürih kasası entegre.",
            "USD, EUR, CHF para giriş-çıkışı.",
            "FINMA için SRO başvurusu.",
            "İlk dış güvenlik denetimi (OpenZeppelin).",
            "Sepolia testnet'te halka açık beta.",
        ]),
        ("Faz 3 — Mainnet lansmanı (Ay 10–14)", [
            "Ethereum mainnet'e gerçek deploy + Treasury Safe devri.",
            "İkinci ve üçüncü güvenlik denetimleri (Trail of Bits, Spearbit).",
            "10 milyon dolar başlangıç rezervi → ilk mint.",
            "TR ve CH arenaları canlı.",
            "1–2 borsada listeleme + 1 piyasa yapıcı.",
            "verify.gold.example halka açık.",
        ]),
        ("Faz 4 — Küresele açıl (Ay 14–24)", [
            "Dubai VARA lisansı, BAE arenası.",
            "Liechtenstein üzerinden MiCA (Avrupa pasaportu).",
            "Avalanche ve BNB Chain'e LayerZero köprüsü.",
            "DEX'lerde likidite havuzları.",
            "Kurumsal API ve banka ortaklıkları.",
            "Şeriat uyumlu varyant.",
        ]),
        ("Faz 5 — Olgun ürün (Ay 24+)", [
            "Layer 2 (Base, Arbitrum) genişleme.",
            "Tokenize altın ETF köprüsü.",
            "Altın leasing / yield ürünleri.",
            "Açık finansal API (yetkilendirme ile).",
        ]),
    ]
    for title, items in phases:
        story.append(Paragraph(title, styles["h2"]))
        story.append(bullets(items, styles["bullet"]))

    story.append(PageBreak())

    # ---------- 6. Teknoloji ----------
    story.append(Paragraph("6. Hangi teknolojileri seçtik, niye?", styles["h1"]))
    story.append(Paragraph(
        "Bir kuralımız var: 'hipster' teknoloji yok. Her seçim, "
        "'denetim havuzu geniş mi, operasyonel olarak yorulmaz mıyız' "
        "diye sorularak yapıldı:",
        styles["body"],
    ))
    story.append(styled_table([
        ["Katman", "Seçim", "Niye"],
        ["Akıllı sözleşme dili", "Solidity 0.8.24", "En geniş denetim havuzu"],
        ["Sözleşme framework", "Foundry", "Hızlı test, modern"],
        ["Sözleşme kütüphanesi", "OpenZeppelin + Solady", "Çoktan denetlenmiş"],
        ["Oracle", "Chainlink", "PoR desteği en olgun"],
        ["Köprü (Faz 4)", "LayerZero OFT", "Merkezi havuz riski yok"],
        ["Cüzdan saklama", "Fireblocks MPC", "Policy engine + HSM"],
        ["Backend dili", "Go + TypeScript", "Operasyonel olarak basit"],
        ["Veritabanı", "PostgreSQL 16", "ACID + zengin özellik"],
        ["Event bus", "NATS JetStream", "Hafif, streaming"],
        ["Web frontend", "Next.js 15 + Tailwind", "SEO + hız"],
        ["Mobil", "Swift + Kotlin + KMP", "Native performans"],
        ["KYC sağlayıcı", "Sumsub + Jumio", "İki sağlayıcı, yedek var"],
        ["Bulut", "AWS birincil + GCP yedek", "Çoklu region"],
        ["Container orkestrasyon", "Kubernetes (EKS)", "Bulut bağımsız"],
    ], [4.5 * cm, 6 * cm, 6 * cm]))

    story.append(PageBreak())

    # ---------- 7. Güvenlik ----------
    story.append(Paragraph("7. Güvenlik — kötü senaryolar ve cevaplarımız", styles["h1"]))
    story.append(Paragraph(
        "Bir tokenize altın platformunda dört şey aynı anda korunmalı: "
        "kod, anahtarlar, fiziksel altın, ve uyum. Birini boş bırakırsak "
        "diğerleri işe yaramaz. İşte tehdit bazlı kısa cevaplar:",
        styles["body"],
    ))
    story.append(styled_table([
        ["Olası tehdit", "Ne yapıyoruz"],
        ["Kontratta exploit", "Çoklu imza + PoR kontrolü + formal verification + 3 bağımsız denetim"],
        ["Kasada iç tehdit", "4-göz kuralı, CCTV, dual control, çubuk başı etiket"],
        ["Özel anahtar sızar", "AWS CloudHSM L3 + Fireblocks MPC + cold storage"],
        ["Kullanıcı phishing", "Hardware wallet desteği, EIP-712 domain bağlama"],
        ["Oracle manipülasyonu", "Chainlink + 3+ kaynak medyan + sapma koruyucu"],
        ["Reentrancy (kontrat oyunu)", "OpenZeppelin ReentrancyGuard + check-effects-interactions"],
        ["KYC by-pass (sahte kimlik)", "Biyometrik + belge ağı analizi + çift vendor"],
        ["API'ye DoS atağı", "CloudFlare + rate limit + WAF"],
        ["Bir ülkede regülatör baskını", "Diğer üç jurisdiction ayakta, İsviçre yedek"],
        ["Bağımlılık zinciri (npm/go)", "Pinned deps, SBOM, Snyk, iç artifact registry"],
    ], [4.5 * cm, 12 * cm]))

    story.append(Paragraph("Pratikte ne anlama geliyor bunlar?", styles["h2"]))
    story.append(bullets([
        "<b>Sıcak cüzdan = az miktar:</b> Toplam altının en fazla %1'i. Günlük işlem için.",
        "<b>Soğuk cüzdan = ana hazine:</b> Çok imzalı, farklı şehirlerde, geofencing alarmlı.",
        "<b>Treasury Safe imzacıları:</b> 5 kişi, 3 farklı şehir, 2 farklı ülke. Hepsi Ledger donanım cüzdanı.",
        "<b>Anahtar rotasyonu:</b> Yılda bir, her olaydan sonra zorunlu.",
        "<b>Bug bounty:</b> Immunefi üzerinden — kritik bug 500 bin USD, yüksek 100 bin USD.",
        "<b>Incident response:</b> Sistem 1 saat içinde geri ayağa kalkmalı (RTO), en fazla 5 dakika veri kaybı (RPO).",
    ], styles["bullet"]))

    story.append(PageBreak())

    # ---------- 8. Ekip ----------
    story.append(Paragraph("8. Kim ne yapacak? — Başlangıç ekibi", styles["h1"]))
    story.append(Paragraph(
        "Faz 0 ve 1 için toplam 13 kişi yetiyor. Şişirilmemiş, herkesin "
        "net bir görevi olan bir takım:",
        styles["body"],
    ))
    story.append(styled_table([
        ["Rol", "Adet", "Ne yapıyor"],
        ["Tech Lead / Chief Architect", "1", "Genel mimari, teknik kararlar"],
        ["Smart Contract Engineer", "2", "Solidity + Foundry, kontrat tasarımı ve testi"],
        ["Backend Engineer (Go)", "3", "Mikroservisler, mint/burn, compliance"],
        ["Frontend Lead (Next.js)", "1", "Web uygulaması + verify portalı"],
        ["Mobile Engineer", "2", "iOS (Swift) + Android (Kotlin)"],
        ["SRE / DevOps", "1", "Kubernetes, CI/CD, gözlemlenebilirlik"],
        ["Security Engineer", "1", "Tehdit modeli, audit hazırlığı, key management"],
        ["QA / Test Engineer", "1", "E2E testleri, regresyon"],
        ["Product Manager", "1", "Sipariş akışları, KYC akışları"],
        ["Compliance Tech Liaison", "1", "Hukuk ile ürün arası köprü"],
    ], [6 * cm, 1.5 * cm, 9 * cm]))
    story.append(Paragraph(
        "Faz 2 ve sonrası: KYC operasyon, müşteri destek, business "
        "development, data analyst, ve üç ek arenanın yerel temsilcileri.",
        styles["body"],
    ))

    story.append(PageBreak())

    # ---------- 9. Açık sorular ----------
    story.append(Paragraph("9. Hala karara bağlanmamış olanlar", styles["h1"]))
    story.append(Paragraph(
        "Nisan 2026 toplantısında bazı şeyleri çözdük. Bazıları hâlâ "
        "düzenleyici görüşü bekliyor. Şeffaf olalım:",
        styles["body"],
    ))

    story.append(Paragraph("Çözülenler (kesin)", styles["h2"]))
    story.append(bullets([
        "<b>Hassasiyet:</b> 18 decimals — DeFi uyumu ve alt-gram fraksiyon için.",
        "<b>Self-custody:</b> Ülkeye göre değişir — TR'de custodial-only, CH/AE/LI'de Enhanced KYC sonrası serbest.",
        "<b>Minimum alım:</b> 1 gram (sistem üzerinden). DEX'te zaten fraksiyon serbest.",
        "<b>Fiziksel teslim minimumu:</b> 1 kg — LBMA çubuğunu bölmek pahalı.",
        "<b>Köprü modeli:</b> Faz 4'te yeniden değerlendirilecek; şimdilik LayerZero OFT default.",
    ], styles["bullet"]))

    story.append(Paragraph("Hala açık olanlar", styles["h2"]))
    story.append(bullets([
        "<b>Yakım onayı:</b> 100 gramın altı otomatik mi, üstü onaylı mı? Henüz karar yok.",
        "<b>KVKK veri ikametgâhı:</b> Türk kullanıcı verisi yurt dışına çıkabilir mi? Hukuki görüş bekliyoruz.",
        "<b>Gas ödeme:</b> Kullanıcı ETH mi ödesin, biz mi ödeyelim (meta-tx)? v1'de EIP-2612 permit, v2'de paymaster planı var.",
        "<b>Layer 2 stratejisi:</b> Day-1 sadece Ethereum mi, yoksa Base/Arbitrum day-1 mi? Öneri: önce mainnet, sonra L2.",
    ], styles["bullet"]))

    story.append(PageBreak())

    # ---------- 10. Özet ----------
    story.append(Paragraph("10. Tek sayfada özet", styles["h1"]))

    story.append(info_box("GOLD nedir?", [
        "Her token tam olarak bir gram altın. Dört ülkede lisanslı, "
        "ayda bir Big Four tarafından denetlenen, kendi rafinerisi olan "
        "bir dijital altın platformu."
    ], styles))
    story.append(Spacer(1, 0.3 * cm))

    story.append(info_box("Nasıl çalışıyor?", [
        "Sen para yatırırsın → kasadaki çubuğun bir kısmı sana "
        "tahsis edilir → 5 imzadan 3'ü tamam derse token cüzdanına "
        "gelir. Satarken tersi: token yakılır, çubuk boşa çıkar, "
        "paran hesabına döner."
    ], styles))
    story.append(Spacer(1, 0.3 * cm))

    story.append(info_box("Rakiplerden farkı?", [
        "1 gram minimum (1 ons değil), dört yetki alanı (TR/CH/AE/LI), "
        "kendi rafinerimiz, çubuk bazında izlenebilirlik, Merkle proof "
        "ile bağımsız doğrulama."
    ], styles))
    story.append(Spacer(1, 0.3 * cm))

    story.append(info_box("Nasıl yapılacak?", [
        "Altı fazlı, 24+ aylık plan. Önce Türkiye MVP, sonra İsviçre + "
        "otomatik denetim, sonra mainnet lansmanı, sonra global ölçek "
        "ve olgun ürün. Hiçbir adım atlanmıyor."
    ], styles))
    story.append(Spacer(1, 0.3 * cm))

    story.append(info_box("Ne kadar sürer, ne kadar tutar?", [
        "İlk MVP (TR arena, manuel denetim): 6 ay. Mainnet lansman "
        "(otomatik PoR + 3 güvenlik denetimi): 14 ay. Tam global ölçek "
        "(4 arena + çoklu zincir): 24+ ay. Başlangıç rezervi: 10 milyon "
        "USD. Başlangıç ekibi 13 kişi."
    ], styles))

    story.append(Spacer(1, 1 * cm))
    story.append(Paragraph(
        "Detaya inmek istersen: <b>docs/system-design.md</b> (972 satır "
        "tam mimari), <b>docs/contracts/README.md</b> (kontrat spec), "
        "<b>docs/backend/README.md</b> (backend spec).",
        styles["note"],
    ))

    story.append(PageBreak())

    # ---------- 11. PAXG ve XAUT ----------
    story.append(Paragraph("11. Rakipler: PAXG ve XAUT nasıl çalışıyor?", styles["h1"]))
    story.append(Paragraph(
        "Tokenize altın dünyasında iki büyük oyuncu var: <b>PAXG</b> "
        "(Paxos Gold) ve <b>XAUT</b> (Tether Gold). Onları konuşmadan "
        "GOLD'un yerini anlamak zor. İkisini tek tek anlatayım, sonra "
        "yan yana koyalım.",
        styles["body"],
    ))

    # --- PAXG ---
    story.append(Paragraph("PAXG — Paxos Gold", styles["h2"]))
    story.append(Paragraph(
        "Paxos, New York merkezli bir kripto şirketi. PAXG'yi 2019'da "
        "çıkardı. Çalışma mantığı şöyle:",
        styles["body"],
    ))
    story.append(bullets([
        "<b>İhraç eden:</b> Paxos Trust Company, New York. NYDFS (New York Eyalet Finansal Hizmetler Departmanı) tarafından lisanslı — kripto dünyasının en sıkı regülatörlerinden biri.",
        "<b>Birim:</b> 1 PAXG = 1 troy ons altın (~31.1 gram). Yani küçük yatırımcı için yüksek bir giriş.",
        "<b>Kasa:</b> Londra'daki Brink's kasalarında, LBMA standartında 400-ons külçeler halinde.",
        "<b>Zincir:</b> Sadece Ethereum (ERC-20).",
        "<b>Denetim:</b> Aylık Withum firması tarafından attestation. Paxos'un sitesinden hangi külçenin senin olduğunu seri numarasıyla görebiliyorsun.",
        "<b>Fiziksel teslim:</b> 430 ons minimum (yaklaşık 13 kg, 2026 fiyatıyla ~30 milyon TL). Yani pratikte sadece kurumsal kullanıcı için.",
        "<b>KYC:</b> Sıkı — Paxos doğrudan satın almak için tam doğrulama, borsalardan alırsan borsa KYC'si.",
    ], styles["bullet"]))

    # --- XAUT ---
    story.append(Paragraph("XAUT — Tether Gold", styles["h2"]))
    story.append(Paragraph(
        "Tether (USDT'nin arkasındaki şirket) 2020'de çıkardı. Mantık "
        "benzer ama önemli detaylar farklı:",
        styles["body"],
    ))
    story.append(bullets([
        "<b>İhraç eden:</b> TG Commodities Limited, British Virgin Islands. Yani offshore — NYDFS gibi sıkı bir regülatöre değil, BVI hafif çerçevesine bağlı.",
        "<b>Birim:</b> 1 XAUT = 1 troy ons altın (~31.1 gram). PAXG ile aynı.",
        "<b>Kasa:</b> İsviçre'de (Tether kesin lokasyon açıklamıyor). LBMA standartı külçeler.",
        "<b>Zincir:</b> Hem Ethereum (ERC-20) hem Tron (TRC-20) — Tron tarafı daha ucuz işlem.",
        "<b>Denetim:</b> Tether tarafından yayınlanan attestation. Tarihsel olarak BDO İtalya yapıyordu ama şeffaflığı PAXG kadar tutarlı değil.",
        "<b>Fiziksel teslim:</b> 50 XAUT (yaklaşık 1.55 kg) — PAXG'den çok daha erişilebilir, ama yine de küçük yatırımcının üstünde.",
        "<b>KYC:</b> Daha gevşek. Bazı kanallardan KYC yapmadan da alınabiliyor (Tether'in genel çizgisi).",
    ], styles["bullet"]))

    # --- Karşılaştırma tablosu ---
    story.append(Paragraph("Üçü yan yana", styles["h2"]))
    story.append(styled_table([
        ["Özellik", "PAXG", "XAUT", "GOLD"],
        ["İhraç eden", "Paxos (NY)", "TG / Tether (BVI)", "GOLD (TR/CH/AE/LI)"],
        ["Düzenleyici", "NYDFS", "BVI offshore", "CMB+FINMA+VARA+FMA"],
        ["Minimum birim", "1 ons (~31g)", "1 ons (~31g)", "1 gram"],
        ["Kasa lokasyonu", "Londra", "İsviçre", "4 kasa, 4 ülke"],
        ["Rafineri", "3. taraf", "3. taraf", "Kendi (Çorum)"],
        ["Zincir", "Ethereum", "Ethereum + Tron", "Ethereum (sonra çoklu)"],
        ["Denetim", "Aylık Withum", "Düzensiz BDO", "Aylık Big Four"],
        ["Merkle proof on-chain", "Hayır", "Hayır", "Evet"],
        ["Fiziksel teslim min.", "~13 kg", "~1.55 kg", "1 kg"],
        ["KYC sıkılık", "Yüksek", "Orta", "Yüksek, ülkeye göre"],
        ["Sigorta beyanı", "Var", "Açık değil", "Lloyd's 500M USD"],
        ["Yaklaşık piyasa değeri", "~600M USD", "~700M USD", "Henüz yok"],
    ], [4.5 * cm, 4 * cm, 4 * cm, 4 * cm]))

    # --- Artıları eksileri ---
    story.append(Paragraph("PAXG'nin artıları ve eksileri", styles["h2"]))
    story.append(bullets([
        "<b>+</b> NYDFS regülasyonu — kripto dünyasının en güvenilir regülatörü.",
        "<b>+</b> Aylık şeffaf denetim, sürekli.",
        "<b>+</b> Brink's Londra — kasada şüphe yok.",
        "<b>+</b> Likidite iyi — Binance, Kraken, Coinbase gibi büyük borsalarda var.",
        "<b>−</b> 1 ons minimum — Türk perakende yatırımcısı için yüksek (yaklaşık 70 bin TL).",
        "<b>−</b> Tek jurisdiction (ABD) — politik risk. SEC bir gün 'durdurun' derse ne olur?",
        "<b>−</b> Fiziksel teslim 430 ons — pratikte sadece kurumsal.",
        "<b>−</b> Rafineri ve kasa hep üçüncü taraf — kendi tedarik zinciri yok.",
    ], styles["bullet"]))

    story.append(Paragraph("XAUT'un artıları ve eksileri", styles["h2"]))
    story.append(bullets([
        "<b>+</b> Fiziksel teslim 50 ons — PAXG'den çok daha erişilebilir.",
        "<b>+</b> İsviçre kasası — politik olarak nötr ülke.",
        "<b>+</b> Ethereum ve Tron'da birden — Tron tarafı çok ucuz işlem.",
        "<b>+</b> Bazı kanallarda KYC'siz alım mümkün (gri alan ama gerçek).",
        "<b>−</b> Tether ile bağlı — USDT'nin geçmişteki rezerv tartışmaları itibarı zedeliyor.",
        "<b>−</b> BVI regülasyonu — offshore, sıkı bir denetleyici yok.",
        "<b>−</b> Şeffaflık raporları PAXG kadar disiplinli değil.",
        "<b>−</b> Likidite PAXG'den daha düşük.",
    ], styles["bullet"]))

    story.append(Paragraph("Peki GOLD nereye oturuyor?", styles["h2"]))
    story.append(Paragraph(
        "PAXG güvenilir ama büyük kurumsal odaklı, küçük yatırımcı için "
        "uzak. XAUT erişilebilir ama Tether itibarı taşıyor. GOLD bu "
        "iki ucun ortasında olmaya çalışıyor:",
        styles["body"],
    ))
    story.append(bullets([
        "<b>Erişilebilirlik:</b> 1 gram minimum, XAUT'tan bile düşük.",
        "<b>Güvenilirlik:</b> 4 ayrı düzenleyici, Big Four denetimi, on-chain Merkle proof — PAXG'den bile bir adım öteye.",
        "<b>Bağımsızlık:</b> Kendi rafinerimiz (Çorum) — ne PAXG'de ne XAUT'ta var.",
        "<b>Politik dayanıklılık:</b> Bir ülke kapanırsa diğer üçü çalışır. PAXG (ABD-only) ve XAUT (BVI tek) buna sahip değil.",
        "<b>Eksisi:</b> Henüz canlı değil, marka tanınmamış, likidite sıfırdan kurulacak. Bu da başlangıçtaki en büyük zorluk.",
    ], styles["bullet"]))

    story.append(Spacer(1, 0.5 * cm))
    story.append(info_box("Tek cümlede konum", [
        "PAXG = güvenilir ama küçük yatırımcıya uzak. "
        "XAUT = erişilebilir ama Tether gölgesi taşıyor. "
        "GOLD = 'PAXG'nin güveni + XAUT'un erişilebilirliği + "
        "kendi rafinerimiz + 4 jurisdiction' formülünü deniyor."
    ], styles))

    # ---- 11.x Blokzincir tarafından karşılaştırma ----
    story.append(PageBreak())
    story.append(Paragraph("11.2. Blokzincir tarafından üçü yan yana", styles["h2"]))
    story.append(Paragraph(
        "Buraya kadar konuştuğumuz fark çoğunlukla iş tarafıydı. Asıl "
        "tokenize altın işinin ruhu kontratlarda gizli. PAXG ve XAUT'un "
        "Ethereum üzerindeki sözleşmeleri açık ve okunabilir; aşağıdaki "
        "tablo onlardan çıkarıldı:",
        styles["body"],
    ))

    story.append(styled_table([
        ["Özellik", "PAXG", "XAUT", "GOLD"],
        ["Deploy yılı", "2019", "2020", "2026 (planlı)"],
        ["Solidity sürümü", "0.5.x", "0.4.17", "0.8.24"],
        ["Decimals", "18", "6", "18"],
        ["Upgrade pattern", "Custom delegate-call", "Deprecation flag", "UUPS + 7 gün Timelock"],
        ["Mint yetkisi", "Tek supplyController", "Tek owner", "K-of-N (3/5)"],
        ["On-chain reserve gate", "Yok", "Yok", "Var (35 gün taze)"],
        ["Merkle proof on-chain", "Yok", "Yok", "Var"],
        ["EIP-712 attestation", "Yok", "Yok", "Var"],
        ["EIP-2612 permit", "Yok", "Yok", "Var"],
        ["On-chain KYC kontrolü", "Yok (off-chain)", "Yok (off-chain)", "Her _update'de"],
        ["Travel Rule on-chain", "Yok", "Yok", "Var (eşik bazlı)"],
        ["Freeze + wipe yetkisi", "Var", "Var (blacklist)", "Freeze var, wipe yok"],
        ["Custom errors (gas)", "Yok", "Yok", "Var"],
        ["ERC-7201 storage", "Yok", "Yok", "Var"],
    ], [4.5 * cm, 4 * cm, 4 * cm, 4 * cm]))

    story.append(Paragraph("Üç kritik fark", styles["h2"]))
    story.append(Paragraph(
        "Tablo uzun ama özü üç maddede toplanabilir:",
        styles["body"],
    ))
    story.append(bullets([
        "<b>1. Mint nasıl kontrol ediliyor?</b> PAXG ve XAUT'ta tek bir cüzdan adresi yetkili. O adres ele geçirilirse veya kötüye kullanılırsa sistem biter. GOLD'da kontrat seviyesinde 5 imzacıdan 3'ü onaylamadıkça hiçbir token basılmıyor — tek bir kişi mint yapamaz, kontrat reddediyor.",
        "<b>2. Rezerv kontrolü zincirde mi, off-chain mi?</b> PAXG ve XAUT'ta 'kasada altın yoksa basamazsın' kuralı issuer'ın iç prosedüründe — kontrat soruyu sormuyor. GOLD'da MintController her execute'ta totalSupply + amount > attestedGrams ise revert ediyor, ayrıca son denetim 35 günü geçmişse de revert.",
        "<b>3. Kullanıcı kendi başına doğrulayabilir mi?</b> PAXG/XAUT'ta 'benim altınım hangi çubukta' sorusunun cevabı issuer'ın websitesinde — onlara güvenmek zorundasın. GOLD'da ReserveOracle.verifyBarInclusion(index, leaf, proof) zincirde çağrılabiliyor, matematik sana 'evet bu çubuk o ayki denetimdeydi' diyor. İssuer'a güvenmek zorunda değilsin.",
    ], styles["bullet"]))

    story.append(Paragraph("PAXG'nin teknik tarafı — kısa kısa", styles["h2"]))
    story.append(bullets([
        "<b>Kontrat adresi:</b> 0x4580...4F78a (Ethereum mainnet).",
        "<b>Mimari:</b> Paxos kendi delegate-call proxy pattern'i. OZ değil.",
        "<b>Roller:</b> owner, assetProtectionRole, supplyController, feeController, feeRecipient, pauser.",
        "<b>Wipe yetkisi:</b> assetProtectionRole bir cüzdanı dondurup wipeFrozenAddress ile bakiyeyi silebilir. NYDFS düzenlemesi için var ama merkezi güç.",
        "<b>Fee:</b> Kontratta feeRate parametresi var, şu an %0 ama issuer açabilir.",
        "<b>Audit:</b> ChainSecurity (kontrat) + Withum (aylık attestation).",
    ], styles["bullet"]))

    story.append(Paragraph("XAUT'un teknik tarafı — kısa kısa", styles["h2"]))
    story.append(bullets([
        "<b>Kontrat adresi:</b> 0x6874...82F38 (Ethereum mainnet); ayrıca Tron'da TRC-20.",
        "<b>Mimari:</b> USDT TetherToken pattern'i — BasicToken + StandardToken + Pausable + BlackList. Yani USDT'nin altın versiyonu denebilir.",
        "<b>6 decimals:</b> DEX entegrasyonlarında dikkat ister — 18 değil.",
        "<b>Blacklist:</b> Owner istediği adresi blackliste atıp destroyBlackFunds ile bakiyesini yakabilir. USDT'deki ile aynı.",
        "<b>Deprecation flag:</b> Owner kontratı 'deprecated' işaretleyip yeni adrese yönlendirebilir — pseudo-upgrade.",
        "<b>Fee:</b> basisPointsRate + maximumFee parametreleri (USDT'den miras), şu an 0.",
    ], styles["bullet"]))

    story.append(PageBreak())

    # ---------- 12. Blokzincir teknik derinlik ----------
    story.append(Paragraph("12. Blokzincir tarafına teknik bakış (GOLD)", styles["h1"]))
    story.append(Paragraph(
        "Bu bölüm yazılım geliştiriciler ve teknik denetçiler için. "
        "Eğer kod tarafıyla işin yoksa atlamak serbest. Burada "
        "kontratların gerçekten ne yaptığını madde madde anlatacağız.",
        styles["body"],
    ))

    story.append(Paragraph("12.1. Genel teknik yığın", styles["h2"]))
    story.append(bullets([
        "<b>Dil:</b> Solidity 0.8.24, Cancun EVM. Built-in overflow check var (0.8+).",
        "<b>Compiler ayarları:</b> via_ir = true, optimizer 10.000 runs. via_ir özellikle MintController gibi karmaşık kontratta stack-too-deep hatalarını çözüyor.",
        "<b>Framework:</b> Foundry. Fuzz testler default 1.000 run, CI'da 10.000. Invariant testler 256 run + 100 depth.",
        "<b>Kütüphaneler:</b> OpenZeppelin v5 (audit havuzu en geniş) + Solady (kritik path'lerde gas optimizasyonu).",
        "<b>Storage pattern:</b> ERC-7201 'namespaced storage' — her kontratın storage'ı keccak hash ile izole slot'a yazılıyor. Upgrade çakışması fiziksel olarak imkânsız.",
        "<b>Custom errors:</b> Tüm revert'ler string yerine custom error. Hem gas hem hata yakalama daha iyi.",
        "<b>Bytecode hash:</b> none, cbor_metadata false — deterministik build için.",
    ], styles["bullet"]))

    story.append(Paragraph("12.2. Roller tablosu — kim ne yapabilir?", styles["h2"]))
    story.append(Paragraph(
        "GOLD'da on iki ayrı rol var. Hiçbir adresin birden fazla "
        "kritik rolü almaması ilkesi — kuvvetler ayrılığı:",
        styles["body"],
    ))
    story.append(styled_table([
        ["Rol", "Verildiği yer", "Ne yapabilir"],
        ["DEFAULT_ADMIN", "Treasury Safe", "Rol verir/alır"],
        ["TREASURY_ROLE", "Treasury Safe", "Parametre değişikliği, controller atama"],
        ["UPGRADER_ROLE", "Treasury Safe", "UUPS _authorizeUpgrade"],
        ["PAUSER_ROLE", "Acil müdahale ekibi", "Token.pause() — sadece dondurma"],
        ["MINT_PROPOSER", "Mint/Burn Service", "Mint önerisi açar"],
        ["MINT_APPROVER", "5 ayrı imzacı", "Önerilen mint'i onaylar"],
        ["MINT_EXECUTOR", "Operations bot", "Onaylanmışı execute eder"],
        ["BURN_OPERATOR", "Mint/Burn Service", "Redemption burn başlatır"],
        ["COMPLIANCE_OFFICER", "Uyum müdürü", "Freeze, sanctions, Travel Rule, operator burn imzası"],
        ["KYC_WRITER", "KYC Service", "WalletProfile yazar"],
        ["AUDITOR_ROLE", "Big Four cüzdanı", "ReserveOracle.publish ile attestation yayınlar"],
        ["FEE_RECIPIENT", "Treasury Safe", "Mint/burn fee'sini alır"],
    ], [4.5 * cm, 4 * cm, 7 * cm]))

    story.append(PageBreak())

    story.append(Paragraph("12.3. Mint akışının zincir üstü mantığı", styles["h2"]))
    story.append(Paragraph(
        "executeMint çağrıldığında kontrat şu beş kontrolü yapıyor — "
        "biri bile düşerse revert:",
        styles["body"],
    ))
    story.append(bullets([
        "<b>1. Onay sayısı:</b> p.approvers.length >= approvalThreshold (default 3). Aksi halde InsufficientApprovals revert.",
        "<b>2. Compliance:</b> ComplianceRegistry.canMint(to, amount, jurisdiction). Alıcı dondurulmuş, sanctions'lı veya KYC süresi geçmişse false döner.",
        "<b>3. Reserve freshness:</b> ReserveOracle.timeSinceLatest() > maxReserveAge (default 35 gün) ise StaleReserveAttestation revert.",
        "<b>4. Reserve invariant:</b> token.totalSupply() + grossAmount <= oracle.latestAttestedGrams(). Aksi halde ReserveInvariantViolated revert.",
        "<b>5. Rate limit (opsiyonel):</b> Bir pencere içinde basılan toplam miktar konfigüre edilmiş tavanı aşamaz.",
    ], styles["bullet"]))
    story.append(Paragraph(
        "Bunlar geçtikten sonra <b>CEI pattern</b> uygulanıyor: önce "
        "state update (ProposalStatus.EXECUTED + allocationUsed = true), "
        "sonra dış çağrı (token.mint). Mint sonucu 25 bps fee Treasury'ye, "
        "kalan kullanıcıya gidiyor. ReentrancyGuard'la ayrıca korunmalı.",
        styles["body"],
    ))

    story.append(Paragraph("12.4. ReserveOracle neden upgradeable değil?", styles["h2"]))
    story.append(Paragraph(
        "Diğer tüm kontratlar UUPS proxy ile upgrade edilebilir. "
        "ReserveOracle bilinçli olarak <b>immutable</b>. Sebep: denetim "
        "geçmişinin değiştirilemezliği sistemin can damarı. Eğer biri "
        "ReserveOracle'ı upgrade edebilirse geçmiş attestation'ları "
        "silebilir. Bunu imkânsızlaştırmak için:",
        styles["body"],
    ))
    story.append(bullets([
        "Kontratta hiç UUPS init yok, _authorizeUpgrade fonksiyonu yok.",
        "Attestation[] private _attestations — sadece push var, delete yok.",
        "publish() içinde monotonicity check: yeni timestamp eski timestamp'ten kesinlikle büyük olmalı.",
        "Geleceğe tarihli attestation kabul edilmiyor (±1 saat tolerans, NTP sapması için).",
        "Bug bulunursa: yeni ReserveOracle deploy edilir, MintController.setOracle() ile bağlanır. Eski kontrat blockchain'de kalır, geçmiş okunabilir.",
    ], styles["bullet"]))

    story.append(Paragraph("12.5. EIP-712 attestation yapısı", styles["h2"]))
    story.append(Paragraph(
        "Denetçinin imzaladığı veri yapısı sabit. publish() çağrısında "
        "kontrat bu hash'i yeniden hesaplayıp ECDSA.recover ile "
        "imzayı doğruluyor. Yapı:",
        styles["body"],
    ))
    story.append(bullets([
        "<b>timestamp:</b> attestation üretildiği zaman (saniye).",
        "<b>asOf:</b> kasanın hangi an itibariyle sayıldığı (timestamp'ten önce).",
        "<b>totalGrams:</b> dört kasanın toplam altın miktarı, 1e18 ile çarpılmış.",
        "<b>merkleRoot:</b> her çubuğun seri/ağırlık/saflık/kasa hash'lerinden oluşan ağacın kökü.",
        "<b>ipfsCid:</b> tam denetim paketinin (PDF, fotoğraflar, CSV) IPFS adresi.",
        "<b>auditor:</b> imzayı atan denetçinin Ethereum adresi. AUDITOR_ROLE'üne sahip olmalı, aksi halde UnknownAuditor revert.",
    ], styles["bullet"]))

    story.append(PageBreak())

    story.append(Paragraph("12.6. Kullanıcı Merkle proof doğrulama akışı", styles["h2"]))
    story.append(Paragraph(
        "verify.gold.example portalında 'altınımı doğrula' tuşuna "
        "basıldığında arka planda olan şu:",
        styles["body"],
    ))
    story.append(bullets([
        "Backend kullanıcının cüzdanını → bar_allocations → gold_bars → vault zincirinden geçirip son audit_snapshot'taki çubuk leaf'lerini buluyor.",
        "Her çubuk için Merkle proof veriyor: leaf = keccak256(barSerial, weightGrams, purity999, vaultCode, refinerLBMAId).",
        "Kullanıcı isterse browser'da Ethers/Viem ile ReserveOracle.verifyBarInclusion(attestationIndex, leaf, proof) çağırıyor.",
        "Kontrat MerkleProof.verifyCalldata(proof, root, leaf) çalıştırıp true/false dönüyor.",
        "true ise: kullanıcı 'o çubuk gerçekten o ayki denetimdeydi' garantisini matematiksel olarak almış oluyor — siteye değil, Ethereum'a güveniyor.",
    ], styles["bullet"]))

    story.append(Paragraph("12.7. GoldToken._update kancası", styles["h2"]))
    story.append(Paragraph(
        "OpenZeppelin v5 ile transfer/mint/burn akışları tek bir _update "
        "hook'unda birleşti (eski _beforeTokenTransfer yerine). Bu hook "
        "şu sırayla çalışıyor:",
        styles["body"],
    ))
    story.append(bullets([
        "<b>whenNotPaused modifier:</b> Pauser kontratı durdurmuşsa transfer yapılmaz.",
        "<b>Sadece transfer ise (from ve to sıfır değil):</b> ComplianceRegistry.canTransfer(from, to, value) çağrılır.",
        "<b>canTransfer false dönerse:</b> Hangi kurala takıldığını bulup spesifik revert at — WalletFrozen, SanctionsHit, KycRequired, TravelRuleRequired. Genel NotAuthorized yerine spesifik mesaj gas'ı biraz artırır ama debug'ı kolaylaştırır.",
        "<b>Mint path (from=0):</b> Sadece mintController adresi mi kontrolü; compliance burada değil MintController'da yapılıyor.",
        "<b>Burn path (to=0):</b> Sadece burnController adresi mi kontrolü.",
    ], styles["bullet"]))

    story.append(Paragraph("12.8. BurnController'da dual-control operator burn", styles["h2"]))
    story.append(Paragraph(
        "Olağan kullanıcı redemption'ında operatör tek başına burn "
        "yapabiliyor. Ama 'operator burn' diye ikinci bir akış var — "
        "mahkeme emriyle dondurulan bir cüzdanın bakiyesini yakmak "
        "gerekirse. Bu çift kontrol ile yapılıyor:",
        styles["body"],
    ))
    story.append(bullets([
        "BURN_OPERATOR_ROLE çağrıyı yapıyor.",
        "Ama beraberinde COMPLIANCE_OFFICER_ROLE'üne sahip birinin EIP-712 imzası gerekiyor.",
        "İmza şu yapı üzerinde: OperatorBurn(from, amount, reasonHash, nonce, deadline).",
        "Nonce her kullanımda otomatik artıyor — replay imkânsız.",
        "Deadline geçmişse DeadlineExpired revert — eski imza geçersiz.",
        "Yani iki farklı kişinin onayı olmadan operator burn yapılamaz. PAXG/XAUT'taki tek kişilik wipe yetkisinden farklı.",
    ], styles["bullet"]))

    story.append(Paragraph("12.9. UUPS upgrade + 7 gün Timelock", styles["h2"]))
    story.append(Paragraph(
        "Token, ComplianceRegistry, MintController, BurnController "
        "UUPS proxy ile upgrade edilebilir. Akış:",
        styles["body"],
    ))
    story.append(bullets([
        "Treasury Safe yeni implementation'a upgrade önerir (Safe transaction olarak).",
        "Safe'in policy engine'i 7 gün timelock'u uyguluyor — bu süre içinde topluluk inceleyebiliyor.",
        "Timelock dolduğunda 3/5 imzacı execute ediyor.",
        "Kontrat seviyesinde _authorizeUpgrade fonksiyonu UPGRADER_ROLE kontrolü yapıyor, ek bir kontrol katmanı.",
        "Tüm upgrade'ler OpenZeppelin Defender üzerinden monitör ediliyor, anormal aktivite Slack'e alert düşürüyor.",
    ], styles["bullet"]))

    story.append(Paragraph("12.10. Fee modeli", styles["h2"]))
    story.append(Paragraph(
        "GOLD'da iki fee var, ikisi de 25 bps (0.25%):",
        styles["body"],
    ))
    story.append(bullets([
        "<b>Mint fee:</b> MintController.executeMint sırasında. Gross amount'tan fee düşülür, kalan kullanıcıya. Fee Treasury'ye mint edilir (ayrı bir mint, jurisdiction tag korunuyor).",
        "<b>Burn fee:</b> BurnController.requestRedemption sırasında. Tam amount yakılıyor; off-chain settlement'ta (amount - fee) gram altın veya nakit kullanıcıya gidiyor. BurnFeeCollected event'i denetlenebilirlik için emit ediliyor.",
        "<b>Transfer fee:</b> SIFIR. Token'daki transferlerde hiçbir fee yok. PAXG'nin aksine kontratta feeRate parametresi de yok — issuer açma ihtimali bile yok.",
    ], styles["bullet"]))

    story.append(Spacer(1, 0.3 * cm))
    story.append(info_box("Sonuç: kontrat seviyesinde GOLD'un üç farkı", [
        "<b>1.</b> Çoklu imza zincirde zorlanıyor — tek bir anahtar çalınması sistemi düşürmüyor.<br/>"
        "<b>2.</b> Rezerv kontrolü kontratta — issuer kasayı dolu söylemese bile MintController kabul etmiyor.<br/>"
        "<b>3.</b> Kullanıcı doğrulaması kontratta — verify portalına değil, Ethereum matematiğine güveniliyor."
    ], styles))

    return story


def main():
    OUT.parent.mkdir(parents=True, exist_ok=True)
    styles = build_styles()
    doc = SimpleDocTemplate(
        str(OUT), pagesize=A4,
        leftMargin=2 * cm, rightMargin=2 * cm,
        topMargin=2 * cm, bottomMargin=2 * cm,
        title="GOLD Token - Basit Anlatim",
        author="GOLD Token Ekibi",
    )
    doc.build(build_story(styles), onFirstPage=footer, onLaterPages=footer)
    print(f"OK: {OUT}")


if __name__ == "__main__":
    main()
