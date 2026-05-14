"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { ApiClientError } from "@/lib/api-client";
import { ArrowLeft, ArrowRight, Eye, EyeOff, Loader2 } from "lucide-react";

export default function LoginPage() {
  const { login } = useAuth();
  const router = useRouter();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPw, setShowPw] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login(email, password);
      router.push("/dashboard");
    } catch (err) {
      if (err instanceof ApiClientError) {
        setError(err.message);
      } else {
        setError("Giriş başarısız. Lütfen tekrar deneyin.");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative min-h-screen bg-night-0 text-white flex items-center justify-center px-4">
      <div className="absolute inset-0 bg-mesh-dark" aria-hidden="true" />
      <div className="absolute inset-0 bg-dot-grid opacity-30" aria-hidden="true" />

      <div className="relative w-full max-w-md anim-rise">
        {/* Logo */}
        <Link href="/" className="flex flex-col items-center text-center mb-8">
          <span className="relative w-14 h-14 rounded-2xl bg-gradient-to-br from-brand-200 to-brand-500 flex items-center justify-center shadow-gold mb-4">
            <span className="text-night-0 font-bold text-2xl">G</span>
            <span className="absolute inset-0 rounded-2xl ring-1 ring-inset ring-white/40" />
          </span>
          <h1 className="text-2xl font-bold tracking-tight">GOLD Token</h1>
          <p className="text-white/55 mt-1 text-sm">Hesabınıza giriş yapın</p>
        </Link>

        <form
          onSubmit={handleSubmit}
          className="glass-dark rounded-3xl p-7 sm:p-8 space-y-5 shadow-xl"
        >
          {error && (
            <div className="rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-200 px-4 py-3 text-sm">
              {error}
            </div>
          )}

          <div>
            <label className="block text-xs uppercase tracking-widest text-white/55 mb-2">
              E-posta
            </label>
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="ornek@email.com"
              className="input input-dark"
              autoComplete="email"
            />
          </div>

          <div>
            <label className="block text-xs uppercase tracking-widest text-white/55 mb-2">
              Şifre
            </label>
            <div className="relative">
              <input
                type={showPw ? "text" : "password"}
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                className="input input-dark pr-10"
                autoComplete="current-password"
              />
              <button
                type="button"
                onClick={() => setShowPw(!showPw)}
                aria-label={showPw ? "Şifreyi gizle" : "Şifreyi göster"}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-white/50 hover:text-white"
              >
                {showPw ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          </div>

          <button
            type="submit"
            disabled={loading}
            className="btn btn-primary w-full py-3 text-sm"
          >
            {loading ? (
              <>
                <Loader2 size={16} className="animate-spin" />
                Giriş yapılıyor…
              </>
            ) : (
              <>
                Giriş Yap
                <ArrowRight size={16} />
              </>
            )}
          </button>

          <div className="text-center text-sm text-white/55">
            <span className="block text-xs text-white/35 mb-2">
              POC: herhangi bir e-posta, şifre ≥ 4 karakter
            </span>
            Hesabınız yok mu?{" "}
            <Link
              href="/register"
              className="text-brand-300 hover:text-brand-200 font-medium transition-colors"
            >
              Kayıt Olun
            </Link>
          </div>
        </form>

        <div className="text-center mt-6">
          <Link
            href="/"
            className="inline-flex items-center gap-1.5 text-white/40 hover:text-white text-sm transition-colors"
          >
            <ArrowLeft size={14} />
            Ana Sayfaya Dön
          </Link>
        </div>
      </div>
    </div>
  );
}
