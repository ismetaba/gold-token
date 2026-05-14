"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { ordersApi, porApi, priceApi, walletApi } from "@/lib/api-client";
import { formatGrams, formatTRY, formatUSD, truncateAddress } from "@/lib/utils";
import type { GoldPrice, Order, ProofOfReserve, Wallet } from "@/lib/types";
import {
  ArrowDownLeft,
  ArrowRight,
  ArrowUpRight,
  CheckCircle,
  Clock,
  History as HistoryIcon,
  RefreshCw,
  Shield,
  ShoppingCart,
  TrendingDown,
  TrendingUp,
  Wallet as WalletIcon,
} from "lucide-react";
import Link from "next/link";
import StatusBadge from "@/components/StatusBadge";
import GoldPriceTicker from "@/components/GoldPriceTicker";

export default function DashboardPage() {
  const { user } = useAuth();
  const [wallet, setWallet] = useState<Wallet | null>(null);
  const [price, setPrice] = useState<GoldPrice | null>(null);
  const [recentOrders, setRecentOrders] = useState<Order[]>([]);
  const [por, setPor] = useState<ProofOfReserve | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const load = async () => {
      const [walletRes, priceRes, ordersRes, porRes] = await Promise.allSettled([
        walletApi.getWallet(),
        priceApi.getCurrentPrice(),
        ordersApi.listOrders(),
        porApi.getLatest(),
      ]);
      if (walletRes.status === "fulfilled") setWallet(walletRes.value.data);
      if (priceRes.status === "fulfilled") setPrice(priceRes.value.data.price);
      if (ordersRes.status === "fulfilled") setRecentOrders(ordersRes.value.data.slice(0, 4));
      if (porRes.status === "fulfilled") setPor(porRes.value.data);
      setLoading(false);
    };
    load();
  }, []);

  const portfolioTRY =
    wallet && price
      ? (parseFloat(wallet.balanceGrams) * parseFloat(price.pricePerGramTRY)).toFixed(2)
      : null;
  const portfolioUSD =
    wallet && price
      ? (parseFloat(wallet.balanceGrams) * parseFloat(price.pricePerGramUSD)).toFixed(2)
      : null;

  if (loading) {
    return (
      <div className="p-6 md:p-10 max-w-6xl space-y-5">
        <div className="h-7 w-56 skeleton" />
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-36 skeleton" />
          ))}
        </div>
        <div className="grid md:grid-cols-2 gap-5">
          <div className="h-64 skeleton" />
          <div className="h-64 skeleton" />
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 md:p-10 max-w-6xl">
      {/* ─────────── Header ─────────── */}
      <div className="mb-7 flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <p className="text-xs uppercase tracking-widest text-ink-3 mb-1">
            {new Date().toLocaleDateString("tr-TR", {
              weekday: "long",
              day: "numeric",
              month: "long",
              year: "numeric",
            })}
          </p>
          <h1 className="text-3xl font-bold tracking-tight text-ink-0">
            Merhaba, {user?.fullName?.split(" ")[0] ?? "yatırımcı"}
            <span aria-hidden="true">  👋</span>
          </h1>
        </div>
        <GoldPriceTicker />
      </div>

      {/* ─────────── KYC banner ─────────── */}
      {user?.kycStatus !== "approved" && (
        <div className="card border-amber-200 bg-gradient-to-r from-amber-50 to-amber-50/40 p-4 mb-6 flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <span className="w-9 h-9 rounded-xl bg-amber-100 flex items-center justify-center">
              <Clock size={16} className="text-amber-700" />
            </span>
            <div>
              <p className="font-semibold text-amber-900">Kimlik doğrulaması gerekli</p>
              <p className="text-sm text-amber-800/80">
                İşlem yapabilmek için KYC onayını tamamlayın.
              </p>
            </div>
          </div>
          <Link
            href="/kyc"
            className="btn btn-primary text-sm whitespace-nowrap"
          >
            Tamamla <ArrowRight size={14} />
          </Link>
        </div>
      )}

      {/* ─────────── Stat cards ─────────── */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        {/* Wallet balance */}
        <div className="card card-hover p-6">
          <div className="flex items-center gap-2 text-ink-2 text-xs uppercase tracking-widest mb-4">
            <WalletIcon size={13} />
            GOLD Bakiyesi
          </div>
          <div className="flex items-baseline gap-1.5 mb-2">
            <span className="text-4xl font-bold tracking-tight text-ink-0 tabular-nums">
              {wallet ? formatGrams(wallet.balanceGrams) : "—"}
            </span>
            <span className="text-sm font-medium text-ink-2">gram</span>
          </div>
          {wallet && (
            <div className="flex items-center gap-2 text-xs text-ink-3">
              <span className="font-mono">{truncateAddress(wallet.address)}</span>
            </div>
          )}
        </div>

        {/* Portfolio value */}
        <div className="relative card card-hover p-6 overflow-hidden">
          <div
            className="absolute inset-0 opacity-90"
            style={{
              backgroundImage:
                "linear-gradient(135deg, #fde68a 0%, #f59e0b 60%, #b45309 100%)",
            }}
            aria-hidden="true"
          />
          <div
            className="absolute -right-10 -top-10 w-40 h-40 rounded-full bg-white/20 blur-2xl"
            aria-hidden="true"
          />
          <div className="relative">
            <div className="flex items-center gap-2 text-amber-950/80 text-xs uppercase tracking-widest mb-4">
              <TrendingUp size={13} />
              Portföy Değeri
            </div>
            <div className="text-4xl font-bold tracking-tight text-amber-950 mb-1 tabular-nums">
              {portfolioTRY ? formatTRY(portfolioTRY) : "—"}
            </div>
            {portfolioUSD && (
              <p className="text-sm text-amber-950/70 tabular-nums">
                ≈ {formatUSD(portfolioUSD)}
              </p>
            )}
          </div>
        </div>

        {/* Gold price */}
        <div className="card card-hover p-6">
          <div className="flex items-center justify-between text-ink-2 text-xs uppercase tracking-widest mb-4">
            <span className="inline-flex items-center gap-2">
              <RefreshCw size={13} />
              Altın Fiyatı
            </span>
            {price && (
              <span className="text-[10px] normal-case font-medium text-ink-3 tracking-normal">
                {price.source}
              </span>
            )}
          </div>
          {price ? (
            <>
              <div className="text-3xl font-bold tracking-tight text-ink-0 mb-1 tabular-nums">
                {formatTRY(price.pricePerGramTRY)}
                <span className="text-sm font-medium text-ink-2"> /gram</span>
              </div>
              <p className="text-xs text-ink-3 tabular-nums">
                {formatUSD(price.pricePerOzUSD)} / ons
              </p>
            </>
          ) : (
            <div className="text-ink-3">Yükleniyor…</div>
          )}
        </div>
      </div>

      {/* ─────────── Quick actions ─────────── */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-7">
        {[
          {
            href: "/buy",
            label: "Altın Al",
            Icon: ShoppingCart,
            color: "from-emerald-500 to-emerald-600 shadow-emerald-500/25",
          },
          {
            href: "/sell",
            label: "Altın Sat",
            Icon: TrendingDown,
            color: "from-sky-500 to-sky-600 shadow-sky-500/25",
          },
          {
            href: "/history",
            label: "İşlemler",
            Icon: HistoryIcon,
            color: "from-slate-700 to-slate-800 shadow-slate-700/25",
          },
          {
            href: "/proof-of-reserve",
            label: "Rezerv Kanıtı",
            Icon: Shield,
            color: "from-violet-500 to-violet-600 shadow-violet-500/25",
          },
        ].map(({ href, label, Icon, color }) => (
          <Link
            key={href}
            href={href}
            className={`group relative overflow-hidden bg-gradient-to-br ${color} text-white rounded-2xl p-4 shadow-md hover:shadow-xl transition-all hover:-translate-y-0.5`}
          >
            <div className="flex items-center justify-between">
              <span className="text-sm font-semibold tracking-tight">{label}</span>
              <Icon size={16} className="opacity-90" />
            </div>
            <ArrowUpRight
              size={16}
              className="absolute right-3 bottom-3 opacity-0 group-hover:opacity-90 transition-opacity"
            />
          </Link>
        ))}
      </div>

      {/* ─────────── Recent orders + PoR summary ─────────── */}
      <div className="grid md:grid-cols-2 gap-5">
        {/* Recent orders */}
        <div className="card overflow-hidden">
          <div className="px-6 py-4 hr-soft flex items-center justify-between">
            <h2 className="font-semibold tracking-tight text-ink-0">Son İşlemler</h2>
            <Link
              href="/history"
              className="text-brand-600 hover:text-brand-700 text-sm inline-flex items-center gap-1 font-medium"
            >
              Tümü <ArrowRight size={12} />
            </Link>
          </div>
          {recentOrders.length === 0 ? (
            <div className="px-6 py-12 text-center text-ink-3 text-sm">
              <HistoryIcon size={20} className="mx-auto mb-2 opacity-50" />
              Henüz işlem bulunmuyor.
            </div>
          ) : (
            <ul className="divide-y divide-black/[0.04]">
              {recentOrders.map((order) => (
                <li key={order.id} className="px-6 py-3 flex items-center justify-between gap-3">
                  <div className="flex items-center gap-3 min-w-0">
                    <span
                      className={`w-9 h-9 rounded-xl flex items-center justify-center shrink-0 ${
                        order.type === "buy"
                          ? "bg-emerald-50 text-emerald-700"
                          : "bg-sky-50 text-sky-700"
                      }`}
                    >
                      {order.type === "buy" ? (
                        <ArrowDownLeft size={16} />
                      ) : (
                        <ArrowUpRight size={16} />
                      )}
                    </span>
                    <div className="min-w-0">
                      <div className="text-sm font-semibold text-ink-0 truncate">
                        {order.type === "buy" ? "Alım" : "Satım"} ·{" "}
                        <span className="tabular-nums">{order.amountGrams}g</span>
                      </div>
                      <div className="text-xs text-ink-3 mt-0.5">
                        {new Date(order.createdAt).toLocaleDateString("tr-TR")}
                      </div>
                    </div>
                  </div>
                  <div className="flex flex-col items-end gap-1 shrink-0">
                    <span className="text-sm font-semibold text-ink-1 tabular-nums">
                      {formatTRY(order.totalTRY)}
                    </span>
                    <StatusBadge status={order.status} />
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* PoR summary */}
        <div className="card overflow-hidden">
          <div className="px-6 py-4 hr-soft flex items-center justify-between">
            <h2 className="font-semibold tracking-tight text-ink-0">Rezerv Özeti</h2>
            <Link
              href="/proof-of-reserve"
              className="text-brand-600 hover:text-brand-700 text-sm inline-flex items-center gap-1 font-medium"
            >
              Detaylar <ArrowRight size={12} />
            </Link>
          </div>
          {por ? (
            <div className="px-6 py-5 space-y-4">
              <div className="inline-flex items-center gap-2 chip border-emerald-200 bg-emerald-50 text-emerald-700">
                <CheckCircle size={12} />
                %100 Rezerv Karşılama
              </div>
              <p className="text-sm text-ink-2">Denetleyici: {por.auditor}</p>

              <div className="grid grid-cols-2 gap-3 pt-1">
                {[
                  {
                    label: "Toplam Rezerv",
                    value: `${parseFloat(por.totalGoldGrams).toLocaleString("tr-TR")} g`,
                  },
                  {
                    label: "Toplam Arz",
                    value: `${parseFloat(por.totalTokenSupplyGrams).toLocaleString("tr-TR")} GOLD`,
                  },
                  { label: "Kasa Sayısı", value: `${por.vaults.length}` },
                  {
                    label: "Son Atestasyon",
                    value: new Date(por.attestedAt).toLocaleDateString("tr-TR", {
                      month: "short",
                      year: "numeric",
                    }),
                  },
                ].map(({ label, value }) => (
                  <div
                    key={label}
                    className="rounded-xl bg-surface-2 px-3 py-2.5"
                  >
                    <p className="text-[11px] uppercase tracking-wider text-ink-3 mb-0.5">
                      {label}
                    </p>
                    <p className="text-sm font-semibold text-ink-0 tabular-nums">
                      {value}
                    </p>
                  </div>
                ))}
              </div>
              <div className="flex items-center gap-2 text-xs text-ink-3 pt-1">
                <Shield size={12} />
                IPFS:{" "}
                <span className="font-mono">
                  {por.ipfsCid.slice(0, 18)}…
                </span>
              </div>
            </div>
          ) : (
            <div className="px-6 py-12 text-center text-ink-3 text-sm">Yükleniyor…</div>
          )}
        </div>
      </div>
    </div>
  );
}
