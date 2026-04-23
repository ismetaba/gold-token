"use client";

import { useEffect, useState } from "react";
import { adminApi } from "@/lib/api-client";
import type { AdminUserRow, KycStatus } from "@/lib/types";
import {
  AlertTriangle,
  CheckCircle,
  ChevronDown,
  Clock,
  Loader2,
  Search,
  ShieldOff,
  UserCheck,
  XCircle,
} from "lucide-react";

// ── Helpers ──────────────────────────────────────────────────────────────────

function kycBadge(status: KycStatus) {
  const map: Record<KycStatus, { label: string; cls: string }> = {
    not_started:  { label: "Not Started",  cls: "bg-slate-700 text-slate-300" },
    pending:      { label: "Pending",       cls: "bg-yellow-500/20 text-yellow-300 border border-yellow-500/40" },
    in_review:    { label: "In Review",     cls: "bg-blue-500/20 text-blue-300 border border-blue-500/40" },
    approved:     { label: "Approved",      cls: "bg-green-500/20 text-green-300 border border-green-500/40" },
    rejected:     { label: "Rejected",      cls: "bg-red-500/20 text-red-300 border border-red-500/40" },
    expired:      { label: "Expired",       cls: "bg-slate-600 text-slate-400" },
  };
  const { label, cls } = map[status] ?? { label: status, cls: "bg-slate-700 text-slate-300" };
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>
      {label}
    </span>
  );
}

const KYC_FILTER_OPTIONS: { value: KycStatus | "all"; label: string }[] = [
  { value: "all",         label: "All" },
  { value: "pending",     label: "Pending" },
  { value: "in_review",   label: "In Review" },
  { value: "approved",    label: "Approved" },
  { value: "rejected",    label: "Rejected" },
  { value: "not_started", label: "Not Started" },
];

// ── Component ────────────────────────────────────────────────────────────────

export default function AdminUsersPage() {
  const [users, setUsers] = useState<AdminUserRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [kycFilter, setKycFilter] = useState<KycStatus | "all">("all");
  const [search, setSearch] = useState("");
  const [reviewing, setReviewing] = useState<string | null>(null); // kycSessionId
  const [reviewResult, setReviewResult] = useState<Record<string, "approved" | "rejected">>({});
  const [reviewError, setReviewError] = useState<string | null>(null);

  const load = async (filter?: KycStatus | "all") => {
    setLoading(true);
    const res = await adminApi.listUsers(
      filter && filter !== "all" ? { kycStatus: filter } : undefined
    );
    setUsers(res.data);
    setLoading(false);
  };

  useEffect(() => {
    load(kycFilter);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kycFilter]);

  const filtered = search.trim()
    ? users.filter(
        (u) =>
          u.fullName.toLowerCase().includes(search.toLowerCase()) ||
          u.email.toLowerCase().includes(search.toLowerCase())
      )
    : users;

  const handleReview = async (
    user: AdminUserRow,
    action: "approve" | "reject"
  ) => {
    if (!user.kycSessionId) return;
    setReviewing(user.kycSessionId);
    setReviewError(null);

    // Snapshot for rollback
    const previousUsers = users;
    const newStatus = action === "approve" ? "approved" : "rejected";

    // Optimistic update
    setUsers((prev) =>
      prev.map((u) => (u.id === user.id ? { ...u, kycStatus: newStatus } : u))
    );

    try {
      await adminApi.reviewKyc(user.kycSessionId, { action });
      setReviewResult((prev) => ({ ...prev, [user.kycSessionId!]: newStatus }));
    } catch (err: unknown) {
      // Rollback optimistic update on failure
      setUsers(previousUsers);
      setReviewError(
        err instanceof Error ? err.message : `Failed to ${action} KYC — please try again.`
      );
    } finally {
      setReviewing(null);
    }
  };

  return (
    <div className="p-6 md:p-8 max-w-7xl">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">Users & KYC</h1>
        <p className="text-slate-400 text-sm mt-0.5">
          Manage users and review KYC applications
        </p>
      </div>

      {/* Review error banner */}
      {reviewError && (
        <div className="mb-4 flex items-center gap-2 bg-red-500/10 border border-red-500/30 text-red-300 text-sm px-4 py-3 rounded-xl">
          <AlertTriangle size={15} className="shrink-0" />
          <span>{reviewError}</span>
          <button
            onClick={() => setReviewError(null)}
            className="ml-auto text-red-400 hover:text-red-200 transition-colors"
            aria-label="Dismiss"
          >
            <XCircle size={14} />
          </button>
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 mb-5">
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
        <div className="relative">
          <select
            value={kycFilter}
            onChange={(e) => setKycFilter(e.target.value as KycStatus | "all")}
            className="appearance-none bg-slate-800 border border-slate-700 rounded-lg px-3 pr-8 py-2 text-sm text-white focus:outline-none focus:border-yellow-400 transition-colors"
          >
            {KYC_FILTER_OPTIONS.map(({ value, label }) => (
              <option key={value} value={value}>
                KYC: {label}
              </option>
            ))}
          </select>
          <ChevronDown size={13} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
        </div>
      </div>

      {/* Table */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 size={24} className="animate-spin text-yellow-400" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="text-center py-20 text-slate-500">No users found.</div>
      ) : (
        <div className="bg-slate-800 border border-slate-700 rounded-2xl overflow-hidden">
          {/* Desktop table */}
          <div className="hidden md:block overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-700 text-slate-400 text-xs uppercase tracking-wider">
                  <th className="px-5 py-3 text-left font-medium">User</th>
                  <th className="px-5 py-3 text-left font-medium">Arena</th>
                  <th className="px-5 py-3 text-left font-medium">KYC Status</th>
                  <th className="px-5 py-3 text-right font-medium">Balance</th>
                  <th className="px-5 py-3 text-right font-medium">Orders</th>
                  <th className="px-5 py-3 text-left font-medium">Joined</th>
                  <th className="px-5 py-3 text-center font-medium">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-700/60">
                {filtered.map((user) => (
                  <tr key={user.id} className="hover:bg-slate-700/30 transition-colors">
                    <td className="px-5 py-3">
                      <div className="font-medium text-white">{user.fullName}</div>
                      <div className="text-xs text-slate-400">{user.email}</div>
                    </td>
                    <td className="px-5 py-3">
                      <span className="uppercase text-xs font-semibold text-slate-300 bg-slate-700 rounded px-1.5 py-0.5">
                        {user.arena}
                      </span>
                    </td>
                    <td className="px-5 py-3">{kycBadge(user.kycStatus)}</td>
                    <td className="px-5 py-3 text-right font-mono text-slate-200">
                      {parseFloat(user.balanceGrams).toFixed(2)} g
                    </td>
                    <td className="px-5 py-3 text-right text-slate-300">{user.totalOrderCount}</td>
                    <td className="px-5 py-3 text-slate-400 text-xs">
                      {new Date(user.createdAt).toLocaleDateString("en-GB", {
                        day: "2-digit", month: "short", year: "numeric",
                      })}
                    </td>
                    <td className="px-5 py-3">
                      <div className="flex items-center justify-center gap-2">
                        {(user.kycStatus === "pending" || user.kycStatus === "in_review") &&
                          user.kycSessionId && (
                            <>
                              <button
                                onClick={() => handleReview(user, "approve")}
                                disabled={reviewing === user.kycSessionId}
                                title="Approve KYC"
                                className="flex items-center gap-1 bg-green-600 hover:bg-green-500 disabled:opacity-50 text-white text-xs px-2.5 py-1.5 rounded-lg transition-colors"
                              >
                                {reviewing === user.kycSessionId ? (
                                  <Loader2 size={11} className="animate-spin" />
                                ) : (
                                  <UserCheck size={11} />
                                )}
                                Approve
                              </button>
                              <button
                                onClick={() => handleReview(user, "reject")}
                                disabled={reviewing === user.kycSessionId}
                                title="Reject KYC"
                                className="flex items-center gap-1 bg-red-600 hover:bg-red-500 disabled:opacity-50 text-white text-xs px-2.5 py-1.5 rounded-lg transition-colors"
                              >
                                {reviewing === user.kycSessionId ? (
                                  <Loader2 size={11} className="animate-spin" />
                                ) : (
                                  <XCircle size={11} />
                                )}
                                Reject
                              </button>
                            </>
                          )}
                        {user.kycStatus === "approved" && (
                          <span className="flex items-center gap-1 text-xs text-green-400">
                            <CheckCircle size={12} /> Approved
                          </span>
                        )}
                        {user.kycStatus === "rejected" && (
                          <span className="flex items-center gap-1 text-xs text-red-400">
                            <ShieldOff size={12} /> Rejected
                          </span>
                        )}
                        {(user.kycStatus === "not_started" || !user.kycSessionId) && (
                          <span className="text-xs text-slate-500">—</span>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Mobile cards */}
          <div className="md:hidden divide-y divide-slate-700/60">
            {filtered.map((user) => (
              <div key={user.id} className="p-4">
                <div className="flex items-start justify-between mb-2">
                  <div>
                    <div className="font-medium text-white">{user.fullName}</div>
                    <div className="text-xs text-slate-400">{user.email}</div>
                  </div>
                  {kycBadge(user.kycStatus)}
                </div>
                <div className="flex items-center gap-3 text-xs text-slate-400 mb-3">
                  <span className="uppercase font-semibold text-slate-300">{user.arena}</span>
                  <span>{parseFloat(user.balanceGrams).toFixed(2)} g</span>
                  <span>{user.totalOrderCount} orders</span>
                </div>
                {(user.kycStatus === "pending" || user.kycStatus === "in_review") && user.kycSessionId && (
                  <div className="flex gap-2">
                    <button
                      onClick={() => handleReview(user, "approve")}
                      disabled={reviewing === user.kycSessionId}
                      className="flex-1 bg-green-600 hover:bg-green-500 disabled:opacity-50 text-white text-xs py-2 rounded-lg transition-colors"
                    >
                      Approve
                    </button>
                    <button
                      onClick={() => handleReview(user, "reject")}
                      disabled={reviewing === user.kycSessionId}
                      className="flex-1 bg-red-600 hover:bg-red-500 disabled:opacity-50 text-white text-xs py-2 rounded-lg transition-colors"
                    >
                      Reject
                    </button>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Count footer */}
      {!loading && (
        <p className="text-xs text-slate-500 mt-3">
          Showing {filtered.length} user{filtered.length !== 1 ? "s" : ""}
          {kycFilter !== "all" ? ` with KYC status: ${kycFilter}` : ""}
          {search ? ` matching "${search}"` : ""}
        </p>
      )}
    </div>
  );
}
