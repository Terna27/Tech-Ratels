"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Store,
  Package,
  ShoppingBag,
  CreditCard,
  Truck,
  AlertTriangle,
  BarChart3,
  CheckCircle2,
  XCircle,
  Clock,
  ShieldCheck,
  Search,
  Filter,
  Eye,
  ExternalLink,
  ChevronRight,
  ArrowRight,
  RefreshCw,
  Building2,
  DollarSign,
  AlertCircle,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { parseProductImages } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";

type AdminTab =
  | "overview"
  | "businesses"
  | "products"
  | "orders"
  | "payments"
  | "fulfilment"
  | "disputes";

export default function AdminMarketplacePage() {
  const router = useRouter();
  const { user, isCoordinator, isLoading: authLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<AdminTab>("overview");
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Data states
  const [overview, setOverview] = useState<any>(null);
  const [businesses, setBusinesses] = useState<any[]>([]);
  const [products, setProducts] = useState<any[]>([]);
  const [orders, setOrders] = useState<any[]>([]);
  const [payments, setPayments] = useState<any[]>([]);
  const [disputes, setDisputes] = useState<any[]>([]);

  // Filter states
  const [businessFilter, setBusinessFilter] = useState("all");
  const [productFilter, setProductFilter] = useState("all");
  const [orderFilter, setOrderFilter] = useState("all");
  const [disputeFilter, setDisputeFilter] = useState("all");

  // Approval Modal State
  const [showApprovalModal, setShowApprovalModal] = useState(false);
  const [selectedBusiness, setSelectedBusiness] = useState<any>(null);
  const [approvalAction, setApprovalAction] = useState<"approve" | "reject">("approve");
  const [approvalReason, setApprovalReason] = useState("");
  const [approvalSubmitting, setApprovalSubmitting] = useState(false);

  // Dispute Resolution Modal State
  const [showResolveModal, setShowResolveModal] = useState(false);
  const [selectedDispute, setSelectedDispute] = useState<any>(null);
  const [resolutionType, setResolutionType] = useState<"resolved_refunded" | "rejected">("resolved_refunded");
  const [resolutionNotes, setResolutionNotes] = useState("");
  const [resolveSubmitting, setResolveSubmitting] = useState(false);

  // Product Moderation Modal
  const [showModerateModal, setShowModerateModal] = useState(false);
  const [selectedProduct, setSelectedProduct] = useState<any>(null);
  const [moderateStatus, setModerateStatus] = useState("published");
  const [moderateSubmitting, setModerateSubmitting] = useState(false);

  // Refund Modal State
  const [showRefundModal, setShowRefundModal] = useState(false);
  const [selectedOrderForRefund, setSelectedOrderForRefund] = useState<any>(null);
  const [refundAmountNaira, setRefundAmountNaira] = useState("");
  const [refundReason, setRefundReason] = useState("");
  const [refundSubmitting, setRefundSubmitting] = useState(false);

  useEffect(() => {
    if (!authLoading && (!user || !isCoordinator)) {
      router.push("/login?redirect=/admin/marketplace");
      return;
    }

    if (user && isCoordinator) {
      loadTabContent(activeTab);
    }
  }, [user, isCoordinator, authLoading, activeTab]);

  const loadTabContent = async (tab: AdminTab) => {
    setLoading(true);
    setError(null);
    try {
      if (tab === "overview") {
        const data = await apiRequest<any>("/marketplace/admin/overview");
        setOverview(data);
      } else if (tab === "businesses") {
        const query = businessFilter !== "all" ? `?status=${businessFilter}` : "";
        const data = await apiRequest<any[]>(`/marketplace/admin/businesses${query}`);
        setBusinesses(data || []);
      } else if (tab === "products") {
        const query = productFilter !== "all" ? `?status=${productFilter}` : "";
        const data = await apiRequest<any[]>(`/marketplace/admin/products${query}`);
        setProducts(data || []);
      } else if (tab === "orders" || tab === "fulfilment") {
        const query = orderFilter !== "all" ? `?order_status=${orderFilter}` : "";
        const data = await apiRequest<any[]>(`/marketplace/admin/orders${query}`);
        setOrders(data || []);
      } else if (tab === "payments") {
        const data = await apiRequest<any[]>("/marketplace/admin/payments");
        setPayments(data || []);
      } else if (tab === "disputes") {
        const query = disputeFilter !== "all" ? `?status=${disputeFilter}` : "";
        const data = await apiRequest<any[]>(`/marketplace/admin/disputes${query}`);
        setDisputes(data || []);
      }
    } catch (err: any) {
      setError(err.message || `Failed to load ${tab} data`);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  const handleApproveBusiness = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedBusiness) return;
    setApprovalSubmitting(true);
    try {
      await apiRequest(`/marketplace/admin/businesses/${selectedBusiness.id}/approve`, {
        method: "POST",
        body: JSON.stringify({
          approve: approvalAction === "approve",
          reason: approvalReason.trim() || (approvalAction === "approve" ? "Verified vendor requirements" : "Compliance failure"),
        }),
      });
      setSuccess(`Business ${selectedBusiness.name} has been ${approvalAction === "approve" ? "approved" : "rejected"}.`);
      setShowApprovalModal(false);
      setSelectedBusiness(null);
      setApprovalReason("");
      loadTabContent("businesses");
    } catch (err: any) {
      setError(err.message || "Failed to process business approval");
    } finally {
      setApprovalSubmitting(false);
    }
  };

  const handleResolveDispute = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedDispute) return;
    setResolveSubmitting(true);
    try {
      await apiRequest(`/marketplace/admin/disputes/${selectedDispute.id}/resolve`, {
        method: "POST",
        body: JSON.stringify({
          resolution: resolutionType,
          notes: resolutionNotes.trim(),
        }),
      });
      setSuccess("Dispute resolution executed. Inventory and payment states updated.");
      setShowResolveModal(false);
      setSelectedDispute(null);
      setResolutionNotes("");
      loadTabContent("disputes");
    } catch (err: any) {
      setError(err.message || "Failed to resolve dispute");
    } finally {
      setResolveSubmitting(false);
    }
  };

  const handleModerateProduct = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedProduct) return;
    setModerateSubmitting(true);
    try {
      await apiRequest(`/marketplace/admin/products/${selectedProduct.id}/status`, {
        method: "PUT",
        body: JSON.stringify({
          status: moderateStatus,
        }),
      });
      setSuccess(`Product "${selectedProduct.title}" status updated to ${moderateStatus}.`);
      setShowModerateModal(false);
      setSelectedProduct(null);
      loadTabContent("products");
    } catch (err: any) {
      setError(err.message || "Failed to moderate product");
    } finally {
      setModerateSubmitting(false);
    }
  };

  const handleProcessRefund = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedOrderForRefund) return;
    setRefundSubmitting(true);
    setError(null);
    try {
      const kobo = refundAmountNaira
        ? Math.round(parseFloat(refundAmountNaira) * 100)
        : selectedOrderForRefund.total_kobo;
      await apiRequest(`/marketplace/admin/orders/${selectedOrderForRefund.id}/refund`, {
        method: "POST",
        body: JSON.stringify({
          amount_kobo: kobo,
          reason: refundReason.trim() || "Administrative refund",
        }),
      });
      setSuccess(`Refund successfully processed for order ${selectedOrderForRefund.order_number}`);
      setShowRefundModal(false);
      setSelectedOrderForRefund(null);
      loadTabContent("orders");
    } catch (err: any) {
      setError(err.message || "Failed to process refund");
    } finally {
      setRefundSubmitting(false);
    }
  };

  const handleAdminCancelOrder = async (orderId: string, orderNumber: string) => {
    if (!window.confirm(`Are you sure you want to cancel order ${orderNumber}? Stock will be restored.`)) {
      return;
    }
    setError(null);
    try {
      await apiRequest(`/marketplace/admin/orders/${orderId}/cancel`, {
        method: "POST",
        body: JSON.stringify({ reason: "Cancelled by administrator" }),
      });
      setSuccess(`Order ${orderNumber} has been cancelled.`);
      loadTabContent("orders");
    } catch (err: any) {
      setError(err.message || "Failed to cancel order");
    }
  };

  if (authLoading || (loading && !overview && !businesses.length && !orders.length)) {
    return <Spinner text="Loading Ratels Marketplace administration..." />;
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Breadcrumb Navigation */}
      <nav className="flex items-center gap-2 text-xs text-gray-500 font-medium">
        <Link href="/admin" className="hover:text-emerald-800 transition">
          Administration
        </Link>
        <ChevronRight className="w-3.5 h-3.5" />
        <span className="text-gray-900 font-semibold">Ratels Marketplace Workspace</span>
      </nav>

      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b border-gray-200 pb-5">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900">
              Ratels Marketplace Workspace
            </h1>
            <Badge variant="primary" size="sm">
              Section 07 Commerce
            </Badge>
          </div>
          <p className="text-xs sm:text-sm text-gray-600 mt-1">
            Nationwide vendor governance, multi-seller product catalogue, payment reconciliations, and dispute arbitration.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => {
              setRefreshing(true);
              loadTabContent(activeTab);
            }}
            disabled={refreshing}
            className="px-3.5 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 text-xs font-bold rounded-xl transition inline-flex items-center gap-1.5"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? "animate-spin" : ""}`} />
            Refresh
          </button>
          <Link
            href="/marketplace"
            target="_blank"
            className="px-3.5 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-xl transition inline-flex items-center gap-1.5 shadow-xs"
          >
            <ExternalLink className="w-3.5 h-3.5" />
            Live Storefront
          </Link>
        </div>
      </div>

      {error && <Alert type="error">{error}</Alert>}
      {success && <Alert type="success">{success}</Alert>}

      {/* 7 Admin Tabs Navigation */}
      <div className="flex gap-2 overflow-x-auto pb-2 border-b border-gray-200 text-xs font-bold no-scrollbar">
        <button
          onClick={() => setActiveTab("overview")}
          className={`pb-3 px-3.5 border-b-2 transition whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === "overview"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <BarChart3 className="w-4 h-4" />
          Overview
        </button>

        <button
          onClick={() => setActiveTab("businesses")}
          className={`pb-3 px-3.5 border-b-2 transition whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === "businesses"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <Store className="w-4 h-4" />
          Businesses & Vendors
        </button>

        <button
          onClick={() => setActiveTab("products")}
          className={`pb-3 px-3.5 border-b-2 transition whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === "products"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <Package className="w-4 h-4" />
          Products & Categories
        </button>

        <button
          onClick={() => setActiveTab("orders")}
          className={`pb-3 px-3.5 border-b-2 transition whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === "orders"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <ShoppingBag className="w-4 h-4" />
          Orders
        </button>

        <button
          onClick={() => setActiveTab("payments")}
          className={`pb-3 px-3.5 border-b-2 transition whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === "payments"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <CreditCard className="w-4 h-4" />
          Payments & Refunds
        </button>

        <button
          onClick={() => setActiveTab("fulfilment")}
          className={`pb-3 px-3.5 border-b-2 transition whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === "fulfilment"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <Truck className="w-4 h-4" />
          Delivery & Fulfilment
        </button>

        <button
          onClick={() => setActiveTab("disputes")}
          className={`pb-3 px-3.5 border-b-2 transition whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === "disputes"
              ? "border-emerald-800 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <AlertTriangle className="w-4 h-4" />
          Disputes & Reports
        </button>
      </div>

      {/* SECTION 1: OVERVIEW */}
      {activeTab === "overview" && (
        <div className="space-y-6">
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
            <div className="bg-white p-4 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-[11px] text-gray-500 font-semibold">Total Businesses</span>
              <div className="text-xl font-black text-gray-900">{overview?.total_businesses ?? 0}</div>
              <span className="text-[10px] text-gray-400">Registered vendors</span>
            </div>

            <div className="bg-white p-4 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-[11px] text-amber-700 font-semibold">Pending Approval</span>
              <div className="text-xl font-black text-amber-700">{overview?.pending_businesses ?? 0}</div>
              <span className="text-[10px] text-amber-600">Awaiting KYC check</span>
            </div>

            <div className="bg-white p-4 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-[11px] text-gray-500 font-semibold">Active Products</span>
              <div className="text-xl font-black text-gray-900">{overview?.active_products ?? 0}</div>
              <span className="text-[10px] text-emerald-700">Published items</span>
            </div>

            <div className="bg-white p-4 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-[11px] text-gray-500 font-semibold">Total Orders</span>
              <div className="text-xl font-black text-gray-900">{overview?.total_orders ?? 0}</div>
              <span className="text-[10px] text-gray-400">Buyer transactions</span>
            </div>

            <div className="bg-white p-4 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-[11px] text-gray-500 font-semibold">Total GMV</span>
              <div className="text-xl font-black text-emerald-800">
                ₦{overview?.total_gmv_kobo ? (overview.total_gmv_kobo / 100).toLocaleString() : "0"}
              </div>
              <span className="text-[10px] text-emerald-700">Gross transaction value</span>
            </div>

            <div className="bg-white p-4 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-[11px] text-red-700 font-semibold">Open Disputes</span>
              <div className="text-xl font-black text-red-700">{overview?.open_disputes ?? 0}</div>
              <span className="text-[10px] text-red-600">Claims in arbitration</span>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-bold text-gray-900 flex items-center gap-2">
                  <Store className="w-4 h-4 text-emerald-700" />
                  Vendor Compliance & Approvals
                </h3>
                <button
                  onClick={() => setActiveTab("businesses")}
                  className="text-xs text-emerald-800 font-bold hover:underline"
                >
                  Manage Vendors
                </button>
              </div>
              <p className="text-xs text-gray-600 leading-relaxed">
                Ratels enforces verified vendor registration to protect buyers against fraud and illicit merchandise. All business profiles must be reviewed before their products are published to the national catalog.
              </p>
            </div>

            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-bold text-gray-900 flex items-center gap-2">
                  <CreditCard className="w-4 h-4 text-emerald-700" />
                  Payment Gateway Reconciliations
                </h3>
                <button
                  onClick={() => setActiveTab("payments")}
                  className="text-xs text-emerald-800 font-bold hover:underline"
                >
                  View Payments
                </button>
              </div>
              <p className="text-xs text-gray-600 leading-relaxed">
                Monetary calculations are computed strictly in integer kobo. Escrow payments are confirmed via server verification and signed HMAC webhook signatures before orders are released for fulfilment.
              </p>
            </div>
          </div>
        </div>
      )}

      {/* SECTION 2: BUSINESSES & VENDORS */}
      {activeTab === "businesses" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3">
            <h2 className="text-sm font-bold text-gray-900">Registered Businesses & Vendors</h2>
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500 font-medium">Filter Status:</span>
              <select
                value={businessFilter}
                onChange={(e) => {
                  setBusinessFilter(e.target.value);
                  loadTabContent("businesses");
                }}
                className="px-3 py-1.5 text-xs border border-gray-300 rounded-xl bg-white"
              >
                <option value="all">All Statuses</option>
                <option value="pending">Pending Approval</option>
                <option value="approved">Approved</option>
                <option value="rejected">Rejected</option>
                <option value="suspended">Suspended</option>
              </select>
            </div>
          </div>

          {businesses.length === 0 ? (
            <EmptyState
              icon={<Store className="w-6 h-6" />}
              title="No Businesses Found"
              description="No vendors match the selected filter criteria."
            />
          ) : (
            <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-gray-600">
                  <thead className="bg-gray-50 text-gray-500 font-semibold border-b border-gray-200">
                    <tr>
                      <th className="py-3 px-4">Business Name</th>
                      <th className="py-3 px-4">Category</th>
                      <th className="py-3 px-4">Owner / Contact</th>
                      <th className="py-3 px-4">Location</th>
                      <th className="py-3 px-4">Status</th>
                      <th className="py-3 px-4 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {businesses.map((b) => (
                      <tr key={b.id} className="hover:bg-gray-50/50">
                        <td className="py-3.5 px-4 font-bold text-gray-900">
                          {b.name}
                          {b.registration_number && (
                            <span className="block text-[10px] text-gray-400 font-normal">
                              Reg: {b.registration_number}
                            </span>
                          )}
                        </td>
                        <td className="py-3.5 px-4 capitalize">{b.category}</td>
                        <td className="py-3.5 px-4">
                          <div>{b.owner_name}</div>
                          <div className="text-[10px] text-gray-400">{b.contact_phone} • {b.contact_email}</div>
                        </td>
                        <td className="py-3.5 px-4">
                          {b.state_name || "National"}
                        </td>
                        <td className="py-3.5 px-4">
                          <Badge
                            variant={
                              b.approval_status === "approved"
                                ? "primary"
                                : b.approval_status === "pending"
                                ? "warning"
                                : "danger"
                            }
                            size="sm"
                          >
                            {b.approval_status}
                          </Badge>
                        </td>
                        <td className="py-3.5 px-4 text-right space-x-2">
                          <button
                            onClick={() => {
                              setSelectedBusiness(b);
                              setApprovalAction("approve");
                              setShowApprovalModal(true);
                            }}
                            className="px-2.5 py-1 bg-emerald-50 text-emerald-800 hover:bg-emerald-100 font-bold rounded-lg text-[11px] transition"
                          >
                            Approve
                          </button>
                          <button
                            onClick={() => {
                              setSelectedBusiness(b);
                              setApprovalAction("reject");
                              setShowApprovalModal(true);
                            }}
                            className="px-2.5 py-1 bg-red-50 text-red-700 hover:bg-red-100 font-bold rounded-lg text-[11px] transition"
                          >
                            Reject
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

      {/* SECTION 3: PRODUCTS & CATEGORIES */}
      {activeTab === "products" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3">
            <h2 className="text-sm font-bold text-gray-900">Multi-Seller Product Catalog</h2>
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500 font-medium">Filter Status:</span>
              <select
                value={productFilter}
                onChange={(e) => {
                  setProductFilter(e.target.value);
                  loadTabContent("products");
                }}
                className="px-3 py-1.5 text-xs border border-gray-300 rounded-xl bg-white"
              >
                <option value="all">All Products</option>
                <option value="published">Published</option>
                <option value="draft">Draft</option>
                <option value="archived">Archived</option>
              </select>
            </div>
          </div>

          <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-600">
                <thead className="bg-gray-50 text-gray-500 font-semibold border-b border-gray-200">
                  <tr>
                    <th className="py-3 px-4">Product Title</th>
                    <th className="py-3 px-4">Vendor / Store</th>
                    <th className="py-3 px-4">Category</th>
                    <th className="py-3 px-4">Price (₦)</th>
                    <th className="py-3 px-4">Stock</th>
                    <th className="py-3 px-4">Status</th>
                    <th className="py-3 px-4 text-right">Moderation</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {products.map((p) => (
                    <tr key={p.id} className="hover:bg-gray-50/50">
                      <td className="py-3.5 px-4 font-bold text-gray-900">
                        <div className="flex items-center gap-2.5">
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
                            ) : null;
                          })()}
                          <div className="truncate">
                            <span className="truncate block">{p.title}</span>
                            {p.sku && <span className="block text-[10px] text-gray-400 font-mono">{p.sku}</span>}
                          </div>
                        </div>
                      </td>
                      <td className="py-3.5 px-4">{p.business_name}</td>
                      <td className="py-3.5 px-4 capitalize">{p.category}</td>
                      <td className="py-3.5 px-4 font-bold text-gray-900">
                        ₦{(p.price_kobo / 100).toLocaleString()}
                      </td>
                      <td className="py-3.5 px-4">{p.stock_quantity} units</td>
                      <td className="py-3.5 px-4">
                        <Badge
                          variant={p.status === "published" ? "primary" : "neutral"}
                          size="sm"
                        >
                          {p.status}
                        </Badge>
                      </td>
                      <td className="py-3.5 px-4 text-right">
                        <button
                          onClick={() => {
                            setSelectedProduct(p);
                            setModerateStatus(p.status === "published" ? "archived" : "published");
                            setShowModerateModal(true);
                          }}
                          className="px-2.5 py-1 bg-gray-100 hover:bg-gray-200 text-gray-800 font-bold rounded-lg text-[11px] transition"
                        >
                          Change Status
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* SECTION 4: ORDERS */}
      {activeTab === "orders" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3">
            <h2 className="text-sm font-bold text-gray-900">Customer Marketplace Orders</h2>
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500 font-medium">Order Status:</span>
              <select
                value={orderFilter}
                onChange={(e) => {
                  setOrderFilter(e.target.value);
                  loadTabContent("orders");
                }}
                className="px-3 py-1.5 text-xs border border-gray-300 rounded-xl bg-white"
              >
                <option value="all">All Orders</option>
                <option value="placed">Placed</option>
                <option value="processing">Processing</option>
                <option value="dispatched">Dispatched</option>
                <option value="delivered">Delivered</option>
                <option value="cancelled">Cancelled</option>
                <option value="refunded">Refunded</option>
              </select>
            </div>
          </div>

          <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-600">
                <thead className="bg-gray-50 text-gray-500 font-semibold border-b border-gray-200">
                  <tr>
                    <th className="py-3 px-4">Order Number</th>
                    <th className="py-3 px-4">Buyer</th>
                    <th className="py-3 px-4">Method</th>
                    <th className="py-3 px-4">Total Amount</th>
                    <th className="py-3 px-4">Payment</th>
                    <th className="py-3 px-4">Fulfilment</th>
                    <th className="py-3 px-4">Date</th>
                    <th className="py-3 px-4 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {orders.map((ord) => (
                    <tr key={ord.id} className="hover:bg-gray-50/50">
                      <td className="py-3.5 px-4 font-mono font-bold text-gray-900">
                        {ord.order_number}
                      </td>
                      <td className="py-3.5 px-4">
                        <div>{ord.buyer_name}</div>
                        <div className="text-[10px] text-gray-400">{ord.buyer_phone}</div>
                      </td>
                      <td className="py-3.5 px-4 capitalize">{ord.delivery_type}</td>
                      <td className="py-3.5 px-4 font-bold text-gray-900">
                        ₦{(ord.total_kobo / 100).toLocaleString()}
                      </td>
                      <td className="py-3.5 px-4">
                        <Badge
                          variant={ord.payment_status === "paid" ? "primary" : "neutral"}
                          size="sm"
                        >
                          {ord.payment_status}
                        </Badge>
                      </td>
                      <td className="py-3.5 px-4">
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
                          {ord.fulfilment_status}
                        </Badge>
                      </td>
                      <td className="py-3.5 px-4 text-gray-400">
                        {new Date(ord.created_at).toLocaleDateString("en-NG")}
                      </td>
                      <td className="py-3.5 px-4 text-right space-x-1.5 whitespace-nowrap">
                        {ord.payment_status === "paid" && ord.order_status !== "refunded" && (
                          <button
                            onClick={() => {
                              setSelectedOrderForRefund(ord);
                              setRefundAmountNaira((ord.total_kobo / 100).toString());
                              setRefundReason("Administrative customer refund");
                              setShowRefundModal(true);
                            }}
                            className="px-2.5 py-1 text-[11px] font-bold text-red-700 bg-red-50 hover:bg-red-100 rounded-lg border border-red-200 transition"
                          >
                            Refund
                          </button>
                        )}
                        {ord.order_status !== "cancelled" &&
                          ord.order_status !== "refunded" &&
                          ord.fulfilment_status !== "dispatched" &&
                          ord.fulfilment_status !== "delivered" && (
                            <button
                              onClick={() => handleAdminCancelOrder(ord.id, ord.order_number)}
                              className="px-2.5 py-1 text-[11px] font-bold text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg border border-gray-200 transition"
                            >
                              Cancel
                            </button>
                          )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* SECTION 5: PAYMENTS & REFUNDS */}
      {activeTab === "payments" && (
        <div className="space-y-4">
          <h2 className="text-sm font-bold text-gray-900">Payment Audit Trail & Gateway Ledger</h2>

          <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-600">
                <thead className="bg-gray-50 text-gray-500 font-semibold border-b border-gray-200">
                  <tr>
                    <th className="py-3 px-4">Payment Reference</th>
                    <th className="py-3 px-4">Provider</th>
                    <th className="py-3 px-4">Amount</th>
                    <th className="py-3 px-4">Status</th>
                    <th className="py-3 px-4">Gateway Reference</th>
                    <th className="py-3 px-4">Paid At</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {payments.map((p) => (
                    <tr key={p.id} className="hover:bg-gray-50/50">
                      <td className="py-3.5 px-4 font-mono font-bold text-gray-900">
                        {p.payment_reference}
                      </td>
                      <td className="py-3.5 px-4 uppercase font-bold text-[10px] text-gray-500">
                        {p.provider}
                      </td>
                      <td className="py-3.5 px-4 font-bold text-gray-900">
                        ₦{(p.amount_kobo / 100).toLocaleString()}
                      </td>
                      <td className="py-3.5 px-4">
                        <Badge
                          variant={
                            p.status === "paid"
                              ? "primary"
                              : p.status === "refunded"
                              ? "warning"
                              : "neutral"
                          }
                          size="sm"
                        >
                          {p.status}
                        </Badge>
                      </td>
                      <td className="py-3.5 px-4 font-mono text-[10px] text-gray-400">
                        {p.provider_reference || "N/A"}
                      </td>
                      <td className="py-3.5 px-4 text-gray-400">
                        {p.paid_at ? new Date(p.paid_at).toLocaleDateString("en-NG") : "Pending"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* SECTION 6: DELIVERY & FULFILMENT */}
      {activeTab === "fulfilment" && (
        <div className="space-y-4">
          <h2 className="text-sm font-bold text-gray-900">Nationwide Consignment & Transit Tracker</h2>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500">Pending Dispatch</span>
              <div className="text-xl font-bold text-gray-900">
                {orders.filter((o) => o.fulfilment_status === "unfulfilled").length}
              </div>
              <span className="text-[10px] text-gray-400">Awaiting vendor handover</span>
            </div>

            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-blue-700">In Transit / Dispatched</span>
              <div className="text-xl font-bold text-blue-700">
                {orders.filter((o) => o.fulfilment_status === "dispatched").length}
              </div>
              <span className="text-[10px] text-blue-600">With courier / carrier</span>
            </div>

            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-emerald-800">Delivered</span>
              <div className="text-xl font-bold text-emerald-800">
                {orders.filter((o) => o.fulfilment_status === "delivered").length}
              </div>
              <span className="text-[10px] text-emerald-700">Fulfilment confirmed</span>
            </div>
          </div>

          <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-600">
                <thead className="bg-gray-50 text-gray-500 font-semibold border-b border-gray-200">
                  <tr>
                    <th className="py-3 px-4">Order</th>
                    <th className="py-3 px-4">Method</th>
                    <th className="py-3 px-4">Delivery Address / Destination</th>
                    <th className="py-3 px-4">State</th>
                    <th className="py-3 px-4">Fulfilment Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {orders.map((ord) => (
                    <tr key={ord.id} className="hover:bg-gray-50/50">
                      <td className="py-3.5 px-4 font-mono font-bold text-gray-900">
                        {ord.order_number}
                      </td>
                      <td className="py-3.5 px-4 capitalize">{ord.delivery_type}</td>
                      <td className="py-3.5 px-4 max-w-xs truncate">{ord.shipping_address}</td>
                      <td className="py-3.5 px-4">{ord.shipping_state_name || "National"}</td>
                      <td className="py-3.5 px-4">
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
                          {ord.fulfilment_status}
                        </Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* SECTION 7: DISPUTES & REPORTS */}
      {activeTab === "disputes" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3">
            <h2 className="text-sm font-bold text-gray-900">Dispute Arbitration & Buyer Protection Claims</h2>
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500 font-medium">Filter Status:</span>
              <select
                value={disputeFilter}
                onChange={(e) => {
                  setDisputeFilter(e.target.value);
                  loadTabContent("disputes");
                }}
                className="px-3 py-1.5 text-xs border border-gray-300 rounded-xl bg-white"
              >
                <option value="all">All Disputes</option>
                <option value="opened">Open</option>
                <option value="under_review">Under Review</option>
                <option value="resolved_refunded">Resolved & Refunded</option>
                <option value="rejected">Rejected</option>
              </select>
            </div>
          </div>

          {disputes.length === 0 ? (
            <EmptyState
              icon={<AlertTriangle className="w-6 h-6" />}
              title="No Active Disputes"
              description="No disputes logged by buyers on Ratels Marketplace."
            />
          ) : (
            <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-gray-600">
                  <thead className="bg-gray-50 text-gray-500 font-semibold border-b border-gray-200">
                    <tr>
                      <th className="py-3 px-4">Order</th>
                      <th className="py-3 px-4">Complainant</th>
                      <th className="py-3 px-4">Reason</th>
                      <th className="py-3 px-4">Requested Action</th>
                      <th className="py-3 px-4">Status</th>
                      <th className="py-3 px-4 text-right">Arbitration</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {disputes.map((d) => (
                      <tr key={d.id} className="hover:bg-gray-50/50">
                        <td className="py-3.5 px-4 font-mono font-bold text-gray-900">
                          {d.order_number}
                        </td>
                        <td className="py-3.5 px-4">{d.raised_by_name}</td>
                        <td className="py-3.5 px-4 capitalize font-medium text-gray-900">
                          {d.reason.replace(/_/g, " ")}
                        </td>
                        <td className="py-3.5 px-4 capitalize">{d.requested_action}</td>
                        <td className="py-3.5 px-4">
                          <Badge
                            variant={
                              d.status === "resolved_refunded"
                                ? "primary"
                                : d.status === "opened"
                                ? "danger"
                                : "neutral"
                            }
                            size="sm"
                          >
                            {d.status}
                          </Badge>
                        </td>
                        <td className="py-3.5 px-4 text-right">
                          {d.status !== "resolved_refunded" && d.status !== "rejected" ? (
                            <button
                              onClick={() => {
                                setSelectedDispute(d);
                                setShowResolveModal(true);
                              }}
                              className="px-2.5 py-1 bg-emerald-800 text-white font-bold rounded-lg text-[11px] hover:bg-emerald-700 transition"
                            >
                              Arbitrate
                            </button>
                          ) : (
                            <span className="text-[11px] text-gray-400">Resolved</span>
                          )}
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

      {/* Business Approval Modal */}
      {showApprovalModal && selectedBusiness && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <h3 className="font-bold text-sm text-gray-900">
              {approvalAction === "approve" ? "Approve Vendor Enterprise" : "Reject Vendor Profile"}
            </h3>
            <p className="text-xs text-gray-600">
              {approvalAction === "approve"
                ? `Authorizing "${selectedBusiness.name}" will grant them permission to list products and sell goods to members nationwide.`
                : `Rejecting "${selectedBusiness.name}" will prevent their profile and products from appearing on the marketplace.`}
            </p>

            <form onSubmit={handleApproveBusiness} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">
                  Compliance / Audit Reason *
                </label>
                <textarea
                  required
                  rows={3}
                  value={approvalReason}
                  onChange={(e) => setApprovalReason(e.target.value)}
                  placeholder="Enter reason for approval or rejection..."
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setShowApprovalModal(false)}
                  className="flex-1 py-2 bg-gray-100 text-gray-700 font-bold rounded-xl"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={approvalSubmitting}
                  className={`flex-1 py-2 text-white font-bold rounded-xl disabled:opacity-50 ${
                    approvalAction === "approve"
                      ? "bg-emerald-800 hover:bg-emerald-700"
                      : "bg-red-700 hover:bg-red-800"
                  }`}
                >
                  {approvalSubmitting ? "Processing..." : approvalAction === "approve" ? "Confirm Approval" : "Confirm Rejection"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Product Moderation Modal */}
      {showModerateModal && selectedProduct && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-sm w-full p-6 shadow-xl space-y-4">
            <h3 className="font-bold text-sm text-gray-900">Moderate Product Listing</h3>
            <p className="text-xs text-gray-600">
              Product: <span className="font-bold text-gray-900">{selectedProduct.title}</span>
            </p>

            <form onSubmit={handleModerateProduct} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Status</label>
                <select
                  value={moderateStatus}
                  onChange={(e) => setModerateStatus(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                >
                  <option value="published">Published</option>
                  <option value="draft">Draft</option>
                  <option value="archived">Archived</option>
                  <option value="suspended">Suspended (Policy Violation)</option>
                </select>
              </div>

              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setShowModerateModal(false)}
                  className="flex-1 py-2 bg-gray-100 text-gray-700 font-bold rounded-xl"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={moderateSubmitting}
                  className="flex-1 py-2 bg-emerald-800 hover:bg-emerald-700 text-white font-bold rounded-xl disabled:opacity-50"
                >
                  {moderateSubmitting ? "Saving..." : "Update Status"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Dispute Resolution Modal */}
      {showResolveModal && selectedDispute && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <h3 className="font-bold text-sm text-gray-900 flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 text-amber-600" />
              Arbitrate Commercial Dispute
            </h3>
            <p className="text-xs text-gray-600">
              Order: <span className="font-bold text-gray-900">{selectedDispute.order_number}</span> • Claim: <span className="font-semibold text-gray-800">{selectedDispute.reason}</span>
            </p>

            <form onSubmit={handleResolveDispute} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Ruling & Action *</label>
                <select
                  value={resolutionType}
                  onChange={(e) => setResolutionType(e.target.value as any)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                >
                  <option value="resolved_refunded">Approve Refund & Restore Stock to Seller</option>
                  <option value="rejected">Dismiss Claim (Ruling in Vendor Favor)</option>
                </select>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">
                  Administrative Findings & Notes *
                </label>
                <textarea
                  required
                  rows={3}
                  value={resolutionNotes}
                  onChange={(e) => setResolutionNotes(e.target.value)}
                  placeholder="Explain findings, evidence examined, and arbitration verdict..."
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setShowResolveModal(false)}
                  className="flex-1 py-2 bg-gray-100 text-gray-700 font-bold rounded-xl"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={resolveSubmitting}
                  className="flex-1 py-2 bg-emerald-800 hover:bg-emerald-700 text-white font-bold rounded-xl disabled:opacity-50"
                >
                  {resolveSubmitting ? "Executing..." : "Execute Ruling"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Refund Modal */}
      {showRefundModal && selectedOrderForRefund && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <h3 className="font-bold text-sm text-gray-900 flex items-center gap-2">
              <DollarSign className="w-4 h-4 text-emerald-700" />
              Process Order Refund
            </h3>
            <p className="text-xs text-gray-600">
              Order: <span className="font-bold font-mono text-gray-900">{selectedOrderForRefund.order_number}</span> • Total Paid: <span className="font-bold text-gray-900">₦{(selectedOrderForRefund.total_kobo / 100).toLocaleString()}</span>
            </p>

            <form onSubmit={handleProcessRefund} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Refund Amount (NGN) *</label>
                <input
                  type="number"
                  step="any"
                  required
                  value={refundAmountNaira}
                  onChange={(e) => setRefundAmountNaira(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500 font-bold"
                />
                <p className="text-[10px] text-gray-400 mt-1">Full refund will automatically restore inventory stock to vendors.</p>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">
                  Reason for Refund *
                </label>
                <textarea
                  required
                  rows={3}
                  value={refundReason}
                  onChange={(e) => setRefundReason(e.target.value)}
                  placeholder="Reason for administrative refund..."
                  className="w-full px-3 py-2 border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-500"
                />
              </div>

              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setShowRefundModal(false)}
                  className="flex-1 py-2 bg-gray-100 text-gray-700 font-bold rounded-xl"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={refundSubmitting}
                  className="flex-1 py-2 bg-red-700 hover:bg-red-800 text-white font-bold rounded-xl disabled:opacity-50"
                >
                  {refundSubmitting ? "Processing..." : "Confirm Refund"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
