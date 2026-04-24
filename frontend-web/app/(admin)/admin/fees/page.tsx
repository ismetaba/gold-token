"use client";

import { useEffect, useState } from "react";
import { adminApi } from "@/lib/api-client";
import type { FeeArena, FeeRule } from "@/lib/types";
import {
  AlertTriangle,
  CheckCircle,
  Edit2,
  Loader2,
  Settings,
  X,
} from "lucide-react";

// ── Helpers ──────────────────────────────────────────────────────────────────

const ORDER_TYPE_LABELS: Record<FeeRule["orderType"], string> = {
  buy:             "Buy",
  sell:            "Sell",
  redeem_cash:     "Redeem (Cash)",
  redeem_physical: "Redeem (Physical)",
};

const ARENA_LABELS: Record<FeeArena, string> = {
  global: "Global (default)",
  tr:     "Turkey (TR)",
  ch:     "Switzerland (CH)",
  ae:     "UAE (AE)",
  eu:     "Europe (EU)",
};

function bpsToPercent(bps: number): string {
  return (bps / 100).toFixed(2) + "%";
}

// ── Edit Modal ────────────────────────────────────────────────────────────────

function EditFeeModal({
  rule,
  onClose,
  onSaved,
}: {
  rule: FeeRule;
  onClose: () => void;
  onSaved: (updated: FeeRule) => void;
}) {
  const [feeBps, setFeeBps] = useState(rule.feeBps.toString());
  const [minFee, setMinFee] = useState(rule.minFeeTRY ?? "");
  const [maxFee, setMaxFee] = useState(rule.maxFeeTRY ?? "");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    const bps = parseInt(feeBps, 10);
    if (isNaN(bps) || bps < 0 || bps > 10000) {
      setError("Fee must be between 0 and 10000 bps (0–100%).");
      return;
    }
    setSubmitting(true);
    try {
      const res = await adminApi.updateFeeRule(rule.id, {
        arena: rule.arena,
        orderType: rule.orderType,
        feeBps: bps,
        minFeeTRY: minFee.trim() || undefined,
        maxFeeTRY: maxFee.trim() || undefined,
      });
      onSaved(res.data);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Save failed.");
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="bg-slate-900 border border-slate-700 rounded-2xl w-full max-w-md">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-700">
          <div>
            <div className="font-semibold text-white">Edit Fee Rule</div>
            <div className="text-xs text-slate-400 mt-0.5">
              {ARENA_LABELS[rule.arena]} — {ORDER_TYPE_LABELS[rule.orderType]}
            </div>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-white transition-colors">
            <X size={18} />
          </button>
        </div>

        <form onSubmit={handleSave} className="p-6 space-y-4">
          <div>
            <label className="block text-xs text-slate-400 mb-1">
              Fee (basis points) <span className="text-slate-500">— 1 bps = 0.01%, 50 bps = 0.5%</span>
            </label>
            <input
              type="number"
              min="0"
              max="10000"
              step="1"
              value={feeBps}
              onChange={(e) => setFeeBps(e.target.value)}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-yellow-400 transition-colors"
            />
            {feeBps && !isNaN(parseInt(feeBps)) && (
              <div className="text-xs text-slate-500 mt-1">
                = {bpsToPercent(parseInt(feeBps))}
              </div>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-slate-400 mb-1">Min fee (TRY) <span className="text-slate-500">optional</span></label>
              <input
                type="number"
                min="0"
                step="0.01"
                value={minFee}
                onChange={(e) => setMinFee(e.target.value)}
                placeholder="e.g. 5.00"
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-600 focus:outline-none focus:border-yellow-400 transition-colors"
              />
            </div>
            <div>
              <label className="block text-xs text-slate-400 mb-1">Max fee (TRY) <span className="text-slate-500">optional</span></label>
              <input
                type="number"
                min="0"
                step="0.01"
                value={maxFee}
                onChange={(e) => setMaxFee(e.target.value)}
                placeholder="e.g. 1000.00"
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-600 focus:outline-none focus:border-yellow-400 transition-colors"
              />
            </div>
          </div>

          {error && (
            <div className="flex items-center gap-2 text-red-400 text-xs">
              <AlertTriangle size={12} />
              {error}
            </div>
          )}

          <div className="flex gap-3 pt-1">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 text-sm py-2.5 rounded-xl transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="flex-1 bg-yellow-400 hover:bg-yellow-300 disabled:opacity-50 text-slate-900 font-semibold text-sm py-2.5 rounded-xl transition-colors flex items-center justify-center gap-2"
            >
              {submitting ? <Loader2 size={14} className="animate-spin" /> : null}
              {submitting ? "Saving…" : "Save Changes"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function AdminFeesPage() {
  const [rules, setRules] = useState<FeeRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingRule, setEditingRule] = useState<FeeRule | null>(null);
  const [savedId, setSavedId] = useState<string | null>(null);

  useEffect(() => {
    adminApi.listFeeRules().then((r) => {
      setRules(r.data);
      setLoading(false);
    });
  }, []);

  const handleSaved = (updated: FeeRule) => {
    setRules((prev) => prev.map((r) => (r.id === updated.id ? updated : r)));
    setEditingRule(null);
    setSavedId(updated.id);
    setTimeout(() => setSavedId(null), 3000);
  };

  // Group by arena
  const arenaOrder: FeeArena[] = ["global", "tr", "ae", "ch", "eu"];
  const byArena = arenaOrder
    .map((arena) => ({
      arena,
      rules: rules.filter((r) => r.arena === arena),
    }))
    .filter(({ rules: r }) => r.length > 0);

  return (
    <div className="p-6 md:p-8 max-w-4xl">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <Settings size={22} className="text-yellow-400" />
          Fee Configuration
        </h1>
        <p className="text-slate-400 text-sm mt-0.5">
          Manage buy, sell, and redemption fees by arena
        </p>
      </div>

      {/* Info banner */}
      <div className="bg-slate-800 border border-slate-700 rounded-2xl p-4 mb-6 flex items-start gap-3">
        <AlertTriangle size={15} className="text-yellow-400 shrink-0 mt-0.5" />
        <p className="text-sm text-slate-400">
          Arena-specific rules override the global defaults. Changes take effect on the next order processed in that arena.
          Fee amounts are in <strong className="text-slate-300">basis points</strong> (50 bps = 0.5%).
        </p>
      </div>

      {loading ? (
        <div className="flex justify-center py-20">
          <Loader2 size={24} className="animate-spin text-yellow-400" />
        </div>
      ) : (
        <div className="space-y-6">
          {byArena.map(({ arena, rules: arenaRules }) => (
            <div key={arena}>
              <h2 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">
                {ARENA_LABELS[arena]}
              </h2>
              <div className="bg-slate-800 border border-slate-700 rounded-2xl overflow-hidden">
                {/* Desktop table */}
                <div className="hidden md:block">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-slate-700 text-slate-400 text-xs uppercase tracking-wider">
                        <th className="px-5 py-3 text-left font-medium">Order Type</th>
                        <th className="px-5 py-3 text-right font-medium">Fee (bps)</th>
                        <th className="px-5 py-3 text-right font-medium">Rate</th>
                        <th className="px-5 py-3 text-right font-medium">Min (TRY)</th>
                        <th className="px-5 py-3 text-right font-medium">Max (TRY)</th>
                        <th className="px-5 py-3 text-left font-medium">Updated</th>
                        <th className="px-5 py-3 text-center font-medium">Action</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-700/60">
                      {arenaRules.map((rule) => (
                        <tr
                          key={rule.id}
                          className={`hover:bg-slate-700/30 transition-colors ${savedId === rule.id ? "bg-green-500/5" : ""}`}
                        >
                          <td className="px-5 py-3 font-medium text-white">
                            {ORDER_TYPE_LABELS[rule.orderType]}
                            {savedId === rule.id && (
                              <CheckCircle size={12} className="inline ml-2 text-green-400" />
                            )}
                          </td>
                          <td className="px-5 py-3 text-right font-mono text-slate-200">{rule.feeBps}</td>
                          <td className="px-5 py-3 text-right font-semibold text-yellow-300">
                            {bpsToPercent(rule.feeBps)}
                          </td>
                          <td className="px-5 py-3 text-right text-slate-300">
                            {rule.minFeeTRY ? `₺${rule.minFeeTRY}` : <span className="text-slate-600">—</span>}
                          </td>
                          <td className="px-5 py-3 text-right text-slate-300">
                            {rule.maxFeeTRY ? `₺${rule.maxFeeTRY}` : <span className="text-slate-600">—</span>}
                          </td>
                          <td className="px-5 py-3 text-slate-400 text-xs">
                            {new Date(rule.updatedAt).toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" })}
                          </td>
                          <td className="px-5 py-3 text-center">
                            <button
                              onClick={() => setEditingRule(rule)}
                              className="inline-flex items-center gap-1 text-xs text-slate-400 hover:text-yellow-400 transition-colors"
                            >
                              <Edit2 size={12} />
                              Edit
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {/* Mobile cards */}
                <div className="md:hidden divide-y divide-slate-700/60">
                  {arenaRules.map((rule) => (
                    <div key={rule.id} className={`p-4 ${savedId === rule.id ? "bg-green-500/5" : ""}`}>
                      <div className="flex items-center justify-between mb-2">
                        <div className="font-medium text-white flex items-center gap-1.5">
                          {ORDER_TYPE_LABELS[rule.orderType]}
                          {savedId === rule.id && <CheckCircle size={12} className="text-green-400" />}
                        </div>
                        <button
                          onClick={() => setEditingRule(rule)}
                          className="flex items-center gap-1 text-xs text-slate-400 hover:text-yellow-400 transition-colors"
                        >
                          <Edit2 size={12} />
                          Edit
                        </button>
                      </div>
                      <div className="flex items-center gap-4 text-xs text-slate-400">
                        <span><span className="text-yellow-300 font-semibold">{bpsToPercent(rule.feeBps)}</span> ({rule.feeBps} bps)</span>
                        {rule.minFeeTRY && <span>Min ₺{rule.minFeeTRY}</span>}
                        {rule.maxFeeTRY && <span>Max ₺{rule.maxFeeTRY}</span>}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Edit modal */}
      {editingRule && (
        <EditFeeModal
          rule={editingRule}
          onClose={() => setEditingRule(null)}
          onSaved={handleSaved}
        />
      )}
    </div>
  );
}
