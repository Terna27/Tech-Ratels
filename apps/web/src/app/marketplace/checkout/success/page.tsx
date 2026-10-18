"use client";

import React, { useEffect, useState, Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  CheckCircle2,
  AlertCircle,
  Package,
  Truck,
  ArrowRight,
  ShoppingBag,
  ExternalLink,
  ShieldCheck,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { Spinner } from "@/components/ui/Spinner";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";

function OrderSuccessContent() {
  const searchParams = useSearchParams();
  const reference = searchParams.get("reference") || searchParams.get("trxref");

  const [loading, setLoading] = useState(true);
  const [verified, setVerified] = useState(false);
  const [order, setOrder] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!reference) {
      setError("No transaction reference was provided in the URL.");
      setLoading(false);
      return;
    }

    async function verifyTransaction() {
      try {
        const res = await apiRequest<{ status: string; order: any }>("/marketplace/payments/verify", {
          method: "POST",
          body: JSON.stringify({ reference }),
        });

        if (res.status === "paid" || res.order) {
          setVerified(true);
          setOrder(res.order);
        } else {
          setError("Payment could not be verified automatically. Please contact support.");
        }
      } catch (err: any) {
        // If already verified or order lookup needed
        try {
          const lookup = await apiRequest<{ order: any; items: any[] }>(`/marketplace/orders/lookup/${reference}`);
          if (lookup.order) {
            setVerified(true);
            setOrder(lookup.order);
            return;
          }
        } catch (_) {}

        setError(err.message || "Failed to verify transaction.");
      } finally {
        setLoading(false);
      }
    }

    verifyTransaction();
  }, [reference]);

  if (loading) {
    return (
      <div className="py-20 text-center space-y-4">
        <Spinner text="Verifying payment with banking gateway..." />
        <p className="text-xs text-gray-500 max-w-sm mx-auto">
          Please wait while we confirm your transaction and reserve your items with the vendor.
        </p>
      </div>
    );
  }

  if (error || !verified) {
    return (
      <div className="max-w-2xl mx-auto px-4 py-16 text-center space-y-6">
        <div className="w-16 h-16 bg-amber-100 rounded-full flex items-center justify-center mx-auto text-amber-700">
          <AlertCircle className="w-8 h-8" />
        </div>
        <h1 className="text-2xl font-bold text-gray-900">Payment Verification Issue</h1>
        <p className="text-gray-600 text-sm max-w-md mx-auto">
          {error || "We could not verify your payment at this moment. If your account was debited, your order will be updated once our banking webhook confirms the transfer."}
        </p>
        <div className="flex flex-col sm:flex-row gap-3 justify-center pt-4">
          <Link
            href="/marketplace"
            className="px-5 py-2.5 bg-gray-100 text-gray-800 text-xs font-semibold rounded-xl hover:bg-gray-200 transition"
          >
            Return to Ratels Marketplace
          </Link>
          <Link
            href="/marketplace/orders"
            className="px-5 py-2.5 bg-emerald-800 text-white text-xs font-semibold rounded-xl hover:bg-emerald-700 transition"
          >
            Check My Orders
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto px-4 py-12 space-y-8">
      {/* Success Banner */}
      <div className="bg-emerald-50 border border-emerald-200 rounded-2xl p-6 sm:p-8 text-center space-y-3">
        <div className="w-14 h-14 bg-emerald-100 text-emerald-800 rounded-full flex items-center justify-center mx-auto">
          <CheckCircle2 className="w-8 h-8" />
        </div>
        <h1 className="text-2xl font-black text-emerald-950">
          Payment Confirmed! Order Placed Successfully
        </h1>
        <p className="text-xs text-emerald-800 max-w-md mx-auto">
          Thank you for shopping on Ratels Marketplace. Your transaction has been securely processed and the vendor has been notified to fulfil your order.
        </p>
        <div className="pt-2">
          <span className="inline-block px-3.5 py-1.5 bg-white text-emerald-950 font-mono text-xs font-bold rounded-lg border border-emerald-300 shadow-xs">
            Order Reference: {order?.order_number || reference}
          </span>
        </div>
      </div>

      {/* Order Details Card */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-6">
        <div className="flex flex-wrap items-center justify-between gap-4 border-b border-gray-100 pb-4">
          <div>
            <div className="text-xs text-gray-500">Order Number</div>
            <div className="text-base font-bold text-gray-900 font-mono">
              {order?.order_number || reference}
            </div>
          </div>
          <div className="flex gap-2">
            <Badge variant="primary" size="sm">
              Payment: Paid
            </Badge>
            <Badge variant="neutral" size="sm">
              Status: Processing
            </Badge>
          </div>
        </div>

        {/* Breakdown */}
        <div className="space-y-3 text-xs">
          <div className="flex justify-between text-gray-600">
            <span>Recipient Name</span>
            <span className="font-semibold text-gray-900">{order?.buyer_name || "Valued Customer"}</span>
          </div>
          <div className="flex justify-between text-gray-600">
            <span>Notification Email</span>
            <span className="font-semibold text-gray-900">{order?.buyer_email || "Email on record"}</span>
          </div>
          <div className="flex justify-between text-gray-600">
            <span>Fulfilment Method</span>
            <span className="font-semibold text-gray-900 capitalize">
              {order?.delivery_type === "pickup" ? "Branch Pickup" : "Doorstep Delivery"}
            </span>
          </div>
          {order?.shipping_address && (
            <div className="flex justify-between text-gray-600">
              <span>Delivery Address</span>
              <span className="font-semibold text-gray-900 text-right max-w-xs">
                {order.shipping_address}
              </span>
            </div>
          )}
          <div className="pt-3 border-t border-gray-100 flex justify-between text-sm font-extrabold text-gray-900">
            <span>Total Paid</span>
            <span className="text-emerald-800">
              ₦{order?.total_kobo ? (order.total_kobo / 100).toLocaleString() : "Confirmed"}
            </span>
          </div>
        </div>

        {/* Protection Note */}
        <div className="p-4 bg-gray-50 rounded-xl text-xs text-gray-600 flex items-start gap-3">
          <ShieldCheck className="w-5 h-5 text-emerald-700 shrink-0 mt-0.5" />
          <div>
            <span className="font-bold text-gray-900">Ratels Buyer Protection:</span> Your funds are held in escrow until the vendor dispatches the merchandise and delivery is confirmed. If you experience any issues, you can file a dispute directly from your orders dashboard.
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex flex-col sm:flex-row gap-3 pt-2">
          <Link
            href="/marketplace"
            className="flex-1 py-3 px-4 bg-gray-100 hover:bg-gray-200 text-gray-800 text-xs font-bold rounded-xl transition text-center flex items-center justify-center gap-2"
          >
            <ShoppingBag className="w-4 h-4" />
            Continue Shopping
          </Link>
          <Link
            href="/marketplace/orders"
            className="flex-1 py-3 px-4 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-xl transition text-center flex items-center justify-center gap-2"
          >
            <Package className="w-4 h-4" />
            Track Order Status
          </Link>
        </div>
      </div>
    </div>
  );
}

export default function OrderSuccessPage() {
  return (
    <Suspense fallback={<Spinner text="Loading order confirmation..." />}>
      <OrderSuccessContent />
    </Suspense>
  );
}
