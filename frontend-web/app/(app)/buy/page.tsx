"use client";

import { useEffect, useState } from "react";
import { ordersApi, priceApi } from "@/lib/api-client";
import { formatGrams, formatTRY } from "@/lib/utils";
import type { GoldPrice, Order } from "@/lib/types";
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle,
  ChevronRight,
  Loader2,
  ShoppingCart,
  Sparkles,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import StatusBadge from "@/components/StatusBadge";

type Step = "form" | "confirm" | "payment" | "status";

const STEP_LABEL: Record<Step, string> = {
  form: "Miktar",
  confirm: "Onay",
  payment: "Ödeme",
  status: "Durum",
};

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
      await new Promise((r) => setTimeout(r, 2000));
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

  const stepIdx = (["form", "confirm", "payment", "status"] as Step[]).indexOf(step);

  return (
    <div className="p-6 md:p-10 max-w-xl">
      {/* Header */}
      <div className="mb-7">
        <h1 className="text-3xl font-bold tracking-tight text-ink-0 mb-1">Altın Al</h1>
        <p className="text-ink-2">
          1 GOLD = 1 gram fiziksel altın. Minimum alım: 1 gram.
        </p>
      </div>

      {/* Stepper */}
      <ol className="flex items-center gap-1 text-sm mb-6" aria-label="Adımlar">
        {(["form", "confirm", "payment", "status"] as Step[]).map((s, i) => {
          const past = i < stepIdx;
          const current = i === stepIdx;
          return (
            <li key={s} className="flex items-center gap-1.5">
              <span
                className={`inline-flex items-center justify-center w-6 h-6 rounded-full text-xs font-semibold transition-colors ${
                  past
                    ? "bg-brand-300 text-night-0"
                    : current
                    ? "bg-brand-500 text-white shadow-gold"
                    : "bg-surface-2 text-ink-3"
                }`}
                aria-current={current ? "step" : undefined}
              >
                {past ? <CheckCircle size={14} /> : i + 1}
              </span>
              <span
                className={`text-sm ${
                  current ? "text-ink-0 font-semibold" : past ? "text-ink-2" : "text-ink-3"
                }`}
              >
                {STEP_LABEL[s]}
              </span>
              {i < 3 && <ChevronRight size={12} className="text-ink-3 mx-1" />}
            </li>
          );
        })}
      </ol>

      {/* STEP: Form */}
      {step === "form" && (
        <div className="card p-6 md:p-7 space-y-5 anim-rise">
          {user?.kycStatus !== "approved" && (
            <div className="rounded-xl bg-amber-50 border border-amber-200 px-4 py-3 flex items-start gap-2 text-sm text-amber-800">
              <AlertCircle size={16} className="mt-0.5 shrink-0" />
              <span>KYC onayı olmadan işlem gerçekleştirilemez. Önce kimliğinizi doğrulayın.</span>
            </div>
          )}

          {/* Price */}
          {price && (
            <div className="flex items-center justify-between rounded-xl bg-surface-2 px-4 py-3 text-sm">
              <span className="text-ink-2">Güncel Fiyat</span>
              <span className="font-semibold text-ink-0 tabular-nums">
                {formatTRY(price.pricePerGramTRY)} <span className="text-ink-2 font-normal">/gram</span>
              </span>
            </div>
          )}

          {/* Presets */}
          <div>
            <label className="text-xs uppercase tracking-widest text-ink-2 block mb-2">
              Miktar (gram)
            </label>
            <div className="flex flex-wrap gap-2 mb-3">
              {PRESET_GRAMS.map((g) => (
                <button
                  key={g}
                  type="button"
                  onClick={() => setAmountGrams(g)}
                  className={`px-3.5 py-1.5 rounded-xl text-sm font-medium border transition-all ${
                    amountGrams === g
                      ? "bg-ink-0 border-ink-0 text-white"
                      : "border-surface-3 text-ink-1 hover:border-ink-3"
                  }`}
                >
                  {g}g
                </button>
              ))}
            </div>
            <input
              type="number"
              inputMode="decimal"
              min="1"
              step="0.1"
              value={amountGrams}
              onChange={(e) => setAmountGrams(e.target.value)}
              className="input text-lg font-semibold tabular-nums"
              placeholder="Özel miktar girin"
            />
          </div>

          {/* Total preview */}
          {grams > 0 && price && (
            <div className="rounded-xl border border-surface-3 bg-surface-2/50 p-4 space-y-2 text-sm">
              <div className="flex justify-between text-ink-2">
                <span>
                  {formatGrams(grams)} g × {formatTRY(price.pricePerGramTRY)}
                </span>
                <span className="tabular-nums">{formatTRY(totalTRY)}</span>
              </div>
              <div className="hr-soft" />
              <div className="flex justify-between items-baseline font-semibold">
                <span className="text-ink-1">Toplam</span>
                <span className="text-2xl text-ink-0 tabular-nums">{formatTRY(totalTRY)}</span>
              </div>
            </div>
          )}

          {error && (
            <div className="rounded-xl bg-rose-50 border border-rose-200 px-4 py-3 text-rose-700 text-sm">
              {error}
            </div>
          )}

          <button
            onClick={handleCreateOrder}
            disabled={loading || grams <= 0 || !price}
            className="btn btn-primary w-full py-3 text-base"
          >
            {loading ? (
              <>
                <Loader2 size={16} className="animate-spin" />
                Oluşturuluyor…
              </>
            ) : (
              <>
                <Sparkles size={16} />
                Siparişi Oluştur
              </>
            )}
          </button>
        </div>
      )}

      {/* STEP: Confirm */}
      {step === "confirm" && order && (
        <div className="card p-6 md:p-7 space-y-5 anim-rise">
          <div className="rounded-xl bg-sky-50 border border-sky-200 px-4 py-3 flex items-center gap-3">
            <span className="w-9 h-9 rounded-xl bg-white border border-sky-200 flex items-center justify-center text-sky-700">
              <ShoppingCart size={16} />
            </span>
            <div className="min-w-0">
              <p className="text-sm font-semibold text-sky-900">Sipariş Özeti</p>
              <p className="text-xs text-sky-700 font-mono truncate">{order.id}</p>
            </div>
          </div>

          <dl className="rounded-xl border border-surface-3 divide-y divide-black/[0.05]">
            {[
              ["Miktar", `${order.amountGrams} gram GOLD`],
              ["Birim Fiyat", `${formatTRY(order.pricePerGramTRY)} / gram`],
              ["Toplam", formatTRY(order.totalTRY)],
            ].map(([label, value]) => (
              <div key={label} className="flex justify-between px-4 py-2.5 text-sm">
                <dt className="text-ink-2">{label}</dt>
                <dd className="font-medium text-ink-0 tabular-nums">{value}</dd>
              </div>
            ))}
            <div className="flex justify-between items-center px-4 py-2.5 text-sm">
              <dt className="text-ink-2">Durum</dt>
              <dd>
                <StatusBadge status={order.status} />
              </dd>
            </div>
          </dl>

          <div className="rounded-xl bg-surface-2 p-4 text-xs text-ink-2">
            <p className="font-semibold text-ink-1 mb-1">Simüle Ödeme (POC)</p>
            Gerçek sistemde kullanıcı İyzico / Stripe ödeme sayfasına yönlendirilir.
            Bu POC&apos;ta ödemeyi simüle edebilirsiniz.
          </div>

          {error && (
            <div className="rounded-xl bg-rose-50 border border-rose-200 px-4 py-3 text-rose-700 text-sm">
              {error}
            </div>
          )}

          <div className="flex gap-3">
            <button
              onClick={reset}
              className="btn btn-secondary flex-1"
            >
              <ArrowLeft size={14} />
              İptal
            </button>
            <button
              onClick={handleSimulatePayment}
              disabled={loading}
              className="btn flex-1 bg-emerald-500 text-white hover:bg-emerald-600 shadow-md"
            >
              {loading ? (
                <>
                  <Loader2 size={16} className="animate-spin" />
                  Ödeniyor…
                </>
              ) : (
                "Ödemeyi Simüle Et"
              )}
            </button>
          </div>
        </div>
      )}

      {/* STEP: Payment processing */}
      {step === "payment" && (
        <div className="card p-12 text-center space-y-4 anim-rise">
          <div className="relative inline-flex">
            <div className="absolute inset-0 rounded-full bg-brand-300/30 blur-xl animate-pulse-soft" />
            <Loader2 size={44} className="relative animate-spin text-brand-500" />
          </div>
          <h2 className="text-xl font-semibold tracking-tight text-ink-0">Ödeme İşleniyor</h2>
          <p className="text-ink-2 text-sm max-w-sm mx-auto">
            Ödemeniz doğrulanıyor ve mint saga başlatılıyor. Bu birkaç saniye sürebilir.
          </p>
        </div>
      )}

      {/* STEP: Status */}
      {step === "status" && order && (
        <div className="card p-6 md:p-7 space-y-5 anim-rise">
          {["MINT_EXECUTED", "COMPLETED"].includes(order.status) ? (
            <div className="text-center py-6">
              <div className="relative inline-flex mb-4">
                <div className="absolute inset-0 rounded-full bg-emerald-400/40 blur-2xl" />
                <CheckCircle size={56} className="relative text-emerald-500" />
              </div>
              <h2 className="text-2xl font-bold tracking-tight text-ink-0 mb-1">Alım Başarılı!</h2>
              <p className="text-ink-2 text-sm">
                <span className="font-semibold text-ink-0 tabular-nums">{order.amountGrams} gram</span>{" "}
                GOLD cüzdanınıza transfer edildi.
              </p>
            </div>
          ) : (
            <div className="text-center py-6">
              <div className="relative inline-flex mb-4">
                <div className="absolute inset-0 rounded-full bg-sky-400/40 blur-2xl" />
                <Loader2 size={56} className="relative animate-spin text-sky-500" />
              </div>
              <h2 className="text-2xl font-bold tracking-tight text-ink-0 mb-1">İşlem Devam Ediyor</h2>
              <p className="text-ink-2 text-sm">
                Mint onay sürecinde. E-posta ile bildirim alacaksınız.
              </p>
            </div>
          )}

          <dl className="rounded-xl border border-surface-3 divide-y divide-black/[0.05]">
            {[
              ["Sipariş ID", order.id.slice(0, 16) + "…"],
              ["Miktar", `${order.amountGrams} gram`],
              ["Toplam", formatTRY(order.totalTRY)],
            ].map(([label, value]) => (
              <div key={label} className="flex justify-between px-4 py-2.5 text-sm">
                <dt className="text-ink-2">{label}</dt>
                <dd className="font-medium text-ink-0 font-mono text-xs tabular-nums">{value}</dd>
              </div>
            ))}
            <div className="flex justify-between items-center px-4 py-2.5 text-sm">
              <dt className="text-ink-2">Durum</dt>
              <dd>
                <StatusBadge status={order.status} />
              </dd>
            </div>
          </dl>

          <button
            onClick={reset}
            className="btn btn-primary w-full py-3"
          >
            Yeni Alım
          </button>
        </div>
      )}
    </div>
  );
}
