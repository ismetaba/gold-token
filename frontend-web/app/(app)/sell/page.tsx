"use client";

import { useEffect, useState } from "react";
import { ordersApi, priceApi, walletApi } from "@/lib/api-client";
import { formatGrams, formatTRY } from "@/lib/utils";
import type { GoldPrice, Order, Wallet } from "@/lib/types";
import { AlertCircle, CheckCircle, ChevronRight, Loader2, TrendingDown } from "lucide-react";
import StatusBadge from "@/components/StatusBadge";

type Step = "form" | "confirm" | "processing" | "done";
type RedeemType = "cash" | "physical";

export default function SellPage() {
  const [price, setPrice] = useState<GoldPrice | null>(null);
  const [wallet, setWallet] = useState<Wallet | null>(null);
  const [amountGrams, setAmountGrams] = useState("1");
  const [redeemType, setRedeemType] = useState<RedeemType>("cash");
  const [iban, setIban] = useState("");
  const [step, setStep] = useState<Step>("form");
  const [order, setOrder] = useState<Order | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    priceApi.getCurrentPrice().then((r) => setPrice(r.data.price)).catch(() => {});
    walletApi.getWallet().then((r) => setWallet(r.data)).catch(() => {});

    const id = setInterval(
      () => priceApi.getCurrentPrice().then((r) => setPrice(r.data.price)).catch(() => {}),
      15_000
    );
    return () => clearInterval(id);
  }, []);

  const grams = parseFloat(amountGrams) || 0;
  const balance = parseFloat(wallet?.balanceGrams ?? "0");
  const totalTRY = price ? (grams * parseFloat(price.pricePerGramTRY)).toFixed(2) : "0";
  const exceedsBalance = grams > balance;

  const handleCreateOrder = async () => {
    if (!price || grams <= 0 || exceedsBalance) return;
    setLoading(true);
    setError("");
    try {
      const idempotencyKey = crypto.randomUUID();
      const res = await ordersApi.createSellOrder({
        amountGrams,
        idempotencyKey,
        ibanOrAddress: iban,
      });
      setOrder(res.data);
      setStep("confirm");
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message ?? "Sipariş oluşturulamadı.");
    } finally {
      setLoading(false);
    }
  };

  const handleConfirm = async () => {
    if (!order) return;
    setLoading(true);
    setStep("processing");
    try {
      await new Promise((r) => setTimeout(r, 2000));
      // Simulate burn + payout
      setOrder((o) => o ? { ...o, status: "COMPLETED", completedAt: new Date().toISOString() } : o);
      setStep("done");
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message ?? "İşlem başarısız.");
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
    setIban("");
  };

  return (
    <div className="p-6 md:p-8 max-w-lg">
      <h1 className="text-2xl font-bold text-slate-900 mb-1">Altın Sat</h1>
      <p className="text-slate-500 mb-6">
        Tokenlarınızı nakit veya fiziksel altın olarak itfa edin.
      </p>

      {/* Breadcrumb */}
      <div className="flex items-center gap-1.5 text-sm mb-6">
        {(["form", "confirm", "processing", "done"] as Step[]).map((s, i, arr) => (
          <span key={s} className="flex items-center gap-1.5">
            <span className={`${step === s ? "text-yellow-600 font-medium" : arr.indexOf(step) > i ? "text-slate-400 line-through" : "text-slate-300"}`}>
              {s === "form" ? "Miktar" : s === "confirm" ? "Onay" : s === "processing" ? "İşlem" : "Tamamlandı"}
            </span>
            {i < arr.length - 1 && <ChevronRight size={12} className="text-slate-300" />}
          </span>
        ))}
      </div>

      {/* STEP: Form */}
      {step === "form" && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-6 space-y-5">
          {wallet && (
            <div className="bg-slate-50 rounded-lg px-4 py-3 text-sm">
              <span className="text-slate-500">Bakiye: </span>
              <span className="font-semibold text-slate-800">{formatGrams(wallet.balanceGrams)} gram GOLD</span>
            </div>
          )}

          {price && (
            <div className="bg-slate-50 rounded-lg px-4 py-3 text-sm">
              <span className="text-slate-500">Güncel Fiyat: </span>
              <span className="font-semibold text-slate-800">{formatTRY(price.pricePerGramTRY)} / gram</span>
            </div>
          )}

          {/* Redeem type */}
          <div>
            <label className="text-sm font-medium text-slate-700 block mb-2">İtfa Türü</label>
            <div className="grid grid-cols-2 gap-2">
              {([
                { value: "cash", label: "💳 Nakit (IBAN)", desc: "TRY banka havalesi" },
                { value: "physical", label: "🏅 Fiziksel Altın", desc: "Min. 1 kg · 10 iş günü" },
              ] as { value: RedeemType; label: string; desc: string }[]).map((opt) => (
                <button
                  key={opt.value}
                  onClick={() => setRedeemType(opt.value)}
                  className={`p-3 rounded-xl border text-left transition-colors ${
                    redeemType === opt.value
                      ? "border-yellow-400 bg-yellow-50"
                      : "border-slate-200 hover:border-slate-300"
                  }`}
                >
                  <p className="text-sm font-medium text-slate-800">{opt.label}</p>
                  <p className="text-xs text-slate-500 mt-0.5">{opt.desc}</p>
                </button>
              ))}
            </div>
          </div>

          {/* Amount */}
          <div>
            <label className="text-sm font-medium text-slate-700 block mb-2">
              Miktar (gram)
              {redeemType === "physical" && (
                <span className="text-xs text-orange-600 ml-2">Min. 1.000 gram</span>
              )}
            </label>
            <input
              type="number"
              min={redeemType === "physical" ? "1000" : "1"}
              step="0.1"
              value={amountGrams}
              onChange={(e) => setAmountGrams(e.target.value)}
              className="w-full border border-slate-200 rounded-lg px-4 py-2.5 text-slate-900 focus:outline-none focus:ring-2 focus:ring-yellow-400"
            />
            {exceedsBalance && (
              <p className="text-xs text-red-600 mt-1 flex items-center gap-1">
                <AlertCircle size={12} /> Bakiyenizi aşıyor ({formatGrams(balance)} gram)
              </p>
            )}
          </div>

          {/* IBAN */}
          {redeemType === "cash" && (
            <div>
              <label className="text-sm font-medium text-slate-700 block mb-2">
                IBAN
              </label>
              <input
                type="text"
                value={iban}
                onChange={(e) => setIban(e.target.value)}
                placeholder="TR00 0000 0000 0000 0000 0000 00"
                className="w-full border border-slate-200 rounded-lg px-4 py-2.5 text-slate-900 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-yellow-400"
              />
            </div>
          )}

          {/* Total */}
          {grams > 0 && price && (
            <div className="border-t border-slate-100 pt-4 space-y-2 text-sm">
              <div className="flex justify-between text-slate-600">
                <span>{formatGrams(grams)} gram × {formatTRY(price.pricePerGramTRY)}</span>
                <span>{formatTRY(totalTRY)}</span>
              </div>
              <div className="flex justify-between font-semibold text-slate-900">
                <span>Alacaksınız</span>
                <span className="text-green-600">{formatTRY(totalTRY)}</span>
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
            disabled={loading || grams <= 0 || exceedsBalance || !price || (redeemType === "physical" && grams < 1000)}
            className="w-full bg-blue-600 text-white py-3 rounded-xl font-semibold hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-2"
          >
            {loading && <Loader2 size={16} className="animate-spin" />}
            <TrendingDown size={16} />
            Satış Siparişi Oluştur
          </button>
        </div>
      )}

      {/* STEP: Confirm */}
      {step === "confirm" && order && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-6 space-y-5">
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 text-sm text-blue-800">
            <p className="font-medium mb-1">İmza İsteği (EIP-2612)</p>
            <p className="text-blue-600 text-xs">
              Gerçek sistemde cüzdanınızdan BurnController'ı yetkilendirmeniz gerekir.
              Bu POC'ta otomatik onaylanacaktır.
            </p>
          </div>

          <div className="space-y-2 text-sm">
            {[
              ["Miktar", `${order.amountGrams} gram GOLD`],
              ["Birim Fiyat", formatTRY(order.pricePerGramTRY) + " / gram"],
              ["Alacaksınız", formatTRY(order.totalTRY)],
              ["İtfa Türü", redeemType === "cash" ? "Nakit (TRY)" : "Fiziksel Altın"],
            ].map(([label, value]) => (
              <div key={label} className="flex justify-between">
                <span className="text-slate-500">{label}</span>
                <span className="font-medium text-slate-800">{value}</span>
              </div>
            ))}
            {iban && (
              <div className="flex justify-between">
                <span className="text-slate-500">IBAN</span>
                <span className="font-mono text-xs text-slate-700">{iban}</span>
              </div>
            )}
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
              onClick={handleConfirm}
              disabled={loading}
              className="flex-1 bg-blue-600 text-white py-2.5 rounded-xl font-semibold hover:bg-blue-700 disabled:opacity-50 transition-colors flex items-center justify-center gap-2"
            >
              {loading && <Loader2 size={16} className="animate-spin" />}
              Satışı Onayla
            </button>
          </div>
        </div>
      )}

      {/* STEP: Processing */}
      {step === "processing" && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-10 text-center space-y-4">
          <Loader2 size={40} className="animate-spin text-blue-500 mx-auto" />
          <h2 className="text-lg font-semibold text-slate-800">Burn Saga İşleniyor</h2>
          <p className="text-slate-500 text-sm">
            Token yakılıyor, kasa rezervasyonu serbest bırakılıyor, ödeme hazırlanıyor...
          </p>
        </div>
      )}

      {/* STEP: Done */}
      {step === "done" && order && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-6 space-y-5">
          <div className="text-center py-4">
            <CheckCircle size={48} className="text-green-500 mx-auto mb-3" />
            <h2 className="text-xl font-bold text-slate-900 mb-1">Satış Tamamlandı!</h2>
            <p className="text-slate-500 text-sm">
              {order.amountGrams} gram GOLD yakıldı.{" "}
              {redeemType === "cash"
                ? `${formatTRY(order.totalTRY)} IBAN'ınıza gönderilecek.`
                : "Fiziksel altın kargolanacak."}
            </p>
          </div>

          <div className="space-y-2 text-sm border-t border-slate-100 pt-4">
            {[
              ["Sipariş ID", order.id.slice(0, 16) + "…"],
              ["Miktar", `${order.amountGrams} gram`],
              ["Tutar", formatTRY(order.totalTRY)],
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
            Yeni Satış
          </button>
        </div>
      )}
    </div>
  );
}
