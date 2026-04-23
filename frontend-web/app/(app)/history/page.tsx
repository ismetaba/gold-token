"use client";

import { useEffect, useState } from "react";
import { ordersApi } from "@/lib/api-client";
import { formatDate, formatGrams, formatTRY, orderTypeLabel } from "@/lib/utils";
import type { Order, OrderType } from "@/lib/types";
import { Loader2, RefreshCw, TrendingDown, TrendingUp } from "lucide-react";
import StatusBadge from "@/components/StatusBadge";

const TYPE_FILTERS: { label: string; value: OrderType | "all" }[] = [
  { label: "Tümü", value: "all" },
  { label: "Alım", value: "buy" },
  { label: "Satım", value: "sell" },
  { label: "Nakit İtfa", value: "redeem_cash" },
  { label: "Fiziksel İtfa", value: "redeem_physical" },
];

export default function HistoryPage() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<OrderType | "all">("all");
  const [cursor, setCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);

  const loadOrders = async (reset = false) => {
    if (reset) setLoading(true);
    try {
      const res = await ordersApi.listOrders(reset ? undefined : cursor);
      setOrders((prev) => (reset ? res.data : [...prev, ...res.data]));
      setCursor(res.meta.cursor);
      setHasMore(res.meta.hasMore);
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  };

  useEffect(() => { loadOrders(true); }, []);

  const loadMore = async () => {
    setLoadingMore(true);
    await loadOrders(false);
  };

  const filtered = filter === "all" ? orders : orders.filter((o) => o.type === filter);

  // Stats
  const totalBought = orders
    .filter((o) => o.type === "buy" && o.status === "COMPLETED")
    .reduce((acc, o) => acc + parseFloat(o.amountGrams), 0);
  const totalSold = orders
    .filter((o) => ["sell", "redeem_cash", "redeem_physical"].includes(o.type) && o.status === "COMPLETED")
    .reduce((acc, o) => acc + parseFloat(o.amountGrams), 0);
  const totalSpentTRY = orders
    .filter((o) => o.type === "buy" && o.status === "COMPLETED")
    .reduce((acc, o) => acc + parseFloat(o.totalTRY), 0);

  return (
    <div className="p-6 md:p-8 max-w-4xl">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">İşlem Geçmişi</h1>
          <p className="text-slate-500 mt-0.5">Tüm alım/satım ve itfa işlemleri</p>
        </div>
        <button
          onClick={() => loadOrders(true)}
          disabled={loading}
          className="p-2 rounded-lg border border-slate-200 text-slate-500 hover:text-slate-800 hover:border-slate-400 transition-colors disabled:opacity-50"
        >
          <RefreshCw size={16} className={loading ? "animate-spin" : ""} />
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <div className="bg-white rounded-xl border border-slate-200 p-4 shadow-sm">
          <div className="flex items-center gap-1.5 text-green-600 text-xs mb-1">
            <TrendingUp size={12} /> Toplam Alım
          </div>
          <p className="text-xl font-bold text-slate-900">{formatGrams(totalBought)}g</p>
        </div>
        <div className="bg-white rounded-xl border border-slate-200 p-4 shadow-sm">
          <div className="flex items-center gap-1.5 text-blue-600 text-xs mb-1">
            <TrendingDown size={12} /> Toplam Satım
          </div>
          <p className="text-xl font-bold text-slate-900">{formatGrams(totalSold)}g</p>
        </div>
        <div className="bg-white rounded-xl border border-slate-200 p-4 shadow-sm">
          <div className="flex items-center gap-1.5 text-slate-600 text-xs mb-1">
            💰 Toplam Harcama
          </div>
          <p className="text-xl font-bold text-slate-900">{formatTRY(totalSpentTRY)}</p>
        </div>
      </div>

      {/* Filter */}
      <div className="flex flex-wrap gap-2 mb-4">
        {TYPE_FILTERS.map(({ label, value }) => (
          <button
            key={value}
            onClick={() => setFilter(value)}
            className={`px-3 py-1.5 rounded-lg text-sm border transition-colors ${
              filter === value
                ? "bg-yellow-400 border-yellow-400 text-slate-900 font-medium"
                : "border-slate-200 text-slate-600 hover:border-slate-400"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 size={24} className="animate-spin text-yellow-500" />
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-12 text-slate-400">
            <p className="text-sm">Henüz işlem bulunmuyor.</p>
          </div>
        ) : (
          <>
            {/* Header */}
            <div className="hidden md:grid grid-cols-5 px-6 py-3 bg-slate-50 border-b border-slate-100 text-xs font-medium text-slate-500 uppercase tracking-wide">
              <span>Tarih</span>
              <span>Tür</span>
              <span>Miktar</span>
              <span>Tutar (TRY)</span>
              <span>Durum</span>
            </div>

            <div className="divide-y divide-slate-100">
              {filtered.map((order) => (
                <div
                  key={order.id}
                  className="px-6 py-4 grid grid-cols-1 md:grid-cols-5 gap-2 md:gap-0 md:items-center hover:bg-slate-50 transition-colors"
                >
                  <div className="text-sm text-slate-500 font-mono">
                    {formatDate(order.createdAt)}
                  </div>
                  <div className="flex items-center gap-2">
                    <span
                      className={`w-2 h-2 rounded-full ${
                        order.type === "buy"
                          ? "bg-green-500"
                          : order.type === "sell"
                          ? "bg-blue-500"
                          : "bg-purple-500"
                      }`}
                    />
                    <span className="text-sm text-slate-700">{orderTypeLabel(order.type)}</span>
                  </div>
                  <div className="text-sm font-medium text-slate-800">
                    {formatGrams(order.amountGrams)} gram
                  </div>
                  <div className="text-sm font-medium text-slate-800">
                    {formatTRY(order.totalTRY)}
                  </div>
                  <div>
                    <StatusBadge status={order.status} />
                  </div>
                </div>
              ))}
            </div>
          </>
        )}

        {hasMore && (
          <div className="px-6 py-4 border-t border-slate-100 text-center">
            <button
              onClick={loadMore}
              disabled={loadingMore}
              className="text-sm text-yellow-600 hover:text-yellow-700 flex items-center gap-1.5 mx-auto"
            >
              {loadingMore && <Loader2 size={14} className="animate-spin" />}
              Daha Fazla Yükle
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
