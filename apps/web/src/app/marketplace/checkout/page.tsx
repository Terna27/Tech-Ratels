"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ShoppingBag,
  Truck,
  Building2,
  ShieldCheck,
  ChevronRight,
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
} from "lucide-react";
import { useCart } from "@/lib/cart-context";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { State, LGA } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

export default function CheckoutPage() {
  const router = useRouter();
  const { items, subtotalKobo, clearCart } = useCart();
  const { user } = useAuth();

  const [buyerName, setBuyerName] = useState("");
  const [buyerEmail, setBuyerEmail] = useState("");
  const [buyerPhone, setBuyerPhone] = useState("");
  const [deliveryType, setDeliveryType] = useState<"delivery" | "pickup">("delivery");
  const [shippingAddress, setShippingAddress] = useState("");
  const [shippingStateId, setShippingStateId] = useState("");
  const [shippingLgaId, setShippingLgaId] = useState("");
  const [notes, setNotes] = useState("");

  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);
  const [loadingGeo, setLoadingGeo] = useState(true);
  const [loadingLgas, setLoadingLgas] = useState(false);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [checkoutStatus, setCheckoutStatus] = useState<{ available: boolean; message: string; provider?: string } | null>(null);

  // Fetch checkout and payment processing status
  useEffect(() => {
    async function fetchStatus() {
      try {
        const status = await apiRequest<{ available: boolean; message: string; provider?: string }>("/marketplace/checkout/status");
        setCheckoutStatus(status);
      } catch (err) {
        // Fallback: non-blocking
      }
    }
    fetchStatus();
  }, []);

  // Pre-fill user data if logged in
  useEffect(() => {
    if (user) {
      if (user.display_name && !buyerName) setBuyerName(user.display_name);
      if (user.email && !buyerEmail) setBuyerEmail(user.email);
      if (user.phone && !buyerPhone) setBuyerPhone(user.phone);
    }
  }, [user]);

  // Load Nigerian states
  useEffect(() => {
    async function fetchStates() {
      try {
        const data = await apiRequest<State[]>("/geography/states");
        setStates(data || []);
      } catch (err: any) {
        console.error("Failed to fetch states:", err);
      } finally {
        setLoadingGeo(false);
      }
    }
    fetchStates();
  }, []);

  // Load LGAs when state changes
  useEffect(() => {
    if (!shippingStateId) {
      setLgas([]);
      setShippingLgaId("");
      return;
    }

    async function fetchLgas() {
      setLoadingLgas(true);
      try {
        const data = await apiRequest<LGA[]>(`/geography/states/${shippingStateId}/lgas`);
        setLgas(data || []);
      } catch (err: any) {
        console.error("Failed to fetch LGAs:", err);
      } finally {
        setLoadingLgas(false);
      }
    }
    fetchLgas();
  }, [shippingStateId]);

  const deliveryFeeKobo = deliveryType === "pickup" ? 0 : 150000;
  const totalKobo = subtotalKobo + deliveryFeeKobo;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (checkoutStatus && !checkoutStatus.available) {
      setError(checkoutStatus.message || "Payment processing is currently undergoing scheduled maintenance.");
      return;
    }

    if (items.length === 0) {
      setError("Your cart is empty. Add products to your cart before proceeding.");
      return;
    }

    if (!buyerName.trim() || !buyerEmail.trim() || !buyerPhone.trim()) {
      setError("Please provide your full name, email address, and phone number.");
      return;
    }

    if (deliveryType === "delivery") {
      if (!shippingAddress.trim() || !shippingStateId || !shippingLgaId) {
        setError("Please provide complete delivery address including Nigerian State and LGA.");
        return;
      }
    }

    setSubmitting(true);

    try {
      // Generate client idempotency key to prevent double charging on network retries
      const idempotencyKey = `ord_${Date.now()}_${Math.random().toString(36).substring(2, 9)}`;
      const callbackURL = typeof window !== "undefined" ? `${window.location.origin}/marketplace/checkout/success` : "";

      const payload = {
        items: items.map((item) => ({
          product_id: item.productId,
          quantity: item.quantity,
        })),
        buyer_name: buyerName.trim(),
        buyer_email: buyerEmail.trim(),
        buyer_phone: buyerPhone.trim(),
        delivery_type: deliveryType,
        shipping_address: deliveryType === "delivery" ? shippingAddress.trim() : "Vendor Branch Pickup",
        shipping_state_id: deliveryType === "delivery" ? shippingStateId : null,
        shipping_lga_id: deliveryType === "delivery" ? shippingLgaId : null,
        notes: notes.trim(),
        idempotency_key: idempotencyKey,
        callback_url: callbackURL,
      };

      const result = await apiRequest<any>("/marketplace/checkout", {
        method: "POST",
        body: JSON.stringify(payload),
      });

      // Clear the local shopping cart
      clearCart();

      if (result.checkout_url) {
        window.location.href = result.checkout_url;
      } else if (result.payment?.payment_reference) {
        router.push(`/marketplace/checkout/success?reference=${result.payment.payment_reference}`);
      } else if (result.order?.order_number) {
        router.push(`/marketplace/checkout/success?reference=${result.order.order_number}`);
      } else {
        router.push("/marketplace/orders");
      }
    } catch (err: any) {
      setError(err.message || "Failed to process checkout. Please try again.");
      setSubmitting(false);
    }
  };

  if (items.length === 0 && !submitting) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-16 text-center space-y-6">
        <div className="w-16 h-16 bg-gray-100 rounded-full flex items-center justify-center mx-auto text-gray-400">
          <ShoppingBag className="w-8 h-8" />
        </div>
        <h1 className="text-2xl font-bold text-gray-900">Your Cart is Empty</h1>
        <p className="text-gray-600 max-w-md mx-auto text-sm">
          You haven&apos;t added any items to your shopping cart yet. Browse verified products from Ratels businesses across Nigeria.
        </p>
        <Link
          href="/marketplace"
          className="inline-flex items-center gap-2 px-6 py-3 bg-emerald-800 text-white font-semibold rounded-xl hover:bg-emerald-700 transition"
        >
          <ArrowLeft className="w-4 h-4" />
          Return to Ratels Marketplace
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Breadcrumb Navigation */}
      <nav className="flex items-center gap-2 text-xs text-gray-500 font-medium">
        <Link href="/marketplace" className="hover:text-emerald-800 transition">
          Ratels Marketplace
        </Link>
        <ChevronRight className="w-3.5 h-3.5" />
        <Link href="/marketplace/cart" className="hover:text-emerald-800 transition">
          Shopping Cart
        </Link>
        <ChevronRight className="w-3.5 h-3.5" />
        <span className="text-gray-900 font-semibold">Checkout</span>
      </nav>

      <div className="border-b border-gray-200 pb-5">
        <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900">
          Ratels Marketplace Checkout
        </h1>
        <p className="text-sm text-gray-600 mt-1">
          Complete your order with verified Nigerian sellers. Multi-item transactions are processed with bank-grade security.
        </p>
      </div>

      {error && <Alert type="error">{error}</Alert>}

      {checkoutStatus && !checkoutStatus.available && (
        <Alert type="warning">
          <strong>Payment Gateway Notice:</strong> {checkoutStatus.message}
        </Alert>
      )}

      <form onSubmit={handleSubmit}>
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          {/* Left Column: Form Details */}
          <div className="lg:col-span-7 space-y-6">
            {/* Contact Details Card */}
            <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4">
              <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
                <span className="w-6 h-6 rounded-full bg-emerald-100 text-emerald-800 text-xs flex items-center justify-center font-bold">
                  1
                </span>
                Customer Information
              </h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="sm:col-span-2">
                  <label className="block text-xs font-semibold text-gray-700 mb-1">
                    Full Name *
                  </label>
                  <input
                    type="text"
                    required
                    value={buyerName}
                    onChange={(e) => setBuyerName(e.target.value)}
                    placeholder="e.g. Chinua Achebe"
                    className="w-full px-3.5 py-2.5 text-sm border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">
                    Email Address *
                  </label>
                  <input
                    type="email"
                    required
                    value={buyerEmail}
                    onChange={(e) => setBuyerEmail(e.target.value)}
                    placeholder="name@example.com"
                    className="w-full px-3.5 py-2.5 text-sm border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">
                    Phone Number *
                  </label>
                  <input
                    type="tel"
                    required
                    value={buyerPhone}
                    onChange={(e) => setBuyerPhone(e.target.value)}
                    placeholder="e.g. 08012345678"
                    className="w-full px-3.5 py-2.5 text-sm border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500"
                  />
                </div>
              </div>
            </div>

            {/* Fulfilment Method Card */}
            <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4">
              <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
                <span className="w-6 h-6 rounded-full bg-emerald-100 text-emerald-800 text-xs flex items-center justify-center font-bold">
                  2
                </span>
                Fulfilment Method
              </h2>

              <div className="grid grid-cols-2 gap-3">
                <button
                  type="button"
                  onClick={() => setDeliveryType("delivery")}
                  className={`p-4 rounded-xl border text-left flex flex-col gap-2 transition ${
                    deliveryType === "delivery"
                      ? "border-emerald-600 bg-emerald-50/50 ring-2 ring-emerald-500"
                      : "border-gray-200 hover:border-gray-300"
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <Truck className={`w-5 h-5 ${deliveryType === "delivery" ? "text-emerald-700" : "text-gray-400"}`} />
                    <span className="text-xs font-bold text-emerald-800">₦1,500</span>
                  </div>
                  <div>
                    <div className="text-sm font-bold text-gray-900">Doorstep Delivery</div>
                    <div className="text-xs text-gray-500">Delivered directly to your home or office address</div>
                  </div>
                </button>

                <button
                  type="button"
                  onClick={() => setDeliveryType("pickup")}
                  className={`p-4 rounded-xl border text-left flex flex-col gap-2 transition ${
                    deliveryType === "pickup"
                      ? "border-emerald-600 bg-emerald-50/50 ring-2 ring-emerald-500"
                      : "border-gray-200 hover:border-gray-300"
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <Building2 className={`w-5 h-5 ${deliveryType === "pickup" ? "text-emerald-700" : "text-gray-400"}`} />
                    <span className="text-xs font-bold text-emerald-800">Free (₦0)</span>
                  </div>
                  <div>
                    <div className="text-sm font-bold text-gray-900">Store / Branch Pickup</div>
                    <div className="text-xs text-gray-500">Pick up from the vendor&apos;s physical branch location</div>
                  </div>
                </button>
              </div>

              {deliveryType === "delivery" ? (
                <div className="space-y-4 pt-3 border-t border-gray-100">
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div>
                      <label className="block text-xs font-semibold text-gray-700 mb-1">
                        State *
                      </label>
                      <select
                        required
                        value={shippingStateId}
                        onChange={(e) => setShippingStateId(e.target.value)}
                        className="w-full px-3.5 py-2.5 text-sm border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500"
                      >
                        <option value="">Select Delivery State...</option>
                        {states.map((st) => (
                          <option key={st.id} value={st.id}>
                            {st.name} {st.is_fct ? "(FCT)" : ""}
                          </option>
                        ))}
                      </select>
                    </div>

                    <div>
                      <label className="block text-xs font-semibold text-gray-700 mb-1">
                        Local Government Area (LGA) *
                      </label>
                      <select
                        required
                        disabled={!shippingStateId || loadingLgas}
                        value={shippingLgaId}
                        onChange={(e) => setShippingLgaId(e.target.value)}
                        className="w-full px-3.5 py-2.5 text-sm border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500 disabled:bg-gray-100 disabled:text-gray-400"
                      >
                        <option value="">
                          {loadingLgas
                            ? "Loading LGAs..."
                            : !shippingStateId
                            ? "Select a State first"
                            : "Select LGA..."}
                        </option>
                        {lgas.map((lga) => (
                          <option key={lga.id} value={lga.id}>
                            {lga.name}
                          </option>
                        ))}
                      </select>
                    </div>
                  </div>

                  <div>
                    <label className="block text-xs font-semibold text-gray-700 mb-1">
                      Street Address & Landmark *
                    </label>
                    <textarea
                      required
                      rows={2}
                      value={shippingAddress}
                      onChange={(e) => setShippingAddress(e.target.value)}
                      placeholder="e.g. Flat 4B, Plot 120 Ahmadu Bello Way, Near General Post Office"
                      className="w-full px-3.5 py-2.5 text-sm border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500"
                    />
                  </div>
                </div>
              ) : (
                <div className="p-4 bg-emerald-50/70 rounded-xl border border-emerald-200 text-xs text-emerald-900 space-y-1">
                  <div className="font-bold flex items-center gap-1.5">
                    <CheckCircle2 className="w-4 h-4 text-emerald-700" />
                    Pickup Instructions
                  </div>
                  <p>
                    Your order will be prepared at the vendor&apos;s registered store branch. You will receive an SMS and email notification with branch details and pickup code once it is marked Ready for Pickup.
                  </p>
                </div>
              )}

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Special Instructions / Delivery Notes (Optional)
                </label>
                <input
                  type="text"
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder="e.g. Call before delivery, gate code 1234"
                  className="w-full px-3.5 py-2.5 text-sm border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500"
                />
              </div>
            </div>
          </div>

          {/* Right Column: Order Summary */}
          <div className="lg:col-span-5 space-y-6">
            <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-5 sticky top-6">
              <h2 className="text-base font-bold text-gray-900 flex items-center justify-between">
                <span>Order Summary</span>
                <span className="text-xs font-normal text-gray-500">
                  {items.reduce((s, i) => s + i.quantity, 0)} item(s)
                </span>
              </h2>

              {/* Items List */}
              <div className="divide-y divide-gray-100 max-h-60 overflow-y-auto pr-1">
                {items.map((item) => (
                  <div key={item.productId} className="py-3 flex items-center justify-between text-xs">
                    <div className="space-y-0.5 max-w-[70%]">
                      <div className="font-bold text-gray-900 truncate">{item.title}</div>
                      <div className="text-gray-500">
                        Qty: {item.quantity} × ₦{(item.priceKobo / 100).toLocaleString()}
                      </div>
                    </div>
                    <div className="font-bold text-gray-900">
                      ₦{((item.priceKobo * item.quantity) / 100).toLocaleString()}
                    </div>
                  </div>
                ))}
              </div>

              {/* Price Breakdown */}
              <div className="space-y-2.5 pt-4 border-t border-gray-100 text-xs">
                <div className="flex justify-between text-gray-600">
                  <span>Subtotal</span>
                  <span className="font-medium text-gray-900">
                    ₦{(subtotalKobo / 100).toLocaleString()}
                  </span>
                </div>
                <div className="flex justify-between text-gray-600">
                  <span>Fulfilment Fee</span>
                  <span className="font-medium text-gray-900">
                    {deliveryFeeKobo === 0 ? "Free" : `₦${(deliveryFeeKobo / 100).toLocaleString()}`}
                  </span>
                </div>
                <div className="flex justify-between text-sm font-extrabold text-gray-900 pt-3 border-t border-gray-200">
                  <span>Total Amount</span>
                  <span className="text-emerald-800 text-base">
                    ₦{(totalKobo / 100).toLocaleString()}
                  </span>
                </div>
              </div>

              {/* Submit Button */}
              <button
                type="submit"
                disabled={submitting || (checkoutStatus !== null && !checkoutStatus.available)}
                className="w-full py-3.5 px-4 bg-emerald-800 hover:bg-emerald-700 text-white font-bold rounded-xl transition shadow-sm disabled:opacity-50 flex items-center justify-center gap-2"
              >
                {submitting ? (
                  <>
                    <Spinner size="sm" />
                    <span>Processing Order...</span>
                  </>
                ) : checkoutStatus !== null && !checkoutStatus.available ? (
                  <>
                    <AlertCircle className="w-4 h-4" />
                    <span>Checkout Temporarily Unavailable</span>
                  </>
                ) : (
                  <>
                    <ShieldCheck className="w-4 h-4" />
                    <span>Pay ₦{(totalKobo / 100).toLocaleString()} Securely</span>
                  </>
                )}
              </button>

              <div className="flex items-center justify-center gap-2 text-[11px] text-gray-500 pt-2">
                <ShieldCheck className="w-3.5 h-3.5 text-emerald-600" />
                <span>Protected by Ratels Buyer Assurance & Paystack Gateway</span>
              </div>
            </div>
          </div>
        </div>
      </form>
    </div>
  );
}
