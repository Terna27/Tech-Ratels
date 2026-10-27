"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Store,
  Plus,
  Package,
  ShoppingBag,
  Truck,
  CheckCircle2,
  Clock,
  AlertCircle,
  Building2,
  BarChart3,
  MapPin,
  Edit,
  Trash2,
  Send,
  Eye,
  ChevronRight,
  ShieldCheck,
  UploadCloud,
  Loader2,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { State, LGA, parseProductImages, R2_PUBLIC_DOMAIN } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";

export default function SellerPortalPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<"overview" | "products" | "orders" | "branches">("overview");
  const [business, setBusiness] = useState<any>(null);
  const [metrics, setMetrics] = useState<any>(null);
  const [products, setProducts] = useState<any[]>([]);
  const [orders, setOrders] = useState<any[]>([]);
  const [branches, setBranches] = useState<any[]>([]);

  const [loading, setLoading] = useState(true);
  const [hasNoBusiness, setHasNoBusiness] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // States & LGAs for registration & branch forms
  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);

  // Registration Form State
  const [regName, setRegName] = useState("");
  const [regCategory, setRegCategory] = useState("apparel");
  const [regDescription, setRegDescription] = useState("");
  const [regNumber, setRegNumber] = useState("");
  const [regPhone, setRegPhone] = useState("");
  const [regEmail, setRegEmail] = useState("");
  const [regAddress, setRegAddress] = useState("");
  const [regStateId, setRegStateId] = useState("");
  const [regLgaId, setRegLgaId] = useState("");
  const [regLogoUrl, setRegLogoUrl] = useState("");
  const [regSubmitting, setRegSubmitting] = useState(false);

  // Product Form Modal State
  const [showProductModal, setShowProductModal] = useState(false);
  const [editingProductId, setEditingProductId] = useState<string | null>(null);
  const [prodTitle, setProdTitle] = useState("");
  const [prodDescription, setProdDescription] = useState("");
  const [prodCategory, setProdCategory] = useState("apparel");
  const [prodSku, setProdSku] = useState("");
  const [prodPriceNaira, setProdPriceNaira] = useState("");
  const [prodCompareNaira, setProdCompareNaira] = useState("");
  const [prodStock, setProdStock] = useState("10");
  const [prodLowStock, setProdLowStock] = useState("3");
  const [productImages, setProductImages] = useState<{
    id: string;
    url: string;
    objectKey?: string;
    isUploading?: boolean;
    progress?: number;
    error?: string;
  }[]>([]);
  const [imageError, setImageError] = useState<string | null>(null);
  const [isUploadingImage, setIsUploadingImage] = useState(false);
  const [isDragOver, setIsDragOver] = useState(false);
  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const [prodStatus, setProdStatus] = useState("published");
  const [prodSubmitting, setProdSubmitting] = useState(false);

  // Dispatch Modal State
  const [showDispatchModal, setShowDispatchModal] = useState(false);
  const [dispatchOrderId, setDispatchOrderId] = useState<string | null>(null);
  const [dispatchShipmentId, setDispatchShipmentId] = useState<string | null>(null);
  const [sellerShipments, setSellerShipments] = useState<any[]>([]);
  const [carrierName, setCarrierName] = useState("GIG Logistics");
  const [trackingNumber, setTrackingNumber] = useState("");
  const [dispatchNotes, setDispatchNotes] = useState("");
  const [dispatchSubmitting, setDispatchSubmitting] = useState(false);

  // Branch Modal State
  const [showBranchModal, setShowBranchModal] = useState(false);
  const [branchName, setBranchName] = useState("");
  const [branchAddress, setBranchAddress] = useState("");
  const [branchPhone, setBranchPhone] = useState("");
  const [branchStateId, setBranchStateId] = useState("");
  const [branchLgaId, setBranchLgaId] = useState("");
  const [branchManager, setBranchManager] = useState("");
  const [branchSubmitting, setBranchSubmitting] = useState(false);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login?redirect=/marketplace/seller");
      return;
    }

    if (user) {
      loadSellerData();
      fetchStates();
    }
  }, [user, authLoading]);

  // Load LGAs for registration or branch
  useEffect(() => {
    const sId = regStateId || branchStateId;
    if (sId) {
      apiRequest<LGA[]>(`/geography/states/${sId}/lgas`)
        .then((data) => setLgas(data || []))
        .catch(console.error);
    }
  }, [regStateId, branchStateId]);

  const fetchStates = async () => {
    try {
      const data = await apiRequest<State[]>("/geography/states");
      setStates(data || []);
    } catch (_) {}
  };

  const loadSellerData = async () => {
    setLoading(true);
    setError(null);
    try {
      const b = await apiRequest<any>("/marketplace/seller/business");
      setBusiness(b);
      setHasNoBusiness(false);

      if (b && (b.status === "approved" || b.status === "active" || b.approval_status === "approved")) {
        // Load metrics, products, orders, branches, shipments
        const [mRes, pRes, oRes, brRes, shRes] = await Promise.allSettled([
          apiRequest<any>("/marketplace/seller/metrics"),
          apiRequest<any[]>("/marketplace/seller/products"),
          apiRequest<any[]>("/marketplace/seller/orders"),
          apiRequest<any[]>(`/marketplace/businesses/${b.id}/branches`),
          apiRequest<any[]>("/marketplace/seller/shipments"),
        ]);

        if (mRes.status === "fulfilled") setMetrics(mRes.value);
        if (pRes.status === "fulfilled") setProducts(pRes.value || []);
        if (oRes.status === "fulfilled") setOrders(oRes.value || []);
        if (brRes.status === "fulfilled") setBranches(brRes.value || []);
        if (shRes.status === "fulfilled") setSellerShipments(shRes.value || []);
      }
    } catch (err: any) {
      if (err.message && (err.message.includes("404") || err.message.includes("No business profile"))) {
        setHasNoBusiness(true);
      } else {
        setError(err.message || "Failed to load seller profile");
      }
    } finally {
      setLoading(false);
    }
  };

  const handleRegisterBusiness = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setRegSubmitting(true);

    try {
      const payload = {
        name: regName.trim(),
        category: regCategory,
        description: regDescription.trim(),
        registration_number: regNumber.trim() || undefined,
        contact_phone: regPhone.trim(),
        contact_email: regEmail.trim(),
        headquarters_address: regAddress.trim() || undefined,
        headquarters_state_id: regStateId || undefined,
        headquarters_lga_id: regLgaId || undefined,
        logo_url: regLogoUrl.trim() || undefined,
      };

      await apiRequest("/marketplace/seller/business", {
        method: "POST",
        body: JSON.stringify(payload),
      });

      setSuccess("Business registered successfully! Your account is submitted for verification.");
      loadSellerData();
    } catch (err: any) {
      setError(err.message || "Failed to register business");
    } finally {
      setRegSubmitting(false);
    }
  };

  const handleFilesSelected = async (files: FileList | File[]) => {
    setImageError(null);
    const fileArray = Array.from(files);
    if (fileArray.length === 0) return;

    if (productImages.length + fileArray.length > 5) {
      setImageError(`A maximum of 5 images is allowed per product (you have ${productImages.length}, selected ${fileArray.length}).`);
      return;
    }

    const allowedTypes = ["image/jpeg", "image/png", "image/webp"];
    const allowedExtensions = [".jpg", ".jpeg", ".png", ".webp"];

    for (const file of fileArray) {
      const ext = "." + file.name.split(".").pop()?.toLowerCase();
      if (!allowedExtensions.includes(ext) && !allowedTypes.includes(file.type)) {
        setImageError(`"${file.name}" is not a supported format. Please select JPG, PNG, or WebP images.`);
        return;
      }
      if (file.size > 5 * 1024 * 1024) {
        setImageError(`"${file.name}" exceeds the 5 MB limit (${(file.size / (1024 * 1024)).toFixed(1)} MB).`);
        return;
      }
    }

    setIsUploadingImage(true);

    for (const file of fileArray) {
      const tempId = "img-" + Math.random().toString(36).substring(2, 9);
      const previewUrl = URL.createObjectURL(file);

      // Add local preview immediately
      setProductImages((prev) => [
        ...prev,
        {
          id: tempId,
          url: previewUrl,
          isUploading: true,
          progress: 10,
        },
      ]);

      try {
        // First try authenticated direct server upload (verifies magic bytes & dimensions)
        const formData = new FormData();
        formData.append("file", file);

        setProductImages((prev) =>
          prev.map((item) => (item.id === tempId ? { ...item, progress: 40 } : item))
        );

        const uploadRes = await apiRequest("/marketplace/seller/products/images/upload", {
          method: "POST",
          body: formData,
        });

        const finalDirectUrl = uploadRes.public_url || (uploadRes.object_key ? `${R2_PUBLIC_DOMAIN}/${uploadRes.object_key}` : "");

        setProductImages((prev) =>
          prev.map((item) =>
            item.id === tempId
              ? {
                  id: tempId,
                  url: finalDirectUrl,
                  objectKey: uploadRes.object_key,
                  isUploading: false,
                  progress: 100,
                }
              : item
          )
        );
      } catch (directErr: any) {
        // If direct upload failed (e.g. server configured for presigned direct R2 upload), request upload authorization
        try {
          const presignRes = await apiRequest("/marketplace/seller/products/images/presign", {
            method: "POST",
            body: JSON.stringify({
              filename: file.name,
              content_type: file.type || "image/jpeg",
              file_size: file.size,
            }),
          });

          // Upload directly to Cloudflare R2
          const putRes = await fetch(presignRes.upload_url, {
            method: "PUT",
            headers: {
              "Content-Type": file.type || "image/jpeg",
            },
            body: file,
          });

          if (!putRes.ok) {
            throw new Error(`Cloudflare R2 upload returned status ${putRes.status}`);
          }

          const finalPresignUrl = presignRes.public_url || (presignRes.object_key ? `${R2_PUBLIC_DOMAIN}/${presignRes.object_key}` : "");

          setProductImages((prev) =>
            prev.map((item) =>
              item.id === tempId
                ? {
                    id: tempId,
                    url: finalPresignUrl,
                    objectKey: presignRes.object_key,
                    isUploading: false,
                    progress: 100,
                  }
                : item
            )
          );
        } catch (presignErr: any) {
          const finalErrMsg = presignErr.message || directErr.message || "Upload failed";
          setProductImages((prev) =>
            prev.map((item) =>
              item.id === tempId ? { ...item, isUploading: false, error: finalErrMsg } : item
            )
          );
          setImageError(`Failed to upload "${file.name}": ${finalErrMsg}`);
        }
      }
    }

    setIsUploadingImage(false);
  };

  const handleRemoveImage = async (index: number) => {
    const item = productImages[index];
    if (item && item.objectKey) {
      try {
        await apiRequest("/marketplace/seller/products/images", {
          method: "DELETE",
          body: JSON.stringify({ object_key: item.objectKey }),
        });
      } catch (_) {
        // continue deletion from UI
      }
    }
    setProductImages((prev) => prev.filter((_, i) => i !== index));
  };

  const handleSetCover = (index: number) => {
    if (index === 0) return;
    setProductImages((prev) => {
      const copy = [...prev];
      const [selected] = copy.splice(index, 1);
      return [selected, ...copy];
    });
  };

  const handleMoveImage = (index: number, direction: "left" | "right") => {
    const targetIndex = direction === "left" ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= productImages.length) return;
    setProductImages((prev) => {
      const copy = [...prev];
      const temp = copy[index];
      copy[index] = copy[targetIndex];
      copy[targetIndex] = temp;
      return copy;
    });
  };

  const handleSaveProduct = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (productImages.some((img) => img.isUploading)) {
      setError("Please wait for all images to finish uploading before saving.");
      return;
    }

    setProdSubmitting(true);

    try {
      const priceKobo = Math.round(parseFloat(prodPriceNaira) * 100);
      const compareKobo = prodCompareNaira ? Math.round(parseFloat(prodCompareNaira) * 100) : undefined;
      const imagesArray = productImages.map((img) => img.url.trim()).filter(Boolean);

      if (imagesArray.length > 5) {
        setError("A maximum of 5 images is allowed per product.");
        setProdSubmitting(false);
        return;
      }

      const payload = {
        title: prodTitle.trim(),
        description: prodDescription.trim(),
        category: prodCategory,
        sku: prodSku.trim(),
        price_kobo: priceKobo,
        compare_at_price_kobo: compareKobo,
        stock_quantity: parseInt(prodStock, 10) || 0,
        low_stock_threshold: parseInt(prodLowStock, 10) || 0,
        images: imagesArray,
        status: prodStatus,
        is_featured: false,
      };

      if (editingProductId) {
        await apiRequest(`/marketplace/seller/products/${editingProductId}`, {
          method: "PUT",
          body: JSON.stringify(payload),
        });
        setSuccess("Product updated successfully.");
      } else {
        await apiRequest("/marketplace/seller/products", {
          method: "POST",
          body: JSON.stringify(payload),
        });
        setSuccess("Product created successfully.");
      }

      setShowProductModal(false);
      resetProductForm();
      loadSellerData();
    } catch (err: any) {
      setError(err.message || "Failed to save product");
    } finally {
      setProdSubmitting(false);
    }
  };

  const handleDeleteProduct = async (id: string) => {
    if (!confirm("Are you sure you want to delete this product?")) return;
    try {
      await apiRequest(`/marketplace/seller/products/${id}`, { method: "DELETE" });
      setSuccess("Product deleted successfully.");
      loadSellerData();
    } catch (err: any) {
      setError(err.message || "Failed to delete product");
    }
  };

  const handleDispatchOrder = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!dispatchOrderId && !dispatchShipmentId) return;
    setDispatchSubmitting(true);
    try {
      const url = dispatchShipmentId
        ? `/marketplace/seller/shipments/${dispatchShipmentId}/dispatch`
        : `/marketplace/seller/orders/${dispatchOrderId}/dispatch`;
      await apiRequest(url, {
        method: "POST",
        body: JSON.stringify({
          carrier_name: carrierName.trim(),
          tracking_number: trackingNumber.trim(),
          delivery_notes: dispatchNotes.trim(),
          notes: dispatchNotes.trim(),
        }),
      });
      setSuccess("Consignment marked as dispatched! Tracking details shared with buyer.");
      setShowDispatchModal(false);
      setDispatchShipmentId(null);
      setDispatchOrderId(null);
      loadSellerData();
    } catch (err: any) {
      setError(err.message || "Failed to dispatch consignment");
    } finally {
      setDispatchSubmitting(false);
    }
  };

  const handleConfirmDelivery = async (id: string, isShipment?: boolean) => {
    if (!confirm("Confirm delivery of this consignment?")) return;
    try {
      const url = isShipment
        ? `/marketplace/seller/shipments/${id}/delivered`
        : `/marketplace/seller/orders/${id}/delivered`;
      await apiRequest(url, {
        method: "POST",
        body: JSON.stringify({
          notes: "Confirmed delivery by vendor",
        }),
      });
      setSuccess("Consignment confirmed delivered!");
      loadSellerData();
    } catch (err: any) {
      setError(err.message || "Failed to confirm delivery");
    }
  };

  const handleCreateBranch = async (e: React.FormEvent) => {
    e.preventDefault();
    setBranchSubmitting(true);
    try {
      await apiRequest("/marketplace/seller/branches", {
        method: "POST",
        body: JSON.stringify({
          branch_name: branchName.trim(),
          address: branchAddress.trim(),
          state_id: branchStateId,
          lga_id: branchLgaId || undefined,
          contact_phone: branchPhone.trim(),
          manager_name: branchManager.trim() || undefined,
          is_headquarters: branches.length === 0,
          is_active: true,
        }),
      });
      setSuccess("Branch / Pickup point created successfully.");
      setShowBranchModal(false);
      setBranchName("");
      setBranchAddress("");
      setBranchPhone("");
      loadSellerData();
    } catch (err: any) {
      setError(err.message || "Failed to create branch");
    } finally {
      setBranchSubmitting(false);
    }
  };

  const openCreateProduct = () => {
    resetProductForm();
    setShowProductModal(true);
  };

  const openEditProduct = (p: any) => {
    setEditingProductId(p.id);
    setProdTitle(p.title);
    setProdDescription(p.description || "");
    setProdCategory(p.category);
    setProdSku(p.sku || "");
    setProdPriceNaira((p.price_kobo / 100).toString());
    setProdCompareNaira(p.compare_at_price_kobo ? (p.compare_at_price_kobo / 100).toString() : "");
    setProdStock(p.stock_quantity.toString());
    setProdLowStock(p.low_stock_threshold.toString());
    
    const urls = parseProductImages(p.images);
    const parsedImages = urls.map((url: string, idx: number) => ({
      id: `existing-${idx}-${url}`,
      url,
      isUploading: false,
    }));
    setProductImages(parsedImages);
    setImageError(null);
    setProdStatus(p.status);
    setShowProductModal(true);
  };

  const resetProductForm = () => {
    setEditingProductId(null);
    setProdTitle("");
    setProdDescription("");
    setProdCategory("apparel");
    setProdSku("");
    setProdPriceNaira("");
    setProdCompareNaira("");
    setProdStock("10");
    setProdLowStock("3");
    setProductImages([]);
    setImageError(null);
    setProdStatus("published");
  };

  if (authLoading || loading) {
    return <Spinner text="Loading vendor portal..." />;
  }

  // 1. If user has no business profile, show registration onboarding
  if (hasNoBusiness) {
    return (
      <div className="max-w-3xl mx-auto px-4 py-12 space-y-8">
        <div className="text-center space-y-3">
          <div className="w-14 h-14 bg-emerald-100 text-emerald-800 rounded-full flex items-center justify-center mx-auto">
            <Store className="w-7 h-7" />
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900">
            Open Your Store on Ratels Marketplace
          </h1>
          <p className="text-xs sm:text-sm text-gray-600 max-w-lg mx-auto">
            Sell directly to thousands of Ratels members and patriotic Nigerian buyers nationwide. Register your enterprise profile for verification.
          </p>
        </div>

        {error && <Alert type="error">{error}</Alert>}

        <form onSubmit={handleRegisterBusiness} className="bg-white rounded-2xl border border-gray-200 p-6 sm:p-8 shadow-sm space-y-6">
          <div className="border-b border-gray-100 pb-4">
            <h2 className="text-sm font-bold text-gray-900">Enterprise Profile & Credentials</h2>
            <p className="text-xs text-gray-500">Provide legal identification and business contact information.</p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
            <div className="sm:col-span-2">
              <label className="block font-semibold text-gray-700 mb-1">Business Name *</label>
              <input
                type="text"
                required
                value={regName}
                onChange={(e) => setRegName(e.target.value)}
                placeholder="e.g. Ratels Agro & Textile Hub"
                className="w-full px-3.5 py-2.5 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
              />
            </div>

            <div>
              <label className="block font-semibold text-gray-700 mb-1">Industry Category *</label>
              <select
                value={regCategory}
                onChange={(e) => setRegCategory(e.target.value)}
                className="w-full px-3.5 py-2.5 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
              >
                <option value="apparel">Apparel & Textiles</option>
                <option value="agriculture">Agriculture & Produce</option>
                <option value="food_beverage">Food & Packaged Beverages</option>
                <option value="electronics">Electronics & Hardware</option>
                <option value="beauty_health">Health & Personal Care</option>
                <option value="home_living">Home & Living</option>
                <option value="artisanal_crafts">Artisanal Crafts & Leather</option>
                <option value="books_stationery">Books & Educational</option>
                <option value="industrial">Industrial Equipment</option>
                <option value="services">Professional Services</option>
                <option value="other">General Merchandise</option>
              </select>
            </div>

            <div>
              <label className="block font-semibold text-gray-700 mb-1">CAC / RC / BN Number (Optional)</label>
              <input
                type="text"
                value={regNumber}
                onChange={(e) => setRegNumber(e.target.value)}
                placeholder="e.g. RC-1234567"
                className="w-full px-3.5 py-2.5 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
              />
            </div>

            <div>
              <label className="block font-semibold text-gray-700 mb-1">Official Contact Phone *</label>
              <input
                type="tel"
                required
                value={regPhone}
                onChange={(e) => setRegPhone(e.target.value)}
                placeholder="08012345678"
                className="w-full px-3.5 py-2.5 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
              />
            </div>

            <div>
              <label className="block font-semibold text-gray-700 mb-1">Official Contact Email *</label>
              <input
                type="email"
                required
                value={regEmail}
                onChange={(e) => setRegEmail(e.target.value)}
                placeholder="vendor@example.com"
                className="w-full px-3.5 py-2.5 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
              />
            </div>

            <div className="sm:col-span-2">
              <label className="block font-semibold text-gray-700 mb-1">Store Description *</label>
              <textarea
                required
                rows={3}
                value={regDescription}
                onChange={(e) => setRegDescription(e.target.value)}
                placeholder="Briefly describe what your business offers and what makes your products authentic..."
                className="w-full px-3.5 py-2.5 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
              />
            </div>

            <div>
              <label className="block font-semibold text-gray-700 mb-1">Headquarters State</label>
              <select
                value={regStateId}
                onChange={(e) => setRegStateId(e.target.value)}
                className="w-full px-3.5 py-2.5 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
              >
                <option value="">Select State...</option>
                {states.map((st) => (
                  <option key={st.id} value={st.id}>
                    {st.name} {st.is_fct ? "(FCT)" : ""}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block font-semibold text-gray-700 mb-1">Headquarters LGA</label>
              <select
                disabled={!regStateId}
                value={regLgaId}
                onChange={(e) => setRegLgaId(e.target.value)}
                className="w-full px-3.5 py-2.5 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 disabled:bg-gray-100"
              >
                <option value="">Select LGA...</option>
                {lgas.map((lga) => (
                  <option key={lga.id} value={lga.id}>
                    {lga.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="sm:col-span-2">
              <label className="block font-semibold text-gray-700 mb-1">Physical Street Address</label>
              <input
                type="text"
                value={regAddress}
                onChange={(e) => setRegAddress(e.target.value)}
                placeholder="Suite 10, Commerce Plaza, Abuja"
                className="w-full px-3.5 py-2.5 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={regSubmitting}
            className="w-full py-3.5 px-4 bg-emerald-800 hover:bg-emerald-700 text-white font-bold rounded-xl transition text-xs flex items-center justify-center gap-2 shadow-sm disabled:opacity-50"
          >
            {regSubmitting ? (
              <>
                <Spinner size="sm" />
                <span>Submitting Registration...</span>
              </>
            ) : (
              <>
                <Store className="w-4 h-4" />
                <span>Register Store on Ratels Marketplace</span>
              </>
            )}
          </button>
        </form>
      </div>
    );
  }

  // 2. If business is pending approval
  if (business && (business.status === "pending_review" || business.status === "pending" || business.approval_status === "pending")) {
    return (
      <div className="max-w-2xl mx-auto px-4 py-16 text-center space-y-6">
        <div className="w-16 h-16 bg-amber-100 rounded-full flex items-center justify-center mx-auto text-amber-700">
          <Clock className="w-8 h-8" />
        </div>
        <h1 className="text-2xl font-black text-gray-900">
          Business Verification Under Review
        </h1>
        <p className="text-sm text-gray-600 max-w-md mx-auto">
          Your enterprise profile for <span className="font-bold text-gray-900">{business.name}</span> has been received and is under administrative compliance review. Once approved, you can immediately publish products and receive orders.
        </p>
        <div className="p-4 bg-gray-50 border border-gray-200 rounded-2xl max-w-sm mx-auto text-xs text-left space-y-1.5">
          <div className="flex justify-between">
            <span className="text-gray-500">Business:</span>
            <span className="font-bold text-gray-900">{business.name}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-500">Category:</span>
            <span className="capitalize text-gray-900">{business.category}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-500">Submitted:</span>
            <span className="text-gray-900">{new Date(business.created_at).toLocaleDateString("en-NG")}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-500">Status:</span>
            <Badge variant="warning" size="sm">
              Pending Admin Approval
            </Badge>
          </div>
        </div>
        <Link
          href="/marketplace"
          className="inline-flex items-center gap-2 px-6 py-2.5 bg-emerald-800 text-white font-semibold rounded-xl text-xs hover:bg-emerald-700 transition"
        >
          Return to Marketplace
        </Link>
      </div>
    );
  }

  // 3. Approved Seller Dashboard
  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b border-gray-200 pb-5">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900">
              {business?.name || "Seller Portal"}
            </h1>
            <Badge variant="primary" size="sm">
              Verified Vendor
            </Badge>
          </div>
          <p className="text-xs text-gray-500 mt-1">
            Ratels Marketplace Vendor Workspace • Category: {business?.category}
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={openCreateProduct}
            className="px-4 py-2.5 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-xl transition inline-flex items-center gap-1.5 shadow-xs"
          >
            <Plus className="w-4 h-4" />
            Add New Product
          </button>
        </div>
      </div>

      {error && <Alert type="error">{error}</Alert>}
      {success && <Alert type="success">{success}</Alert>}

      {/* Tabs */}
      <div className="flex gap-2 border-b border-gray-200 text-xs font-bold">
        <button
          onClick={() => setActiveTab("overview")}
          className={`pb-3 px-3 border-b-2 transition flex items-center gap-1.5 ${
            activeTab === "overview"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <BarChart3 className="w-4 h-4" />
          Overview
        </button>

        <button
          onClick={() => setActiveTab("products")}
          className={`pb-3 px-3 border-b-2 transition flex items-center gap-1.5 ${
            activeTab === "products"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <Package className="w-4 h-4" />
          Products ({products.length})
        </button>

        <button
          onClick={() => setActiveTab("orders")}
          className={`pb-3 px-3 border-b-2 transition flex items-center gap-1.5 ${
            activeTab === "orders"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <ShoppingBag className="w-4 h-4" />
          Customer Orders ({orders.length})
        </button>

        <button
          onClick={() => setActiveTab("branches")}
          className={`pb-3 px-3 border-b-2 transition flex items-center gap-1.5 ${
            activeTab === "branches"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <Building2 className="w-4 h-4" />
          Branches & Pickup Points ({branches.length})
        </button>
      </div>

      {/* TAB 1: OVERVIEW */}
      {activeTab === "overview" && (
        <div className="space-y-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500">Gross Sales Revenue</span>
              <div className="text-xl font-black text-gray-900">
                ₦{metrics?.total_revenue_kobo ? (metrics.total_revenue_kobo / 100).toLocaleString() : "0"}
              </div>
              <span className="text-[10px] text-emerald-700">Paid merchant orders</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500">Total Orders</span>
              <div className="text-xl font-black text-gray-900">{metrics?.total_orders || orders.length}</div>
              <span className="text-[10px] text-gray-400">All customer checkouts</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500">Active Products</span>
              <div className="text-xl font-black text-gray-900">{metrics?.active_products || products.length}</div>
              <span className="text-[10px] text-emerald-700">Published in catalog</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500">Pending Fulfilment</span>
              <div className="text-xl font-black text-amber-700">
                {orders.filter((o) => o.fulfilment_status === "unfulfilled" && o.payment_status === "paid").length}
              </div>
              <span className="text-[10px] text-amber-600">Awaiting dispatch</span>
            </div>
          </div>
        </div>
      )}

      {/* TAB 2: PRODUCTS */}
      {activeTab === "products" && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <h2 className="text-sm font-bold text-gray-900">Catalog Inventory</h2>
            <button
              onClick={openCreateProduct}
              className="px-3.5 py-1.5 bg-emerald-800 text-white text-xs font-semibold rounded-xl hover:bg-emerald-700 transition inline-flex items-center gap-1.5"
            >
              <Plus className="w-3.5 h-3.5" />
              Add Product
            </button>
          </div>

          {products.length === 0 ? (
            <EmptyState
              icon={<Package className="w-6 h-6" />}
              title="No Products in Catalog"
              description="Add your first product to start selling on the Ratels Marketplace."
              actionText="Add Product"
              onAction={openCreateProduct}
            />
          ) : (
            <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-gray-600">
                  <thead className="bg-gray-50 text-gray-500 font-semibold border-b border-gray-200">
                    <tr>
                      <th className="py-3 px-4">Product</th>
                      <th className="py-3 px-4">Category</th>
                      <th className="py-3 px-4">Price</th>
                      <th className="py-3 px-4">Stock</th>
                      <th className="py-3 px-4">Status</th>
                      <th className="py-3 px-4 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {products.map((p) => (
                      <tr key={p.id} className="hover:bg-gray-50/50">
                        <td className="py-3 px-4 font-bold text-gray-900 flex items-center gap-3">
                          {(() => {
                            const prodImgs = parseProductImages(p.images);
                            const firstImg = prodImgs[0] || "";
                            return firstImg ? (
                              <img
                                src={firstImg}
                                alt={p.title}
                                className="w-8 h-8 rounded-lg object-cover border border-gray-100 shrink-0"
                                onError={(e) => {
                                  (e.target as HTMLElement).style.display = "none";
                                }}
                              />
                            ) : (
                              <div className="w-8 h-8 rounded-lg bg-gray-100 flex items-center justify-center shrink-0 text-gray-400">
                                <Package className="w-4 h-4" />
                              </div>
                            );
                          })()}
                          <span className="truncate max-w-xs">{p.title}</span>
                        </td>
                        <td className="py-3 px-4 capitalize">{p.category}</td>
                        <td className="py-3 px-4 font-bold text-gray-900">
                          ₦{(p.price_kobo / 100).toLocaleString()}
                        </td>
                        <td className="py-3 px-4">
                          <span
                            className={
                              p.stock_quantity <= p.low_stock_threshold
                                ? "text-red-700 font-bold"
                                : "text-gray-900"
                            }
                          >
                            {p.stock_quantity} in stock
                          </span>
                        </td>
                        <td className="py-3 px-4">
                          <Badge
                            variant={p.status === "published" ? "primary" : "neutral"}
                            size="sm"
                          >
                            {p.status}
                          </Badge>
                        </td>
                        <td className="py-3 px-4 text-right space-x-2">
                          <button
                            onClick={() => openEditProduct(p)}
                            className="p-1.5 text-gray-500 hover:text-emerald-800 transition"
                            title="Edit Product"
                          >
                            <Edit className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handleDeleteProduct(p.id)}
                            className="p-1.5 text-gray-500 hover:text-red-700 transition"
                            title="Delete Product"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}

      {/* TAB 3: CUSTOMER ORDERS */}
      {activeTab === "orders" && (
        <div className="space-y-4">
          <h2 className="text-sm font-bold text-gray-900">Fulfilment & Dispatch Orders</h2>

          {sellerShipments.length === 0 && orders.length === 0 ? (
            <EmptyState
              icon={<ShoppingBag className="w-6 h-6" />}
              title="No Orders Received"
              description="Customer purchases of your products will appear here for fulfilment."
            />
          ) : sellerShipments.length > 0 ? (
            <div className="space-y-3">
              {sellerShipments.map((s: any) => {
                const sh = s.shipment;
                const ord = s.order || {};
                const shItems = s.items || [];
                const isPending = sh.status === "pending" || sh.status === "processing";
                const isDispatched = sh.status === "dispatched" || sh.status === "in_transit";
                const isDelivered = sh.status === "delivered";

                return (
                  <div
                    key={sh.id}
                    className="bg-white rounded-2xl border border-gray-200 p-5 shadow-sm space-y-3"
                  >
                    <div className="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 pb-3">
                      <div className="flex items-center gap-2">
                        <span className="font-mono font-bold text-sm text-gray-900">
                          {ord.order_number || "Order"}
                        </span>
                        <span className="text-xs text-gray-400">•</span>
                        <span className="text-xs text-gray-500">
                          Buyer: {ord.buyer_name || "Customer"} ({ord.buyer_phone || ""})
                        </span>
                      </div>

                      <div className="flex gap-2">
                        <Badge
                          variant={ord.payment_status === "paid" ? "primary" : "neutral"}
                          size="sm"
                        >
                          Payment: {ord.payment_status || "paid"}
                        </Badge>
                        <Badge
                          variant={
                            isDelivered
                              ? "primary"
                              : isDispatched
                              ? "info"
                              : "warning"
                          }
                          size="sm"
                        >
                          Shipment: {sh.status.toUpperCase()}
                        </Badge>
                      </div>
                    </div>

                    <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3 text-xs">
                      <div className="space-y-1">
                        <div className="text-gray-500">
                          Destination: <span className="font-semibold text-gray-900">{ord.shipping_address || "Standard Delivery"}</span>
                        </div>
                        {shItems.length > 0 && (
                          <div className="text-gray-600">
                            Your Products: <span className="font-semibold">{shItems.map((itm: any) => `${itm.quantity}x ${itm.title_snapshot}`).join(", ")}</span>
                          </div>
                        )}
                        {sh.carrier_name && (
                          <div className="text-gray-500 text-[11px]">
                            Carrier: <span className="font-bold text-gray-900">{sh.carrier_name}</span>
                            {sh.tracking_number && ` | Tracking: ${sh.tracking_number}`}
                          </div>
                        )}
                      </div>

                      <div className="flex items-center gap-3">
                        {isPending && ord.payment_status === "paid" && (
                          <button
                            onClick={() => {
                              setDispatchShipmentId(sh.id);
                              setDispatchOrderId(sh.order_id);
                              setShowDispatchModal(true);
                            }}
                            className="px-3.5 py-1.5 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-xl transition inline-flex items-center gap-1.5"
                          >
                            <Send className="w-3.5 h-3.5" />
                            Dispatch Shipment
                          </button>
                        )}

                        {isDispatched && (
                          <button
                            onClick={() => handleConfirmDelivery(sh.id, true)}
                            className="px-3.5 py-1.5 bg-blue-700 hover:bg-blue-800 text-white text-xs font-bold rounded-xl transition inline-flex items-center gap-1.5"
                          >
                            <CheckCircle2 className="w-3.5 h-3.5" />
                            Confirm Delivered
                          </button>
                        )}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="space-y-3">
              {orders.map((ord) => (
                <div
                  key={ord.id}
                  className="bg-white rounded-2xl border border-gray-200 p-5 shadow-sm space-y-3"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 pb-3">
                    <div className="flex items-center gap-2">
                      <span className="font-mono font-bold text-sm text-gray-900">
                        {ord.order_number}
                      </span>
                      <span className="text-xs text-gray-400">•</span>
                      <span className="text-xs text-gray-500">
                        Buyer: {ord.buyer_name} ({ord.buyer_phone})
                      </span>
                    </div>

                    <div className="flex gap-2">
                      <Badge
                        variant={ord.payment_status === "paid" ? "primary" : "neutral"}
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
                            : "warning"
                        }
                        size="sm"
                      >
                        {ord.fulfilment_status}
                      </Badge>
                    </div>
                  </div>

                  <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3 text-xs">
                    <div>
                      <div className="text-gray-500">Fulfilment Type: <span className="font-semibold text-gray-900 capitalize">{ord.delivery_type}</span></div>
                      <div className="text-gray-500">Destination: <span className="font-semibold text-gray-900">{ord.shipping_address}</span></div>
                    </div>

                    <div className="flex items-center gap-3">
                      <div className="text-right">
                        <div className="text-[11px] text-gray-400">Order Total</div>
                        <div className="text-sm font-bold text-gray-900">
                          ₦{(ord.total_kobo / 100).toLocaleString()}
                        </div>
                      </div>

                      {ord.payment_status === "paid" && ord.fulfilment_status === "unfulfilled" && (
                        <button
                          onClick={() => {
                            setDispatchOrderId(ord.id);
                            setShowDispatchModal(true);
                          }}
                          className="px-3.5 py-1.5 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-xl transition inline-flex items-center gap-1.5"
                        >
                          <Send className="w-3.5 h-3.5" />
                          Dispatch Order
                        </button>
                      )}

                      {ord.fulfilment_status === "dispatched" && (
                        <button
                          onClick={() => handleConfirmDelivery(ord.id)}
                          className="px-3.5 py-1.5 bg-blue-700 hover:bg-blue-800 text-white text-xs font-bold rounded-xl transition inline-flex items-center gap-1.5"
                        >
                          <CheckCircle2 className="w-3.5 h-3.5" />
                          Confirm Delivery
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* TAB 4: BRANCHES */}
      {activeTab === "branches" && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <h2 className="text-sm font-bold text-gray-900">Registered Branches & Pickup Points</h2>
            <button
              onClick={() => setShowBranchModal(true)}
              className="px-3.5 py-1.5 bg-emerald-800 text-white text-xs font-semibold rounded-xl hover:bg-emerald-700 transition inline-flex items-center gap-1.5"
            >
              <Plus className="w-3.5 h-3.5" />
              Add Branch
            </button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {branches.map((b) => (
              <div key={b.id} className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-2 text-xs">
                <div className="flex justify-between items-start">
                  <div className="font-bold text-gray-900 text-sm flex items-center gap-1.5">
                    <Building2 className="w-4 h-4 text-emerald-700" />
                    {b.branch_name}
                  </div>
                  {b.is_headquarters && (
                    <Badge variant="primary" size="sm">
                      Headquarters
                    </Badge>
                  )}
                </div>
                <div className="text-gray-600 flex items-center gap-1">
                  <MapPin className="w-3.5 h-3.5 text-gray-400" />
                  {b.address}
                </div>
                <div className="text-gray-500 pt-1">
                  Phone: {b.contact_phone} {b.manager_name && `• Manager: ${b.manager_name}`}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Product Create/Edit Modal */}
      {showProductModal && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 shadow-xl space-y-4 max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between border-b border-gray-100 pb-3">
              <h3 className="font-bold text-sm text-gray-900">
                {editingProductId ? "Edit Marketplace Product" : "Publish New Product"}
              </h3>
              <button onClick={() => setShowProductModal(false)} className="text-gray-400 hover:text-gray-600">✕</button>
            </div>

            <form onSubmit={handleSaveProduct} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Product Title *</label>
                <input
                  type="text"
                  required
                  value={prodTitle}
                  onChange={(e) => setProdTitle(e.target.value)}
                  placeholder="e.g. Official Ratels Worldwide Premium Beret"
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Category *</label>
                  <select
                    value={prodCategory}
                    onChange={(e) => setProdCategory(e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                  >
                    <option value="apparel">Apparel & Uniforms</option>
                    <option value="agriculture">Agriculture & Feed</option>
                    <option value="food_beverage">Food & Beverages</option>
                    <option value="electronics">Electronics</option>
                    <option value="beauty_health">Health & Beauty</option>
                    <option value="artisanal_crafts">Crafts & Leather</option>
                    <option value="books_stationery">Books & Stationery</option>
                    <option value="other">General</option>
                  </select>
                </div>
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">SKU / Code *</label>
                  <input
                    type="text"
                    required
                    value={prodSku}
                    onChange={(e) => setProdSku(e.target.value)}
                    placeholder="e.g. RAT-BER-01"
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Selling Price (₦) *</label>
                  <input
                    type="number"
                    step="0.01"
                    min="10"
                    required
                    value={prodPriceNaira}
                    onChange={(e) => setProdPriceNaira(e.target.value)}
                    placeholder="5000"
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 font-bold"
                  />
                </div>
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Compare-at Price (₦)</label>
                  <input
                    type="number"
                    step="0.01"
                    value={prodCompareNaira}
                    onChange={(e) => setProdCompareNaira(e.target.value)}
                    placeholder="7500"
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Available Stock *</label>
                  <input
                    type="number"
                    required
                    min="0"
                    value={prodStock}
                    onChange={(e) => setProdStock(e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                  />
                </div>
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Low Stock Warning Threshold</label>
                  <input
                    type="number"
                    required
                    min="0"
                    value={prodLowStock}
                    onChange={(e) => setProdLowStock(e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                  />
                </div>
              </div>

              {/* Native Cloudflare R2 Image Upload & Management Control */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="block font-semibold text-gray-700">
                    Product Images <span className="text-gray-400 font-normal">({productImages.length}/5)</span>
                  </label>
                  <span className="text-[10px] text-gray-500">First image is primary cover</span>
                </div>

                {imageError && (
                  <div className="p-2.5 bg-red-50 border border-red-200 text-red-700 rounded-xl text-xs flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <AlertCircle className="w-4 h-4 shrink-0 text-red-500" />
                      <span>{imageError}</span>
                    </div>
                    <button
                      type="button"
                      onClick={() => setImageError(null)}
                      className="text-red-500 hover:text-red-800 text-sm ml-2 font-bold"
                    >
                      ✕
                    </button>
                  </div>
                )}

                {/* Hidden File Input */}
                <input
                  type="file"
                  ref={fileInputRef}
                  multiple
                  accept="image/jpeg,image/png,image/webp"
                  className="hidden"
                  onChange={(e) => {
                    if (e.target.files) {
                      handleFilesSelected(e.target.files);
                      e.target.value = "";
                    }
                  }}
                />

                {/* Drag and Drop Upload Zone */}
                {productImages.length < 5 && (
                  <div
                    onDragOver={(e) => {
                      e.preventDefault();
                      setIsDragOver(true);
                    }}
                    onDragLeave={() => setIsDragOver(false)}
                    onDrop={(e) => {
                      e.preventDefault();
                      setIsDragOver(false);
                      if (e.dataTransfer.files) {
                        handleFilesSelected(e.dataTransfer.files);
                      }
                    }}
                    onClick={() => fileInputRef.current?.click()}
                    className={`border-2 border-dashed rounded-2xl p-4 text-center cursor-pointer transition-all ${
                      isDragOver
                        ? "border-emerald-500 bg-emerald-50/50"
                        : "border-gray-300 hover:border-emerald-400 hover:bg-gray-50/70"
                    }`}
                  >
                    <div className="flex flex-col items-center justify-center space-y-1.5">
                      <div className="w-10 h-10 rounded-full bg-emerald-100 text-emerald-600 flex items-center justify-center">
                        <UploadCloud className="w-5 h-5" />
                      </div>
                      <div className="text-xs font-semibold text-gray-800">
                        {isUploadingImage ? (
                          <span className="flex items-center gap-1.5 text-emerald-600">
                            <Loader2 className="w-3.5 h-3.5 animate-spin" /> Uploading image to Cloudflare R2...
                          </span>
                        ) : (
                          <span>Click to browse device or drag and drop images</span>
                        )}
                      </div>
                      <p className="text-[10px] text-gray-500">
                        JPG, JPEG, PNG, or WebP &bull; Max 5 MB per image &bull; Up to 5 images
                      </p>
                      <button
                        type="button"
                        className="mt-1 px-3 py-1 bg-emerald-600 hover:bg-emerald-700 text-white font-medium rounded-lg text-[11px] shadow-sm transition"
                        onClick={(e) => {
                          e.stopPropagation();
                          fileInputRef.current?.click();
                        }}
                      >
                        Select Images from Device
                      </button>
                    </div>
                  </div>
                )}

                {/* Uploaded Images Grid */}
                {productImages.length > 0 && (
                  <div className="grid grid-cols-5 gap-2 pt-1">
                    {productImages.map((img, idx) => (
                      <div
                        key={img.id}
                        className={`relative group rounded-xl overflow-hidden border bg-gray-50 aspect-square flex items-center justify-center ${
                          idx === 0 ? "border-emerald-500 ring-2 ring-emerald-500/20" : "border-gray-200"
                        }`}
                      >
                        <img
                          src={img.url}
                          alt={`Product media ${idx + 1}`}
                          className="w-full h-full object-cover"
                          onError={(e) => {
                            (e.target as HTMLImageElement).src =
                              "data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='100' height='100' viewBox='0 0 24 24' fill='none' stroke='%239ca3af' stroke-width='2'><rect width='18' height='18' x='3' y='3' rx='2'/><circle cx='9' cy='9' r='2'/><path d='m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21'/></svg>";
                          }}
                        />

                        {/* Primary Cover Badge */}
                        {idx === 0 && (
                          <div className="absolute top-1 left-1 bg-emerald-600 text-white text-[9px] font-bold px-1.5 py-0.5 rounded shadow">
                            Cover
                          </div>
                        )}

                        {/* Uploading Spinner Overlay */}
                        {img.isUploading && (
                          <div className="absolute inset-0 bg-black/60 flex flex-col items-center justify-center text-white text-[9px] p-1">
                            <Loader2 className="w-4 h-4 animate-spin text-white mb-1" />
                            <span>Uploading...</span>
                          </div>
                        )}

                        {/* Error Overlay */}
                        {img.error && (
                          <div className="absolute inset-0 bg-red-600/80 flex flex-col items-center justify-center text-white text-[9px] p-1 text-center">
                            <AlertCircle className="w-4 h-4 text-white mb-0.5" />
                            <span className="line-clamp-2">Error</span>
                          </div>
                        )}

                        {/* Actions Overlay */}
                        {!img.isUploading && (
                          <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity flex flex-col justify-between p-1">
                            <div className="flex justify-between items-center">
                              {idx > 0 ? (
                                <button
                                  type="button"
                                  onClick={() => handleSetCover(idx)}
                                  title="Set as primary cover image"
                                  className="bg-white/90 hover:bg-emerald-600 hover:text-white text-gray-800 text-[9px] font-bold px-1 py-0.5 rounded shadow transition"
                                >
                                  Cover
                                </button>
                              ) : <span />}

                              <button
                                type="button"
                                onClick={() => handleRemoveImage(idx)}
                                title="Remove this image"
                                className="w-5 h-5 rounded-full bg-red-500 hover:bg-red-600 text-white flex items-center justify-center shadow transition"
                              >
                                ✕
                              </button>
                            </div>

                            {/* Reordering Controls */}
                            <div className="flex justify-between items-center bg-black/40 rounded px-1 py-0.5">
                              <button
                                type="button"
                                disabled={idx === 0}
                                onClick={() => handleMoveImage(idx, "left")}
                                className={`text-white text-[10px] hover:text-emerald-300 transition ${
                                  idx === 0 ? "opacity-30 cursor-not-allowed" : ""
                                }`}
                                title="Move left"
                              >
                                ◀
                              </button>
                              <span className="text-[9px] text-white/80 font-mono">#{idx + 1}</span>
                              <button
                                type="button"
                                disabled={idx === productImages.length - 1}
                                onClick={() => handleMoveImage(idx, "right")}
                                className={`text-white text-[10px] hover:text-emerald-300 transition ${
                                  idx === productImages.length - 1 ? "opacity-30 cursor-not-allowed" : ""
                                }`}
                                title="Move right"
                              >
                                ▶
                              </button>
                            </div>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Product Description</label>
                <textarea
                  rows={3}
                  value={prodDescription}
                  onChange={(e) => setProdDescription(e.target.value)}
                  placeholder="Details, materials, sizing, and origin..."
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Publication Status</label>
                <select
                  value={prodStatus}
                  onChange={(e) => setProdStatus(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                >
                  <option value="published">Published (Visible in Storefront)</option>
                  <option value="draft">Draft (Private)</option>
                </select>
              </div>

              <div className="flex gap-2 pt-2 border-t border-gray-100">
                <button
                  type="button"
                  onClick={() => setShowProductModal(false)}
                  className="flex-1 py-2.5 bg-gray-100 text-gray-700 font-bold rounded-xl"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={prodSubmitting}
                  className="flex-1 py-2.5 bg-emerald-800 hover:bg-emerald-700 text-white font-bold rounded-xl disabled:opacity-50"
                >
                  {prodSubmitting ? "Saving..." : "Save Product"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Dispatch Order Modal */}
      {showDispatchModal && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex items-center justify-between border-b border-gray-100 pb-3">
              <h3 className="font-bold text-sm text-gray-900 flex items-center gap-1.5">
                <Truck className="w-4 h-4 text-emerald-700" />
                Dispatch & Consignment Details
              </h3>
              <button onClick={() => setShowDispatchModal(false)} className="text-gray-400">✕</button>
            </div>

            <form onSubmit={handleDispatchOrder} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Carrier / Logistics Provider *</label>
                <input
                  type="text"
                  required
                  value={carrierName}
                  onChange={(e) => setCarrierName(e.target.value)}
                  placeholder="e.g. GIG Logistics, DHL, In-house Rider"
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Waybill / Tracking Code</label>
                <input
                  type="text"
                  value={trackingNumber}
                  onChange={(e) => setTrackingNumber(e.target.value)}
                  placeholder="e.g. GIG-7829103"
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Dispatch Notes</label>
                <input
                  type="text"
                  value={dispatchNotes}
                  onChange={(e) => setDispatchNotes(e.target.value)}
                  placeholder="Package handed over to dispatch agent"
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div className="flex gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowDispatchModal(false)}
                  className="flex-1 py-2.5 bg-gray-100 text-gray-700 font-bold rounded-xl"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={dispatchSubmitting}
                  className="flex-1 py-2.5 bg-emerald-800 hover:bg-emerald-700 text-white font-bold rounded-xl disabled:opacity-50"
                >
                  {dispatchSubmitting ? "Dispatching..." : "Confirm Dispatch"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Branch Modal */}
      {showBranchModal && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex items-center justify-between border-b border-gray-100 pb-3">
              <h3 className="font-bold text-sm text-gray-900 flex items-center gap-1.5">
                <Building2 className="w-4 h-4 text-emerald-700" />
                Add Physical Branch / Pickup Point
              </h3>
              <button onClick={() => setShowBranchModal(false)} className="text-gray-400">✕</button>
            </div>

            <form onSubmit={handleCreateBranch} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Branch Name *</label>
                <input
                  type="text"
                  required
                  value={branchName}
                  onChange={(e) => setBranchName(e.target.value)}
                  placeholder="e.g. Lagos Island Branch"
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Street Address *</label>
                <input
                  type="text"
                  required
                  value={branchAddress}
                  onChange={(e) => setBranchAddress(e.target.value)}
                  placeholder="12 Broad Street, Marina"
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">State *</label>
                  <select
                    required
                    value={branchStateId}
                    onChange={(e) => setBranchStateId(e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                  >
                    <option value="">Select State...</option>
                    {states.map((st) => (
                      <option key={st.id} value={st.id}>{st.name}</option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block font-semibold text-gray-700 mb-1">LGA</label>
                  <select
                    disabled={!branchStateId}
                    value={branchLgaId}
                    onChange={(e) => setBranchLgaId(e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                  >
                    <option value="">Select LGA...</option>
                    {lgas.map((lga) => (
                      <option key={lga.id} value={lga.id}>{lga.name}</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Contact Phone *</label>
                  <input
                    type="tel"
                    required
                    value={branchPhone}
                    onChange={(e) => setBranchPhone(e.target.value)}
                    placeholder="08012345678"
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                  />
                </div>
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Branch Manager</label>
                  <input
                    type="text"
                    value={branchManager}
                    onChange={(e) => setBranchManager(e.target.value)}
                    placeholder="e.g. Terna Moses"
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                  />
                </div>
              </div>

              <div className="flex gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowBranchModal(false)}
                  className="flex-1 py-2.5 bg-gray-100 text-gray-700 font-bold rounded-xl"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={branchSubmitting}
                  className="flex-1 py-2.5 bg-emerald-800 hover:bg-emerald-700 text-white font-bold rounded-xl disabled:opacity-50"
                >
                  {branchSubmitting ? "Creating..." : "Save Branch"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
