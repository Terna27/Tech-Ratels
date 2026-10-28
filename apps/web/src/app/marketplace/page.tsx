"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Search,
  Store,
  ShoppingBag,
  Filter,
  CheckCircle2,
  Truck,
  ShieldCheck,
  ChevronRight,
  Package,
  Plus,
  ArrowRight,
  MapPin,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { MarketplaceProduct, parseProductImages } from "@/lib/types";
import { useCart } from "@/lib/cart-context";
import { useAuth } from "@/lib/auth-context";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

const CATEGORIES = [
  { id: "", label: "All Items" },
  { id: "sanitation_supplies", label: "Sanitation Gear" },
  { id: "safety_gear", label: "Safety & PPE" },
  { id: "tools_hardware", label: "Tools & Equipment" },
  { id: "apparel_merch", label: "Apparel & Merch" },
  { id: "agricultural_produce", label: "Agro & Food" },
  { id: "health_firstaid", label: "Health & First Aid" },
  { id: "books_education", label: "Education" },
  { id: "general_goods", label: "General Goods" },
];

export default function MarketplaceStorefrontPage() {
  const router = useRouter();
  const { user } = useAuth();
  const { addToCart } = useCart();

  const [products, setProducts] = useState<MarketplaceProduct[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [selectedCategory, setSelectedCategory] = useState<string>("");
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [addedPopup, setAddedPopup] = useState<string | null>(null);

  useEffect(() => {
    fetchProducts();
  }, [selectedCategory]);

  const fetchProducts = async (query = searchQuery) => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (selectedCategory) params.set("category", selectedCategory);
      if (query) params.set("q", query);

      const res = await apiRequest<MarketplaceProduct[]>(`/marketplace/products?${params.toString()}`);
      setProducts(Array.isArray(res) ? res : []);
    } catch (err: any) {
      setError(err.message || "Failed to load marketplace products");
    } finally {
      setLoading(false);
    }
  };

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    fetchProducts(searchQuery);
  };

  const handleAddToCart = (p: MarketplaceProduct, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();

    if (!user) {
      router.push(`/login?redirect=${encodeURIComponent("/marketplace")}`);
      return;
    }

    const pImages = parseProductImages(p.images);
    const img = pImages.length > 0 ? pImages[0] : "";

    addToCart({
      productId: p.id,
      title: p.title,
      priceKobo: p.price_kobo,
      quantity: 1,
      sku: p.sku,
      imageUrl: img,
      businessName: p.business_name,
    });

    setAddedPopup(p.title);
    setTimeout(() => setAddedPopup(null), 3000);
  };

  const formatNaira = (kobo: number) => {
    const naira = kobo / 100;
    return `₦${naira.toLocaleString("en-NG", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  };

  return (
    <div className="min-h-screen bg-gray-50 pb-16 overflow-x-hidden">
      {/* Toast Notification when item added */}
      {addedPopup && (
        <div className="fixed bottom-6 right-6 z-50 bg-emerald-900 text-white px-5 py-3 rounded-xl shadow-lg flex items-center gap-3 animate-fade-in border border-emerald-700">
          <CheckCircle2 className="w-5 h-5 text-emerald-400" />
          <div className="text-sm font-medium">
            Added <span className="font-bold underline">{addedPopup}</span> to cart
          </div>
          <Link
            href="/marketplace/cart"
            className="ml-2 text-xs font-bold bg-emerald-800 hover:bg-emerald-700 px-2.5 py-1 rounded-md"
          >
            View Cart
          </Link>
        </div>
      )}

      {/* Hero Header */}
      <div className="bg-emerald-950 text-white border-b border-emerald-900">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 sm:py-14">
          <div className="flex flex-col lg:flex-row justify-between items-start lg:items-center gap-6">
            <div className="space-y-3 max-w-2xl">
              <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-bold tracking-wide uppercase bg-emerald-900/80 text-emerald-300 border border-emerald-800">
                <Store className="w-3.5 h-3.5" />
                Ratels Worldwide Commerce
              </div>
              <Link href="/marketplace/subscriptions" className="text-emerald-100 underline">Store plans &amp; pricing</Link>
              <h1 className="text-3xl sm:text-4xl lg:text-5xl font-black tracking-tight text-white">
                Ratels Marketplace
              </h1>
              <p className="text-emerald-200 text-sm sm:text-base leading-relaxed">
                Empowering Nigerian grassroots enterprise. Discover verified community sanitation supplies, personal protective gear, industrial tools, agricultural produce, and movement merchandise.
              </p>
            </div>

            {/* Quick Actions */}
            <div className="flex flex-wrap items-center gap-3 w-full md:w-auto">
              <Link
                href="/marketplace/seller"
                className="flex-1 md:flex-none inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl text-sm font-bold bg-white text-emerald-950 hover:bg-emerald-50 transition shadow-sm"
              >
                <Store className="w-4 h-4 text-emerald-800" />
                Seller Portal
              </Link>
              {user && (
                <Link
                  href="/marketplace/orders"
                  className="flex-1 md:flex-none inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl text-sm font-bold bg-emerald-900 text-emerald-100 hover:bg-emerald-800 border border-emerald-700 transition"
                >
                  <Package className="w-4 h-4" />
                  My Orders
                </Link>
              )}
            </div>
          </div>

          {/* Value Propositions Strip */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mt-8 pt-8 border-t border-emerald-900/80 text-xs text-emerald-200">
            <div className="flex items-center gap-2.5">
              <ShieldCheck className="w-5 h-5 text-emerald-400 shrink-0" />
              <span>Verified member vendors & approved community partners</span>
            </div>
            <div className="flex items-center gap-2.5">
              <Truck className="w-5 h-5 text-emerald-400 shrink-0" />
              <span>Nationwide delivery with dispatch tracking references</span>
            </div>
            <div className="flex items-center gap-2.5">
              <CheckCircle2 className="w-5 h-5 text-emerald-400 shrink-0" />
              <span>Server-verified payments via Paystack with instant receipts</span>
            </div>
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
        {/* Search & Category Filter Bar */}
        <div className="space-y-4">
          <form onSubmit={handleSearchSubmit} className="flex gap-2">
            <div className="relative flex-1">
              <Search className="w-5 h-5 absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search products by title, SKU, or description..."
                className="w-full pl-11 pr-4 py-3 rounded-xl border border-gray-300 bg-white text-sm focus:outline-none focus:ring-2 focus:ring-emerald-700 focus:border-transparent shadow-sm"
              />
            </div>
            <button
              type="submit"
              className="px-6 py-3 bg-emerald-900 hover:bg-emerald-950 text-white font-bold text-sm rounded-xl transition shadow-sm"
            >
              Search
            </button>
          </form>

          {/* Category Chips Scroll */}
          <div className="w-full max-w-full overflow-x-auto pb-2 scrollbar-none min-w-0">
            <div className="flex items-center gap-2 min-w-max">
              {CATEGORIES.map((cat) => (
                <button
                  key={cat.id}
                  onClick={() => setSelectedCategory(cat.id)}
                  className={`px-4 py-2 rounded-full text-xs font-semibold whitespace-nowrap transition border ${
                    selectedCategory === cat.id
                      ? "bg-emerald-900 text-white border-emerald-900 shadow-sm"
                      : "bg-white text-gray-700 border-gray-200 hover:border-gray-300 hover:bg-gray-50"
                  }`}
                >
                  {cat.label}
                </button>
              ))}
            </div>
          </div>
        </div>

        {error && <Alert type="error">{error}</Alert>}

        {/* Product Catalog Grid */}
        {loading ? (
          <Spinner text="Loading Ratels Marketplace catalogue..." />
        ) : products.length === 0 ? (
          <div className="bg-white rounded-2xl border border-gray-200 p-12 text-center space-y-4">
            <div className="w-16 h-16 rounded-full bg-emerald-50 text-emerald-800 flex items-center justify-center mx-auto">
              <Package className="w-8 h-8" />
            </div>
            <h3 className="text-lg font-bold text-gray-900">No Products Available</h3>
            <p className="text-xs text-gray-500 max-w-md mx-auto">
              No products found matching your current filter. Check back soon or register your business to sell on Ratels Marketplace.
            </p>
            <div className="pt-2">
              <Link
                href="/marketplace/seller"
                className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-bold bg-emerald-900 text-white hover:bg-emerald-950 transition"
              >
                <Plus className="w-4 h-4" />
                List Your Product
              </Link>
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 sm:gap-5">
            {products.map((product) => {
              const inStock = product.stock_quantity > 0;
              const prodImages = parseProductImages(product.images);
              const firstImage = prodImages.length > 0 ? prodImages[0] : "";

              return (
                <Link
                  key={product.id}
                  href={`/marketplace/products/${product.id}`}
                  className="bg-white rounded-xl border border-gray-200 overflow-hidden shadow-xs hover:shadow-md hover:border-emerald-600 transition group flex flex-col justify-between"
                >
                  <div>
                    {/* Card Header: Category & Stock Status */}
                    <div className="px-3 pt-2.5 pb-2 flex items-center justify-between gap-1.5 bg-white">
                      <span className="bg-emerald-50 text-emerald-900 border border-emerald-100 text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded-md truncate max-w-[60%]">
                        {product.category.replace(/_/g, " ")}
                      </span>
                      <div className="shrink-0">
                        {inStock ? (
                          <Badge variant="success" size="sm">
                            {product.stock_quantity <= product.low_stock_threshold
                              ? `${product.stock_quantity} left`
                              : "In Stock"}
                          </Badge>
                        ) : (
                          <Badge variant="danger" size="sm">Out of Stock</Badge>
                        )}
                      </div>
                    </div>

                    {/* Compact Image Container: Reduced ~70% from ~280px to h-24/h-28 (~80-100px, ~30% linear dimensions) */}
                    <div className="h-24 sm:h-28 w-full bg-gray-50/80 flex items-center justify-center p-2 relative overflow-hidden border-y border-gray-100">
                      {firstImage ? (
                        <img
                          src={firstImage}
                          alt={product.title}
                          className="max-h-full max-w-full object-contain group-hover:scale-105 transition duration-300"
                          onError={(e) => {
                            (e.target as HTMLElement).style.display = "none";
                          }}
                        />
                      ) : (
                        <div className="flex flex-col items-center gap-1 text-gray-400">
                          <Package className="w-7 h-7" />
                          <span className="text-[9px] uppercase font-bold tracking-wider">Ratels Supply</span>
                        </div>
                      )}
                    </div>

                    {/* Content */}
                    <div className="p-3 space-y-1.5">
                      {product.business_name && (
                        <div className="text-[11px] font-semibold text-emerald-800 flex items-center gap-1">
                          <Store className="w-3 h-3 shrink-0" />
                          <span className="truncate">{product.business_name}</span>
                        </div>
                      )}
                      <h3 className="font-bold text-gray-900 text-xs sm:text-sm line-clamp-2 leading-snug group-hover:text-emerald-900 transition min-h-[2.25rem]">
                        {product.title}
                      </h3>
                      {product.branch_state_name && (
                        <div className="text-[10px] text-gray-500 flex items-center gap-1">
                          <MapPin className="w-3 h-3 text-gray-400 shrink-0" />
                          <span className="truncate">{product.branch_state_name}</span>
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Price and Cart Action */}
                  <div className="p-3 pt-2 border-t border-gray-100 flex items-center justify-between gap-2 bg-gray-50/30">
                    <div className="min-w-0">
                      <div className="text-sm sm:text-base font-black text-gray-900 truncate">
                        {formatNaira(product.price_kobo)}
                      </div>
                      {product.compare_at_price_kobo && product.compare_at_price_kobo > product.price_kobo && (
                        <div className="text-[10px] text-gray-400 line-through truncate">
                          {formatNaira(product.compare_at_price_kobo)}
                        </div>
                      )}
                    </div>

                    <button
                      type="button"
                      onClick={(e) => handleAddToCart(product, e)}
                      disabled={!inStock}
                      title={inStock ? (user ? "Add to Cart" : "Sign In to Add to Cart") : "Out of Stock"}
                      className={`p-2 rounded-lg font-bold transition flex items-center justify-center shrink-0 ${
                        inStock
                          ? "bg-emerald-900 hover:bg-emerald-950 text-white shadow-xs"
                          : "bg-gray-100 text-gray-400 cursor-not-allowed"
                      }`}
                    >
                      <ShoppingBag className="w-4 h-4" />
                    </button>
                  </div>
                </Link>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
