import type { OrderStatus } from "@/lib/types";
import { orderStatusColor, orderStatusLabel } from "@/lib/utils";

export default function StatusBadge({ status }: { status: OrderStatus }) {
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${orderStatusColor(status)}`}
    >
      {orderStatusLabel(status)}
    </span>
  );
}
