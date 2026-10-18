"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Package,
  ShoppingBag,
  ChevronRight,
  Clock,
  Truck,
  Building2,
  AlertCircle,
  ExternalLink,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { Spinner } from "@/components/ui/Spinner";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";

interface OrderItemSummary {
  id: string;
  order_number: string;
  order_status: string;
  payment_status: string;
  fulfilment_status: string;
  delivery_type: string;
  total_kobo: number;
  item_count: number;
  created_at: string;
}

export default function BuyerOrdersPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [orders, setOrders] = useState<OrderItemSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login?redirect=/marketplace/orders");
      return;
    }

    if (user) {
      async function fetchOrders() {
        try {
          const data = await apiRequest<OrderItemSummary[]>("/marketplace/orders");
          setOrders(data || []);
        } catch (err: any) {
          setError(err.message || "Failed to load orders");
        } finally {
          setLoading(false);
        }
      }
      fetchOrders();
    }
  }, [user, authLoading, router]);

  if (authLoading || (loading && user)) {
    return <Spinner text="Loading your Ratels Marketplace orders..." />;
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Breadcrumb Navigation */}
      <nav className="flex items-center gap-2 text-xs text-gray-500 font-medium">
        <Link href="/marketplace" className="hover:text-emerald-800 transition">
          Ratels Marketplace
        </Link>
        <ChevronRight className="w-3.5 h-3.5" />
        <span className="text-gray-900 font-semibold">My Orders</span>
      </nav>

      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b border-gray-200 pb-5">
        <div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900">
            My Marketplace Orders
          </h1>
          <p className="text-sm text-gray-600 mt-1">
            Track real-time shipment status, view receipts, and manage order deliveries.
          </p>
        </div>
        <Link
          href="/marketplace"
          className="px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-xl transition inline-flex items-center gap-2 shadow-xs"
        >
          <ShoppingBag className="w-4 h-4" />
          Continue Shopping
        </Link>
      </div>

      {error && <Alert type="error">{error}</Alert>}

      {orders.length === 0 ? (
        <EmptyState
          icon={<Package className="w-6 h-6" />}
          title="No Orders Placed Yet"
          description="You haven't placed any orders on the Ratels Marketplace yet. Discover verified goods and products from Nigerian businesses."
          actionText="Explore Marketplace"
          onAction={() => router.push("/marketplace")}
        />
      ) : (
        <div className="space-y-4">
          {orders.map((ord) => {
            const dateStr = new Date(ord.created_at).toLocaleDateString("en-NG", {
              day: "numeric",
              month: "short",
              year: "numeric",
              hour: "2-digit",
              minute: "2-digit",
            });

            return (
              <div
                key={ord.id}
                className="bg-white rounded-2xl border border-gray-200 p-5 shadow-sm hover:border-emerald-300 transition flex flex-col md:flex-row items-start md:items-center justify-between gap-4"
              >
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-mono font-bold text-sm text-gray-900">
                      {ord.order_number}
                    </span>
                    <span className="text-xs text-gray-400">•</span>
                    <span className="text-xs text-gray-500 flex items-center gap-1">
                      <Clock className="w-3.5 h-3.5" />
                      {dateStr}
                    </span>
                  </div>

                  <div className="flex flex-wrap items-center gap-2 text-xs">
                    <Badge
                      variant={
                        ord.payment_status === "paid"
                          ? "primary"
                          : ord.payment_status === "failed"
                          ? "danger"
                          : "neutral"
                      }
                      size="sm"
                    >
                      Payment: {ord.payment_status}
                    </Badge>

                    <Badge
                      variant={
                        ord.fulfilment_status === "delivered"
                          ? "primary"
                          : ord.fulfilment_status === "dispatched"
                          ? "info"
                          : "neutral"
                      }
                      size="sm"
                    >
                      Fulfilment: {ord.fulfilment_status}
                    </Badge>

                    <span className="text-gray-500 capitalize flex items-center gap-1">
                      {ord.delivery_type === "pickup" ? (
                        <>
                          <Building2 className="w-3.5 h-3.5 text-gray-400" />
                          Pickup
                        </>
                      ) : (
                        <>
                          <Truck className="w-3.5 h-3.5 text-gray-400" />
                          Delivery
                        </>
                      )}
                    </span>
                  </div>
                </div>

                <div className="flex items-center justify-between md:justify-end gap-6 w-full md:w-auto pt-3 md:pt-0 border-t md:border-t-0 border-gray-100">
                  <div className="text-left md:text-right">
                    <div className="text-[11px] text-gray-400">Total Amount</div>
                    <div className="text-base font-extrabold text-gray-900">
                      ₦{(ord.total_kobo / 100).toLocaleString()}
                    </div>
                  </div>

                  <Link
                    href={`/marketplace/orders/${ord.id}`}
                    className="px-4 py-2 bg-gray-50 hover:bg-emerald-50 text-emerald-800 hover:text-emerald-900 border border-gray-200 hover:border-emerald-200 text-xs font-bold rounded-xl transition inline-flex items-center gap-1.5"
                  >
                    <span>View Details</span>
                    <ChevronRight className="w-4 h-4" />
                  </Link>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
