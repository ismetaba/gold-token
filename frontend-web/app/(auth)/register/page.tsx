"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { ApiClientError } from "@/lib/api-client";
import { ArrowLeft, ArrowRight, Eye, EyeOff, Loader2 } from "lucide-react";

const ARENAS = [
  { value: "tr", label: "🇹🇷 Türkiye (TR)" },
  { value: "ch", label: "🇨🇭 İsviçre (CH)" },
  { value: "ae", label: "🇦🇪 BAE (AE)" },
  { value: "eu", label: "🇪🇺 Avrupa (EU)" },
];

function passwordStrength(pw: string): { score: 0 | 1 | 2 | 3 | 4; label: string; color: string } {
  let score = 0;
  if (pw.length >= 8) score++;
  if (/[A-Z]/.test(pw) && /[a-z]/.test(pw)) score++;
  if (/\d/.test(pw)) score++;
  if (/[^A-Za-z0-9]/.test(pw)) score++;
  const map = [
    { label: "Çok zayıf", color: "bg-rose-500" },
    { label: "Zayıf", color: "bg-rose-400" },
    { label: "Orta", color: "bg-amber-400" },
    { label: "İyi", color: "bg-emerald-400" },
    { label: "Güçlü", color: "bg-emerald-500" },
  ] as const;
  return { score: score as 0 | 1 | 2 | 3 | 4, ...map[score] };
}

export default function RegisterPage() {
  const { register } = useAuth();
  const router = useRouter();

  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [arena, setArena] = useState("tr");
  const [showPw, setShowPw] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const strength = passwordStrength(password);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (password.length < 8) {
      setError("Şifre en az 8 karakter olmalıdır.");
      return;
    }
    setLoading(true);
    try {
      await register({ email, password, fullName, arena });
      router.push("/kyc");
    } catch (err) {
      if (err instanceof ApiClientError) {
        setError(err.message);
      } else {
        setError("Kayıt başarısız. Lütfen tekrar deneyin.");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative min-h-screen bg-night-0 text-white flex items-center justify-center px-4 py-12">
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
          <p className="text-white/55 mt-1 text-sm">Ücretsiz hesap açın</p>
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
              Ad Soyad
            </label>
            <input
              type="text"
              required
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              placeholder="Ayşe Yılmaz"
              className="input input-dark"
              autoComplete="name"
            />
          </div>

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
              Yetki Alanı
            </label>
            <select
              value={arena}
              onChange={(e) => setArena(e.target.value)}
              className="input input-dark appearance-none cursor-pointer"
              style={{
                backgroundImage:
                  "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='8' viewBox='0 0 12 8' fill='none'%3E%3Cpath d='M1 1L6 6L11 1' stroke='%23ffffff80' stroke-width='1.5' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E\")",
                backgroundRepeat: "no-repeat",
                backgroundPosition: "right 14px center",
                paddingRight: "36px",
              }}
            >
              {ARENAS.map((a) => (
                <option key={a.value} value={a.value} className="bg-night-2 text-white">
                  {a.label}
                </option>
              ))}
            </select>
            <p className="text-xs text-white/40 mt-1.5">
              Hesap yetki alanı sonradan değiştirilemez.
            </p>
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
                placeholder="En az 8 karakter"
                className="input input-dark pr-10"
                autoComplete="new-password"
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
            {password.length > 0 && (
              <div className="mt-2 flex items-center gap-2">
                <div className="flex-1 h-1 rounded-full bg-white/10 overflow-hidden">
                  <div
                    className={`h-full ${strength.color} transition-all duration-300`}
                    style={{ width: `${(strength.score / 4) * 100}%` }}
                  />
                </div>
                <span className="text-xs text-white/50 w-16 text-right">
                  {strength.label}
                </span>
              </div>
            )}
          </div>

          <button
            type="submit"
            disabled={loading}
            className="btn btn-primary w-full py-3 text-sm"
          >
            {loading ? (
              <>
                <Loader2 size={16} className="animate-spin" />
                Hesap oluşturuluyor…
              </>
            ) : (
              <>
                Hesap Oluştur
                <ArrowRight size={16} />
              </>
            )}
          </button>

          <p className="text-xs text-white/40 text-center leading-relaxed">
            Hesap oluşturarak{" "}
            <span className="text-white/65">Kullanım Şartları</span> ve{" "}
            <span className="text-white/65">Gizlilik Politikası</span>&apos;nı kabul
            etmiş sayılırsınız.
          </p>

          <div className="text-center text-sm text-white/55">
            Zaten hesabınız var mı?{" "}
            <Link
              href="/login"
              className="text-brand-300 hover:text-brand-200 font-medium transition-colors"
            >
              Giriş Yapın
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
