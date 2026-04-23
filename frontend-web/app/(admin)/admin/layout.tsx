"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/contexts/AuthContext";
import {
  BarChart2,
  Coins,
  LayoutDashboard,
  Loader2,
  LogOut,
  Menu,
  Shield,
  Users,
  X,
} from "lucide-react";

const adminNavItems = [
  { href: "/admin", label: "Overview", icon: LayoutDashboard, exact: true },
  { href: "/admin/users", label: "Users & KYC", icon: Users },
  { href: "/admin/tokens", label: "Token Ops", icon: Coins },
  { href: "/admin/reserves", label: "Gold Reserves", icon: Shield },
];

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const { user, loading, logout } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    if (!loading) {
      if (!user) {
        router.push("/login");
      } else if (user.role !== "admin" && user.role !== "superadmin") {
        router.push("/dashboard");
      }
    }
  }, [user, loading, router]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-950">
        <Loader2 size={32} className="animate-spin text-yellow-400" />
      </div>
    );
  }

  if (!user || (user.role !== "admin" && user.role !== "superadmin")) return null;

  const handleLogout = () => {
    logout();
    router.push("/login");
  };

  return (
    <div className="flex min-h-screen bg-slate-950">
      {/* Desktop sidebar */}
      <aside className="hidden md:flex flex-col w-64 shrink-0 bg-slate-900 border-r border-slate-800">
        {/* Logo + badge */}
        <div className="p-6 border-b border-slate-800">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-full bg-yellow-400 flex items-center justify-center shrink-0">
              <span className="text-slate-900 font-bold text-sm">G</span>
            </div>
            <div>
              <div className="font-semibold text-white text-sm leading-tight">GOLD Token</div>
              <div className="text-[10px] font-medium text-yellow-400 uppercase tracking-wider">Admin Panel</div>
            </div>
          </div>
        </div>

        {/* Nav */}
        <nav className="flex-1 p-4 space-y-1">
          {adminNavItems.map(({ href, label, icon: Icon, exact }) => {
            const active = exact ? pathname === href : pathname.startsWith(href);
            return (
              <Link
                key={href}
                href={href}
                className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                  active
                    ? "bg-yellow-400 text-slate-900 font-medium"
                    : "text-slate-400 hover:bg-slate-800 hover:text-white"
                }`}
              >
                <Icon size={16} />
                {label}
              </Link>
            );
          })}
        </nav>

        {/* User */}
        <div className="p-4 border-t border-slate-800">
          <div className="text-sm text-slate-300 truncate mb-0.5">{user.fullName}</div>
          <div className="text-xs text-slate-500 truncate mb-3">{user.email}</div>
          <div className="flex items-center justify-between">
            <Link href="/dashboard" className="text-xs text-slate-400 hover:text-white transition-colors">
              ← User App
            </Link>
            <button
              onClick={handleLogout}
              className="flex items-center gap-1.5 text-xs text-slate-400 hover:text-white transition-colors"
            >
              <LogOut size={12} />
              Logout
            </button>
          </div>
        </div>
      </aside>

      {/* Mobile top bar */}
      <div className="md:hidden fixed top-0 left-0 right-0 z-40 bg-slate-900 border-b border-slate-800 px-4 py-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-full bg-yellow-400 flex items-center justify-center">
            <span className="text-slate-900 font-bold text-xs">G</span>
          </div>
          <span className="font-semibold text-white text-sm">Admin Panel</span>
        </div>
        <button onClick={() => setMobileOpen(!mobileOpen)} className="text-slate-400">
          {mobileOpen ? <X size={20} /> : <Menu size={20} />}
        </button>
      </div>

      {/* Mobile drawer */}
      {mobileOpen && (
        <div className="md:hidden fixed inset-0 z-30 bg-slate-900 pt-14">
          <nav className="p-4 space-y-1">
            {adminNavItems.map(({ href, label, icon: Icon, exact }) => {
              const active = exact ? pathname === href : pathname.startsWith(href);
              return (
                <Link
                  key={href}
                  href={href}
                  onClick={() => setMobileOpen(false)}
                  className={`flex items-center gap-3 px-3 py-3 rounded-lg text-sm ${
                    active ? "bg-yellow-400 text-slate-900 font-medium" : "text-slate-300"
                  }`}
                >
                  <Icon size={16} />
                  {label}
                </Link>
              );
            })}
          </nav>
          <div className="px-4 pt-4 border-t border-slate-800">
            <button onClick={handleLogout} className="flex items-center gap-2 text-sm text-slate-400">
              <LogOut size={14} />
              Logout
            </button>
          </div>
        </div>
      )}

      {/* Content */}
      <main className="flex-1 overflow-auto md:pt-0 pt-14 text-white">
        {children}
      </main>
    </div>
  );
}
