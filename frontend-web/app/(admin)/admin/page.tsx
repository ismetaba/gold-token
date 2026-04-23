"use client";

import { useEffect, useState } from "react";
import { adminApi } from "@/lib/api-client";
import type { AdminStats } from "@/lib/types";
import { formatGrams, formatTRY } from "@/lib/utils";
import {
  ArrowRight,
  BarChart2,
  CheckCircle,
  Clock,
  Coins,
  Loader2,
  Shield,
  TrendingDown,
  TrendingUp,
  Users,
  XCircle,
} from "lucide-react";
import Link from "next/link";

function StatCard({
  label,
  value,
  sub,
  icon: Icon,
  accent,
}: {
  label: string;
  value: string;
  sub?: string;
  icon: React.ElementType;
  accent?: string;
}) {
  return (
    <div className={`rounded-2xl p-5 border ${accent ?? "bg-slate-800 border-slate-700"}`}>
      <div className="flex items-center gap-2 text-slate-400 text-xs mb-3">
        <Icon size={13} />
        {label}
      </div>
      <div className="text-2xl font-bold text-white">{value}</div>
      {sub && <div className="text-xs text-slate-400 mt-1">{sub}</div>}
    </div>
  );
}

export default function AdminOverviewPage() {
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    adminApi.getStats().then((r) => {
      setStats(r.data);
      setLoading(false);
    });
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <Loader2 size={28} className="animate-spin text-yellow-400" />
      </div>
    );
  }

  if (!stats) return null;

  const kycPendingPct = Math.round((stats.kycPendingCount / stats.totalUsers) * 100);
  const kycApprovedPct = Math.round((stats.kycApprovedCount / stats.totalUsers) * 100);

  return (
    <div className="p-6 md:p-8 max-w-6xl">
      {/* Header */}
      <div className="mb-7">
        <h1 className="text-2xl font-bold text-white">Admin Overview</h1>
        <p className="text-slate-400 text-sm mt-0.5">
          {new Date().toLocaleDateString("en-GB", { weekday: "long", day: "numeric", month: "long", year: "numeric" })}
        </p>
      </div>

      {/* Primary stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard
          label="Total Users"
          value={stats.totalUsers.toLocaleString()}
          sub={`${stats.activeUsersLast30d} active / 30d`}
          icon={Users}
        />
        <StatCard
          label="KYC Pending"
          value={stats.kycPendingCount.toString()}
          sub={`${kycPendingPct}% of users`}
          icon={Clock}
          accent="bg-yellow-500/10 border-yellow-500/30"
        />
        <StatCard
          label="KYC Approved"
          value={stats.kycApprovedCount.toLocaleString()}
          sub={`${kycApprovedPct}% of users`}
          icon={CheckCircle}
          accent="bg-green-500/10 border-green-500/30"
        />
        <StatCard
          label="KYC Rejected"
          value={stats.kycRejectedCount.toString()}
          icon={XCircle}
          accent="bg-red-500/10 border-red-500/30"
        />
      </div>

      {/* Token + Reserve stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <StatCard
          label="Token Supply"
          value={`${parseFloat(stats.totalTokenSupplyGrams).toLocaleString()} g`}
          sub="Total GOLD tokens in circulation"
          icon={Coins}
          accent="bg-yellow-400/10 border-yellow-400/30"
        />
        <StatCard
          label="Gold Reserve"
          value={`${parseFloat(stats.totalGoldReserveGrams).toLocaleString()} g`}
          sub={`Coverage: ${(parseFloat(stats.coverageRatio) * 100).toFixed(2)}%`}
          icon={Shield}
          accent={parseFloat(stats.coverageRatio) >= 1 ? "bg-green-500/10 border-green-500/30" : "bg-red-500/10 border-red-500/30"}
        />
        <StatCard
          label="Order Volume (total)"
          value={formatTRY(stats.totalOrderVolumeTRY)}
          sub="All-time completed orders"
          icon={BarChart2}
        />
      </div>

      {/* Mint / Burn summary */}
      <div className="grid grid-cols-2 gap-4 mb-8">
        <div className="bg-slate-800 border border-slate-700 rounded-2xl p-5">
          <div className="flex items-center gap-2 text-slate-400 text-xs mb-3">
            <TrendingUp size={13} className="text-green-400" />
            Total Minted
          </div>
          <div className="text-2xl font-bold text-green-400">
            +{parseFloat(stats.mintTotalGrams).toLocaleString()} g
          </div>
          <div className="text-xs text-slate-500 mt-1">GOLD tokens minted all-time</div>
        </div>
        <div className="bg-slate-800 border border-slate-700 rounded-2xl p-5">
          <div className="flex items-center gap-2 text-slate-400 text-xs mb-3">
            <TrendingDown size={13} className="text-red-400" />
            Total Burned
          </div>
          <div className="text-2xl font-bold text-red-400">
            -{parseFloat(stats.burnTotalGrams).toLocaleString()} g
          </div>
          <div className="text-xs text-slate-500 mt-1">GOLD tokens burned all-time</div>
        </div>
      </div>

      {/* Quick-action cards */}
      <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-3">Quick Actions</h2>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {[
          {
            href: "/admin/users",
            title: "Review KYC",
            desc: `${stats.kycPendingCount} applications pending review`,
            color: "border-yellow-500/40 hover:border-yellow-400",
            badge: stats.kycPendingCount > 0 ? stats.kycPendingCount : null,
          },
          {
            href: "/admin/tokens",
            title: "Token Operations",
            desc: "Mint, burn, or transfer GOLD tokens",
            color: "border-blue-500/40 hover:border-blue-400",
            badge: null,
          },
          {
            href: "/admin/reserves",
            title: "Attest Reserve",
            desc: "Trigger PoR attestation & view history",
            color: "border-purple-500/40 hover:border-purple-400",
            badge: null,
          },
        ].map(({ href, title, desc, color, badge }) => (
          <Link
            key={href}
            href={href}
            className={`bg-slate-800 rounded-2xl p-5 border ${color} transition-colors group`}
          >
            <div className="flex items-center justify-between mb-2">
              <span className="font-semibold text-white">{title}</span>
              {badge !== null && (
                <span className="bg-yellow-400 text-slate-900 text-xs font-bold rounded-full px-2 py-0.5">
                  {badge}
                </span>
              )}
            </div>
            <p className="text-sm text-slate-400">{desc}</p>
            <div className="flex items-center gap-1 text-xs text-slate-500 group-hover:text-slate-300 mt-3 transition-colors">
              Open <ArrowRight size={11} />
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
