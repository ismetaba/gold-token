"use client";

import { useEffect, useState } from "react";
import { adminApi } from "@/lib/api-client";
import type { ServiceHealth, ServiceStatusLevel, SystemHealthSummary } from "@/lib/types";
import {
  Activity,
  AlertTriangle,
  CheckCircle,
  Clock,
  Loader2,
  RefreshCw,
  XCircle,
} from "lucide-react";

// ── Helpers ──────────────────────────────────────────────────────────────────

function statusConfig(status: ServiceStatusLevel) {
  const map: Record<ServiceStatusLevel, { label: string; dot: string; badge: string; border: string }> = {
    operational: {
      label: "Operational",
      dot: "bg-green-500",
      badge: "bg-green-500/20 text-green-300 border border-green-500/40",
      border: "border-slate-700",
    },
    degraded: {
      label: "Degraded",
      dot: "bg-yellow-400 animate-pulse",
      badge: "bg-yellow-500/20 text-yellow-300 border border-yellow-500/40",
      border: "border-yellow-500/30",
    },
    outage: {
      label: "Outage",
      dot: "bg-red-500 animate-pulse",
      badge: "bg-red-500/20 text-red-300 border border-red-500/40",
      border: "border-red-500/40",
    },
    maintenance: {
      label: "Maintenance",
      dot: "bg-blue-400",
      badge: "bg-blue-500/20 text-blue-300 border border-blue-500/40",
      border: "border-blue-500/30",
    },
  };
  return map[status];
}

function overallStatusIcon(status: ServiceStatusLevel) {
  switch (status) {
    case "operational": return <CheckCircle size={20} className="text-green-400" />;
    case "degraded":    return <AlertTriangle size={20} className="text-yellow-400" />;
    case "outage":      return <XCircle size={20} className="text-red-400" />;
    case "maintenance": return <Clock size={20} className="text-blue-400" />;
  }
}

function latencyBar(ms?: number) {
  if (!ms) return null;
  const pct = Math.min(ms / 2000, 1) * 100;
  const color = ms < 100 ? "bg-green-500" : ms < 500 ? "bg-yellow-400" : "bg-red-500";
  return (
    <div className="flex items-center gap-2 text-xs text-slate-400">
      <div className="w-16 h-1.5 bg-slate-700 rounded-full overflow-hidden">
        <div className={`h-full ${color} rounded-full`} style={{ width: `${pct}%` }} />
      </div>
      <span>{ms} ms</span>
    </div>
  );
}

// ── Service Card ──────────────────────────────────────────────────────────────

function ServiceCard({ svc }: { svc: ServiceHealth }) {
  const cfg = statusConfig(svc.status);
  return (
    <div className={`bg-slate-800 border rounded-2xl p-5 ${cfg.border}`}>
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${cfg.dot} shrink-0 mt-0.5`} />
          <div>
            <div className="font-semibold text-white text-sm">{svc.name}</div>
            <div className="text-xs text-slate-400 mt-0.5">{svc.description}</div>
          </div>
        </div>
        <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${cfg.badge} whitespace-nowrap shrink-0 ml-2`}>
          {cfg.label}
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-4 text-xs">
        {svc.latencyMs !== undefined && (
          <div>
            <div className="text-slate-500 mb-0.5">Latency</div>
            {latencyBar(svc.latencyMs)}
          </div>
        )}
        {svc.uptimePct !== undefined && (
          <div>
            <div className="text-slate-500 mb-0.5">Uptime (30d)</div>
            <span className={`font-semibold ${svc.uptimePct >= 99.9 ? "text-green-400" : svc.uptimePct >= 99 ? "text-yellow-400" : "text-red-400"}`}>
              {svc.uptimePct.toFixed(2)}%
            </span>
          </div>
        )}
        <div>
          <div className="text-slate-500 mb-0.5">Last checked</div>
          <span className="text-slate-300">
            {new Date(svc.lastCheckedAt).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}
          </span>
        </div>
      </div>

      {svc.incidentNote && (
        <div className="mt-3 flex items-start gap-2 bg-yellow-500/10 border border-yellow-500/20 text-yellow-300 text-xs px-3 py-2 rounded-xl">
          <AlertTriangle size={12} className="shrink-0 mt-0.5" />
          <span>{svc.incidentNote}</span>
        </div>
      )}
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function AdminSystemPage() {
  const [health, setHealth] = useState<SystemHealthSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const load = async (isRefresh = false) => {
    if (isRefresh) setRefreshing(true);
    else setLoading(true);
    const res = await adminApi.getSystemHealth();
    setHealth(res.data);
    if (isRefresh) setRefreshing(false);
    else setLoading(false);
  };

  useEffect(() => { load(); }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <Loader2 size={28} className="animate-spin text-yellow-400" />
      </div>
    );
  }

  if (!health) return null;

  const operationalCount = health.services.filter((s) => s.status === "operational").length;
  const degradedCount    = health.services.filter((s) => s.status === "degraded").length;
  const outageCount      = health.services.filter((s) => s.status === "outage").length;
  const overallCfg       = statusConfig(health.overall);

  return (
    <div className="p-6 md:p-8 max-w-5xl">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Activity size={22} className="text-yellow-400" />
            System Health
          </h1>
          <p className="text-slate-400 text-sm mt-0.5">
            Live service status and uptime overview
          </p>
        </div>
        <button
          onClick={() => load(true)}
          disabled={refreshing}
          className="flex items-center gap-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 text-sm px-4 py-2 rounded-xl transition-colors disabled:opacity-50"
        >
          <RefreshCw size={13} className={refreshing ? "animate-spin" : ""} />
          Refresh
        </button>
      </div>

      {/* Overall status banner */}
      <div className={`bg-slate-800 border rounded-2xl p-5 mb-6 ${overallCfg.border}`}>
        <div className="flex items-center gap-3">
          {overallStatusIcon(health.overall)}
          <div>
            <div className="font-semibold text-white">
              {health.overall === "operational"
                ? "All systems operational"
                : health.overall === "degraded"
                ? "Partial service degradation"
                : health.overall === "outage"
                ? "Service outage in progress"
                : "Scheduled maintenance"}
            </div>
            <div className="text-xs text-slate-400 mt-0.5">
              Last updated {new Date(health.checkedAt).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}
            </div>
          </div>
        </div>
      </div>

      {/* Quick stats */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <div className="bg-green-500/10 border border-green-500/30 rounded-2xl p-4 text-center">
          <div className="text-2xl font-bold text-green-400">{operationalCount}</div>
          <div className="text-xs text-slate-400 mt-0.5">Operational</div>
        </div>
        <div className={`${degradedCount > 0 ? "bg-yellow-500/10 border-yellow-500/30" : "bg-slate-800 border-slate-700"} border rounded-2xl p-4 text-center`}>
          <div className={`text-2xl font-bold ${degradedCount > 0 ? "text-yellow-400" : "text-slate-500"}`}>{degradedCount}</div>
          <div className="text-xs text-slate-400 mt-0.5">Degraded</div>
        </div>
        <div className={`${outageCount > 0 ? "bg-red-500/10 border-red-500/30" : "bg-slate-800 border-slate-700"} border rounded-2xl p-4 text-center`}>
          <div className={`text-2xl font-bold ${outageCount > 0 ? "text-red-400" : "text-slate-500"}`}>{outageCount}</div>
          <div className="text-xs text-slate-400 mt-0.5">Outage</div>
        </div>
      </div>

      {/* Services grid */}
      <h2 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">Services</h2>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {health.services.map((svc) => (
          <ServiceCard key={svc.id} svc={svc} />
        ))}
      </div>
    </div>
  );
}
