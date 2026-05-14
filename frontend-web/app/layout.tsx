import type { Metadata, Viewport } from "next";
import "./globals.css";
import { AuthProvider } from "@/contexts/AuthContext";

export const metadata: Metadata = {
  title: "GOLD Token — Dijital Altın Platformu",
  description:
    "1 GOLD = 1 gram fiziksel altın. Blokzincir üzerinde güvenli, denetlenebilir altın yatırımı.",
  applicationName: "GOLD Token",
  authors: [{ name: "GOLD Token" }],
  keywords: ["altın", "gold token", "tahsisli altın", "proof of reserve", "ERC-20"],
  openGraph: {
    title: "GOLD Token — Dijital Altın Platformu",
    description:
      "1 GOLD = 1 gram fiziksel altın. Aylık bağımsız denetim, %100 rezerv koruması.",
    type: "website",
    locale: "tr_TR",
  },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#fafafa" },
    { media: "(prefers-color-scheme: dark)", color: "#06070a" },
  ],
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="tr" suppressHydrationWarning>
      <body className="antialiased bg-surface-1 text-ink-0 min-h-screen">
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
