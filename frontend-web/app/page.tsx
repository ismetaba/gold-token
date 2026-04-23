"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { priceApi } from "@/lib/api-client";
import { formatTRY, formatUSD } from "@/lib/utils";
import type { GoldPrice } from "@/lib/types";
import { ArrowRight, BarChart2, CheckCircle, Lock, Shield, Zap } from "lucide-react";

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
    <div className="min-h-screen bg-slate-900 text-white">
      {/* Header */}
      <header className="px-6 py-4 flex items-center justify-between max-w-6xl mx-auto">
        <div className="flex items-center gap-2">
          <div className="w-9 h-9 rounded-full bg-yellow-400 flex items-center justify-center">
            <span className="text-slate-900 font-bold">G</span>
          </div>
          <span className="font-semibold text-xl">GOLD Token</span>
        </div>
        <div className="flex items-center gap-3">
          <Link
            href="/login"
            className="text-slate-300 hover:text-white text-sm transition-colors"
          >
            Giriş Yap
          </Link>
          <Link
            href="/register"
            className="bg-yellow-400 text-slate-900 rounded-lg px-4 py-2 text-sm font-medium hover:bg-yellow-300 transition-colors"
          >
            Hesap Aç
          </Link>
        </div>
      </header>

      {/* Live price ticker */}
      <div className="border-y border-slate-700 bg-slate-800/50 py-2.5 px-6">
        <div className="max-w-6xl mx-auto flex items-center gap-6 text-sm overflow-x-auto">
          <span className="text-slate-400 whitespace-nowrap">Anlık Altın Fiyatı</span>
          {price ? (
            <>
              <span className="font-mono text-yellow-400 font-semibold whitespace-nowrap">
                {formatTRY(price.pricePerGramTRY)} / gram
              </span>
              <span className="text-slate-500">·</span>
              <span className="font-mono text-slate-300 whitespace-nowrap">
                {formatUSD(price.pricePerGramUSD)} / gram
              </span>
              <span className="text-slate-500">·</span>
              <span className="font-mono text-slate-300 whitespace-nowrap">
                {formatUSD(price.pricePerOzUSD)} / ons
              </span>
              <span className="text-slate-500">·</span>
              <span className="text-slate-500 text-xs whitespace-nowrap">
                {price.source}
              </span>
            </>
          ) : (
            <span className="text-slate-500">Yükleniyor...</span>
          )}
        </div>
      </div>

      {/* Hero */}
      <section className="max-w-6xl mx-auto px-6 py-24 text-center">
        <div className="inline-flex items-center gap-2 bg-yellow-400/10 border border-yellow-400/20 rounded-full px-4 py-1.5 text-yellow-400 text-sm mb-8">
          <CheckCircle size={14} />
          Aylık bağımsız denetim ile korunan gerçek altın
        </div>
        <h1 className="text-5xl md:text-6xl font-bold mb-6 leading-tight">
          1 GOLD = 1 gram
          <br />
          <span className="text-yellow-400">fiziksel altın</span>
        </h1>
        <p className="text-xl text-slate-300 max-w-2xl mx-auto mb-10 leading-relaxed">
          Blokzincir üzerinde tahsisli (allocated) altın. Tokenınız, kasa
          envanterinde fiziksel bir altın çubuğa seri numarası bazında bağlıdır.
          Fractional reserve yok.
        </p>
        <div className="flex flex-col sm:flex-row gap-4 justify-center">
          <Link
            href="/register"
            className="bg-yellow-400 text-slate-900 rounded-xl px-8 py-4 text-lg font-semibold hover:bg-yellow-300 transition-colors flex items-center gap-2 justify-center"
          >
            Yatırıma Başla
            <ArrowRight size={20} />
          </Link>
          <Link
            href="/proof-of-reserve"
            className="border border-slate-600 rounded-xl px-8 py-4 text-lg hover:border-slate-400 transition-colors flex items-center gap-2 justify-center text-slate-300"
          >
            <Shield size={20} />
            Rezerv Kanıtı
          </Link>
        </div>
      </section>

      {/* Features */}
      <section className="max-w-6xl mx-auto px-6 py-20">
        <h2 className="text-3xl font-bold text-center mb-12">
          Neden GOLD Token?
        </h2>
        <div className="grid md:grid-cols-3 gap-8">
          {[
            {
              icon: Shield,
              title: "Tam Rezerv Koruması",
              desc: "Her token fiziksel altına tahsislidir. Big Four tarafından aylık denetlenen rezervler IPFS ve blokzincirde yayınlanır.",
            },
            {
              icon: Zap,
              title: "Anında Transfer",
              desc: "ERC-20 token olarak saniyeler içinde global transfer. Borsa entegrasyonu, DeFi protokolleri ile uyumlu.",
            },
            {
              icon: Lock,
              title: "Düzenleyici Uyum",
              desc: "TR/CH/AE/EU yetki alanlarında faaliyet. KYC/AML, KVKK, GDPR ve FINMA standartlarına tam uyum.",
            },
            {
              icon: BarChart2,
              title: "Şeffaf Fiyatlama",
              desc: "LBMA ve Chainlink oracle entegrasyonu ile manipüle edilemez, gerçek zamanlı fiyat akışı.",
            },
            {
              icon: CheckCircle,
              title: "Zincir Üstü Doğrulama",
              desc: "Mint ve burn işlemleri 3/5 çoklu imza ile yürütülür. Her işlem Ethereum üzerinde herkese açık doğrulanabilir.",
            },
            {
              icon: ArrowRight,
              title: "Fiziksel İtfa",
              desc: "İstediğinde tokenlarını gerçek altına dönüştür. Brink's / Loomis ile sigortalı kapı teslimi.",
            },
          ].map(({ icon: Icon, title, desc }) => (
            <div
              key={title}
              className="bg-slate-800 rounded-2xl p-6 border border-slate-700 hover:border-slate-500 transition-colors"
            >
              <div className="w-10 h-10 rounded-lg bg-yellow-400/10 flex items-center justify-center mb-4">
                <Icon size={20} className="text-yellow-400" />
              </div>
              <h3 className="font-semibold text-lg mb-2">{title}</h3>
              <p className="text-slate-400 text-sm leading-relaxed">{desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Stats */}
      <section className="bg-slate-800/50 border-y border-slate-700 py-16">
        <div className="max-w-6xl mx-auto px-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-8 text-center">
            {[
              { value: "1.25M", label: "Toplam Rezerv (gram)" },
              { value: "4", label: "Yetki Alanı" },
              { value: "100%", label: "Rezerv Karşılama Oranı" },
              { value: "Aylık", label: "Bağımsız Denetim" },
            ].map(({ value, label }) => (
              <div key={label}>
                <div className="text-3xl font-bold text-yellow-400 mb-1">{value}</div>
                <div className="text-slate-400 text-sm">{label}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="max-w-6xl mx-auto px-6 py-24 text-center">
        <h2 className="text-3xl font-bold mb-4">Altın yatırımı artık erişilebilir</h2>
        <p className="text-slate-400 mb-8 text-lg">
          Minimum 1 gram ile başla. Anlık al, sat ya da fiziksel teslim al.
        </p>
        <Link
          href="/register"
          className="bg-yellow-400 text-slate-900 rounded-xl px-10 py-4 text-lg font-semibold hover:bg-yellow-300 transition-colors inline-flex items-center gap-2"
        >
          Ücretsiz Hesap Aç
          <ArrowRight size={20} />
        </Link>
      </section>

      {/* Footer */}
      <footer className="border-t border-slate-700 px-6 py-8 text-slate-500 text-sm">
        <div className="max-w-6xl mx-auto flex flex-col md:flex-row justify-between gap-4">
          <span>© 2026 GOLD Token. Tüm hakları saklıdır.</span>
          <span>
            TR Arena · CMB lisanslı · KVKK uyumlu
          </span>
        </div>
      </footer>
    </div>
  );
}
