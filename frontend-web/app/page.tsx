"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { priceApi } from "@/lib/api-client";
import { formatTRY, formatUSD } from "@/lib/utils";
import type { GoldPrice } from "@/lib/types";
import {
  ArrowRight,
  BarChart2,
  CheckCircle,
  Lock,
  Shield,
  Sparkles,
  TrendingUp,
  Zap,
} from "lucide-react";

export default function LandingPage() {
  const [price, setPrice] = useState<GoldPrice | null>(null);

  useEffect(() => {
    priceApi.getCurrentPrice().then((r) => setPrice(r.data.price)).catch(() => {});
    const id = setInterval(
      () => priceApi.getCurrentPrice().then((r) => setPrice(r.data.price)).catch(() => {}),
      15_000
    );
    return () => clearInterval(id);
  }, []);

  return (
    <div className="min-h-screen bg-night-0 text-white selection:bg-brand-300/30">
      {/* ─────────── Top sticky glass nav ─────────── */}
      <header className="sticky top-0 z-40 backdrop-blur-md bg-night-0/70 border-b border-white/5">
        <div className="max-w-6xl mx-auto px-6 py-4 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-2.5 group">
            <span className="relative w-9 h-9 rounded-2xl bg-gradient-to-br from-brand-200 to-brand-500 flex items-center justify-center shadow-gold">
              <span className="text-night-0 font-bold">G</span>
              <span className="absolute inset-0 rounded-2xl ring-1 ring-inset ring-white/40" />
            </span>
            <span className="font-semibold tracking-tight text-lg">GOLD Token</span>
          </Link>
          <nav className="hidden md:flex items-center gap-7 text-sm text-white/65">
            <a href="#why" className="hover:text-white transition-colors">Neden GOLD?</a>
            <a href="#stats" className="hover:text-white transition-colors">Rezerv</a>
            <Link href="/proof-of-reserve" className="hover:text-white transition-colors">Kanıt</Link>
          </nav>
          <div className="flex items-center gap-2">
            <Link
              href="/login"
              className="text-white/70 hover:text-white text-sm px-3 py-2 rounded-lg transition-colors"
            >
              Giriş
            </Link>
            <Link
              href="/register"
              className="btn btn-primary text-sm"
            >
              Hesap Aç
              <ArrowRight size={14} />
            </Link>
          </div>
        </div>
      </header>

      {/* ─────────── Live price strip ─────────── */}
      <div className="border-b border-white/5 bg-white/[0.02]">
        <div className="max-w-6xl mx-auto px-6 py-2.5 flex items-center gap-3 text-xs sm:text-sm overflow-x-auto">
          <span className="inline-flex items-center gap-1.5 text-white/45 whitespace-nowrap">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-60 animate-ping" />
              <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-400" />
            </span>
            Canlı altın fiyatı
          </span>
          {price ? (
            <>
              <span className="font-mono font-semibold text-brand-300 whitespace-nowrap">
                {formatTRY(price.pricePerGramTRY)} <span className="text-white/40 font-normal">/gram</span>
              </span>
              <span className="text-white/15">·</span>
              <span className="font-mono text-white/70 whitespace-nowrap">
                {formatUSD(price.pricePerGramUSD)} <span className="text-white/40">/gram</span>
              </span>
              <span className="text-white/15">·</span>
              <span className="font-mono text-white/70 whitespace-nowrap">
                {formatUSD(price.pricePerOzUSD)} <span className="text-white/40">/oz</span>
              </span>
              <span className="text-white/15">·</span>
              <span className="text-white/40 whitespace-nowrap">{price.source}</span>
            </>
          ) : (
            <span className="text-white/40">Yükleniyor…</span>
          )}
        </div>
      </div>

      {/* ─────────── Hero ─────────── */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-mesh-dark" aria-hidden="true" />
        <div className="absolute inset-0 bg-dot-grid opacity-40" aria-hidden="true" />
        <div className="relative max-w-6xl mx-auto px-6 pt-24 pb-28 text-center anim-rise">
          <div className="inline-flex items-center gap-2 chip border-brand-300/30 bg-brand-300/[0.08] text-brand-200 mb-8">
            <Sparkles size={12} />
            Aylık bağımsız denetim · %100 fiziksel rezerv
          </div>
          <h1 className="text-5xl md:text-7xl font-bold tracking-tight leading-[1.05] mb-7 text-balance">
            1 GOLD
            <span className="text-white/35 mx-3 font-light">=</span>
            <br className="md:hidden" />
            1 gram <span className="text-gradient-gold">fiziksel altın</span>
          </h1>
          <p className="text-lg md:text-xl text-white/65 max-w-2xl mx-auto mb-10 leading-relaxed text-pretty">
            Tahsisli, seri numarası bazında denetlenebilir altın. Saniyeler içinde transfer,
            istediğin an fiziksel teslim. <span className="text-white">Fractional reserve yok.</span>
          </p>
          <div className="flex flex-col sm:flex-row gap-3 justify-center">
            <Link
              href="/register"
              className="btn btn-primary text-base px-7 py-3.5"
            >
              Yatırıma Başla
              <ArrowRight size={18} />
            </Link>
            <Link
              href="/proof-of-reserve"
              className="btn glass-dark text-white text-base px-7 py-3.5 hover:bg-white/10"
            >
              <Shield size={18} />
              Rezerv Kanıtını İncele
            </Link>
          </div>

          {/* Trust marks */}
          <div className="mt-14 flex flex-wrap items-center justify-center gap-x-8 gap-y-3 text-xs uppercase tracking-widest text-white/35">
            <span>LBMA</span>
            <span className="text-white/15">·</span>
            <span>Chainlink PoR</span>
            <span className="text-white/15">·</span>
            <span>Big Four Audit</span>
            <span className="text-white/15">·</span>
            <span>Brink&apos;s · Loomis</span>
            <span className="text-white/15">·</span>
            <span>FINMA / CMB</span>
          </div>
        </div>
      </section>

      {/* ─────────── Features ─────────── */}
      <section id="why" className="max-w-6xl mx-auto px-6 py-24">
        <div className="text-center mb-14">
          <span className="chip bg-white/5 border-white/10 text-white/60 mb-4">Neden GOLD?</span>
          <h2 className="text-3xl md:text-4xl font-bold tracking-tight">
            Kurumsal güvence,<br className="md:hidden" /> bireysel erişim
          </h2>
        </div>
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-5">
          {[
            {
              icon: Shield,
              title: "Tam Rezerv Koruması",
              desc:
                "Her token, kasa envanterinde seri numarası bazında bir altın çubuğa tahsislidir. Big Four bağımsız denetim, IPFS ve zincir üstünde yayınlanır.",
            },
            {
              icon: Zap,
              title: "Anında Transfer",
              desc:
                "ERC-20 token olarak saniyeler içinde global transfer. Önde gelen borsalar ve DeFi protokolleri ile yerel entegrasyon.",
            },
            {
              icon: Lock,
              title: "Düzenleyici Uyum",
              desc:
                "TR / CH / AE / EU yetki alanlarında lisanslı operasyon. KYC, AML, KVKK, GDPR ve FINMA standartlarına tam uyum.",
            },
            {
              icon: BarChart2,
              title: "Şeffaf Fiyatlama",
              desc:
                "LBMA ve Chainlink oracle ile manipüle edilemez gerçek zamanlı fiyat akışı. Spread her zaman görünür.",
            },
            {
              icon: CheckCircle,
              title: "Zincir Üstü Doğrulama",
              desc:
                "Mint ve burn işlemleri 3/5 çoklu imza ile yürütülür. Her hareket Ethereum üzerinde herkese açık doğrulanabilir.",
            },
            {
              icon: TrendingUp,
              title: "Fiziksel İtfa",
              desc:
                "İstediğin an tokenı gerçek altına dönüştür. Brink&apos;s / Loomis ile sigortalı kapı teslimi veya kasa transferi.",
            },
          ].map(({ icon: Icon, title, desc }) => (
            <div
              key={title}
              className="group relative glass-dark rounded-2xl p-6 hover:bg-white/[0.06] transition-colors"
            >
              <div className="absolute inset-x-0 -top-px h-px bg-gradient-to-r from-transparent via-white/10 to-transparent" aria-hidden="true" />
              <div className="w-11 h-11 rounded-xl bg-brand-300/10 ring-1 ring-brand-300/25 flex items-center justify-center mb-5">
                <Icon size={20} className="text-brand-300" />
              </div>
              <h3 className="font-semibold text-lg tracking-tight mb-2">{title}</h3>
              <p className="text-white/55 text-sm leading-relaxed">{desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* ─────────── Stats ─────────── */}
      <section id="stats" className="relative">
        <div className="absolute inset-0 bg-gradient-to-b from-transparent via-white/[0.02] to-transparent" />
        <div className="relative max-w-6xl mx-auto px-6 py-16">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-6 md:gap-8 text-center">
            {[
              { value: "1.25M", label: "Toplam rezerv (gram)" },
              { value: "4", label: "Lisanslı yetki alanı" },
              { value: "100%", label: "Rezerv karşılama oranı" },
              { value: "Aylık", label: "Bağımsız denetim" },
            ].map(({ value, label }) => (
              <div key={label} className="anim-rise">
                <div className="text-4xl md:text-5xl font-bold tracking-tight text-gradient-gold mb-2">
                  {value}
                </div>
                <div className="text-white/50 text-sm">{label}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ─────────── CTA ─────────── */}
      <section className="max-w-6xl mx-auto px-6 py-24">
        <div className="relative overflow-hidden rounded-3xl glass-dark p-10 md:p-14 text-center">
          <div
            className="pointer-events-none absolute inset-0 opacity-60"
            style={{
              backgroundImage:
                "radial-gradient(800px 240px at 50% -60%, rgba(253,176,34,0.35) 0%, transparent 60%)",
            }}
          />
          <h2 className="relative text-3xl md:text-4xl font-bold tracking-tight mb-4">
            Altın yatırımı artık <span className="text-gradient-gold">erişilebilir</span>
          </h2>
          <p className="relative text-white/60 mb-8 text-lg max-w-xl mx-auto">
            Minimum 1 gram ile başla. Anlık al, sat ya da fiziksel teslim al.
          </p>
          <Link
            href="/register"
            className="relative btn btn-primary text-base px-8 py-3.5 inline-flex"
          >
            Ücretsiz Hesap Aç
            <ArrowRight size={18} />
          </Link>
        </div>
      </section>

      {/* ─────────── Footer ─────────── */}
      <footer className="border-t border-white/5">
        <div className="max-w-6xl mx-auto px-6 py-8 flex flex-col md:flex-row justify-between gap-3 text-white/40 text-sm">
          <span>© 2026 GOLD Token. Tüm hakları saklıdır.</span>
          <span>TR Arena · CMB lisanslı · KVKK uyumlu</span>
        </div>
      </footer>
    </div>
  );
}
