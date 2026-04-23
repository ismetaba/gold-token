"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { ordersApi, porApi, priceApi, walletApi } from "@/lib/api-client";
import { formatGrams, formatTRY, formatUSD, truncateAddress, weiToGrams } from "@/lib/utils";
import type { GoldPrice, Order, ProofOfReserve, Wallet } from "@/lib/types";
import { ArrowRight, CheckCircle, Clock, RefreshCw, Shield, TrendingUp, Wallet as WalletIcon } from "lucide-react";
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
      if (ordersRes.status === "fulfilled") setRecentOrders(ordersRes.value.data.slice(0, 3));
      if (porRes.status === "fulfilled") setPor(porRes.value.data);
      setLoading(false);
    };
    load();
  }, []);

  // Compute portfolio value
  const portfolioTRY =
    wallet && price
      ? (parseFloat(wallet.balanceGrams) * parseFloat(price.pricePerGramTRY)).toFixed(2)
      : null;

  if (loading) {
    return (
      <div className="p-8">
        <div className="animate-pulse space-y-4">
          <div className="h-8 bg-slate-200 rounded w-48" />
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-32 bg-slate-200 rounded-xl" />
            ))}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 md:p-8 max-w-6xl">
      {/* Header */}
      <div className="mb-6 flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">
            Merhaba, {user?.fullName?.split(" ")[0]} 👋
          </h1>
          <p className="text-slate-500 mt-0.5">
            {new Date().toLocaleDateString("tr-TR", {
              weekday: "long",
              day: "numeric",
              month: "long",
              year: "numeric",
            })}
          </p>
        </div>
        <GoldPriceTicker />
      </div>

      {/* KYC warning */}
      {user?.kycStatus !== "approved" && (
        <div className="bg-yellow-50 border border-yellow-200 rounded-xl p-4 mb-6 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Clock size={18} className="text-yellow-600" />
            <div>
              <p className="font-medium text-yellow-800">Kimlik doğrulaması gerekli</p>
              <p className="text-sm text-yellow-600">İşlem yapabilmek için KYC onayını tamamlayın.</p>
            </div>
          </div>
          <Link href="/kyc" className="bg-yellow-400 text-slate-900 px-4 py-2 rounded-lg text-sm font-medium hover:bg-yellow-300 transition-colors whitespace-nowrap">
            Tamamla
          </Link>
        </div>
      )}

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        {/* Wallet balance */}
        <div className="bg-white rounded-2xl p-6 border border-slate-200 shadow-sm">
          <div className="flex items-center gap-2 text-slate-500 text-sm mb-3">
            <WalletIcon size={14} />
            GOLD Bakiyesi
          </div>
          <div className="text-3xl font-bold text-slate-900 mb-1">
            {wallet ? formatGrams(wallet.balanceGrams) : "—"} <span className="text-lg font-medium text-slate-500">gram</span>
          </div>
          {wallet && (
            <p className="text-xs text-slate-500 truncate">
              {truncateAddress(wallet.address)}
            </p>
          )}
        </div>

        {/* Portfolio value */}
        <div className="bg-gradient-to-br from-yellow-400 to-yellow-500 rounded-2xl p-6 shadow-sm">
          <div className="flex items-center gap-2 text-yellow-900 text-sm mb-3">
            <TrendingUp size={14} />
            Portföy Değeri
          </div>
          <div className="text-3xl font-bold text-slate-900 mb-1">
            {portfolioTRY ? formatTRY(portfolioTRY) : "—"}
          </div>
          {price && (
            <p className="text-xs text-yellow-800">
              ≈ {portfolioTRY ? formatUSD((parseFloat(portfolioTRY) / 33).toFixed(2)) : "—"}
            </p>
          )}
        </div>

        {/* Gold price */}
        <div className="bg-white rounded-2xl p-6 border border-slate-200 shadow-sm">
          <div className="flex items-center gap-2 text-slate-500 text-sm mb-3">
            <RefreshCw size={14} />
            Altın Fiyatı
          </div>
          {price ? (
            <>
              <div className="text-2xl font-bold text-slate-900 mb-1">
                {formatTRY(price.pricePerGramTRY)}
                <span className="text-sm font-normal text-slate-500"> /gram</span>
              </div>
              <p className="text-xs text-slate-500">
                {formatUSD(price.pricePerOzUSD)} / ons · {price.source}
              </p>
            </>
          ) : (
            <div className="text-slate-400">Yükleniyor...</div>
          )}
        </div>
      </div>

      {/* Quick actions */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
        {[
          { href: "/buy", label: "Altın Al", color: "bg-green-500 hover:bg-green-600" },
          { href: "/sell", label: "Altın Sat", color: "bg-blue-500 hover:bg-blue-600" },
          { href: "/history", label: "İşlemler", color: "bg-slate-600 hover:bg-slate-700" },
          { href: "/proof-of-reserve", label: "Rezerv Kanıtı", color: "bg-purple-500 hover:bg-purple-600" },
        ].map(({ href, label, color }) => (
          <Link
            key={href}
            href={href}
            className={`${color} text-white rounded-xl py-3 px-4 text-sm font-medium text-center transition-colors`}
          >
            {label}
          </Link>
        ))}
      </div>

      {/* Recent orders + PoR summary */}
      <div className="grid md:grid-cols-2 gap-6">
        {/* Recent orders */}
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm">
          <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
            <h2 className="font-semibold text-slate-900">Son İşlemler</h2>
            <Link href="/history" className="text-yellow-600 hover:text-yellow-700 text-sm flex items-center gap-1">
              Tümü <ArrowRight size={12} />
            </Link>
          </div>
          {recentOrders.length === 0 ? (
            <div className="px-6 py-8 text-center text-slate-400 text-sm">
              Henüz işlem bulunmuyor.
            </div>
          ) : (
            <div className="divide-y divide-slate-100">
              {recentOrders.map((order) => (
                <div key={order.id} className="px-6 py-3 flex items-center justify-between">
                  <div>
                    <div className="text-sm font-medium text-slate-800">
                      {order.type === "buy" ? "Alım" : "Satım"} · {order.amountGrams}g
                    </div>
                    <div className="text-xs text-slate-400 mt-0.5">
                      {new Date(order.createdAt).toLocaleDateString("tr-TR")}
                    </div>
                  </div>
                  <div className="flex flex-col items-end gap-1">
                    <span className="text-sm font-medium text-slate-700">
                      {formatTRY(order.totalTRY)}
                    </span>
                    <StatusBadge status={order.status} />
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* PoR summary */}
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm">
          <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
            <h2 className="font-semibold text-slate-900">Rezerv Özeti</h2>
            <Link href="/proof-of-reserve" className="text-yellow-600 hover:text-yellow-700 text-sm flex items-center gap-1">
              Detaylar <ArrowRight size={12} />
            </Link>
          </div>
          {por ? (
            <div className="px-6 py-4 space-y-3">
              <div className="flex items-center gap-2 text-green-600 text-sm font-medium">
                <CheckCircle size={16} />
                %100 Rezerv Karşılama — {por.auditor}
              </div>
              <div className="grid grid-cols-2 gap-3 text-sm">
                <div>
                  <p className="text-slate-500">Toplam Rezerv</p>
                  <p className="font-semibold text-slate-800">
                    {parseFloat(por.totalGoldGrams).toLocaleString("tr-TR")} gram
                  </p>
                </div>
                <div>
                  <p className="text-slate-500">Toplam Arz</p>
                  <p className="font-semibold text-slate-800">
                    {parseFloat(por.totalTokenSupplyGrams).toLocaleString("tr-TR")} GOLD
                  </p>
                </div>
                <div>
                  <p className="text-slate-500">Kasa Sayısı</p>
                  <p className="font-semibold text-slate-800">{por.vaults.length}</p>
                </div>
                <div>
                  <p className="text-slate-500">Son Atestasyon</p>
                  <p className="font-semibold text-slate-800">
                    {new Date(por.attestedAt).toLocaleDateString("tr-TR", { month: "long", year: "numeric" })}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2 text-xs text-slate-400 pt-1">
                <Shield size={12} />
                IPFS: {por.ipfsCid.slice(0, 20)}…
              </div>
            </div>
          ) : (
            <div className="px-6 py-8 text-center text-slate-400 text-sm">
              Yükleniyor...
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
