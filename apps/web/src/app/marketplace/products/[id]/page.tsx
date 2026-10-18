"use client";

import React, { useEffect, useState, use } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Store,
  MapPin,
  ShieldCheck,
  Truck,
  RotateCcw,
  ShoppingBag,
  Plus,
  Minus,
  CheckCircle2,
  Package,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { MarketplaceProduct } from "@/lib/types";
import { useCart } from "@/lib/cart-context";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

export default function ProductDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const router = useRouter();
  const { id } = use(params);
  const { addToCart } = useCart();

  const [product, setProduct] = useState<MarketplaceProduct | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [quantity, setQuantity] = useState(1);
  const [activeImageIndex, setActiveImageIndex] = useState(0);
  const [addedToast, setAddedToast] = useState(false);

  useEffect(() => {
    async function fetchProduct() {
      setLoading(true);
      setError(null);
      try {
        const data = await apiRequest<MarketplaceProduct>(`/marketplace/products/${id}`);
        setProduct(data);
      } catch (err: any) {
        setError(err.message || "Failed to load product details");
      } finally {
        setLoading(false);
      }
    }
    fetchProduct();
  }, [id]);

  if (loading) return <Spinner text="Loading product details..." />;
  if (error || !product) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-16 text-center space-y-4">
        <Alert type="error">{error || "Product not found"}</Alert>
        <Link
          href="/marketplace"
          className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-900 text-white rounded-xl text-xs font-bold"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Ratels Marketplace
        </Link>
      </div>
    );
  }

  const inStock = product.stock_quantity > 0;
  const images = Array.isArray(product.images) ? product.images : [];
  const currentImage = images.length > 0 ? images[activeImageIndex] : "";

  const formatNaira = (kobo: number) => {
    const naira = kobo / 100;
    return `₦${naira.toLocaleString("en-NG", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  };

  const handleAddToCart = () => {
    addToCart({
      productId: product.id,
      title: product.title,
      priceKobo: product.price_kobo,
      quantity,
      sku: product.sku,
      imageUrl: images[0] || "",
      businessName: product.business_name,
    });
    setAddedToast(true);
    setTimeout(() => setAddedToast(false), 3000);
  };

  const handleBuyNow = () => {
    handleAddToCart();
    router.push("/marketplace/checkout");
  };

  return (
    <div className="min-h-screen bg-gray-50 py-10">
      {addedToast && (
        <div className="fixed bottom-6 right-6 z-50 bg-emerald-900 text-white px-5 py-3 rounded-xl shadow-lg flex items-center gap-3 animate-fade-in border border-emerald-700">
          <CheckCircle2 className="w-5 h-5 text-emerald-400" />
          <div className="text-sm font-medium">Added to cart successfully</div>
          <Link
            href="/marketplace/cart"
            className="ml-2 text-xs font-bold bg-emerald-800 hover:bg-emerald-700 px-2.5 py-1 rounded-md"
          >
            Go to Cart
          </Link>
        </div>
      )}

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 space-y-6">
        {/* Breadcrumb Navigation */}
        <div className="flex items-center gap-2 text-xs text-gray-500 font-medium">
          <Link href="/marketplace" className="hover:text-emerald-900 transition">
            Ratels Marketplace
          </Link>
          <span>/</span>
          <span className="capitalize">{product.category.replace(/_/g, " ")}</span>
          <span>/</span>
          <span className="text-gray-900 font-bold truncate max-w-xs">{product.title}</span>
        </div>

        {/* Product Container */}
        <div className="bg-white rounded-3xl border border-gray-200 overflow-hidden shadow-sm grid grid-cols-1 lg:grid-cols-2 gap-8 p-6 sm:p-10">
          {/* Images Section */}
          <div className="space-y-4">
            <div className="aspect-square bg-gray-100 rounded-2xl overflow-hidden relative flex items-center justify-center border border-gray-100">
              {currentImage ? (
                <img
                  src={currentImage}
                  alt={product.title}
                  className="w-full h-full object-contain p-4"
                />
              ) : (
                <div className="flex flex-col items-center gap-3 text-gray-400">
                  <Package className="w-16 h-16" />
                  <span className="text-xs font-bold uppercase tracking-wider">Ratels Verified Product</span>
                </div>
              )}
            </div>

            {/* Thumbnails */}
            {images.length > 1 && (
              <div className="flex gap-3 overflow-x-auto pb-1">
                {images.map((img: string, idx: number) => (
                  <button
                    key={idx}
                    type="button"
                    onClick={() => setActiveImageIndex(idx)}
                    className={`w-16 h-16 rounded-xl border-2 overflow-hidden bg-gray-50 transition shrink-0 ${
                      activeImageIndex === idx ? "border-emerald-700" : "border-gray-200 hover:border-gray-400"
                    }`}
                  >
                    <img src={img} alt={`Thumbnail ${idx + 1}`} className="w-full h-full object-cover" />
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Details & Purchasing Section */}
          <div className="flex flex-col justify-between space-y-6">
            <div className="space-y-4">
              {/* Category & Stock */}
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="px-3 py-1 bg-emerald-50 text-emerald-900 text-xs font-bold uppercase tracking-wider rounded-full">
                  {product.category.replace(/_/g, " ")}
                </span>
                <div>
                  {inStock ? (
                    <Badge variant="success" size="sm">
                      {product.stock_quantity <= product.low_stock_threshold
                        ? `Only ${product.stock_quantity} Left In Stock`
                        : "In Stock & Ready for Dispatch"}
                    </Badge>
                  ) : (
                    <Badge variant="danger" size="sm">Currently Out of Stock</Badge>
                  )}
                </div>
              </div>

              {/* Title & SKU */}
              <div>
                <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 leading-tight">
                  {product.title}
                </h1>
                <p className="text-xs text-gray-400 font-mono mt-1">SKU: {product.sku}</p>
              </div>

              {/* Price */}
              <div className="pt-2">
                <div className="text-3xl font-black text-gray-900">
                  {formatNaira(product.price_kobo)}
                </div>
                {product.compare_at_price_kobo && product.compare_at_price_kobo > product.price_kobo && (
                  <div className="text-sm text-gray-400 line-through mt-0.5">
                    Original Price: {formatNaira(product.compare_at_price_kobo)}
                  </div>
                )}
              </div>

              {/* Seller & Branch Information */}
              <div className="bg-gray-50 rounded-2xl p-4 border border-gray-100 space-y-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Store className="w-4 h-4 text-emerald-800" />
                    <span className="text-sm font-bold text-gray-900">{product.business_name || "Community Vendor"}</span>
                  </div>
                  {product.business_ownership_type && (
                    <span className="text-[10px] font-semibold uppercase tracking-wider text-emerald-800 bg-white px-2 py-0.5 rounded border border-gray-200">
                      {product.business_ownership_type.replace(/_/g, " ")}
                    </span>
                  )}
                </div>

                {product.branch_name && (
                  <div className="flex items-center gap-1.5 text-xs text-gray-600">
                    <MapPin className="w-3.5 h-3.5 text-emerald-700 shrink-0" />
                    <span>Dispatched from {product.branch_name} {product.branch_state_name ? `(${product.branch_state_name})` : ""}</span>
                  </div>
                )}
              </div>

              {/* Description */}
              {product.description && (
                <div className="space-y-1">
                  <h3 className="text-xs font-bold uppercase tracking-wider text-gray-500">About this product</h3>
                  <p className="text-sm text-gray-700 leading-relaxed whitespace-pre-line">
                    {product.description}
                  </p>
                </div>
              )}

              {/* Delivery and Returns Policies */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2 text-xs text-gray-600">
                <div className="flex items-start gap-2 bg-gray-50 p-3 rounded-xl border border-gray-100">
                  <Truck className="w-4 h-4 text-emerald-800 shrink-0 mt-0.5" />
                  <div>
                    <span className="font-bold text-gray-900 block">Nationwide Delivery</span>
                    <span>{product.delivery_restrictions || "Standard courier dispatch across all 36 States + FCT."}</span>
                  </div>
                </div>

                <div className="flex items-start gap-2 bg-gray-50 p-3 rounded-xl border border-gray-100">
                  <RotateCcw className="w-4 h-4 text-emerald-800 shrink-0 mt-0.5" />
                  <div>
                    <span className="font-bold text-gray-900 block">Return & Dispute Policy</span>
                    <span>{product.return_policy || "Eligible for resolution via Ratels customer dispute workflow."}</span>
                  </div>
                </div>
              </div>
            </div>

            {/* Quantity Selector & CTAs */}
            <div className="space-y-4 pt-4 border-t border-gray-200">
              <div className="flex items-center gap-4">
                <span className="text-xs font-bold uppercase tracking-wider text-gray-600">Quantity</span>
                <div className="flex items-center border border-gray-300 rounded-xl bg-white shadow-sm overflow-hidden">
                  <button
                    type="button"
                    onClick={() => setQuantity(Math.max(1, quantity - 1))}
                    disabled={quantity <= 1 || !inStock}
                    className="p-2 text-gray-600 hover:bg-gray-100 disabled:opacity-40"
                  >
                    <Minus className="w-3.5 h-3.5" />
                  </button>
                  <span className="w-10 text-center text-sm font-bold text-gray-900">{quantity}</span>
                  <button
                    type="button"
                    onClick={() => setQuantity(Math.min(product.stock_quantity, quantity + 1))}
                    disabled={quantity >= product.stock_quantity || !inStock}
                    className="p-2 text-gray-600 hover:bg-gray-100 disabled:opacity-40"
                  >
                    <Plus className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <button
                  type="button"
                  onClick={handleAddToCart}
                  disabled={!inStock}
                  className="w-full py-3.5 px-6 rounded-xl font-bold text-sm bg-emerald-900 hover:bg-emerald-950 text-white transition flex items-center justify-center gap-2 shadow-sm disabled:bg-gray-200 disabled:text-gray-400 disabled:cursor-not-allowed"
                >
                  <ShoppingBag className="w-4 h-4" />
                  Add to Cart
                </button>

                <button
                  type="button"
                  onClick={handleBuyNow}
                  disabled={!inStock}
                  className="w-full py-3.5 px-6 rounded-xl font-bold text-sm bg-emerald-800 hover:bg-emerald-700 text-white transition flex items-center justify-center gap-2 shadow-sm disabled:bg-gray-200 disabled:text-gray-400 disabled:cursor-not-allowed"
                >
                  Buy Now
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
