"use client";

import React from "react";
import Link from "next/link";
import {
  ShoppingBag,
  Trash2,
  Plus,
  Minus,
  ArrowRight,
  ArrowLeft,
  ShieldCheck,
  Truck,
  Store,
} from "lucide-react";
import { useCart } from "@/lib/cart-context";
import { useAuth } from "@/lib/auth-context";
import { resolveImageUrl } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";

export default function CartPage() {
  const { user, isLoading: authLoading } = useAuth();
  const { items, updateQuantity, removeFromCart, clearCart, subtotalKobo, totalItems } = useCart();

  const deliveryFeeKobo = items.length > 0 ? 150000 : 0; // ₦1,500 flat standard
  const totalKobo = subtotalKobo + deliveryFeeKobo;

  const formatNaira = (kobo: number) => {
    const naira = kobo / 100;
    return `₦${naira.toLocaleString("en-NG", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  };

  if (authLoading) {
    return (
      <div className="min-h-screen bg-gray-50 py-16 flex items-center justify-center">
        <Spinner text="Loading cart..." />
      </div>
    );
  }

  if (!user) {
    return (
      <div className="min-h-screen bg-gray-50 py-16">
        <div className="max-w-md mx-auto px-4 text-center space-y-4">
          <div className="w-16 h-16 rounded-full bg-emerald-50 text-emerald-800 flex items-center justify-center mx-auto">
            <ShoppingBag className="w-8 h-8" />
          </div>
          <h2 className="text-xl font-bold text-gray-900">Sign In Required</h2>
          <p className="text-xs text-gray-500 leading-relaxed">
            Please sign in to your Ratels account to access your shopping cart, manage items, and proceed to checkout.
          </p>
          <div className="pt-2">
            <Link
              href="/login?redirect=/marketplace/cart"
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl text-xs font-bold bg-emerald-900 hover:bg-emerald-950 text-white transition shadow-sm"
            >
              Sign In to View Cart
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 py-10">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 space-y-8">
        {/* Header */}
        <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
          <div>
            <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 flex items-center gap-2.5">
              <ShoppingBag className="w-7 h-7 text-emerald-800" />
              Shopping Cart
            </h1>
            <p className="text-xs text-gray-500 mt-1">
              Review selected items from Ratels Marketplace before proceeding to checkout.
            </p>
          </div>

          {items.length > 0 && (
            <button
              type="button"
              onClick={clearCart}
              className="text-xs font-semibold text-rose-700 hover:text-rose-900 flex items-center gap-1.5 transition"
            >
              <Trash2 className="w-3.5 h-3.5" />
              Clear Entire Cart
            </button>
          )}
        </div>

        {items.length === 0 ? (
          /* Empty Cart State */
          <div className="bg-white rounded-3xl border border-gray-200 p-12 text-center space-y-4 max-w-lg mx-auto shadow-sm">
            <div className="w-16 h-16 rounded-full bg-emerald-50 text-emerald-800 flex items-center justify-center mx-auto">
              <ShoppingBag className="w-8 h-8" />
            </div>
            <h2 className="text-lg font-bold text-gray-900">Your Cart is Empty</h2>
            <p className="text-xs text-gray-500 leading-relaxed">
              Explore the Ratels Marketplace for operational gear, sanitation tools, merchandise, and grassroots supplies.
            </p>
            <div className="pt-2">
              <Link
                href="/marketplace"
                className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl text-xs font-bold bg-emerald-900 hover:bg-emerald-950 text-white transition shadow-sm"
              >
                <ArrowLeft className="w-4 h-4" />
                Browse Ratels Marketplace
              </Link>
            </div>
          </div>
        ) : (
          /* Cart Grid */
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
            {/* Items List */}
            <div className="lg:col-span-2 space-y-4">
              <div className="bg-white rounded-3xl border border-gray-200 overflow-hidden shadow-sm divide-y divide-gray-100">
                {items.map((item) => (
                  <div key={item.productId} className="p-4 sm:p-6 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
                    <div className="flex items-center gap-4">
                      {/* Image */}
                      <div className="w-16 h-16 rounded-xl bg-gray-100 border border-gray-200 overflow-hidden flex items-center justify-center shrink-0">
                        {item.imageUrl ? (
                          <img
                            src={resolveImageUrl(item.imageUrl)}
                            alt={item.title}
                            className="w-full h-full object-cover"
                            onError={(e) => {
                              (e.target as HTMLElement).style.display = "none";
                            }}
                          />
                        ) : (
                          <ShoppingBag className="w-6 h-6 text-gray-400" />
                        )}
                      </div>

                      {/* Title & Price */}
                      <div className="space-y-1">
                        {item.businessName && (
                          <div className="text-[11px] font-semibold text-emerald-800 flex items-center gap-1">
                            <Store className="w-3 h-3" />
                            <span>{item.businessName}</span>
                          </div>
                        )}
                        <Link
                          href={`/marketplace/products/${item.productId}`}
                          className="font-bold text-gray-900 text-sm hover:text-emerald-900 transition line-clamp-1"
                        >
                          {item.title}
                        </Link>
                        <div className="text-xs text-gray-500">Unit Price: {formatNaira(item.priceKobo)}</div>
                      </div>
                    </div>

                    {/* Quantity & Actions */}
                    <div className="flex items-center justify-between sm:justify-end gap-6 w-full sm:w-auto pt-2 sm:pt-0 border-t sm:border-0 border-gray-100">
                      {/* Quantity Controller */}
                      <div className="flex items-center border border-gray-200 rounded-lg bg-gray-50">
                        <button
                          type="button"
                          onClick={() => updateQuantity(item.productId, item.quantity - 1)}
                          className="p-1.5 text-gray-600 hover:bg-gray-200 rounded-l-lg transition"
                        >
                          <Minus className="w-3 h-3" />
                        </button>
                        <span className="w-8 text-center text-xs font-bold text-gray-900">{item.quantity}</span>
                        <button
                          type="button"
                          onClick={() => updateQuantity(item.productId, item.quantity + 1)}
                          className="p-1.5 text-gray-600 hover:bg-gray-200 rounded-r-lg transition"
                        >
                          <Plus className="w-3 h-3" />
                        </button>
                      </div>

                      {/* Subtotal */}
                      <div className="text-right">
                        <div className="text-sm font-black text-gray-900">
                          {formatNaira(item.priceKobo * item.quantity)}
                        </div>
                      </div>

                      {/* Delete */}
                      <button
                        type="button"
                        onClick={() => removeFromCart(item.productId)}
                        title="Remove item"
                        className="p-1.5 text-gray-400 hover:text-rose-600 rounded-md transition"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>

              <div className="flex justify-between items-center text-xs text-gray-500 px-2">
                <Link href="/marketplace" className="inline-flex items-center gap-1 font-bold text-emerald-900 hover:underline">
                  <ArrowLeft className="w-3.5 h-3.5" />
                  Continue Shopping
                </Link>
                <span>Total Items: {totalItems}</span>
              </div>
            </div>

            {/* Order Summary Box */}
            <div className="space-y-4">
              <div className="bg-white rounded-3xl border border-gray-200 p-6 shadow-sm space-y-4">
                <h2 className="font-bold text-gray-900 text-base">Order Summary</h2>

                <div className="space-y-2.5 text-xs text-gray-600 border-b border-gray-100 pb-4">
                  <div className="flex justify-between">
                    <span>Subtotal ({totalItems} items)</span>
                    <span className="font-bold text-gray-900">{formatNaira(subtotalKobo)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="flex items-center gap-1">
                      <Truck className="w-3.5 h-3.5 text-emerald-700" />
                      Standard Courier Delivery
                    </span>
                    <span className="font-bold text-gray-900">{formatNaira(deliveryFeeKobo)}</span>
                  </div>
                  <p className="text-[10px] text-gray-400">
                    Pickup options with ₦0 delivery fee can be chosen during checkout.
                  </p>
                </div>

                <div className="flex justify-between items-center">
                  <span className="text-sm font-bold text-gray-900">Total Due</span>
                  <span className="text-xl font-black text-emerald-900">{formatNaira(totalKobo)}</span>
                </div>

                <Link
                  href="/marketplace/checkout"
                  className="w-full py-3.5 px-4 rounded-xl font-bold text-sm bg-emerald-900 hover:bg-emerald-950 text-white transition flex items-center justify-center gap-2 shadow-sm text-center"
                >
                  Proceed to Checkout
                  <ArrowRight className="w-4 h-4" />
                </Link>

                <div className="pt-2 space-y-2 text-[11px] text-gray-500">
                  <div className="flex items-center gap-1.5">
                    <ShieldCheck className="w-4 h-4 text-emerald-700 shrink-0" />
                    <span>Secure Checkout • Integer Kobo Integrity</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
