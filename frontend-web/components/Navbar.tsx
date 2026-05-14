"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import {
  CreditCard,
  History,
  LayoutDashboard,
  LogOut,
  Settings,
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

const isAdmin = (role?: string) => role === "admin" || role === "superadmin";

function NavLink({
  href,
  label,
  Icon,
  active,
  onClick,
}: {
  href: string;
  label: string;
  Icon: typeof LayoutDashboard;
  active: boolean;
  onClick?: () => void;
}) {
  return (
    <Link
      href={href}
      onClick={onClick}
      aria-current={active ? "page" : undefined}
      className={`relative flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm transition-all
      ${
        active
          ? "bg-white/[0.07] text-white shadow-sm ring-1 ring-white/10"
          : "text-white/60 hover:text-white hover:bg-white/[0.04]"
      }`}
    >
      {active && (
        <span
          className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-0.5 rounded-full bg-brand-300"
          aria-hidden="true"
        />
      )}
      <Icon size={16} className={active ? "text-brand-300" : ""} />
      <span className="truncate">{label}</span>
    </Link>
  );
}

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

  const initials =
    user.fullName
      ?.split(" ")
      .filter(Boolean)
      .slice(0, 2)
      .map((n) => n[0])
      .join("")
      .toUpperCase() ?? "G";

  return (
    <>
      {/* ─────────── Desktop sidebar ─────────── */}
      <aside className="hidden md:flex flex-col w-64 min-h-screen sticky top-0 shrink-0 bg-night-1 text-white">
        {/* Logo */}
        <div className="px-5 pt-6 pb-5 border-b border-white/5">
          <Link href="/dashboard" className="flex items-center gap-2.5 group">
            <span className="relative w-9 h-9 rounded-2xl bg-gradient-to-br from-brand-200 to-brand-500 flex items-center justify-center shadow-gold">
              <span className="text-night-0 font-bold">G</span>
              <span className="absolute inset-0 rounded-2xl ring-1 ring-inset ring-white/40" />
            </span>
            <span className="font-semibold tracking-tight text-lg">GOLD Token</span>
          </Link>
          <p className="text-[11px] tracking-widest text-white/35 mt-2 uppercase">
            {user.arena} Arena
          </p>
        </div>

        {/* Nav links */}
        <nav className="flex-1 p-3 space-y-0.5 overflow-y-auto">
          <p className="text-[11px] tracking-widest text-white/35 uppercase px-3 mt-2 mb-2">
            Menü
          </p>
          {navItems.map(({ href, label, icon }) => {
            const active = pathname === href || pathname.startsWith(href + "/");
            return (
              <NavLink
                key={href}
                href={href}
                label={label}
                Icon={icon}
                active={active}
              />
            );
          })}

          {isAdmin(user?.role) && (
            <>
              <p className="text-[11px] tracking-widest text-white/35 uppercase px-3 mt-5 mb-2">
                Yönetim
              </p>
              <NavLink
                href="/admin"
                label="Admin Panel"
                Icon={Settings}
                active={pathname.startsWith("/admin")}
              />
            </>
          )}
        </nav>

        {/* User card + logout */}
        <div className="m-3 p-3 rounded-2xl bg-white/[0.04] border border-white/5">
          <div className="flex items-center gap-3 mb-3">
            <div className="w-9 h-9 rounded-full bg-gradient-to-br from-brand-300 to-brand-500 flex items-center justify-center text-night-0 font-semibold text-sm shrink-0">
              {initials}
            </div>
            <div className="min-w-0">
              <p className="text-sm text-white truncate">{user.fullName}</p>
              <p className="text-xs text-white/40 truncate">{user.email}</p>
            </div>
          </div>
          <button
            onClick={handleLogout}
            className="w-full inline-flex items-center justify-center gap-2 text-xs text-white/60 hover:text-white hover:bg-white/5 py-2 rounded-lg transition-colors"
          >
            <LogOut size={13} />
            Çıkış Yap
          </button>
        </div>
      </aside>

      {/* ─────────── Mobile top bar ─────────── */}
      <div className="md:hidden fixed top-0 left-0 right-0 z-40 bg-night-1/90 backdrop-blur-md border-b border-white/5 text-white px-4 py-3 flex items-center justify-between">
        <Link href="/dashboard" className="flex items-center gap-2">
          <span className="w-8 h-8 rounded-xl bg-gradient-to-br from-brand-200 to-brand-500 flex items-center justify-center">
            <span className="text-night-0 font-bold text-xs">G</span>
          </span>
          <span className="font-semibold tracking-tight">GOLD Token</span>
        </Link>
        <button
          aria-label={mobileOpen ? "Menüyü kapat" : "Menüyü aç"}
          onClick={() => setMobileOpen(!mobileOpen)}
          className="p-2 rounded-lg hover:bg-white/5"
        >
          {mobileOpen ? <X size={20} /> : <Menu size={20} />}
        </button>
      </div>

      {/* ─────────── Mobile drawer ─────────── */}
      {mobileOpen && (
        <div className="md:hidden fixed inset-0 z-30 bg-night-0/95 backdrop-blur-md text-white pt-14 anim-rise">
          <nav className="p-4 space-y-1">
            {navItems.map(({ href, label, icon }) => {
              const active = pathname === href || pathname.startsWith(href + "/");
              return (
                <NavLink
                  key={href}
                  href={href}
                  label={label}
                  Icon={icon}
                  active={active}
                  onClick={() => setMobileOpen(false)}
                />
              );
            })}
            {isAdmin(user?.role) && (
              <NavLink
                href="/admin"
                label="Admin Panel"
                Icon={Settings}
                active={pathname.startsWith("/admin")}
                onClick={() => setMobileOpen(false)}
              />
            )}
          </nav>
          <div className="px-4 pt-4 mt-4 border-t border-white/5">
            <div className="flex items-center gap-3 mb-3">
              <div className="w-9 h-9 rounded-full bg-gradient-to-br from-brand-300 to-brand-500 flex items-center justify-center text-night-0 font-semibold text-sm">
                {initials}
              </div>
              <div className="min-w-0">
                <p className="text-sm truncate">{user.fullName}</p>
                <p className="text-xs text-white/40 truncate">{user.email}</p>
              </div>
            </div>
            <button
              onClick={handleLogout}
              className="flex items-center gap-2 text-sm text-white/60 hover:text-white"
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
