"use client";

import { useEffect, useState } from "react";
import { adminApi } from "@/lib/api-client";
import type { ProofOfReserve } from "@/lib/types";
import {
  AlertTriangle,
  CheckCircle,
  ExternalLink,
  Loader2,
  MapPin,
  RefreshCw,
  Shield,
  Zap,
} from "lucide-react";

// ── Helpers ──────────────────────────────────────────────────────────────────

function CoverageBar({ ratio }: { ratio: string }) {
  const pct = Math.min(parseFloat(ratio) * 100, 100);
  const safe = pct >= 100;
  return (
    <div>
      <div className="flex items-center justify-between text-xs mb-1">
        <span className="text-slate-400">Coverage ratio</span>
        <span className={safe ? "text-green-400 font-semibold" : "text-red-400 font-semibold"}>
          {(parseFloat(ratio) * 100).toFixed(2)}%
        </span>
      </div>
      <div className="h-2 bg-slate-700 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all ${safe ? "bg-green-500" : "bg-red-500"}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

function PorCard({
  por,
  isLatest,
}: {
  por: ProofOfReserve;
  isLatest?: boolean;
}) {
  const [expanded, setExpanded] = useState(isLatest);

  return (
    <div
      className={`bg-slate-800 border rounded-2xl overflow-hidden transition-colors ${
        isLatest ? "border-yellow-500/40" : "border-slate-700"
      }`}
    >
      {/* Header */}
      <button
        onClick={() => setExpanded((v) => !v)}
        className="w-full px-5 py-4 flex items-center justify-between text-left hover:bg-slate-700/30 transition-colors"
      >
        <div className="flex items-center gap-3">
          {isLatest && (
            <span className="bg-yellow-400 text-slate-900 text-[10px] font-bold px-1.5 py-0.5 rounded uppercase tracking-wider">
              Latest
            </span>
          )}
          <div>
            <div className="font-semibold text-white text-sm">
              {new Date(por.attestedAt).toLocaleDateString("en-GB", {
                day: "2-digit", month: "long", year: "numeric",
              })}
            </div>
            <div className="text-xs text-slate-400 mt-0.5">
              {parseFloat(por.totalGoldGrams).toLocaleString()} g reserve ·{" "}
              {parseFloat(por.totalTokenSupplyGrams).toLocaleString()} g supply · {por.auditor}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <CheckCircle size={15} className="text-green-400 shrink-0" />
          <span className="text-slate-400 text-lg leading-none">
            {expanded ? "−" : "+"}
          </span>
        </div>
      </button>

      {/* Expanded detail */}
      {expanded && (
        <div className="px-5 pb-5 space-y-4 border-t border-slate-700/60 pt-4">
          <CoverageBar ratio={por.coverageRatio} />

          {/* Key numbers */}
          <div className="grid grid-cols-2 md:grid-cols-3 gap-3 text-sm">
            <div className="bg-slate-900 rounded-xl p-3">
              <div className="text-slate-500 text-xs mb-1">Physical Gold</div>
              <div className="font-semibold text-white">{parseFloat(por.totalGoldGrams).toLocaleString()} g</div>
            </div>
            <div className="bg-slate-900 rounded-xl p-3">
              <div className="text-slate-500 text-xs mb-1">Token Supply</div>
              <div className="font-semibold text-white">{parseFloat(por.totalTokenSupplyGrams).toLocaleString()} GOLD</div>
            </div>
            <div className="bg-slate-900 rounded-xl p-3">
              <div className="text-slate-500 text-xs mb-1">Auditor</div>
              <div className="font-semibold text-white">{por.auditor}</div>
            </div>
          </div>

          {/* Vault breakdown */}
          <div>
            <h4 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">
              Vault Breakdown
            </h4>
            <div className="space-y-2">
              {por.vaults.map((vault) => (
                <div key={vault.vaultId} className="flex items-center justify-between bg-slate-900 rounded-xl px-4 py-2.5">
                  <div className="flex items-center gap-2">
                    <MapPin size={12} className="text-yellow-400 shrink-0" />
                    <div>
                      <div className="text-sm text-white font-medium">{vault.vaultName}</div>
                      <div className="text-xs text-slate-400">{vault.location} · {vault.barCount} bars</div>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-sm font-semibold text-white">
                      {parseFloat(vault.totalGoldGrams).toLocaleString()} g
                    </div>
                    <div className="text-xs text-slate-500">
                      {((parseFloat(vault.totalGoldGrams) / parseFloat(por.totalGoldGrams)) * 100).toFixed(1)}%
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Cryptographic proof */}
          <div className="space-y-1.5 text-xs">
            <div className="flex items-start gap-2 text-slate-400">
              <span className="text-slate-500 shrink-0 w-20">Merkle Root</span>
              <span className="font-mono text-slate-300 break-all">{por.merkleRoot}</span>
            </div>
            <div className="flex items-start gap-2 text-slate-400">
              <span className="text-slate-500 shrink-0 w-20">IPFS CID</span>
              <span className="font-mono text-slate-300 break-all">{por.ipfsCid}</span>
            </div>
            <div className="flex items-start gap-2 text-slate-400">
              <span className="text-slate-500 shrink-0 w-20">On-chain Tx</span>
              <span className="font-mono text-slate-300 break-all">{por.onChainTxHash}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function AdminReservesPage() {
  const [porHistory, setPorHistory] = useState<ProofOfReserve[]>([]);
  const [loading, setLoading] = useState(true);
  const [attesting, setAttesting] = useState(false);
  const [attestError, setAttestError] = useState<string | null>(null);
  const [attestSuccess, setAttestSuccess] = useState(false);

  const load = async () => {
    setLoading(true);
    const res = await adminApi.listPorHistory();
    setPorHistory(res.data);
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  const handleAttest = async () => {
    setAttesting(true);
    setAttestError(null);
    setAttestSuccess(false);
    try {
      const res = await adminApi.triggerPorAttestation();
      setPorHistory((prev) => [res.data, ...prev]);
      setAttestSuccess(true);
      setTimeout(() => setAttestSuccess(false), 4000);
    } catch (err: unknown) {
      setAttestError(err instanceof Error ? err.message : "Attestation failed.");
    } finally {
      setAttesting(false);
    }
  };

  const latest = porHistory[0];

  return (
    <div className="p-6 md:p-8 max-w-4xl">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Shield size={22} className="text-yellow-400" />
            Gold Reserves
          </h1>
          <p className="text-slate-400 text-sm mt-0.5">
            Proof of Reserve attestation history and triggers
          </p>
        </div>

        <button
          onClick={handleAttest}
          disabled={attesting}
          className="flex items-center gap-2 bg-yellow-400 hover:bg-yellow-300 disabled:opacity-50 text-slate-900 font-semibold text-sm px-5 py-2.5 rounded-xl transition-colors"
        >
          {attesting ? (
            <><Loader2 size={14} className="animate-spin" /> Attesting…</>
          ) : (
            <><Zap size={14} /> Trigger Attestation</>
          )}
        </button>
      </div>

      {/* Attest feedback */}
      {attestSuccess && (
        <div className="mb-4 flex items-center gap-2 bg-green-500/10 border border-green-500/30 text-green-300 text-sm px-4 py-3 rounded-xl">
          <CheckCircle size={15} />
          New attestation submitted and anchored on-chain.
        </div>
      )}
      {attestError && (
        <div className="mb-4 flex items-center gap-2 bg-red-500/10 border border-red-500/30 text-red-300 text-sm px-4 py-3 rounded-xl">
          <AlertTriangle size={15} />
          {attestError}
        </div>
      )}

      {/* Current coverage summary */}
      {latest && (
        <div className="bg-slate-800 border border-slate-700 rounded-2xl p-5 mb-6">
          <h2 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">
            Current Reserve Status
          </h2>
          <CoverageBar ratio={latest.coverageRatio} />
          <div className="mt-3 grid grid-cols-3 gap-3 text-center">
            <div>
              <div className="text-lg font-bold text-white">
                {parseFloat(latest.totalGoldGrams).toLocaleString()} g
              </div>
              <div className="text-xs text-slate-400">Physical Gold</div>
            </div>
            <div>
              <div className="text-lg font-bold text-white">
                {parseFloat(latest.totalTokenSupplyGrams).toLocaleString()} g
              </div>
              <div className="text-xs text-slate-400">Token Supply</div>
            </div>
            <div>
              <div className="text-lg font-bold text-white">{latest.vaults.length}</div>
              <div className="text-xs text-slate-400">Vaults</div>
            </div>
          </div>
        </div>
      )}

      {/* History */}
      <h2 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">
        Attestation History
      </h2>

      {loading ? (
        <div className="flex justify-center py-16">
          <Loader2 size={24} className="animate-spin text-yellow-400" />
        </div>
      ) : porHistory.length === 0 ? (
        <div className="text-center py-16 text-slate-500">No attestations yet.</div>
      ) : (
        <div className="space-y-3">
          {porHistory.map((por, i) => (
            <PorCard key={por.id} por={por} isLatest={i === 0} />
          ))}
        </div>
      )}
    </div>
  );
}
