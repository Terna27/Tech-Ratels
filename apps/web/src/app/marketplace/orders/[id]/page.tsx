"use client";

import React, { useEffect, useState, use } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Package,
  Truck,
  CheckCircle2,
  Clock,
  ChevronRight,
  ArrowLeft,
  AlertTriangle,
  Building2,
  ShieldAlert,
  ExternalLink,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { resolveImageUrl } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";

interface OrderDetailProps {
  params: Promise<{ id: string }>;
}

export default function OrderDetailPage({ params }: OrderDetailProps) {
  const resolvedParams = use(params);
  const orderId = resolvedParams.id;
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Dispute Modal State
  const [showDisputeModal, setShowDisputeModal] = useState(false);
  const [disputeReason, setDisputeReason] = useState("never_received");
  const [requestedAction, setRequestedAction] = useState("refund");
  const [disputeNotes, setDisputeNotes] = useState("");
  const [disputeSubmitting, setDisputeSubmitting] = useState(false);
  const [disputeSuccess, setDisputeSuccess] = useState<string | null>(null);
  const [disputeError, setDisputeError] = useState<string | null>(null);

  const [cancelling, setCancelling] = useState(false);
  const [confirmingDelivery, setConfirmingDelivery] = useState(false);
  const [actionMessage, setActionMessage] = useState<string | null>(null);

  const fetchOrder = async () => {
    try {
      const res = await apiRequest<any>(`/marketplace/orders/${orderId}`);
      setData(res);
    } catch (err: any) {
      setError(err.message || "Failed to load order details");
    } finally {
      setLoading(false);
    }
  };

  const handleCancelOrder = async () => {
    if (!window.confirm("Are you sure you want to cancel this order? Any reserved stock will be returned.")) {
      return;
    }
    setCancelling(true);
    setError(null);
    try {
      await apiRequest(`/marketplace/orders/${orderId}/cancel`, {
        method: "POST",
        body: JSON.stringify({ reason: "Cancelled by buyer" }),
      });
      setActionMessage("Order has been successfully cancelled.");
      fetchOrder();
    } catch (err: any) {
      setError(err.message || "Failed to cancel order");
    } finally {
      setCancelling(false);
    }
  };

  const handleConfirmDelivery = async () => {
    if (!window.confirm("Confirm that you have received this shipment in good condition?")) {
      return;
    }
    setConfirmingDelivery(true);
    setError(null);
    try {
      await apiRequest(`/marketplace/orders/${orderId}/confirm-delivery`, {
        method: "POST",
        body: JSON.stringify({ notes: "Delivery confirmed by buyer" }),
      });
      setActionMessage("Receipt confirmed. Thank you for shopping on Ratels Marketplace!");
      fetchOrder();
    } catch (err: any) {
      setError(err.message || "Failed to confirm delivery");
    } finally {
      setConfirmingDelivery(false);
    }
  };

  useEffect(() => {
    if (!authLoading && !user) {
      router.push(`/login?redirect=/marketplace/orders/${orderId}`);
      return;
    }

    if (user && orderId) {
      fetchOrder();
    }
  }, [user, authLoading, orderId]);

  const handleRaiseDispute = async (e: React.FormEvent) => {
    e.preventDefault();
    setDisputeError(null);
    setDisputeSubmitting(true);

    try {
      await apiRequest(`/marketplace/orders/${orderId}/dispute`, {
        method: "POST",
        body: JSON.stringify({
          reason: disputeReason,
          requested_action: requestedAction,
          notes: disputeNotes.trim(),
        }),
      });

      setDisputeSuccess("Your dispute has been logged. Ratels administration will review your claim.");
      setShowDisputeModal(false);
      fetchOrder();
    } catch (err: any) {
      setDisputeError(err.message || "Failed to raise dispute");
    } finally {
      setDisputeSubmitting(false);
    }
  };

  if (authLoading || loading) {
    return <Spinner text="Loading order tracking information..." />;
  }

  if (error || !data || !data.order) {
    return (
      <div className="max-w-2xl mx-auto px-4 py-16 text-center space-y-4">
        <h1 className="text-xl font-bold text-gray-900">Order Not Found</h1>
        <p className="text-xs text-gray-600">{error || "The requested order could not be located."}</p>
        <Link
          href="/marketplace/orders"
          className="inline-flex items-center gap-2 px-5 py-2.5 bg-emerald-800 text-white text-xs font-semibold rounded-xl"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Orders
        </Link>
      </div>
    );
  }

  const { order, items = [], shipment } = data;
  const isPaid = order.payment_status === "paid";
  const isDispatched = order.fulfilment_status === "dispatched" || order.fulfilment_status === "delivered";
  const isDelivered = order.fulfilment_status === "delivered";

  return (
    <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Breadcrumb Navigation */}
      <nav className="flex items-center gap-2 text-xs text-gray-500 font-medium">
        <Link href="/marketplace" className="hover:text-emerald-800 transition">
          Ratels Marketplace
        </Link>
        <ChevronRight className="w-3.5 h-3.5" />
        <Link href="/marketplace/orders" className="hover:text-emerald-800 transition">
          My Orders
        </Link>
        <ChevronRight className="w-3.5 h-3.5" />
        <span className="text-gray-900 font-semibold font-mono">{order.order_number}</span>
      </nav>

      {disputeSuccess && <Alert type="success">{disputeSuccess}</Alert>}
      {actionMessage && <Alert type="success">{actionMessage}</Alert>}

      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b border-gray-200 pb-5">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-black text-gray-900 font-mono">
              {order.order_number}
            </h1>
            <Badge
              variant={
                order.order_status === "delivered"
                  ? "primary"
                  : order.order_status === "cancelled"
                  ? "danger"
                  : "info"
              }
            >
              {order.order_status.toUpperCase()}
            </Badge>
          </div>
          <p className="text-xs text-gray-500 mt-1">
            Placed on{" "}
            {new Date(order.created_at).toLocaleDateString("en-NG", {
              day: "numeric",
              month: "long",
              year: "numeric",
              hour: "2-digit",
              minute: "2-digit",
            })}
          </p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          {order.order_status !== "cancelled" &&
            order.order_status !== "refunded" &&
            order.order_status !== "delivered" &&
            order.fulfilment_status !== "dispatched" && (
              <button
                onClick={handleCancelOrder}
                disabled={cancelling}
                className="px-4 py-2 border border-gray-300 text-gray-700 bg-white hover:bg-gray-50 text-xs font-bold rounded-xl transition disabled:opacity-50"
              >
                {cancelling ? "Cancelling..." : "Cancel Order"}
              </button>
            )}

          {order.fulfilment_status === "dispatched" && order.order_status !== "delivered" && (
            <button
              onClick={handleConfirmDelivery}
              disabled={confirmingDelivery}
              className="px-4 py-2 border border-emerald-300 text-emerald-800 bg-emerald-50 hover:bg-emerald-100 text-xs font-bold rounded-xl transition disabled:opacity-50 inline-flex items-center gap-1.5"
            >
              <CheckCircle2 className="w-4 h-4 text-emerald-600" />
              {confirmingDelivery ? "Confirming..." : "Confirm Received"}
            </button>
          )}

          <button
            onClick={() => setShowDisputeModal(true)}
            className="px-4 py-2 border border-red-200 text-red-700 bg-red-50 hover:bg-red-100 text-xs font-bold rounded-xl transition inline-flex items-center gap-1.5"
          >
            <ShieldAlert className="w-4 h-4" />
            Raise Dispute
          </button>
        </div>
      </div>

      {/* Progress Timeline Tracker */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm">
        <h2 className="text-sm font-bold text-gray-900 mb-6">Order Lifecycle & Fulfilment</h2>
        <div className="grid grid-cols-4 gap-2 relative">
          <div className="text-center space-y-2">
            <div className="w-10 h-10 rounded-full bg-emerald-700 text-white flex items-center justify-center mx-auto text-xs font-bold shadow-xs">
              <CheckCircle2 className="w-5 h-5" />
            </div>
            <div className="text-xs font-bold text-gray-900">Order Placed</div>
            <div className="text-[10px] text-gray-400">Order received</div>
          </div>

          <div className="text-center space-y-2">
            <div
              className={`w-10 h-10 rounded-full flex items-center justify-center mx-auto text-xs font-bold shadow-xs transition ${
                isPaid ? "bg-emerald-700 text-white" : "bg-gray-100 text-gray-400"
              }`}
            >
              {isPaid ? <CheckCircle2 className="w-5 h-5" /> : "2"}
            </div>
            <div className={`text-xs font-bold ${isPaid ? "text-gray-900" : "text-gray-400"}`}>
              Payment Verified
            </div>
            <div className="text-[10px] text-gray-400">{isPaid ? "Escrow secured" : "Awaiting payment"}</div>
          </div>

          <div className="text-center space-y-2">
            <div
              className={`w-10 h-10 rounded-full flex items-center justify-center mx-auto text-xs font-bold shadow-xs transition ${
                isDispatched ? "bg-emerald-700 text-white" : "bg-gray-100 text-gray-400"
              }`}
            >
              {isDispatched ? <Truck className="w-5 h-5" /> : "3"}
            </div>
            <div className={`text-xs font-bold ${isDispatched ? "text-gray-900" : "text-gray-400"}`}>
              Dispatched
            </div>
            <div className="text-[10px] text-gray-400">
              {isDispatched ? "Handed to carrier" : "Vendor packing"}
            </div>
          </div>

          <div className="text-center space-y-2">
            <div
              className={`w-10 h-10 rounded-full flex items-center justify-center mx-auto text-xs font-bold shadow-xs transition ${
                isDelivered ? "bg-emerald-700 text-white" : "bg-gray-100 text-gray-400"
              }`}
            >
              {isDelivered ? <CheckCircle2 className="w-5 h-5" /> : "4"}
            </div>
            <div className={`text-xs font-bold ${isDelivered ? "text-gray-900" : "text-gray-400"}`}>
              Delivered
            </div>
            <div className="text-[10px] text-gray-400">{isDelivered ? "Receipt confirmed" : "Pending delivery"}</div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-12 gap-8">
        {/* Order Items */}
        <div className="md:col-span-7 space-y-4">
          <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4">
            <h2 className="text-sm font-bold text-gray-900">
              Purchased Items ({items.length})
            </h2>

            <div className="divide-y divide-gray-100">
              {items.map((item: any) => (
                <div key={item.id} className="py-3.5 flex items-center justify-between gap-4">
                  <div className="flex items-center gap-3">
                    {item.image_url_snapshot ? (
                      <img
                        src={resolveImageUrl(item.image_url_snapshot)}
                        alt={item.title_snapshot}
                        className="w-12 h-12 rounded-xl object-cover border border-gray-100 bg-gray-50"
                        onError={(e) => {
                          (e.target as HTMLElement).style.display = "none";
                        }}
                      />
                    ) : (
                      <div className="w-12 h-12 rounded-xl bg-gray-100 flex items-center justify-center text-gray-400">
                        <Package className="w-6 h-6" />
                      </div>
                    )}
                    <div>
                      <div className="text-xs font-bold text-gray-900">{item.title_snapshot}</div>
                      <div className="text-[11px] text-gray-500">
                        SKU: {item.sku_snapshot || "N/A"} • Qty: {item.quantity}
                      </div>
                    </div>
                  </div>
                  <div className="text-xs font-bold text-gray-900">
                    ₦{(item.subtotal_kobo / 100).toLocaleString()}
                  </div>
                </div>
              ))}
            </div>

            {/* Financial Totals */}
            <div className="space-y-2 pt-4 border-t border-gray-100 text-xs">
              <div className="flex justify-between text-gray-600">
                <span>Subtotal</span>
                <span className="font-semibold text-gray-900">
                  ₦{(order.subtotal_kobo / 100).toLocaleString()}
                </span>
              </div>
              <div className="flex justify-between text-gray-600">
                <span>Fulfilment Fee ({order.delivery_type})</span>
                <span className="font-semibold text-gray-900">
                  ₦{(order.delivery_fee_kobo / 100).toLocaleString()}
                </span>
              </div>
              <div className="flex justify-between text-sm font-extrabold text-gray-900 pt-3 border-t border-gray-200">
                <span>Total Amount Paid</span>
                <span className="text-emerald-800 text-base">
                  ₦{(order.total_kobo / 100).toLocaleString()}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Shipping & Consignment Information */}
        <div className="md:col-span-5 space-y-4">
          <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4 text-xs">
            <h2 className="text-sm font-bold text-gray-900 flex items-center gap-2">
              <Truck className="w-4 h-4 text-emerald-700" />
              Delivery & Destination
            </h2>

            <div className="space-y-2.5 text-gray-600">
              <div>
                <span className="block text-[11px] text-gray-400">Recipient Name</span>
                <span className="font-semibold text-gray-900">{order.buyer_name}</span>
              </div>
              <div>
                <span className="block text-[11px] text-gray-400">Contact Phone</span>
                <span className="font-semibold text-gray-900">{order.buyer_phone}</span>
              </div>
              <div>
                <span className="block text-[11px] text-gray-400">Contact Email</span>
                <span className="font-semibold text-gray-900">{order.buyer_email}</span>
              </div>
              <div>
                <span className="block text-[11px] text-gray-400">Shipping Address</span>
                <span className="font-semibold text-gray-900">{order.shipping_address}</span>
              </div>
              {order.notes && (
                <div>
                  <span className="block text-[11px] text-gray-400">Buyer Notes</span>
                  <span className="italic text-gray-700">{order.notes}</span>
                </div>
              )}
            </div>

            {/* Shipment Dispatch Details */}
            {shipment ? (
              <div className="mt-4 pt-4 border-t border-gray-100 space-y-2">
                <div className="text-xs font-bold text-emerald-950 flex items-center gap-1.5">
                  <Package className="w-3.5 h-3.5 text-emerald-700" />
                  Shipment Tracking Details
                </div>
                <div className="bg-gray-50 rounded-xl p-3 space-y-1.5 text-[11px]">
                  <div className="flex justify-between">
                    <span className="text-gray-500">Carrier / Courier:</span>
                    <span className="font-bold text-gray-900">{shipment.carrier_name}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-500">Tracking Code:</span>
                    <span className="font-mono font-bold text-gray-900">
                      {shipment.tracking_number || "Direct Handover"}
                    </span>
                  </div>
                  {shipment.dispatched_at && (
                    <div className="flex justify-between">
                      <span className="text-gray-500">Dispatched:</span>
                      <span className="text-gray-700">
                        {new Date(shipment.dispatched_at).toLocaleDateString("en-NG")}
                      </span>
                    </div>
                  )}
                </div>
              </div>
            ) : (
              <div className="p-3 bg-gray-50 rounded-xl text-[11px] text-gray-500">
                The seller is currently preparing your order. Once dispatched, tracking and courier credentials will appear here.
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Dispute Modal */}
      {showDisputeModal && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex items-center justify-between border-b border-gray-100 pb-3">
              <h3 className="font-bold text-base text-gray-900 flex items-center gap-2">
                <AlertTriangle className="w-5 h-5 text-red-600" />
                Raise Order Dispute
              </h3>
              <button
                onClick={() => setShowDisputeModal(false)}
                className="text-gray-400 hover:text-gray-600 text-sm font-bold"
              >
                ✕
              </button>
            </div>

            {disputeError && <Alert type="error">{disputeError}</Alert>}

            <form onSubmit={handleRaiseDispute} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">
                  Reason for Dispute *
                </label>
                <select
                  value={disputeReason}
                  onChange={(e) => setDisputeReason(e.target.value)}
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                >
                  <option value="never_received">Goods Never Received / Package Lost</option>
                  <option value="damaged_item">Item Arrived Damaged / Broken</option>
                  <option value="wrong_item">Received Wrong Item or Specification</option>
                  <option value="counterfeit">Item Does Not Match Description</option>
                  <option value="other">Other Commercial Dispute</option>
                </select>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">
                  Requested Resolution *
                </label>
                <select
                  value={requestedAction}
                  onChange={(e) => setRequestedAction(e.target.value)}
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                >
                  <option value="refund">Full Refund to Source Account</option>
                  <option value="replacement">Vendor Replacement Item</option>
                  <option value="mediation">Ratels Administrative Mediation</option>
                </select>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">
                  Detailed Explanation *
                </label>
                <textarea
                  required
                  rows={4}
                  value={disputeNotes}
                  onChange={(e) => setDisputeNotes(e.target.value)}
                  placeholder="Provide precise details about the issue to assist administrators in resolving your case..."
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div className="flex gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowDisputeModal(false)}
                  className="flex-1 py-2.5 bg-gray-100 hover:bg-gray-200 text-gray-700 font-bold rounded-xl transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={disputeSubmitting}
                  className="flex-1 py-2.5 bg-red-700 hover:bg-red-800 text-white font-bold rounded-xl transition disabled:opacity-50"
                >
                  {disputeSubmitting ? "Submitting Claim..." : "Submit Dispute"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
