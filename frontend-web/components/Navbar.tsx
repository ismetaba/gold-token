"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import {
  BarChart2,
  CreditCard,
  History,
  LayoutDashboard,
  LogOut,
  Shield,
  ShoppingCart,
  TrendingDown,
  Menu,
  X,
} from "lucide-react";
import { useState } from "react";

const navItems = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/buy", label: "Altın Al", icon: ShoppingCart },
  { href: "/sell", label: "Altın Sat", icon: TrendingDown },
  { href: "/history", label: "İşlemler", icon: History },
  { href: "/kyc", label: "Kimlik", icon: CreditCard },
  { href: "/proof-of-reserve", label: "Rezerv Kanıtı", icon: Shield },
];

export default function Navbar() {
  const { user, logout } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const [mobileOpen, setMobileOpen] = useState(false);

  const handleLogout = () => {
    logout();
    router.push("/login");
  };

  if (!user) return null;

  return (
    <>
      {/* Desktop sidebar */}
      <aside className="hidden md:flex flex-col w-60 min-h-screen bg-slate-900 text-white shrink-0">
        {/* Logo */}
        <div className="p-6 border-b border-slate-700">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-full bg-yellow-400 flex items-center justify-center">
              <span className="text-slate-900 font-bold text-sm">G</span>
            </div>
            <span className="font-semibold text-lg">GOLD Token</span>
          </div>
          <p className="text-xs text-slate-400 mt-1">{user.arena.toUpperCase()} Arena</p>
        </div>

        {/* Nav links */}
        <nav className="flex-1 p-4 space-y-1">
          {navItems.map(({ href, label, icon: Icon }) => {
            const active = pathname === href || pathname.startsWith(href + "/");
            return (
              <Link
                key={href}
                href={href}
                className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                  active
                    ? "bg-yellow-400 text-slate-900 font-medium"
                    : "text-slate-300 hover:bg-slate-800 hover:text-white"
                }`}
              >
                <Icon size={16} />
                {label}
              </Link>
            );
          })}
        </nav>

        {/* User + logout */}
        <div className="p-4 border-t border-slate-700">
          <div className="text-sm text-slate-300 mb-2 truncate">{user.fullName}</div>
          <div className="text-xs text-slate-500 mb-3 truncate">{user.email}</div>
          <button
            onClick={handleLogout}
            className="flex items-center gap-2 text-xs text-slate-400 hover:text-white transition-colors"
          >
            <LogOut size={14} />
            Çıkış Yap
          </button>
        </div>
      </aside>

      {/* Mobile top bar */}
      <div className="md:hidden fixed top-0 left-0 right-0 z-40 bg-slate-900 text-white px-4 py-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-full bg-yellow-400 flex items-center justify-center">
            <span className="text-slate-900 font-bold text-xs">G</span>
          </div>
          <span className="font-semibold">GOLD Token</span>
        </div>
        <button onClick={() => setMobileOpen(!mobileOpen)}>
          {mobileOpen ? <X size={20} /> : <Menu size={20} />}
        </button>
      </div>

      {/* Mobile drawer */}
      {mobileOpen && (
        <div className="md:hidden fixed inset-0 z-30 bg-slate-900 text-white pt-14">
          <nav className="p-4 space-y-1">
            {navItems.map(({ href, label, icon: Icon }) => {
              const active = pathname === href;
              return (
                <Link
                  key={href}
                  href={href}
                  onClick={() => setMobileOpen(false)}
                  className={`flex items-center gap-3 px-3 py-3 rounded-lg text-sm ${
                    active
                      ? "bg-yellow-400 text-slate-900 font-medium"
                      : "text-slate-300"
                  }`}
                >
                  <Icon size={16} />
                  {label}
                </Link>
              );
            })}
          </nav>
          <div className="px-4 pt-4 border-t border-slate-700">
            <button
              onClick={handleLogout}
              className="flex items-center gap-2 text-sm text-slate-400"
            >
              <LogOut size={14} />
              Çıkış Yap
            </button>
          </div>
        </div>
      )}
    </>
  );
}
