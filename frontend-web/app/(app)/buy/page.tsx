"use client";

import { useEffect, useState } from "react";
import { ordersApi, priceApi } from "@/lib/api-client";
import { formatGrams, formatTRY } from "@/lib/utils";
import type { GoldPrice, Order } from "@/lib/types";
import {
  AlertCircle,
  CheckCircle,
  ChevronRight,
  Loader2,
  ShoppingCart,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import StatusBadge from "@/components/StatusBadge";

type Step = "form" | "confirm" | "payment" | "status";

const PRESET_GRAMS = ["1", "5", "10", "25", "50"];

export default function BuyPage() {
  const { user } = useAuth();
  const [price, setPrice] = useState<GoldPrice | null>(null);
  const [amountGrams, setAmountGrams] = useState("1");
  const [step, setStep] = useState<Step>("form");
  const [order, setOrder] = useState<Order | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    priceApi.getCurrentPrice().then((r) => setPrice(r.data.price)).catch(() => {});
    const id = setInterval(
      () => priceApi.getCurrentPrice().then((r) => setPrice(r.data.price)).catch(() => {}),
      15_000
    );
    return () => clearInterval(id);
  }, []);

  const grams = parseFloat(amountGrams) || 0;
  const totalTRY = price ? (grams * parseFloat(price.pricePerGramTRY)).toFixed(2) : "0";

  const handleCreateOrder = async () => {
    if (!price || grams <= 0) return;
    setLoading(true);
    setError("");
    try {
      const idempotencyKey = crypto.randomUUID();
      const res = await ordersApi.createBuyOrder({ amountGrams: amountGrams, idempotencyKey });
      setOrder(res.data);
      setStep("confirm");
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message ?? "Sipariş oluşturulamadı.");
    } finally {
      setLoading(false);
    }
  };

  const handleSimulatePayment = async () => {
    if (!order) return;
    setLoading(true);
    setStep("payment");
    try {
      await new Promise((r) => setTimeout(r, 2000)); // simulate payment gateway
      const res = await ordersApi.simulatePayment(order.id);
      setOrder(res.data);
      setStep("status");
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message ?? "Ödeme işlemi başarısız.");
      setStep("confirm");
    } finally {
      setLoading(false);
    }
  };

  const reset = () => {
    setStep("form");
    setOrder(null);
    setError("");
    setAmountGrams("1");
  };

  return (
    <div className="p-6 md:p-8 max-w-lg">
      <h1 className="text-2xl font-bold text-slate-900 mb-1">Altın Al</h1>
      <p className="text-slate-500 mb-6">
        1 GOLD = 1 gram fiziksel altın. Minimum alım: 1 gram.
      </p>

      {/* Breadcrumb */}
      <div className="flex items-center gap-1.5 text-sm mb-6">
        {(["form", "confirm", "payment", "status"] as Step[]).map((s, i, arr) => (
          <span key={s} className="flex items-center gap-1.5">
            <span
              className={`${
                step === s
                  ? "text-yellow-600 font-medium"
                  : arr.indexOf(step) > i
                  ? "text-slate-400 line-through"
                  : "text-slate-300"
              }`}
            >
              {s === "form" ? "Miktar" : s === "confirm" ? "Onay" : s === "payment" ? "Ödeme" : "Durum"}
            </span>
            {i < arr.length - 1 && <ChevronRight size={12} className="text-slate-300" />}
          </span>
        ))}
      </div>

      {/* STEP: Form */}
      {step === "form" && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-6 space-y-5">
          {/* KYC warning */}
          {user?.kycStatus !== "approved" && (
            <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3 flex items-center gap-2 text-sm text-yellow-700">
              <AlertCircle size={14} />
              KYC onayı olmadan işlem gerçekleştirilemez.
            </div>
          )}

          {/* Price */}
          {price && (
            <div className="bg-slate-50 rounded-lg px-4 py-3 text-sm">
              <span className="text-slate-500">Güncel Fiyat: </span>
              <span className="font-semibold text-slate-800">{formatTRY(price.pricePerGramTRY)} / gram</span>
            </div>
          )}

          {/* Presets */}
          <div>
            <label className="text-sm font-medium text-slate-700 block mb-2">Miktar (gram)</label>
            <div className="flex flex-wrap gap-2 mb-3">
              {PRESET_GRAMS.map((g) => (
                <button
                  key={g}
                  onClick={() => setAmountGrams(g)}
                  className={`px-3 py-1.5 rounded-lg text-sm border transition-colors ${
                    amountGrams === g
                      ? "bg-yellow-400 border-yellow-400 text-slate-900 font-medium"
                      : "border-slate-200 text-slate-600 hover:border-slate-400"
                  }`}
                >
                  {g}g
                </button>
              ))}
            </div>
            <input
              type="number"
              min="1"
              step="0.1"
              value={amountGrams}
              onChange={(e) => setAmountGrams(e.target.value)}
              className="w-full border border-slate-200 rounded-lg px-4 py-2.5 text-slate-900 focus:outline-none focus:ring-2 focus:ring-yellow-400"
              placeholder="Özel miktar girin"
            />
          </div>

          {/* Total */}
          {grams > 0 && price && (
            <div className="border-t border-slate-100 pt-4 space-y-2 text-sm">
              <div className="flex justify-between text-slate-600">
                <span>{formatGrams(grams)} gram × {formatTRY(price.pricePerGramTRY)}</span>
                <span>{formatTRY(totalTRY)}</span>
              </div>
              <div className="flex justify-between font-semibold text-slate-900">
                <span>Toplam</span>
                <span>{formatTRY(totalTRY)}</span>
              </div>
            </div>
          )}

          {error && (
            <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-red-700 text-sm">
              {error}
            </div>
          )}

          <button
            onClick={handleCreateOrder}
            disabled={loading || grams <= 0 || !price}
            className="w-full bg-yellow-400 text-slate-900 py-3 rounded-xl font-semibold hover:bg-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-2"
          >
            {loading && <Loader2 size={16} className="animate-spin" />}
            Siparişi Oluştur
          </button>
        </div>
      )}

      {/* STEP: Confirm */}
      {step === "confirm" && order && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-6 space-y-5">
          <div className="flex items-center gap-3 bg-blue-50 border border-blue-200 rounded-lg px-4 py-3">
            <ShoppingCart size={16} className="text-blue-600" />
            <div>
              <p className="text-sm font-medium text-blue-800">Sipariş Özeti</p>
              <p className="text-xs text-blue-600 font-mono">{order.id}</p>
            </div>
          </div>

          <div className="space-y-3 text-sm">
            {[
              ["Miktar", `${order.amountGrams} gram GOLD`],
              ["Birim Fiyat", formatTRY(order.pricePerGramTRY) + " / gram"],
              ["Toplam", formatTRY(order.totalTRY)],
              ["Durum", ""],
            ].map(([label, value]) =>
              label === "Durum" ? (
                <div key={label} className="flex justify-between items-center">
                  <span className="text-slate-500">{label}</span>
                  <StatusBadge status={order.status} />
                </div>
              ) : (
                <div key={label} className="flex justify-between">
                  <span className="text-slate-500">{label}</span>
                  <span className="font-medium text-slate-800">{value}</span>
                </div>
              )
            )}
          </div>

          <div className="bg-slate-50 rounded-lg p-4 text-xs text-slate-500">
            <p className="font-medium text-slate-700 mb-1">Simüle Ödeme (POC)</p>
            Gerçek sistemde kullanıcı İyzico/Stripe ödeme sayfasına yönlendirilir.
            Bu POC'ta ödemeyi simüle edebilirsiniz.
          </div>

          {error && (
            <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-red-700 text-sm">
              {error}
            </div>
          )}

          <div className="flex gap-3">
            <button
              onClick={reset}
              className="flex-1 border border-slate-200 text-slate-600 py-2.5 rounded-xl text-sm hover:border-slate-400 transition-colors"
            >
              İptal
            </button>
            <button
              onClick={handleSimulatePayment}
              disabled={loading}
              className="flex-1 bg-green-500 text-white py-2.5 rounded-xl font-semibold hover:bg-green-600 disabled:opacity-50 transition-colors flex items-center justify-center gap-2"
            >
              {loading && <Loader2 size={16} className="animate-spin" />}
              Ödemeyi Simüle Et
            </button>
          </div>
        </div>
      )}

      {/* STEP: Payment processing */}
      {step === "payment" && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-10 text-center space-y-4">
          <Loader2 size={40} className="animate-spin text-yellow-500 mx-auto" />
          <h2 className="text-lg font-semibold text-slate-800">Ödeme İşleniyor</h2>
          <p className="text-slate-500 text-sm">
            Ödemeniz doğrulanıyor ve mint saga başlatılıyor...
          </p>
        </div>
      )}

      {/* STEP: Status */}
      {step === "status" && order && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-6 space-y-5">
          {["MINT_EXECUTED", "COMPLETED"].includes(order.status) ? (
            <div className="text-center py-4">
              <CheckCircle size={48} className="text-green-500 mx-auto mb-3" />
              <h2 className="text-xl font-bold text-slate-900 mb-1">Alım Başarılı!</h2>
              <p className="text-slate-500 text-sm">
                {order.amountGrams} gram GOLD cüzdanınıza transfer edildi.
              </p>
            </div>
          ) : (
            <div className="text-center py-4">
              <Loader2 size={48} className="animate-spin text-blue-500 mx-auto mb-3" />
              <h2 className="text-xl font-bold text-slate-900 mb-1">İşlem Devam Ediyor</h2>
              <p className="text-slate-500 text-sm">
                Mint onay sürecinde. E-posta ile bildirim alacaksınız.
              </p>
            </div>
          )}

          <div className="space-y-2 text-sm border-t border-slate-100 pt-4">
            {[
              ["Sipariş ID", order.id.slice(0, 16) + "…"],
              ["Miktar", `${order.amountGrams} gram`],
              ["Toplam", formatTRY(order.totalTRY)],
            ].map(([label, value]) => (
              <div key={label} className="flex justify-between">
                <span className="text-slate-500">{label}</span>
                <span className="font-medium text-slate-800 font-mono text-xs">{value}</span>
              </div>
            ))}
            <div className="flex justify-between items-center">
              <span className="text-slate-500">Durum</span>
              <StatusBadge status={order.status} />
            </div>
          </div>

          <button
            onClick={reset}
            className="w-full bg-yellow-400 text-slate-900 py-3 rounded-xl font-semibold hover:bg-yellow-300 transition-colors"
          >
            Yeni Alım
          </button>
        </div>
      )}
    </div>
  );
}
