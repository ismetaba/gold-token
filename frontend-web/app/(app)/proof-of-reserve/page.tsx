"use client";

import { useEffect, useState } from "react";
import { porApi } from "@/lib/api-client";
import type { ProofOfReserve } from "@/lib/types";
import {
  AlertCircle,
  CheckCircle,
  ExternalLink,
  Loader2,
  RefreshCw,
  Shield,
} from "lucide-react";

export default function ProofOfReservePage() {
  const [por, setPor] = useState<ProofOfReserve | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    porApi
      .getLatest()
      .then((r) => setPor(r.data))
      .catch(() => setError("Rezerv verileri yüklenemedi."))
      .finally(() => setLoading(false));
  }, []);

  const coveragePercent = por
    ? (parseFloat(por.coverageRatio) * 100).toFixed(2)
    : null;
  const isPerfect = por && parseFloat(por.coverageRatio) >= 1.0;

  if (loading) {
    return (
      <div className="p-8 flex items-center justify-center">
        <Loader2 size={24} className="animate-spin text-yellow-500" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="bg-red-50 border border-red-200 rounded-xl p-4 flex items-center gap-3 text-red-700">
          <AlertCircle size={16} />
          {error}
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 md:p-8 max-w-4xl">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900 flex items-center gap-2">
          <Shield size={24} className="text-yellow-500" />
          Rezerv Kanıtı
        </h1>
        <p className="text-slate-500 mt-1">
          Her GOLD tokeni fiziksel bir altın çubuğa tahsislidir. Aylık bağımsız
          denetim sonuçları blokzincirde ve IPFS'te yayınlanır.
        </p>
      </div>

      {por && (
        <>
          {/* Coverage banner */}
          <div
            className={`rounded-2xl border p-6 mb-6 ${
              isPerfect
                ? "bg-green-50 border-green-200"
                : "bg-red-50 border-red-200"
            }`}
          >
            <div className="flex items-center gap-3 mb-4">
              {isPerfect ? (
                <CheckCircle size={28} className="text-green-600" />
              ) : (
                <AlertCircle size={28} className="text-red-600" />
              )}
              <div>
                <h2
                  className={`text-xl font-bold ${
                    isPerfect ? "text-green-800" : "text-red-800"
                  }`}
                >
                  %{coveragePercent} Rezerv Karşılama
                </h2>
                <p
                  className={`text-sm ${
                    isPerfect ? "text-green-600" : "text-red-600"
                  }`}
                >
                  Atestasyon:{" "}
                  {new Date(por.attestedAt).toLocaleDateString("tr-TR", {
                    day: "numeric",
                    month: "long",
                    year: "numeric",
                  })}{" "}
                  · Denetçi: {por.auditor}
                </p>
              </div>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
              {[
                {
                  label: "Toplam Altın Rezervi",
                  value:
                    parseFloat(por.totalGoldGrams).toLocaleString("tr-TR") +
                    " gram",
                },
                {
                  label: "Toplam Token Arzı",
                  value:
                    parseFloat(por.totalTokenSupplyGrams).toLocaleString(
                      "tr-TR"
                    ) + " GOLD",
                },
                {
                  label: "Karşılama Oranı",
                  value: "%" + coveragePercent,
                },
                {
                  label: "Döngü ID",
                  value: por.cycleId,
                },
              ].map(({ label, value }) => (
                <div key={label}>
                  <p
                    className={`text-xs mb-0.5 ${
                      isPerfect ? "text-green-700" : "text-red-700"
                    }`}
                  >
                    {label}
                  </p>
                  <p
                    className={`font-semibold ${
                      isPerfect ? "text-green-900" : "text-red-900"
                    }`}
                  >
                    {value}
                  </p>
                </div>
              ))}
            </div>
          </div>

          {/* Vault breakdown */}
          <div className="bg-white rounded-2xl border border-slate-200 shadow-sm mb-6">
            <div className="px-6 py-4 border-b border-slate-100">
              <h2 className="font-semibold text-slate-900">Kasa Dağılımı</h2>
            </div>
            <div className="divide-y divide-slate-100">
              {por.vaults.map((vault) => {
                const pct = (
                  (parseFloat(vault.totalGoldGrams) /
                    parseFloat(por.totalGoldGrams)) *
                  100
                ).toFixed(1);
                return (
                  <div key={vault.vaultId} className="px-6 py-4">
                    <div className="flex items-start justify-between mb-2">
                      <div>
                        <p className="font-medium text-slate-800 text-sm">
                          {vault.vaultName}
                        </p>
                        <p className="text-xs text-slate-500">{vault.location}</p>
                      </div>
                      <div className="text-right">
                        <p className="font-semibold text-slate-800 text-sm">
                          {parseFloat(vault.totalGoldGrams).toLocaleString(
                            "tr-TR"
                          )}{" "}
                          gram
                        </p>
                        <p className="text-xs text-slate-500">
                          {vault.barCount} çubuk · %{pct}
                        </p>
                      </div>
                    </div>
                    {/* Bar */}
                    <div className="w-full bg-slate-100 rounded-full h-2">
                      <div
                        className="bg-yellow-400 h-2 rounded-full"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                    <p className="text-xs text-slate-400 mt-1">
                      Son denetim:{" "}
                      {new Date(vault.lastInspected).toLocaleDateString(
                        "tr-TR"
                      )}
                    </p>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Verification links */}
          <div className="bg-white rounded-2xl border border-slate-200 shadow-sm mb-6">
            <div className="px-6 py-4 border-b border-slate-100">
              <h2 className="font-semibold text-slate-900">Zincir Üstü Doğrulama</h2>
              <p className="text-sm text-slate-500 mt-0.5">
                Aşağıdaki veriler Ethereum mainnet ve IPFS üzerinde herkese açıktır.
              </p>
            </div>
            <div className="px-6 py-4 space-y-4">
              {[
                {
                  label: "Merkle Root",
                  value: por.merkleRoot,
                  mono: true,
                  link: null,
                },
                {
                  label: "IPFS Atestasyon",
                  value: por.ipfsCid,
                  mono: true,
                  link: `https://ipfs.io/ipfs/${por.ipfsCid}`,
                },
                {
                  label: "On-Chain TX",
                  value: por.onChainTxHash,
                  mono: true,
                  link: `https://etherscan.io/tx/${por.onChainTxHash}`,
                },
              ].map(({ label, value, mono, link }) => (
                <div key={label}>
                  <p className="text-xs font-medium text-slate-500 mb-1">
                    {label}
                  </p>
                  <div className="flex items-center gap-2 bg-slate-50 rounded-lg px-3 py-2">
                    <span
                      className={`text-xs text-slate-700 flex-1 break-all ${
                        mono ? "font-mono" : ""
                      }`}
                    >
                      {value}
                    </span>
                    {link && (
                      <a
                        href={link}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-yellow-600 hover:text-yellow-700 shrink-0"
                      >
                        <ExternalLink size={14} />
                      </a>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* How PoR works */}
          <div className="bg-slate-50 rounded-2xl border border-slate-200 p-6">
            <h3 className="font-semibold text-slate-800 mb-3">
              Rezerv Kanıtı Nasıl Çalışır?
            </h3>
            <ol className="space-y-2 text-sm text-slate-600">
              {[
                "Her ay kasa denetçileri fiziksel altın çubuklarını sayar ve tartarak doğrular.",
                "Denetim sonuçları Merkle ağacına dönüştürülür; her çubuk ayrı ayrı doğrulanabilir.",
                "Big Four denetçisi atestasyon raporu imzalar.",
                "Merkle root ve IPFS bağlantısı Ethereum'daki ReserveOracle sözleşmesine yayınlanır.",
                "Herkes atestasyon döngüsünü, IPFS'teki tam raporu ve zincir üstü hash'i doğrulayabilir.",
              ].map((step, i) => (
                <li key={i} className="flex items-start gap-3">
                  <span className="w-5 h-5 rounded-full bg-yellow-400 text-slate-900 text-xs flex items-center justify-center font-bold shrink-0 mt-0.5">
                    {i + 1}
                  </span>
                  {step}
                </li>
              ))}
            </ol>
          </div>
        </>
      )}
    </div>
  );
}
