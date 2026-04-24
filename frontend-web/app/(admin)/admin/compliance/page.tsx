"use client";

import { useEffect, useState } from "react";
import { adminApi } from "@/lib/api-client";
import type {
  ComplianceFlag,
  ComplianceFlagReason,
  ComplianceFlagStatus,
  SanctionsScreeningResult,
} from "@/lib/types";
import {
  AlertTriangle,
  CheckCircle,
  ChevronDown,
  Clock,
  Loader2,
  Search,
  Shield,
  XCircle,
} from "lucide-react";

// ── Helpers ──────────────────────────────────────────────────────────────────

function riskBadge(score: number) {
  const cls =
    score >= 8
      ? "bg-red-500/20 text-red-300 border border-red-500/40"
      : score >= 5
      ? "bg-yellow-500/20 text-yellow-300 border border-yellow-500/40"
      : "bg-green-500/20 text-green-300 border border-green-500/40";
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold ${cls}`}>
      {score}/10
    </span>
  );
}

function flagStatusBadge(status: ComplianceFlagStatus) {
  const map: Record<ComplianceFlagStatus, { label: string; cls: string }> = {
    open:          { label: "Open",          cls: "bg-red-500/20 text-red-300 border border-red-500/40" },
    investigating: { label: "Investigating", cls: "bg-yellow-500/20 text-yellow-300 border border-yellow-500/40" },
    cleared:       { label: "Cleared",       cls: "bg-green-500/20 text-green-300 border border-green-500/40" },
    escalated:     { label: "Escalated",     cls: "bg-purple-500/20 text-purple-300 border border-purple-500/40" },
  };
  const { label, cls } = map[status] ?? { label: status, cls: "bg-slate-700 text-slate-300" };
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>{label}</span>;
}

function sanctionsStatusBadge(status: SanctionsScreeningResult["status"]) {
  const map = {
    no_match:        { label: "No Match",        cls: "bg-green-500/20 text-green-300 border border-green-500/40" },
    potential_match: { label: "Potential Match",  cls: "bg-yellow-500/20 text-yellow-300 border border-yellow-500/40" },
    confirmed_hit:   { label: "Confirmed Hit",    cls: "bg-red-500/20 text-red-300 border border-red-500/40" },
  };
  const { label, cls } = map[status];
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>{label}</span>;
}

const REASON_LABELS: Record<ComplianceFlagReason, string> = {
  sanctions_hit:             "Sanctions Hit",
  pep_match:                 "PEP Match",
  adverse_media:             "Adverse Media",
  high_risk_jurisdiction:    "High-Risk Jurisdiction",
  unusual_activity:          "Unusual Activity",
  structuring_detected:      "Structuring Detected",
};

const FLAG_STATUS_OPTIONS: { value: ComplianceFlagStatus | "all"; label: string }[] = [
  { value: "all",          label: "All Statuses" },
  { value: "open",         label: "Open" },
  { value: "investigating", label: "Investigating" },
  { value: "escalated",    label: "Escalated" },
  { value: "cleared",      label: "Cleared" },
];

// ── Component ────────────────────────────────────────────────────────────────

export default function AdminCompliancePage() {
  const [flags, setFlags] = useState<ComplianceFlag[]>([]);
  const [screenings, setScreenings] = useState<SanctionsScreeningResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<"flags" | "screenings">("flags");
  const [statusFilter, setStatusFilter] = useState<ComplianceFlagStatus | "all">("all");
  const [search, setSearch] = useState("");
  const [updating, setUpdating] = useState<string | null>(null);
  const [updateError, setUpdateError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      adminApi.listComplianceFlags(),
      adminApi.listSanctionsScreenings(),
    ]).then(([flagsRes, screeningsRes]) => {
      setFlags(flagsRes.data);
      setScreenings(screeningsRes.data);
      setLoading(false);
    });
  }, []);

  const filteredFlags = flags.filter((f) => {
    const matchStatus = statusFilter === "all" || f.status === statusFilter;
    const matchSearch =
      !search.trim() ||
      f.userFullName.toLowerCase().includes(search.toLowerCase()) ||
      f.userEmail.toLowerCase().includes(search.toLowerCase());
    return matchStatus && matchSearch;
  });

  const filteredScreenings = screenings.filter(
    (s) =>
      !search.trim() ||
      s.userFullName.toLowerCase().includes(search.toLowerCase()) ||
      s.userEmail.toLowerCase().includes(search.toLowerCase())
  );

  const handleUpdateStatus = async (flag: ComplianceFlag, newStatus: ComplianceFlagStatus) => {
    setUpdating(flag.id);
    setUpdateError(null);
    const prev = flags;
    setFlags((all) => all.map((f) => f.id === flag.id ? { ...f, status: newStatus, updatedAt: new Date().toISOString() } : f));
    try {
      await adminApi.updateComplianceFlagStatus(flag.id, newStatus);
    } catch (err: unknown) {
      setFlags(prev);
      setUpdateError(err instanceof Error ? err.message : "Update failed.");
    } finally {
      setUpdating(null);
    }
  };

  const openCount  = flags.filter((f) => f.status === "open").length;
  const investCount = flags.filter((f) => f.status === "investigating").length;
  const pendingScreenings = screenings.filter((s) => s.status === "potential_match" || s.status === "confirmed_hit").length;

  return (
    <div className="p-6 md:p-8 max-w-7xl">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <Shield size={22} className="text-yellow-400" />
          Compliance Dashboard
        </h1>
        <p className="text-slate-400 text-sm mt-0.5">
          Sanctions screening, flagged accounts, and AML monitoring
        </p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        {[
          { label: "Open Flags",           value: openCount,          accent: openCount > 0 ? "bg-red-500/10 border-red-500/30" : "bg-slate-800 border-slate-700" },
          { label: "Investigating",         value: investCount,        accent: investCount > 0 ? "bg-yellow-500/10 border-yellow-500/30" : "bg-slate-800 border-slate-700" },
          { label: "Pending Screenings",    value: pendingScreenings,  accent: pendingScreenings > 0 ? "bg-yellow-500/10 border-yellow-500/30" : "bg-slate-800 border-slate-700" },
          { label: "Total Flags (all time)", value: flags.length,      accent: "bg-slate-800 border-slate-700" },
        ].map(({ label, value, accent }) => (
          <div key={label} className={`rounded-2xl p-5 border ${accent}`}>
            <div className="text-xs text-slate-400 mb-2">{label}</div>
            <div className="text-2xl font-bold text-white">{value}</div>
          </div>
        ))}
      </div>

      {/* Error */}
      {updateError && (
        <div className="mb-4 flex items-center gap-2 bg-red-500/10 border border-red-500/30 text-red-300 text-sm px-4 py-3 rounded-xl">
          <AlertTriangle size={15} className="shrink-0" />
          <span>{updateError}</span>
          <button onClick={() => setUpdateError(null)} className="ml-auto">
            <XCircle size={14} className="text-red-400 hover:text-red-200" />
          </button>
        </div>
      )}

      {/* Tabs + filters */}
      <div className="flex flex-col sm:flex-row sm:items-center gap-3 mb-5">
        <div className="flex rounded-xl overflow-hidden border border-slate-700 shrink-0">
          {(["flags", "screenings"] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-4 py-2 text-sm font-medium transition-colors ${
                tab === t ? "bg-yellow-400 text-slate-900" : "bg-slate-800 text-slate-400 hover:text-white"
              }`}
            >
              {t === "flags" ? "Flagged Accounts" : "Sanctions Screenings"}
            </button>
          ))}
        </div>

        <div className="relative flex-1">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
          <input
            type="text"
            placeholder="Search by name or email…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-slate-800 border border-slate-700 rounded-lg pl-8 pr-4 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-yellow-400 transition-colors"
          />
        </div>

        {tab === "flags" && (
          <div className="relative">
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as ComplianceFlagStatus | "all")}
              className="appearance-none bg-slate-800 border border-slate-700 rounded-lg px-3 pr-8 py-2 text-sm text-white focus:outline-none focus:border-yellow-400 transition-colors"
            >
              {FLAG_STATUS_OPTIONS.map(({ value, label }) => (
                <option key={value} value={value}>{label}</option>
              ))}
            </select>
            <ChevronDown size={13} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
          </div>
        )}
      </div>

      {/* Content */}
      {loading ? (
        <div className="flex justify-center py-20">
          <Loader2 size={24} className="animate-spin text-yellow-400" />
        </div>
      ) : tab === "flags" ? (
        filteredFlags.length === 0 ? (
          <div className="text-center py-20 text-slate-500">No flags found.</div>
        ) : (
          <div className="space-y-3">
            {filteredFlags.map((flag) => (
              <div
                key={flag.id}
                className={`bg-slate-800 border rounded-2xl p-5 ${
                  flag.status === "open" || flag.status === "escalated"
                    ? "border-red-500/30"
                    : flag.status === "investigating"
                    ? "border-yellow-500/30"
                    : "border-slate-700"
                }`}
              >
                <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap mb-1">
                      <span className="font-semibold text-white">{flag.userFullName}</span>
                      <span className="text-xs text-slate-400">{flag.userEmail}</span>
                      <span className="uppercase text-[10px] font-bold text-slate-400 bg-slate-700 px-1.5 py-0.5 rounded">
                        {flag.arena}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 flex-wrap mb-2">
                      <span className="text-xs font-medium text-slate-300 bg-slate-700 rounded-full px-2 py-0.5">
                        {REASON_LABELS[flag.reason]}
                      </span>
                      {flagStatusBadge(flag.status)}
                      {riskBadge(flag.riskScore)}
                    </div>
                    <p className="text-sm text-slate-400">{flag.description}</p>
                    <div className="text-xs text-slate-500 mt-2">
                      Created {new Date(flag.createdAt).toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" })}
                      {flag.assignedTo && " · Assigned to compliance team"}
                    </div>
                  </div>

                  {/* Actions */}
                  {flag.status !== "cleared" && (
                    <div className="flex flex-wrap gap-2 shrink-0">
                      {flag.status === "open" && (
                        <button
                          onClick={() => handleUpdateStatus(flag, "investigating")}
                          disabled={updating === flag.id}
                          className="flex items-center gap-1.5 bg-yellow-500/20 hover:bg-yellow-500/30 border border-yellow-500/40 text-yellow-300 text-xs px-3 py-1.5 rounded-lg transition-colors disabled:opacity-50"
                        >
                          {updating === flag.id ? <Loader2 size={11} className="animate-spin" /> : <Clock size={11} />}
                          Investigate
                        </button>
                      )}
                      {(flag.status === "open" || flag.status === "investigating") && (
                        <>
                          <button
                            onClick={() => handleUpdateStatus(flag, "cleared")}
                            disabled={updating === flag.id}
                            className="flex items-center gap-1.5 bg-green-500/20 hover:bg-green-500/30 border border-green-500/40 text-green-300 text-xs px-3 py-1.5 rounded-lg transition-colors disabled:opacity-50"
                          >
                            {updating === flag.id ? <Loader2 size={11} className="animate-spin" /> : <CheckCircle size={11} />}
                            Clear
                          </button>
                          <button
                            onClick={() => handleUpdateStatus(flag, "escalated")}
                            disabled={updating === flag.id}
                            className="flex items-center gap-1.5 bg-purple-500/20 hover:bg-purple-500/30 border border-purple-500/40 text-purple-300 text-xs px-3 py-1.5 rounded-lg transition-colors disabled:opacity-50"
                          >
                            {updating === flag.id ? <Loader2 size={11} className="animate-spin" /> : <AlertTriangle size={11} />}
                            Escalate
                          </button>
                        </>
                      )}
                    </div>
                  )}
                  {flag.status === "cleared" && (
                    <div className="flex items-center gap-1.5 text-green-400 text-xs">
                      <CheckCircle size={13} /> Cleared
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        )
      ) : (
        /* Screenings tab */
        filteredScreenings.length === 0 ? (
          <div className="text-center py-20 text-slate-500">No screenings found.</div>
        ) : (
          <div className="bg-slate-800 border border-slate-700 rounded-2xl overflow-hidden">
            <div className="hidden md:block overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-slate-700 text-slate-400 text-xs uppercase tracking-wider">
                    <th className="px-5 py-3 text-left font-medium">User</th>
                    <th className="px-5 py-3 text-left font-medium">List</th>
                    <th className="px-5 py-3 text-left font-medium">Matched Name</th>
                    <th className="px-5 py-3 text-center font-medium">Match Score</th>
                    <th className="px-5 py-3 text-left font-medium">Status</th>
                    <th className="px-5 py-3 text-left font-medium">Screened At</th>
                    <th className="px-5 py-3 text-left font-medium">Resolved</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-700/60">
                  {filteredScreenings.map((s) => (
                    <tr key={s.id} className="hover:bg-slate-700/30 transition-colors">
                      <td className="px-5 py-3">
                        <div className="font-medium text-white">{s.userFullName}</div>
                        <div className="text-xs text-slate-400">{s.userEmail}</div>
                      </td>
                      <td className="px-5 py-3">
                        <span className="font-mono text-xs text-yellow-300 bg-yellow-500/10 px-2 py-0.5 rounded">
                          {s.listSource}
                        </span>
                      </td>
                      <td className="px-5 py-3 text-slate-300 font-medium">{s.matchedName}</td>
                      <td className="px-5 py-3 text-center">
                        <span
                          className={`text-sm font-bold ${
                            s.matchScore >= 70
                              ? "text-red-400"
                              : s.matchScore >= 40
                              ? "text-yellow-400"
                              : "text-green-400"
                          }`}
                        >
                          {s.matchScore}%
                        </span>
                      </td>
                      <td className="px-5 py-3">{sanctionsStatusBadge(s.status)}</td>
                      <td className="px-5 py-3 text-slate-400 text-xs">
                        {new Date(s.screenedAt).toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" })}
                      </td>
                      <td className="px-5 py-3 text-xs text-slate-400">
                        {s.resolvedAt
                          ? new Date(s.resolvedAt).toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" })
                          : <span className="text-slate-600">—</span>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Mobile cards */}
            <div className="md:hidden divide-y divide-slate-700/60">
              {filteredScreenings.map((s) => (
                <div key={s.id} className="p-4">
                  <div className="flex items-start justify-between mb-1.5">
                    <div>
                      <div className="font-medium text-white">{s.userFullName}</div>
                      <div className="text-xs text-slate-400">{s.userEmail}</div>
                    </div>
                    {sanctionsStatusBadge(s.status)}
                  </div>
                  <div className="flex items-center gap-2 flex-wrap text-xs text-slate-400">
                    <span className="font-mono text-yellow-300 bg-yellow-500/10 px-1.5 py-0.5 rounded">{s.listSource}</span>
                    <span>Match: <span className={s.matchScore >= 70 ? "text-red-400 font-bold" : s.matchScore >= 40 ? "text-yellow-400 font-bold" : "text-green-400 font-bold"}>{s.matchScore}%</span></span>
                    <span>{s.matchedName}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )
      )}

      {!loading && (
        <p className="text-xs text-slate-500 mt-3">
          {tab === "flags"
            ? `Showing ${filteredFlags.length} flag${filteredFlags.length !== 1 ? "s" : ""}`
            : `Showing ${filteredScreenings.length} screening${filteredScreenings.length !== 1 ? "s" : ""}`}
        </p>
      )}
    </div>
  );
}
