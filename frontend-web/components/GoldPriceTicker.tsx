"use client";

import { useEffect, useRef, useState } from "react";
import { formatTRY, formatUSD } from "@/lib/utils";
import { usePollingPrice } from "@/lib/hooks/usePollingPrice";
import { TrendingUp } from "lucide-react";

export default function GoldPriceTicker() {
  const price = usePollingPrice();
  const prevValRef = useRef<number | null>(null);
  const [direction, setDirection] = useState<"up" | "down" | null>(null);

  // Flash an up/down indicator whenever the polled price changes.
  useEffect(() => {
    if (!price) return;
    const newVal = parseFloat(price.pricePerGramTRY);
    if (prevValRef.current !== null && newVal !== prevValRef.current) {
      setDirection(newVal >= prevValRef.current ? "up" : "down");
      const id = setTimeout(() => setDirection(null), 2000);
      prevValRef.current = newVal;
      return () => clearTimeout(id);
    }
    prevValRef.current = newVal;
  }, [price]);

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
