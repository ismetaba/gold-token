"use client";

import { useEffect, useState } from "react";
import { priceApi } from "@/lib/api-client";
import { formatTRY, formatUSD } from "@/lib/utils";
import type { GoldPrice } from "@/lib/types";
import { TrendingDown, TrendingUp } from "lucide-react";

export default function GoldPriceTicker() {
  const [price, setPrice] = useState<GoldPrice | null>(null);
  const [direction, setDirection] = useState<"up" | "down" | null>(null);

  useEffect(() => {
    const fetchPrice = async () => {
      try {
        const res = await priceApi.getCurrentPrice();
        setPrice((prev) => {
          if (prev) {
            const oldVal = parseFloat(prev.pricePerGramTRY);
            const newVal = parseFloat(res.data.price.pricePerGramTRY);
            if (newVal !== oldVal) {
              setDirection(newVal > oldVal ? "up" : "down");
              setTimeout(() => setDirection(null), 2400);
            }
          }
          return res.data.price;
        });
      } catch {
        // ignore fetch errors in ticker
      }
    };

    fetchPrice();
    const interval = setInterval(fetchPrice, 15_000);
    return () => clearInterval(interval);
  }, []);

  if (!price) {
    return <div className="h-11 w-64 skeleton" />;
  }

  const accent =
    direction === "up"
      ? "text-emerald-600"
      : direction === "down"
      ? "text-rose-600"
      : "text-brand-600";

  const Icon = direction === "down" ? TrendingDown : TrendingUp;

  return (
    <div className="inline-flex items-center gap-3 bg-surface-0 border border-surface-3 rounded-2xl px-4 py-2 shadow-xs">
      <span className="inline-flex items-center gap-1.5 text-ink-2 text-xs uppercase tracking-wider">
        <span className="relative flex h-2 w-2">
          <span className="absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-60 animate-ping" />
          <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
        </span>
        Canlı
      </span>
      <span className="h-5 w-px bg-surface-3" />
      <div className="flex items-baseline gap-1.5">
        <Icon size={14} className={accent} />
        <span className={`font-mono font-semibold transition-colors duration-500 ${accent}`}>
          {formatTRY(price.pricePerGramTRY)}
        </span>
        <span className="text-ink-3 text-xs">/gr</span>
      </div>
      <span className="h-5 w-px bg-surface-3 hidden sm:block" />
      <span className="text-ink-2 font-mono text-xs hidden sm:inline">
        {formatUSD(price.pricePerGramUSD)}/gr
      </span>
    </div>
  );
}
