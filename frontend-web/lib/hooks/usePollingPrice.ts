"use client";

import { useEffect, useState } from "react";
import { priceApi } from "@/lib/api-client";
import type { GoldPrice } from "@/lib/types";

/** How often the live gold price is polled, in milliseconds. */
export const PRICE_POLL_INTERVAL_MS = 15_000;

/**
 * Polls the current gold price: fetches once on mount, then every
 * PRICE_POLL_INTERVAL_MS until unmount. Fetch errors are swallowed (the last
 * good price is kept), matching the previous inline behavior. Returns the
 * latest price, or null until the first successful fetch.
 */
export function usePollingPrice(): GoldPrice | null {
  const [price, setPrice] = useState<GoldPrice | null>(null);

  useEffect(() => {
    let ignore = false;

    const fetchPrice = async () => {
      try {
        const res = await priceApi.getCurrentPrice();
        if (!ignore) setPrice(res.data.price);
      } catch {
        // ignore fetch errors; keep last good price
      }
    };

    fetchPrice();
    const id = setInterval(fetchPrice, PRICE_POLL_INTERVAL_MS);
    return () => {
      ignore = true;
      clearInterval(id);
    };
  }, []);

  return price;
}
