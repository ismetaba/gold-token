"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import Navbar from "@/components/Navbar";
import { Loader2 } from "lucide-react";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !user) {
      router.push("/login");
    }
  }, [user, loading, router]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-surface-1">
        <div className="flex flex-col items-center gap-3 text-ink-2">
          <Loader2 size={28} className="animate-spin text-brand-500" />
          <span className="text-sm">Yükleniyor…</span>
        </div>
      </div>
    );
  }

  if (!user) return null;

  return (
    <div className="flex min-h-screen bg-mesh-light">
      <Navbar />
      <main className="flex-1 overflow-auto md:pt-0 pt-14">
        <div className="anim-rise">{children}</div>
      </main>
    </div>
  );
}
