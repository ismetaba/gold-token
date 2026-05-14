import type { OrderStatus, OrderType } from "./types";

export function formatGrams(grams: string | number): string {
  const n = typeof grams === "string" ? parseFloat(grams) : grams;
  return n.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 4 });
}

export function formatTRY(amount: string | number): string {
  const n = typeof amount === "string" ? parseFloat(amount) : amount;
  return n.toLocaleString("tr-TR", {
    style: "currency",
    currency: "TRY",
    minimumFractionDigits: 2,
  });
}

export function formatUSD(amount: string | number): string {
  const n = typeof amount === "string" ? parseFloat(amount) : amount;
  return n.toLocaleString("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
  });
}

export function truncateAddress(address: string): string {
  if (address.length < 12) return address;
  return `${address.slice(0, 6)}…${address.slice(-4)}`;
}

export function orderTypeLabel(type: OrderType): string {
  switch (type) {
    case "buy": return "Alım";
    case "sell": return "Satım";
    case "redeem_cash": return "Nakit İtfa";
    case "redeem_physical": return "Fiziksel İtfa";
  }
}

export function orderStatusLabel(status: OrderStatus): string {
  switch (status) {
    case "CREATED": return "Oluşturuldu";
    case "PAYMENT_PENDING": return "Ödeme Bekliyor";
    case "PAID": return "Ödendi";
    case "RESERVING_BARS": return "Altın Rezerve Ediliyor";
    case "MINT_PROPOSED": return "Mint Önerildi";
    case "MINT_EXECUTED": return "Mint Gerçekleşti";
    case "COMPLETED": return "Tamamlandı";
    case "CANCELLED": return "İptal Edildi";
    case "FAILED_NO_STOCK": return "Stok Yetersiz";
  }
}

export function orderStatusColor(status: OrderStatus): string {
  switch (status) {
    case "COMPLETED": return "text-emerald-700 bg-emerald-50 border-emerald-200";
    case "CANCELLED":
    case "FAILED_NO_STOCK": return "text-rose-700 bg-rose-50 border-rose-200";
    case "PAYMENT_PENDING": return "text-amber-700 bg-amber-50 border-amber-200";
    default: return "text-sky-700 bg-sky-50 border-sky-200";
  }
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("tr-TR", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function weiToGrams(wei: string): string {
  const n = BigInt(wei);
  const grams = Number(n) / 1e18;
  return grams.toFixed(4);
}
