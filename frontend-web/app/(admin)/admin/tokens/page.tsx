"use client";

import { useEffect, useState } from "react";
import { adminApi } from "@/lib/api-client";
import type { AdminTokenOpPayload, TokenOperation, TokenOpType } from "@/lib/types";
import {
  AlertTriangle,
  ArrowRight,
  CheckCircle,
  Clock,
  Coins,
  Loader2,
  Plus,
  TrendingDown,
  TrendingUp,
  XCircle,
} from "lucide-react";

// ── Helpers ──────────────────────────────────────────────────────────────────

function opTypeBadge(type: TokenOpType) {
  const map = {
    mint:     { label: "Mint",     cls: "bg-green-500/20 text-green-300 border border-green-500/40" },
    burn:     { label: "Burn",     cls: "bg-red-500/20 text-red-300 border border-red-500/40" },
    transfer: { label: "Transfer", cls: "bg-blue-500/20 text-blue-300 border border-blue-500/40" },
  };
  const { label, cls } = map[type];
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>{label}</span>;
}

function statusIcon(status: TokenOperation["status"]) {
  switch (status) {
    case "confirmed": return <CheckCircle size={13} className="text-green-400" />;
    case "submitted": return <Clock size={13} className="text-yellow-400" />;
    case "pending":   return <Clock size={13} className="text-slate-400" />;
    case "failed":    return <XCircle size={13} className="text-red-400" />;
  }
}

function truncateHash(hash?: string) {
  if (!hash) return "—";
  return `${hash.slice(0, 10)}…${hash.slice(-6)}`;
}

// ── New Op Form ───────────────────────────────────────────────────────────────

function NewOpForm({ onSubmitted }: { onSubmitted: (op: TokenOperation) => void }) {
  const [type, setType] = useState<TokenOpType>("mint");
  const [amountGrams, setAmountGrams] = useState("");
  const [targetUserId, setTargetUserId] = useState("");
  const [toAddress, setToAddress] = useState("");
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    const grams = parseFloat(amountGrams);
    if (!grams || grams <= 0) { setError("Enter a valid amount."); return; }
    if (!reason.trim()) { setError("Reason is required."); return; }
    if (type === "transfer" && !toAddress.trim()) { setError("Destination address required for transfers."); return; }

    setSubmitting(true);
    try {
      const payload: AdminTokenOpPayload = {
        type,
        amountGrams,
        reason,
        ...(targetUserId.trim() ? { targetUserId: targetUserId.trim() } : {}),
        ...(toAddress.trim() ? { toAddress: toAddress.trim() } : {}),
      };
      const res = await adminApi.submitTokenOp(payload);
      onSubmitted(res.data);
      setSuccess(true);
      setAmountGrams("");
      setTargetUserId("");
      setToAddress("");
      setReason("");
      setTimeout(() => setSuccess(false), 3000);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Submission failed.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="bg-slate-800 border border-slate-700 rounded-2xl p-6">
      <h2 className="font-semibold text-white mb-5 flex items-center gap-2">
        <Plus size={16} className="text-yellow-400" />
        New Token Operation
      </h2>

      {/* Op type */}
      <div className="flex gap-2 mb-4">
        {(["mint", "burn", "transfer"] as TokenOpType[]).map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setType(t)}
            className={`flex-1 py-2 rounded-lg text-sm font-medium transition-colors capitalize ${
              type === t
                ? t === "mint"
                  ? "bg-green-600 text-white"
                  : t === "burn"
                  ? "bg-red-600 text-white"
                  : "bg-blue-600 text-white"
                : "bg-slate-700 text-slate-400 hover:text-white"
            }`}
          >
            {t === "mint" && <TrendingUp size={13} className="inline mr-1" />}
            {t === "burn" && <TrendingDown size={13} className="inline mr-1" />}
            {t === "transfer" && <ArrowRight size={13} className="inline mr-1" />}
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </button>
        ))}
      </div>

      <div className="space-y-3">
        {/* Amount */}
        <div>
          <label className="block text-xs text-slate-400 mb-1">Amount (grams)</label>
          <input
            type="number"
            step="0.01"
            min="0.01"
            value={amountGrams}
            onChange={(e) => setAmountGrams(e.target.value)}
            placeholder="e.g. 100.00"
            className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-yellow-400 transition-colors"
          />
        </div>

        {/* Target user (optional for mint/burn) */}
        {(type === "mint" || type === "burn") && (
          <div>
            <label className="block text-xs text-slate-400 mb-1">
              Target User ID <span className="text-slate-500">(optional)</span>
            </label>
            <input
              type="text"
              value={targetUserId}
              onChange={(e) => setTargetUserId(e.target.value)}
              placeholder="usr-000..."
              className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-yellow-400 transition-colors"
            />
          </div>
        )}

        {/* Destination address (required for transfer) */}
        {type === "transfer" && (
          <div>
            <label className="block text-xs text-slate-400 mb-1">
              Destination Address <span className="text-red-400">*</span>
            </label>
            <input
              type="text"
              value={toAddress}
              onChange={(e) => setToAddress(e.target.value)}
              placeholder="0x..."
              className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-yellow-400 transition-colors font-mono"
            />
          </div>
        )}

        {/* Reason */}
        <div>
          <label className="block text-xs text-slate-400 mb-1">
            Reason / Justification <span className="text-red-400">*</span>
          </label>
          <textarea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={2}
            placeholder="Document the reason for this operation…"
            className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-yellow-400 transition-colors resize-none"
          />
        </div>
      </div>

      {error && (
        <div className="mt-3 flex items-center gap-2 text-red-400 text-xs">
          <AlertTriangle size={12} />
          {error}
        </div>
      )}
      {success && (
        <div className="mt-3 flex items-center gap-2 text-green-400 text-xs">
          <CheckCircle size={12} />
          Operation submitted successfully.
        </div>
      )}

      <button
        type="submit"
        disabled={submitting}
        className="mt-4 w-full bg-yellow-400 hover:bg-yellow-300 disabled:opacity-50 text-slate-900 font-semibold text-sm py-2.5 rounded-lg transition-colors flex items-center justify-center gap-2"
      >
        {submitting ? (
          <><Loader2 size={14} className="animate-spin" /> Submitting…</>
        ) : (
          <>Submit Operation</>
        )}
      </button>
    </form>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function AdminTokensPage() {
  const [ops, setOps] = useState<TokenOperation[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    adminApi.listTokenOps().then((r) => {
      setOps(r.data);
      setLoading(false);
    });
  }, []);

  const handleNewOp = (op: TokenOperation) => {
    setOps((prev) => [op, ...prev]);
  };

  return (
    <div className="p-6 md:p-8 max-w-6xl">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <Coins size={22} className="text-yellow-400" />
          Token Operations
        </h1>
        <p className="text-slate-400 text-sm mt-0.5">
          Mint, burn, or transfer GOLD tokens on-chain
        </p>
      </div>

      <div className="grid md:grid-cols-5 gap-6">
        {/* Form — 2 cols */}
        <div className="md:col-span-2">
          <NewOpForm onSubmitted={handleNewOp} />
        </div>

        {/* History — 3 cols */}
        <div className="md:col-span-3">
          <h2 className="font-semibold text-slate-300 mb-3 text-sm uppercase tracking-wider">
            Recent Operations
          </h2>

          {loading ? (
            <div className="flex justify-center py-16">
              <Loader2 size={24} className="animate-spin text-yellow-400" />
            </div>
          ) : ops.length === 0 ? (
            <div className="text-center py-16 text-slate-500">No operations yet.</div>
          ) : (
            <div className="bg-slate-800 border border-slate-700 rounded-2xl overflow-hidden">
              <div className="divide-y divide-slate-700/60">
                {ops.map((op) => (
                  <div key={op.id} className="px-5 py-4">
                    <div className="flex items-center justify-between mb-1.5">
                      <div className="flex items-center gap-2">
                        {opTypeBadge(op.type)}
                        <span className="font-semibold text-white text-sm">
                          {parseFloat(op.amountGrams).toLocaleString()} g
                        </span>
                      </div>
                      <div className="flex items-center gap-1.5 text-xs text-slate-400">
                        {statusIcon(op.status)}
                        <span className="capitalize">{op.status}</span>
                      </div>
                    </div>

                    <div className="text-xs text-slate-400 space-y-0.5">
                      {op.targetUserId && (
                        <div>User: <span className="font-mono text-slate-300">{op.targetUserId}</span></div>
                      )}
                      {op.toAddress && (
                        <div>To: <span className="font-mono text-slate-300">{op.toAddress.slice(0, 12)}…{op.toAddress.slice(-6)}</span></div>
                      )}
                      {op.txHash && (
                        <div>Tx: <span className="font-mono text-slate-300">{truncateHash(op.txHash)}</span></div>
                      )}
                      <div className="text-slate-500 pt-0.5">
                        {new Date(op.createdAt).toLocaleString("en-GB", {
                          day: "2-digit", month: "short", year: "numeric",
                          hour: "2-digit", minute: "2-digit",
                        })}
                        {op.confirmedAt && (
                          <> · confirmed {new Date(op.confirmedAt).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" })}</>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
