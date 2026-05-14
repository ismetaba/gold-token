import type { OrderStatus } from "@/lib/types";
import { orderStatusColor, orderStatusLabel } from "@/lib/utils";

export default function StatusBadge({ status }: { status: OrderStatus }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-medium tracking-wide border ${orderStatusColor(status)}`}
    >
      <span
        className="w-1.5 h-1.5 rounded-full bg-current opacity-70"
        aria-hidden="true"
      />
      {orderStatusLabel(status)}
    </span>
  );
}
