"use client";

import { useEffect, useRef, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { kycApi } from "@/lib/api-client";
import type { KycDocument, KycSession } from "@/lib/types";
import {
  AlertCircle,
  CheckCircle,
  Clock,
  FileText,
  Loader2,
  Upload,
  XCircle,
} from "lucide-react";

const DOC_TYPES: { value: KycDocument["type"]; label: string; desc: string }[] = [
  { value: "passport", label: "Pasaport", desc: "Geçerli pasaportunuzun tüm sayfaları" },
  { value: "national_id", label: "Kimlik Kartı", desc: "TC kimlik kartı (ön ve arka yüz)" },
  { value: "drivers_license", label: "Ehliyet", desc: "Geçerli sürücü belgesi" },
  { value: "proof_of_address", label: "Adres Belgesi", desc: "Son 3 aya ait fatura / banka ekstresi" },
];

function kycStatusInfo(status: KycSession["status"]): {
  icon: React.ReactNode;
  label: string;
  color: string;
  bg: string;
} {
  switch (status) {
    case "approved":
      return {
        icon: <CheckCircle size={20} />,
        label: "Onaylandı",
        color: "text-green-700",
        bg: "bg-green-50 border-green-200",
      };
    case "in_review":
      return {
        icon: <Clock size={20} />,
        label: "İnceleniyor",
        color: "text-blue-700",
        bg: "bg-blue-50 border-blue-200",
      };
    case "pending":
      return {
        icon: <Clock size={20} />,
        label: "Belgeler Bekleniyor",
        color: "text-yellow-700",
        bg: "bg-yellow-50 border-yellow-200",
      };
    case "rejected":
      return {
        icon: <XCircle size={20} />,
        label: "Reddedildi",
        color: "text-red-700",
        bg: "bg-red-50 border-red-200",
      };
    case "expired":
      return {
        icon: <AlertCircle size={20} />,
        label: "Süresi Doldu",
        color: "text-orange-700",
        bg: "bg-orange-50 border-orange-200",
      };
    default:
      return {
        icon: <FileText size={20} />,
        label: "Başlatılmadı",
        color: "text-slate-700",
        bg: "bg-slate-50 border-slate-200",
      };
  }
}

export default function KycPage() {
  const { user } = useAuth();
  const [session, setSession] = useState<KycSession | null>(null);
  const [loading, setLoading] = useState(true);
  const [starting, setStarting] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [uploadingDoc, setUploadingDoc] = useState<string | null>(null);
  const fileInputRefs = useRef<Record<string, HTMLInputElement | null>>({});

  useEffect(() => {
    kycApi.getSession().then((r) => {
      setSession(r.data);
      setLoading(false);
    }).catch(() => setLoading(false));
  }, []);

  const startSession = async () => {
    setStarting(true);
    const res = await kycApi.startSession();
    setSession(res.data);
    setStarting(false);
  };

  const uploadDocument = async (docType: KycDocument["type"], file: File) => {
    if (!session) return;
    setUploadingDoc(docType);
    try {
      const res = await kycApi.uploadDocument(session.id, docType, file);
      setSession((s) =>
        s
          ? { ...s, documents: [...s.documents.filter((d) => d.type !== docType), res.data] }
          : s
      );
    } finally {
      setUploadingDoc(null);
    }
  };

  const submitSession = async () => {
    if (!session) return;
    setSubmitting(true);
    const res = await kycApi.submitSession(session.id);
    setSession(res.data);
    setSubmitting(false);
  };

  if (loading) {
    return (
      <div className="p-8 flex items-center justify-center">
        <Loader2 size={24} className="animate-spin text-yellow-500" />
      </div>
    );
  }

  const statusInfo = kycStatusInfo(session?.status ?? "not_started");
  const uploadedTypes = new Set(session?.documents.map((d) => d.type) ?? []);
  const hasRequiredDocs = uploadedTypes.has("passport") || uploadedTypes.has("national_id");
  const canSubmit = hasRequiredDocs && session?.status === "pending";

  return (
    <div className="p-6 md:p-8 max-w-2xl">
      <h1 className="text-2xl font-bold text-slate-900 mb-2">Kimlik Doğrulama</h1>
      <p className="text-slate-500 mb-6">
        İşlem yapabilmek için KYC (Müşterini Tanı) sürecini tamamlamanız gerekmektedir.
      </p>

      {/* Status banner */}
      {session && (
        <div className={`border rounded-xl px-5 py-4 mb-6 flex items-center gap-3 ${statusInfo.bg}`}>
          <span className={statusInfo.color}>{statusInfo.icon}</span>
          <div>
            <p className={`font-semibold ${statusInfo.color}`}>Durum: {statusInfo.label}</p>
            {session.reviewedAt && (
              <p className="text-sm text-slate-500">
                {new Date(session.reviewedAt).toLocaleDateString("tr-TR")} tarihinde güncellendi
              </p>
            )}
            {session.rejectionReason && (
              <p className="text-sm text-red-600 mt-1">Sebep: {session.rejectionReason}</p>
            )}
          </div>
        </div>
      )}

      {/* Not started */}
      {!session && (
        <div className="bg-white rounded-2xl border border-slate-200 p-8 text-center shadow-sm">
          <FileText size={40} className="text-slate-300 mx-auto mb-4" />
          <h2 className="text-lg font-semibold text-slate-800 mb-2">KYC sürecini başlatın</h2>
          <p className="text-slate-500 mb-6 text-sm">
            Kimliğinizi doğrulamak için gerekli belgeleri yükleyin.
            Onay süreci genellikle 1–2 iş günü içinde tamamlanır.
          </p>
          <button
            onClick={startSession}
            disabled={starting}
            className="bg-yellow-400 text-slate-900 px-6 py-3 rounded-xl font-semibold hover:bg-yellow-300 disabled:opacity-50 transition-colors flex items-center gap-2 mx-auto"
          >
            {starting && <Loader2 size={16} className="animate-spin" />}
            KYC Başlat
          </button>
        </div>
      )}

      {/* Approved */}
      {session?.status === "approved" && (
        <div className="bg-green-50 border border-green-200 rounded-2xl p-6 text-center">
          <CheckCircle size={40} className="text-green-500 mx-auto mb-3" />
          <h2 className="text-lg font-semibold text-green-800 mb-1">Kimliğiniz doğrulandı</h2>
          <p className="text-green-600 text-sm">
            Artık altın alım/satım işlemi yapabilirsiniz.
          </p>
        </div>
      )}

      {/* In review */}
      {session?.status === "in_review" && (
        <div className="bg-blue-50 border border-blue-200 rounded-2xl p-6 text-center">
          <Clock size={40} className="text-blue-500 mx-auto mb-3" />
          <h2 className="text-lg font-semibold text-blue-800 mb-1">Belgeleriniz inceleniyor</h2>
          <p className="text-blue-600 text-sm">
            Uyum ekibimiz belgelerinizi inceliyor. Onay süreci 1–2 iş günü sürer.
          </p>
        </div>
      )}

      {/* Document upload form */}
      {session && ["pending", "rejected"].includes(session.status) && (
        <div className="space-y-4">
          <h2 className="font-semibold text-slate-800">Belge Yükleme</h2>
          <p className="text-sm text-slate-500">
            En az bir kimlik belgesi (pasaport veya kimlik kartı) ve adres belgesi gereklidir.
          </p>

          {DOC_TYPES.map((docType) => {
            const uploaded = session.documents.find((d) => d.type === docType.value);
            const isUploading = uploadingDoc === docType.value;

            return (
              <div
                key={docType.value}
                className="bg-white border border-slate-200 rounded-xl p-4 flex items-center justify-between shadow-sm"
              >
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <p className="font-medium text-slate-800 text-sm">{docType.label}</p>
                    {uploaded && (
                      <span
                        className={`text-xs px-2 py-0.5 rounded-full ${
                          uploaded.status === "accepted"
                            ? "bg-green-100 text-green-700"
                            : uploaded.status === "rejected"
                            ? "bg-red-100 text-red-700"
                            : "bg-blue-100 text-blue-700"
                        }`}
                      >
                        {uploaded.status === "accepted"
                          ? "Onaylandı"
                          : uploaded.status === "rejected"
                          ? "Reddedildi"
                          : "Yüklendi"}
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-slate-400 mt-0.5">{docType.desc}</p>
                </div>

                <input
                  type="file"
                  accept="image/*,.pdf"
                  className="hidden"
                  ref={(el) => { fileInputRefs.current[docType.value] = el; }}
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) uploadDocument(docType.value, file);
                  }}
                />
                <button
                  onClick={() => fileInputRefs.current[docType.value]?.click()}
                  disabled={isUploading}
                  className="ml-4 flex items-center gap-1.5 text-sm text-yellow-600 hover:text-yellow-700 border border-yellow-300 hover:border-yellow-400 rounded-lg px-3 py-1.5 transition-colors disabled:opacity-50"
                >
                  {isUploading ? (
                    <Loader2 size={14} className="animate-spin" />
                  ) : (
                    <Upload size={14} />
                  )}
                  {uploaded ? "Güncelle" : "Yükle"}
                </button>
              </div>
            );
          })}

          <button
            onClick={submitSession}
            disabled={!canSubmit || submitting}
            className="w-full bg-yellow-400 text-slate-900 py-3 rounded-xl font-semibold hover:bg-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-2 mt-2"
          >
            {submitting && <Loader2 size={16} className="animate-spin" />}
            {submitting ? "Gönderiliyor..." : "Belgeleri Gönder"}
          </button>
          {!hasRequiredDocs && (
            <p className="text-xs text-slate-500 text-center">
              Göndermek için en az bir kimlik belgesi yüklemeniz gerekir.
            </p>
          )}
        </div>
      )}

      {/* Steps */}
      <div className="mt-8 bg-slate-50 rounded-xl p-5 border border-slate-200">
        <h3 className="font-medium text-slate-700 mb-3 text-sm">KYC Süreci</h3>
        <ol className="space-y-2 text-sm text-slate-600">
          <li className="flex items-start gap-2">
            <span className="w-5 h-5 rounded-full bg-yellow-400 text-slate-900 text-xs flex items-center justify-center font-bold shrink-0 mt-0.5">1</span>
            Belgeleri yükle ve formu gönder
          </li>
          <li className="flex items-start gap-2">
            <span className="w-5 h-5 rounded-full bg-yellow-400 text-slate-900 text-xs flex items-center justify-center font-bold shrink-0 mt-0.5">2</span>
            Uyum ekibi 1–2 iş günü içinde inceler
          </li>
          <li className="flex items-start gap-2">
            <span className="w-5 h-5 rounded-full bg-yellow-400 text-slate-900 text-xs flex items-center justify-center font-bold shrink-0 mt-0.5">3</span>
            Onay sonrası altın alım/satımı başlatabilirsiniz
          </li>
        </ol>
      </div>
    </div>
  );
}
