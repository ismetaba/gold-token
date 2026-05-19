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
    canvas.drawString(2 * cm, 1.2 * cm, "GOLD Token — Basit Anlatım v0.2 — GİZLİ")
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
