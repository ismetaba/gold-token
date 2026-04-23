"use client";

import { useEffect, useState } from "react";
import { priceApi } from "@/lib/api-client";
import { formatTRY, formatUSD } from "@/lib/utils";
import type { GoldPrice } from "@/lib/types";
import { TrendingUp } from "lucide-react";

export default function GoldPriceTicker() {
  const [price, setPrice] = useState<GoldPrice | null>(null);
  const [prevPrice, setPrevPrice] = useState<string | null>(null);
  const [direction, setDirection] = useState<"up" | "down" | null>(null);

  useEffect(() => {
    const fetchPrice = async () => {
      try {
        const res = await priceApi.getCurrentPrice();
        setPrice((prev) => {
          if (prev) {
            const oldVal = parseFloat(prev.pricePerGramTRY);
            const newVal = parseFloat(res.data.price.pricePerGramTRY);
            setDirection(newVal >= oldVal ? "up" : "down");
            setPrevPrice(prev.pricePerGramTRY);
            setTimeout(() => setDirection(null), 2000);
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
    return (
      <div className="h-12 bg-slate-800 animate-pulse rounded-lg" />
    );
  }

  const colorClass =
    direction === "up"
      ? "text-green-400"
      : direction === "down"
      ? "text-red-400"
      : "text-yellow-400";

  return (
    <div className="flex items-center gap-4 bg-slate-800 rounded-lg px-4 py-2.5 text-sm">
      <div className="flex items-center gap-1.5 text-slate-400">
        <TrendingUp size={14} />
        <span>GOLD/gr</span>
      </div>
      <span className={`font-mono font-semibold transition-colors duration-500 ${colorClass}`}>
        {formatTRY(price.pricePerGramTRY)}
      </span>
      <span className="text-slate-500">·</span>
      <span className="text-slate-400 font-mono">
        {formatUSD(price.pricePerGramUSD)}/gr
      </span>
      <span className="text-slate-500">·</span>
      <span className="text-slate-500 text-xs">
        {price.source}
      </span>
    </div>
  );
}
