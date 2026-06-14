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
    case "COMPLETED": return "text-green-600 bg-green-50";
    case "CANCELLED":
    case "FAILED_NO_STOCK": return "text-red-600 bg-red-50";
    case "PAYMENT_PENDING": return "text-yellow-600 bg-yellow-50";
    default: return "text-blue-600 bg-blue-50";
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

/**
 * Convert a wei-denominated balance (18 decimals) to grams as a string with
 * 4 fractional digits. Works entirely in BigInt/string space so it stays
 * exact for arbitrarily large balances (Number(bigint)/1e18 loses precision
 * above ~9 GOLD).
 */
export function weiToGrams(wei: string): string {
  const negative = wei.trim().startsWith("-");
  const n = BigInt(wei);
  const zero = BigInt(0);
  const abs = n < zero ? -n : n;
  const weiPerGram = BigInt("1000000000000000000"); // 1e18
  const whole = abs / weiPerGram;
  // 4 decimal places: scale the fractional part to 4 digits (truncated).
  const fracScaled = ((abs % weiPerGram) * BigInt(10000)) / weiPerGram;
  const frac = fracScaled.toString().padStart(4, "0");
  return `${negative ? "-" : ""}${whole.toString()}.${frac}`;
}
