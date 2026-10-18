"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Boxes,
  PackagePlus,
  Warehouse,
  Truck,
  AlertTriangle,
  FileSpreadsheet,
  CheckCircle2,
  XCircle,
  Clock,
  ArrowRight,
  ArrowLeft,
  Filter,
  Plus,
  Search,
  RefreshCw,
  Eye,
  ShieldCheck,
  Send,
  CornerDownRight,
  X,
  FileCheck2,
  Layers,
  ArrowUpRight,
  ArrowDownLeft,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import {
  InventoryItem,
  StorageLocation,
  StockBalance,
  MaterialRequest,
  MaterialTransfer,
  TransferDiscrepancy,
  StockLedgerEntry,
  InventorySummary,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

type TabType = "overview" | "items" | "locations" | "requests" | "transfers" | "discrepancies" | "ledger";

export default function AdminInventoryPage() {
  const router = useRouter();
  const { user, isCoordinator, isSuperAdmin, isLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<TabType>("overview");
  const [loading, setLoading] = useState(true);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  // Live Data States
  const [summary, setSummary] = useState<InventorySummary | null>(null);
  const [items, setItems] = useState<InventoryItem[]>([]);
  const [locations, setLocations] = useState<StorageLocation[]>([]);
  const [balances, setBalances] = useState<StockBalance[]>([]);
  const [lowStockAlerts, setLowStockAlerts] = useState<StockBalance[]>([]);
  const [requests, setRequests] = useState<MaterialRequest[]>([]);
  const [transfers, setTransfers] = useState<MaterialTransfer[]>([]);
  const [discrepancies, setDiscrepancies] = useState<TransferDiscrepancy[]>([]);
  const [ledgerEntries, setLedgerEntries] = useState<StockLedgerEntry[]>([]);
  const [ledgerTotal, setLedgerTotal] = useState<number>(0);

  // Filters
  const [itemCategoryFilter, setItemCategoryFilter] = useState("");
  const [itemSearch, setItemSearch] = useState("");
  const [locationTypeFilter, setLocationTypeFilter] = useState("");
  const [requestStatusFilter, setRequestStatusFilter] = useState("");
  const [transferStatusFilter, setTransferStatusFilter] = useState("");
  const [discrepancyStatusFilter, setDiscrepancyStatusFilter] = useState("");
  const [ledgerLocationFilter, setLedgerLocationFilter] = useState("");
  const [ledgerMovementFilter, setLedgerMovementFilter] = useState("");

  // Modals & Action States
  const [showAddItemModal, setShowAddItemModal] = useState(false);
  const [showAddLocationModal, setShowAddLocationModal] = useState(false);
  const [showReceiveStockModal, setShowReceiveStockModal] = useState(false);
  const [showCreateRequestModal, setShowCreateRequestModal] = useState(false);
  const [showDispatchModal, setShowDispatchModal] = useState(false);
  const [selectedReviewRequest, setSelectedReviewRequest] = useState<MaterialRequest | null>(null);
  const [selectedAllocateRequest, setSelectedAllocateRequest] = useState<MaterialRequest | null>(null);
  const [selectedReceiveTransfer, setSelectedReceiveTransfer] = useState<MaterialTransfer | null>(null);
  const [transferItemsForReceipt, setTransferItemsForReceipt] = useState<any[]>([]);
  const [selectedResolveDiscrepancy, setSelectedResolveDiscrepancy] = useState<TransferDiscrepancy | null>(null);
  const [viewWarehouseBalances, setViewWarehouseBalances] = useState<StorageLocation | null>(null);

  // Form Inputs
  const [newItem, setNewItem] = useState({
    sku: "",
    name: "",
    description: "",
    category: "sanitation_gear",
    unit_of_measure: "pcs",
    min_threshold_quantity: 10,
    is_batch_tracked: false,
  });

  const [newLocation, setNewLocation] = useState({
    code: "",
    name: "",
    location_type: "national",
    address: "",
    contact_phone: "",
  });

  const [receiveStockInput, setReceiveStockInput] = useState({
    location_id: "",
    item_id: "",
    quantity: 10,
    batch_number: "",
    movement_type: "initial_receipt",
    notes: "",
  });

  const [newRequestInput, setNewRequestInput] = useState({
    destination_location_id: "",
    jurisdiction_type: "national",
    intended_use: "",
    required_date: new Date(Date.now() + 7 * 86400000).toISOString().split("T")[0],
    item_id: "",
    quantity: 10,
  });

  const [reviewInput, setReviewInput] = useState({
    action: "approve",
    rejection_reason: "",
    reduction_reason: "",
    approved_quantity: 0,
  });

  const [allocateInput, setAllocateInput] = useState({
    supplying_location_id: "",
  });

  const [dispatchInput, setDispatchInput] = useState({
    request_id: "",
    source_location_id: "",
    destination_location_id: "",
    item_id: "",
    quantity: 10,
    carrier_name: "",
    carrier_phone: "",
    tracking_number: "",
    dispatch_notes: "",
    auto_dispatch: true,
  });

  const [receiptQuantities, setReceiptQuantities] = useState<Record<string, number>>({});
  const [receiptDiscrepancyReason, setReceiptDiscrepancyReason] = useState("missing_in_transit");
  const [receiptEvidenceURL, setReceiptEvidenceURL] = useState("");

  const [resolveDiscrepancyInput, setResolveDiscrepancyInput] = useState({
    resolution: "write_off",
    notes: "",
  });

  const loadAllData = async () => {
    try {
      setLoading(true);
      const [sum, itms, locs, bals, alerts, reqs, trfs, discs, led] = await Promise.all([
        apiRequest<InventorySummary>("/inventory/summary").catch(() => null),
        apiRequest<InventoryItem[]>("/inventory/items").catch(() => []),
        apiRequest<StorageLocation[]>("/inventory/locations").catch(() => []),
        apiRequest<StockBalance[]>("/inventory/balances").catch(() => []),
        apiRequest<StockBalance[]>("/inventory/balances/low-stock").catch(() => []),
        apiRequest<MaterialRequest[]>("/inventory/requests").catch(() => []),
        apiRequest<MaterialTransfer[]>("/inventory/transfers").catch(() => []),
        apiRequest<TransferDiscrepancy[]>("/inventory/discrepancies").catch(() => []),
        apiRequest<{ entries: StockLedgerEntry[]; total: number }>("/inventory/ledger?limit=50").catch(() => ({ entries: [], total: 0 })),
      ]);

      setSummary(sum);
      setItems(Array.isArray(itms) ? itms : []);
      setLocations(Array.isArray(locs) ? locs : []);
      setBalances(Array.isArray(bals) ? bals : []);
      setLowStockAlerts(Array.isArray(alerts) ? alerts : []);
      setRequests(Array.isArray(reqs) ? reqs : []);
      setTransfers(Array.isArray(trfs) ? trfs : []);
      setDiscrepancies(Array.isArray(discs) ? discs : []);
      setLedgerEntries(Array.isArray(led?.entries) ? led.entries : []);
      setLedgerTotal(typeof led?.total === "number" ? led.total : 0);
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to load inventory data" });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isLoading && (!user || !isCoordinator)) {
      router.push("/login?redirect=/admin/inventory");
      return;
    }
    if (user && isCoordinator) {
      loadAllData();
    }
  }, [user, isCoordinator, isLoading, router]);

  // Handlers for Mutations

  const handleCreateItem = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await apiRequest("/inventory/items", {
        method: "POST",
        body: JSON.stringify(newItem),
      });
      setFeedback({ type: "success", message: `Item '${newItem.name}' added successfully` });
      setShowAddItemModal(false);
      setNewItem({
        sku: "",
        name: "",
        description: "",
        category: "sanitation_gear",
        unit_of_measure: "pcs",
        min_threshold_quantity: 10,
        is_batch_tracked: false,
      });
      loadAllData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to create item" });
    }
  };

  const handleCreateLocation = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await apiRequest("/inventory/locations", {
        method: "POST",
        body: JSON.stringify(newLocation),
      });
      setFeedback({ type: "success", message: `Storage location '${newLocation.name}' added successfully` });
      setShowAddLocationModal(false);
      setNewLocation({ code: "", name: "", location_type: "national", address: "", contact_phone: "" });
      loadAllData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to create storage location" });
    }
  };

  const handleReceiveStock = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await apiRequest("/inventory/balances/receive", {
        method: "POST",
        body: JSON.stringify({
          location_id: receiveStockInput.location_id,
          item_id: receiveStockInput.item_id,
          quantity: Number(receiveStockInput.quantity),
          batch_number: receiveStockInput.batch_number,
          movement_type: receiveStockInput.movement_type,
          notes: receiveStockInput.notes || undefined,
        }),
      });
      setFeedback({ type: "success", message: "Stock received and logged to immutable ledger successfully" });
      setShowReceiveStockModal(false);
      loadAllData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to receive stock" });
    }
  };

  const handleCreateRequest = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await apiRequest("/inventory/requests", {
        method: "POST",
        body: JSON.stringify({
          destination_location_id: newRequestInput.destination_location_id,
          jurisdiction_type: newRequestInput.jurisdiction_type,
          intended_use: newRequestInput.intended_use,
          required_date: new Date(newRequestInput.required_date).toISOString(),
          items: [
            {
              item_id: newRequestInput.item_id,
              requested_quantity: Number(newRequestInput.quantity),
              unit_of_measure: "pcs",
            },
          ],
        }),
      });
      setFeedback({ type: "success", message: "Material request submitted successfully" });
      setShowCreateRequestModal(false);
      loadAllData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to create material request" });
    }
  };

  const handleReviewRequest = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedReviewRequest) return;
    try {
      const payload: any = { action: reviewInput.action };
      if (reviewInput.action === "reject") {
        payload.rejection_reason = reviewInput.rejection_reason;
      } else if (reviewInput.action === "reduce") {
        payload.reduction_reason = reviewInput.reduction_reason;
      }
      await apiRequest(`/inventory/requests/${selectedReviewRequest.id}/review`, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      setFeedback({ type: "success", message: `Request ${selectedReviewRequest.reference} ${reviewInput.action}d successfully` });
      setSelectedReviewRequest(null);
      loadAllData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to review request" });
    }
  };

  const handleAllocateRequest = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedAllocateRequest) return;
    try {
      await apiRequest(`/inventory/requests/${selectedAllocateRequest.id}/allocate`, {
        method: "POST",
        body: JSON.stringify({ supplying_location_id: allocateInput.supplying_location_id }),
      });
      setFeedback({ type: "success", message: `Stock successfully reserved and allocated from supplying warehouse` });
      setSelectedAllocateRequest(null);
      loadAllData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Stock allocation failed" });
    }
  };

  const handleCreateAndDispatchTransfer = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await apiRequest("/inventory/transfers", {
        method: "POST",
        body: JSON.stringify({
          source_location_id: dispatchInput.source_location_id,
          destination_location_id: dispatchInput.destination_location_id,
          request_id: dispatchInput.request_id || undefined,
          carrier_name: dispatchInput.carrier_name,
          carrier_phone: dispatchInput.carrier_phone,
          tracking_number: dispatchInput.tracking_number,
          dispatch_notes: dispatchInput.dispatch_notes,
          items: [
            {
              item_id: dispatchInput.item_id,
              dispatched_quantity: Number(dispatchInput.quantity),
              unit_of_measure: "pcs",
            },
          ],
          auto_dispatch: dispatchInput.auto_dispatch,
        }),
      });
      setFeedback({ type: "success", message: "Material transfer dispatched successfully. Stock in transit." });
      setShowDispatchModal(false);
      loadAllData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Transfer dispatch failed" });
    }
  };

  const openReceiveModal = async (t: MaterialTransfer) => {
    setSelectedReceiveTransfer(t);
    try {
      const details = await apiRequest<{ transfer: MaterialTransfer; items: any[] }>(`/inventory/transfers/${t.id}`);
      setTransferItemsForReceipt(details.items || []);
      const initial: Record<string, number> = {};
      (details.items || []).forEach((itm) => {
        initial[itm.item_id] = itm.dispatched_quantity;
      });
      setReceiptQuantities(initial);
    } catch (err) {
      setTransferItemsForReceipt([]);
    }
  };

  const handleConfirmReceipt = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedReceiveTransfer) return;
    try {
      const itemsPayload = Object.entries(receiptQuantities).map(([item_id, received_quantity]) => ({
        item_id,
        received_quantity: Number(received_quantity),
      }));

      const res = await apiRequest(`/inventory/transfers/${selectedReceiveTransfer.id}/receive`, {
        method: "POST",
        body: JSON.stringify({
          items: itemsPayload,
          discrepancy_reason: receiptDiscrepancyReason,
          supporting_evidence_url: receiptEvidenceURL || undefined,
        }),
      });

      if (res.status === "disputed") {
        setFeedback({
          type: "error",
          message: `Shipment received with discrepancy. Discrepancy logged for review.`,
        });
      } else {
        setFeedback({
          type: "success",
          message: `Shipment ${selectedReceiveTransfer.reference} confirmed and destination warehouse stock credited.`,
        });
      }
      setSelectedReceiveTransfer(null);
      loadAllData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Receipt confirmation failed" });
    }
  };

  const handleResolveDiscrepancy = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedResolveDiscrepancy) return;
    try {
      await apiRequest(`/inventory/discrepancies/${selectedResolveDiscrepancy.id}/resolve`, {
        method: "POST",
        body: JSON.stringify(resolveDiscrepancyInput),
      });
      setFeedback({ type: "success", message: "Discrepancy resolved and compensating ledger recorded" });
      setSelectedResolveDiscrepancy(null);
      loadAllData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Resolution failed" });
    }
  };

  if (isLoading || loading) return <Spinner text="Loading Nigerian Ratels Inventory System..." />;
  if (!user) return null;

  // Filtered Lists
  const filteredItems = items.filter((itm) => {
    if (itemCategoryFilter && itm.category !== itemCategoryFilter) return false;
    if (itemSearch) {
      const q = itemSearch.toLowerCase();
      return itm.name.toLowerCase().includes(q) || itm.sku.toLowerCase().includes(q);
    }
    return true;
  });

  const filteredLocations = locations.filter((loc) => {
    if (locationTypeFilter && loc.location_type !== locationTypeFilter) return false;
    return true;
  });

  const filteredRequests = requests.filter((r) => {
    if (requestStatusFilter && r.status !== requestStatusFilter) return false;
    return true;
  });

  const filteredTransfers = transfers.filter((t) => {
    if (transferStatusFilter && t.status !== transferStatusFilter) return false;
    return true;
  });

  const filteredDiscrepancies = discrepancies.filter((d) => {
    if (discrepancyStatusFilter && d.status !== discrepancyStatusFilter) return false;
    return true;
  });

  const filteredLedger = ledgerEntries.filter((l) => {
    if (ledgerLocationFilter && l.location_id !== ledgerLocationFilter) return false;
    if (ledgerMovementFilter && l.movement_type !== ledgerMovementFilter) return false;
    return true;
  });

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b border-gray-200 pb-6">
        <div>
          <div className="flex items-center gap-2">
            <Link
              href="/admin"
              className="text-xs font-semibold text-emerald-800 hover:text-emerald-950 flex items-center gap-1 transition"
            >
              <ArrowLeft className="w-3.5 h-3.5" /> Back to Admin Overview
            </Link>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 mt-1 flex items-center gap-3">
            <Boxes className="w-8 h-8 text-emerald-800" />
            Inventory & Material Distribution
          </h1>
          <p className="text-xs text-gray-500 mt-1">
            Nigerian Ratels Complete Blueprint v2.0 Section 05 — Nationwide Supply Chain & Multi-Tier Material Tracking
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => loadAllData()}
            className="p-2 border border-gray-300 rounded-lg hover:bg-gray-50 text-gray-600 transition"
            title="Refresh"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
          <button
            onClick={() => setShowReceiveStockModal(true)}
            className="px-3.5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white text-xs font-bold rounded-lg shadow-sm flex items-center gap-1.5 transition"
          >
            <Plus className="w-4 h-4" /> Receive Stock
          </button>
          <button
            onClick={() => setShowDispatchModal(true)}
            className="px-3.5 py-2 bg-emerald-950 hover:bg-black text-white text-xs font-bold rounded-lg shadow-sm flex items-center gap-1.5 transition"
          >
            <Truck className="w-4 h-4" /> Dispatch Shipment
          </button>
        </div>
      </div>

      {feedback && (
        <Alert type={feedback.type}>
          <div className="flex justify-between items-center">
            <span>{feedback.message}</span>
            <button
              onClick={() => setFeedback(null)}
              className="text-xs font-bold underline ml-4 hover:opacity-80"
            >
              Dismiss
            </button>
          </div>
        </Alert>
      )}

      {/* Tabs Navigation */}
      <div className="flex overflow-x-auto border-b border-gray-200 gap-1 pb-1 text-xs font-semibold scrollbar-none">
        {[
          { id: "overview", label: "Overview & Alerts", icon: Boxes },
          { id: "items", label: `Catalogue (${items.length})`, icon: Layers },
          { id: "locations", label: `Warehouses (${locations.length})`, icon: Warehouse },
          { id: "requests", label: `Material Requests (${requests.length})`, icon: FileCheck2 },
          { id: "transfers", label: `Transfers & In-Transit (${transfers.length})`, icon: Truck },
          { id: "discrepancies", label: `Discrepancies (${discrepancies.length})`, icon: AlertTriangle },
          { id: "ledger", label: "Immutable Stock Ledger", icon: FileSpreadsheet },
        ].map((tab) => {
          const Icon = tab.icon;
          const isActive = activeTab === tab.id;
          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as TabType)}
              className={`flex items-center gap-2 px-4 py-2.5 rounded-lg whitespace-nowrap transition ${
                isActive
                  ? "bg-emerald-900 text-white shadow-sm"
                  : "text-gray-600 hover:text-gray-900 hover:bg-gray-100"
              }`}
            >
              <Icon className="w-4 h-4" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* ========================================================================= */}
      {/* 1. OVERVIEW SUB-VIEW */}
      {/* ========================================================================= */}
      {activeTab === "overview" && (
        <div className="space-y-6">
          {/* KPI Cards */}
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Catalogue Items</span>
              <div className="text-2xl font-bold text-gray-900">{summary?.total_items ?? items.length}</div>
              <span className="text-[10px] text-gray-400">Operational supplies</span>
            </div>
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Active Warehouses</span>
              <div className="text-2xl font-bold text-emerald-800">{summary?.total_locations ?? locations.length}</div>
              <span className="text-[10px] text-gray-400">National/Zonal/State/LGA</span>
            </div>
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">In-Transit Shipments</span>
              <div className="text-2xl font-bold text-amber-700">{summary?.in_transit_transfers ?? 0}</div>
              <span className="text-[10px] text-gray-400">Dispatched, awaiting arrival</span>
            </div>
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Open Discrepancies</span>
              <div className="text-2xl font-bold text-rose-700">{summary?.open_discrepancies ?? discrepancies.filter(d => d.status === "open").length}</div>
              <span className="text-[10px] text-gray-400">Delivery quantity mismatch</span>
            </div>
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Low Stock Alerts</span>
              <div className="text-2xl font-bold text-rose-800">{lowStockAlerts.length}</div>
              <span className="text-[10px] text-gray-400">Under replenishment threshold</span>
            </div>
          </div>

          {/* Low Stock Alerts Banner */}
          {lowStockAlerts.length > 0 && (
            <div className="bg-rose-50 border border-rose-200 rounded-xl p-5 space-y-3">
              <div className="flex items-center gap-2 text-rose-900 font-bold text-sm">
                <AlertTriangle className="w-5 h-5 text-rose-600" />
                Critical Low Stock Notice ({lowStockAlerts.length} items below minimum safety threshold)
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                {lowStockAlerts.slice(0, 6).map((al, idx) => (
                  <div key={idx} className="bg-white p-3 rounded-lg border border-rose-100 shadow-2xs text-xs space-y-1">
                    <div className="font-bold text-gray-900 flex justify-between">
                      <span>{al.item_name}</span>
                      <span className="text-rose-600 font-extrabold">{al.available_quantity} {al.unit_of_measure}</span>
                    </div>
                    <div className="text-gray-500 text-[11px] flex justify-between">
                      <span>{al.location_name}</span>
                      <span>Min Threshold: {al.min_threshold_quantity}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* In-Transit Quick Receipt List */}
          <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-200 flex justify-between items-center">
              <h2 className="text-sm font-bold text-gray-900 flex items-center gap-2">
                <Truck className="w-4 h-4 text-emerald-800" />
                Active In-Transit Dispatches Requiring Destination Confirmation
              </h2>
              <button
                onClick={() => setActiveTab("transfers")}
                className="text-xs text-emerald-800 font-bold hover:underline flex items-center gap-1"
              >
                View All Transfers <ArrowRight className="w-3 h-3" />
              </button>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-700">
                <thead className="bg-gray-50 text-gray-500 uppercase tracking-wider text-[11px] border-b border-gray-200">
                  <tr>
                    <th className="px-6 py-3">Reference</th>
                    <th className="px-6 py-3">Origin Depot</th>
                    <th className="px-6 py-3">Destination Depot</th>
                    <th className="px-6 py-3">Carrier / Tracking</th>
                    <th className="px-6 py-3">Status</th>
                    <th className="px-6 py-3 text-right">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {transfers.filter(t => t.status === "dispatched").length === 0 ? (
                    <tr>
                      <td colSpan={6} className="px-6 py-8 text-center text-gray-400">
                        No shipments currently in transit.
                      </td>
                    </tr>
                  ) : (
                    transfers.filter(t => t.status === "dispatched").map((t) => (
                      <tr key={t.id} className="hover:bg-gray-50 transition">
                        <td className="px-6 py-4 font-mono font-bold text-gray-900">{t.reference}</td>
                        <td className="px-6 py-4">{t.source_location_name}</td>
                        <td className="px-6 py-4 font-semibold text-emerald-950">{t.destination_location_name}</td>
                        <td className="px-6 py-4 text-gray-500">
                          {t.carrier_name || "Official Logistics"} {t.tracking_number && `(${t.tracking_number})`}
                        </td>
                        <td className="px-6 py-4">
                          <Badge variant="warning">In-Transit</Badge>
                        </td>
                        <td className="px-6 py-4 text-right">
                          <button
                            onClick={() => openReceiveModal(t)}
                            className="px-3 py-1.5 bg-emerald-800 hover:bg-emerald-900 text-white rounded-md text-xs font-bold transition shadow-xs"
                          >
                            Confirm Receipt
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* 2. CATALOGUE SUB-VIEW */}
      {/* ========================================================================= */}
      {activeTab === "items" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
            <div className="flex flex-wrap items-center gap-3 w-full sm:w-auto">
              <div className="relative w-full sm:w-64">
                <Search className="w-4 h-4 text-gray-400 absolute left-3 top-3" />
                <input
                  type="text"
                  placeholder="Search SKU or name..."
                  value={itemSearch}
                  onChange={(e) => setItemSearch(e.target.value)}
                  className="w-full pl-9 pr-3 py-2 border border-gray-300 rounded-lg text-xs focus:ring-1 focus:ring-emerald-700 outline-none"
                />
              </div>

              <select
                value={itemCategoryFilter}
                onChange={(e) => setItemCategoryFilter(e.target.value)}
                className="px-3 py-2 border border-gray-300 rounded-lg text-xs focus:ring-1 focus:ring-emerald-700 outline-none"
              >
                <option value="">All Categories</option>
                <option value="sanitation_gear">Sanitation Gear</option>
                <option value="protective_equipment">Protective Equipment</option>
                <option value="tools">Clean-up Tools</option>
                <option value="first_aid">First Aid & Medical</option>
                <option value="promotional_materials">Promotional Materials</option>
                <option value="identity_gear">Official Identity Gear</option>
              </select>
            </div>

            <button
              onClick={() => setShowAddItemModal(true)}
              className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white text-xs font-bold rounded-lg shadow-sm flex items-center gap-1.5 transition"
            >
              <PackagePlus className="w-4 h-4" /> Add Item to Catalogue
            </button>
          </div>

          <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-700">
                <thead className="bg-gray-50 text-gray-500 uppercase tracking-wider text-[11px] border-b border-gray-200">
                  <tr>
                    <th className="px-6 py-3">SKU</th>
                    <th className="px-6 py-3">Name & Description</th>
                    <th className="px-6 py-3">Category</th>
                    <th className="px-6 py-3">Unit</th>
                    <th className="px-6 py-3">Safety Threshold</th>
                    <th className="px-6 py-3">Batch Tracked</th>
                    <th className="px-6 py-3">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {filteredItems.map((itm) => (
                    <tr key={itm.id} className="hover:bg-gray-50 transition">
                      <td className="px-6 py-4 font-mono font-bold text-gray-900">{itm.sku}</td>
                      <td className="px-6 py-4">
                        <div className="font-bold text-gray-900">{itm.name}</div>
                        {itm.description && <div className="text-[11px] text-gray-500">{itm.description}</div>}
                      </td>
                      <td className="px-6 py-4">
                        <Badge variant="neutral">{itm.category.replace(/_/g, " ")}</Badge>
                      </td>
                      <td className="px-6 py-4 font-semibold">{itm.unit_of_measure}</td>
                      <td className="px-6 py-4 font-bold text-gray-900">{itm.min_threshold_quantity}</td>
                      <td className="px-6 py-4">{itm.is_batch_tracked ? "Yes (Batch/Lot)" : "No"}</td>
                      <td className="px-6 py-4">
                        <Badge variant={itm.is_active ? "success" : "danger"}>
                          {itm.is_active ? "Active" : "Inactive"}
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

      {/* ========================================================================= */}
      {/* 3. WAREHOUSES & DEPOTS SUB-VIEW */}
      {/* ========================================================================= */}
      {activeTab === "locations" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
            <div className="flex items-center gap-3">
              <select
                value={locationTypeFilter}
                onChange={(e) => setLocationTypeFilter(e.target.value)}
                className="px-3 py-2 border border-gray-300 rounded-lg text-xs focus:ring-1 focus:ring-emerald-700 outline-none"
              >
                <option value="">All Administrative Tiers</option>
                <option value="national">National Hub</option>
                <option value="zonal">Zonal Storage Depot</option>
                <option value="state">State Sanitation Depot</option>
                <option value="lga">LGA / Area Council Store</option>
              </select>
            </div>

            <button
              onClick={() => setShowAddLocationModal(true)}
              className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white text-xs font-bold rounded-lg shadow-sm flex items-center gap-1.5 transition"
            >
              <Warehouse className="w-4 h-4" /> Add Storage Location
            </button>
          </div>

          <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-700">
                <thead className="bg-gray-50 text-gray-500 uppercase tracking-wider text-[11px] border-b border-gray-200">
                  <tr>
                    <th className="px-6 py-3">Code</th>
                    <th className="px-6 py-3">Depot Name</th>
                    <th className="px-6 py-3">Tier</th>
                    <th className="px-6 py-3">Scope / Jurisdiction</th>
                    <th className="px-6 py-3">Custodian</th>
                    <th className="px-6 py-3">Contact</th>
                    <th className="px-6 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {filteredLocations.map((loc) => (
                    <tr key={loc.id} className="hover:bg-gray-50 transition">
                      <td className="px-6 py-4 font-mono font-bold text-gray-900">{loc.code}</td>
                      <td className="px-6 py-4 font-bold text-gray-900">
                        {loc.name}
                        {loc.address && <div className="text-[11px] text-gray-500 font-normal">{loc.address}</div>}
                      </td>
                      <td className="px-6 py-4">
                        <Badge variant="primary">{loc.location_type.toUpperCase()}</Badge>
                      </td>
                      <td className="px-6 py-4 text-gray-600">
                        {loc.state_name || loc.zone_name || loc.lga_name || "National"}
                      </td>
                      <td className="px-6 py-4">{loc.custodian_name || "Unassigned"}</td>
                      <td className="px-6 py-4 text-gray-500">{loc.contact_phone || "—"}</td>
                      <td className="px-6 py-4 text-right">
                        <button
                          onClick={() => setViewWarehouseBalances(loc)}
                          className="px-2.5 py-1.5 border border-gray-300 hover:bg-gray-50 rounded-md text-xs font-semibold text-gray-700 transition"
                        >
                          View Stock
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

      {/* ========================================================================= */}
      {/* 4. MATERIAL REQUESTS SUB-VIEW */}
      {/* ========================================================================= */}
      {activeTab === "requests" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
            <div className="flex items-center gap-3">
              <select
                value={requestStatusFilter}
                onChange={(e) => setRequestStatusFilter(e.target.value)}
                className="px-3 py-2 border border-gray-300 rounded-lg text-xs focus:ring-1 focus:ring-emerald-700 outline-none"
              >
                <option value="">All Statuses</option>
                <option value="submitted">Submitted (Pending Review)</option>
                <option value="approved">Approved</option>
                <option value="reduced">Reduced</option>
                <option value="allocated">Allocated / Reserved</option>
                <option value="fulfilled">Fulfilled</option>
                <option value="rejected">Rejected</option>
              </select>
            </div>

            <button
              onClick={() => setShowCreateRequestModal(true)}
              className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white text-xs font-bold rounded-lg shadow-sm flex items-center gap-1.5 transition"
            >
              <FileCheck2 className="w-4 h-4" /> New Material Request
            </button>
          </div>

          <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-700">
                <thead className="bg-gray-50 text-gray-500 uppercase tracking-wider text-[11px] border-b border-gray-200">
                  <tr>
                    <th className="px-6 py-3">Reference</th>
                    <th className="px-6 py-3">Requester</th>
                    <th className="px-6 py-3">Destination Depot</th>
                    <th className="px-6 py-3">Intended Use</th>
                    <th className="px-6 py-3">Required Date</th>
                    <th className="px-6 py-3">Status</th>
                    <th className="px-6 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {filteredRequests.map((req) => (
                    <tr key={req.id} className="hover:bg-gray-50 transition">
                      <td className="px-6 py-4 font-mono font-bold text-gray-900">{req.reference}</td>
                      <td className="px-6 py-4">
                        <div className="font-bold text-gray-900">{req.requesting_user_name || "Official"}</div>
                        <div className="text-[11px] text-gray-400">{req.requesting_user_email}</div>
                      </td>
                      <td className="px-6 py-4 font-semibold text-emerald-950">{req.destination_location_name}</td>
                      <td className="px-6 py-4 max-w-xs truncate">{req.intended_use}</td>
                      <td className="px-6 py-4 font-mono">{new Date(req.required_date).toLocaleDateString()}</td>
                      <td className="px-6 py-4">
                        <Badge
                          variant={
                            req.status === "fulfilled"
                              ? "success"
                              : req.status === "approved" || req.status === "allocated"
                              ? "primary"
                              : req.status === "reduced"
                              ? "warning"
                              : req.status === "rejected"
                              ? "danger"
                              : "neutral"
                          }
                        >
                          {req.status.toUpperCase()}
                        </Badge>
                      </td>
                      <td className="px-6 py-4 text-right space-x-2">
                        {req.status === "submitted" && (
                          <button
                            onClick={() => setSelectedReviewRequest(req)}
                            className="px-2.5 py-1.5 bg-emerald-800 hover:bg-emerald-900 text-white rounded-md text-xs font-bold transition shadow-xs"
                          >
                            Review
                          </button>
                        )}
                        {(req.status === "approved" || req.status === "reduced") && (
                          <button
                            onClick={() => setSelectedAllocateRequest(req)}
                            className="px-2.5 py-1.5 bg-emerald-950 hover:bg-black text-white rounded-md text-xs font-bold transition shadow-xs"
                          >
                            Allocate Stock
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

      {/* ========================================================================= */}
      {/* 5. TRANSFERS & DISPATCHES SUB-VIEW */}
      {/* ========================================================================= */}
      {activeTab === "transfers" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
            <div className="flex items-center gap-3">
              <select
                value={transferStatusFilter}
                onChange={(e) => setTransferStatusFilter(e.target.value)}
                className="px-3 py-2 border border-gray-300 rounded-lg text-xs focus:ring-1 focus:ring-emerald-700 outline-none"
              >
                <option value="">All Transfer Statuses</option>
                <option value="draft">Draft</option>
                <option value="dispatched">Dispatched (In-Transit)</option>
                <option value="received">Received (Full)</option>
                <option value="disputed">Disputed / Discrepancy</option>
              </select>
            </div>

            <button
              onClick={() => setShowDispatchModal(true)}
              className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white text-xs font-bold rounded-lg shadow-sm flex items-center gap-1.5 transition"
            >
              <Truck className="w-4 h-4" /> Create & Dispatch Shipment
            </button>
          </div>

          <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-700">
                <thead className="bg-gray-50 text-gray-500 uppercase tracking-wider text-[11px] border-b border-gray-200">
                  <tr>
                    <th className="px-6 py-3">Reference</th>
                    <th className="px-6 py-3">Source Warehouse</th>
                    <th className="px-6 py-3">Destination Warehouse</th>
                    <th className="px-6 py-3">Carrier / Tracking</th>
                    <th className="px-6 py-3">Status</th>
                    <th className="px-6 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {filteredTransfers.map((t) => (
                    <tr key={t.id} className="hover:bg-gray-50 transition">
                      <td className="px-6 py-4 font-mono font-bold text-gray-900">{t.reference}</td>
                      <td className="px-6 py-4 font-semibold">{t.source_location_name}</td>
                      <td className="px-6 py-4 font-semibold text-emerald-950">{t.destination_location_name}</td>
                      <td className="px-6 py-4 text-gray-500">
                        {t.carrier_name || "—"} {t.tracking_number && `(${t.tracking_number})`}
                      </td>
                      <td className="px-6 py-4">
                        <Badge
                          variant={
                            t.status === "received"
                              ? "success"
                              : t.status === "dispatched"
                              ? "warning"
                              : t.status === "disputed"
                              ? "danger"
                              : "neutral"
                          }
                        >
                          {t.status.toUpperCase()}
                        </Badge>
                      </td>
                      <td className="px-6 py-4 text-right">
                        {t.status === "dispatched" && (
                          <button
                            onClick={() => openReceiveModal(t)}
                            className="px-3 py-1.5 bg-emerald-800 hover:bg-emerald-900 text-white rounded-md text-xs font-bold transition shadow-xs"
                          >
                            Confirm Receipt
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

      {/* ========================================================================= */}
      {/* 6. DISCREPANCIES SUB-VIEW */}
      {/* ========================================================================= */}
      {activeTab === "discrepancies" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
            <div className="flex items-center gap-3">
              <select
                value={discrepancyStatusFilter}
                onChange={(e) => setDiscrepancyStatusFilter(e.target.value)}
                className="px-3 py-2 border border-gray-300 rounded-lg text-xs focus:ring-1 focus:ring-emerald-700 outline-none"
              >
                <option value="">All Discrepancy Statuses</option>
                <option value="open">Open (Unresolved)</option>
                <option value="resolved_writeoff">Resolved (Loss Write-Off)</option>
                <option value="resolved_returned">Resolved (Goods Returned)</option>
              </select>
            </div>
          </div>

          <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-700">
                <thead className="bg-gray-50 text-gray-500 uppercase tracking-wider text-[11px] border-b border-gray-200">
                  <tr>
                    <th className="px-6 py-3">Transfer Ref</th>
                    <th className="px-6 py-3">Origin / Destination</th>
                    <th className="px-6 py-3">Item</th>
                    <th className="px-6 py-3">Expected</th>
                    <th className="px-6 py-3">Received</th>
                    <th className="px-6 py-3 text-rose-700">Missing / Difference</th>
                    <th className="px-6 py-3">Reason</th>
                    <th className="px-6 py-3">Status</th>
                    <th className="px-6 py-3 text-right">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {filteredDiscrepancies.length === 0 ? (
                    <tr>
                      <td colSpan={9} className="px-6 py-8 text-center text-gray-400">
                        No shipment discrepancies recorded.
                      </td>
                    </tr>
                  ) : (
                    filteredDiscrepancies.map((d) => (
                      <tr key={d.id} className="hover:bg-gray-50 transition">
                        <td className="px-6 py-4 font-mono font-bold text-gray-900">{d.transfer_reference}</td>
                        <td className="px-6 py-4">
                          <div className="text-gray-900">{d.source_location_name}</div>
                          <div className="text-emerald-800 text-[11px]">→ {d.destination_location_name}</div>
                        </td>
                        <td className="px-6 py-4 font-semibold">{d.item_name}</td>
                        <td className="px-6 py-4 font-bold">{d.expected_quantity}</td>
                        <td className="px-6 py-4 font-bold text-emerald-800">{d.received_quantity}</td>
                        <td className="px-6 py-4 font-extrabold text-rose-700">-{d.difference_quantity}</td>
                        <td className="px-6 py-4 capitalize text-gray-600">{d.reason.replace(/_/g, " ")}</td>
                        <td className="px-6 py-4">
                          <Badge variant={d.status === "open" ? "danger" : "success"}>
                            {d.status.toUpperCase()}
                          </Badge>
                        </td>
                        <td className="px-6 py-4 text-right">
                          {d.status === "open" && (
                            <button
                              onClick={() => setSelectedResolveDiscrepancy(d)}
                              className="px-2.5 py-1.5 bg-rose-700 hover:bg-rose-800 text-white rounded-md text-xs font-bold transition shadow-xs"
                            >
                              Resolve
                            </button>
                          )}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* 7. IMMUTABLE STOCK LEDGER SUB-VIEW */}
      {/* ========================================================================= */}
      {activeTab === "ledger" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
            <div className="flex flex-wrap items-center gap-3">
              <select
                value={ledgerLocationFilter}
                onChange={(e) => setLedgerLocationFilter(e.target.value)}
                className="px-3 py-2 border border-gray-300 rounded-lg text-xs focus:ring-1 focus:ring-emerald-700 outline-none"
              >
                <option value="">All Storage Locations</option>
                {locations.map((loc) => (
                  <option key={loc.id} value={loc.id}>
                    {loc.name} ({loc.code})
                  </option>
                ))}
              </select>

              <select
                value={ledgerMovementFilter}
                onChange={(e) => setLedgerMovementFilter(e.target.value)}
                className="px-3 py-2 border border-gray-300 rounded-lg text-xs focus:ring-1 focus:ring-emerald-700 outline-none"
              >
                <option value="">All Movement Types</option>
                <option value="initial_receipt">Initial Receipt</option>
                <option value="donation_in_kind">Donation in Kind</option>
                <option value="transfer_dispatch">Transfer Dispatch (-)</option>
                <option value="transfer_receipt">Transfer Receipt (+)</option>
                <option value="write_off_loss">Loss Write-Off (-)</option>
                <option value="write_off_damage">Damage Write-Off (-)</option>
              </select>
            </div>

            <div className="text-xs text-gray-500 font-medium">
              Append-only audit trail • Total Records: {ledgerTotal}
            </div>
          </div>

          <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-gray-700">
                <thead className="bg-gray-50 text-gray-500 uppercase tracking-wider text-[11px] border-b border-gray-200">
                  <tr>
                    <th className="px-6 py-3">Ledger Ref</th>
                    <th className="px-6 py-3">Timestamp</th>
                    <th className="px-6 py-3">Depot Location</th>
                    <th className="px-6 py-3">Item</th>
                    <th className="px-6 py-3">Movement Type</th>
                    <th className="px-6 py-3">Quantity</th>
                    <th className="px-6 py-3">Balance After</th>
                    <th className="px-6 py-3">Actor / Notes</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {filteredLedger.map((e) => (
                    <tr key={e.id} className="hover:bg-gray-50 transition">
                      <td className="px-6 py-4 font-mono font-bold text-gray-900">{e.reference}</td>
                      <td className="px-6 py-4 font-mono text-gray-500">
                        {new Date(e.occurred_at).toLocaleString()}
                      </td>
                      <td className="px-6 py-4 font-semibold">{e.location_name}</td>
                      <td className="px-6 py-4 font-bold">{e.item_name}</td>
                      <td className="px-6 py-4">
                        <Badge
                          variant={
                            e.movement_type.includes("receipt") || e.movement_type.includes("donation")
                              ? "success"
                              : e.movement_type.includes("dispatch")
                              ? "warning"
                              : "danger"
                          }
                        >
                          {e.movement_type.replace(/_/g, " ").toUpperCase()}
                        </Badge>
                      </td>
                      <td className={`px-6 py-4 font-extrabold font-mono ${e.quantity > 0 ? "text-emerald-700" : "text-rose-700"}`}>
                        {e.quantity > 0 ? `+${e.quantity}` : e.quantity} {e.unit_of_measure}
                      </td>
                      <td className="px-6 py-4 font-extrabold font-mono text-gray-900">
                        {e.balance_after} {e.unit_of_measure}
                      </td>
                      <td className="px-6 py-4 max-w-xs truncate text-gray-500">
                        {e.notes || e.actor_name || "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* MODAL 1: ADD ITEM */}
      {/* ========================================================================= */}
      {showAddItemModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <PackagePlus className="w-5 h-5 text-emerald-800" /> Add Operational Supply Item
              </h3>
              <button onClick={() => setShowAddItemModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleCreateItem} className="space-y-3 text-xs">
              <div>
                <label className="font-semibold text-gray-700 block mb-1">SKU Code (e.g. RAT-SAN-010)</label>
                <input
                  required
                  type="text"
                  value={newItem.sku}
                  onChange={(e) => setNewItem({ ...newItem, sku: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none uppercase font-mono"
                />
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Item Name</label>
                <input
                  required
                  type="text"
                  value={newItem.name}
                  onChange={(e) => setNewItem({ ...newItem, name: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                />
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Category</label>
                <select
                  value={newItem.category}
                  onChange={(e) => setNewItem({ ...newItem, category: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                >
                  <option value="sanitation_gear">Sanitation Gear</option>
                  <option value="protective_equipment">Protective Equipment (PPE)</option>
                  <option value="tools">Clean-up Tools</option>
                  <option value="first_aid">First Aid & Health</option>
                  <option value="promotional_materials">Promotional & Outreach</option>
                  <option value="identity_gear">Identity & Credentials</option>
                </select>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Unit of Measure</label>
                  <input
                    required
                    type="text"
                    value={newItem.unit_of_measure}
                    onChange={(e) => setNewItem({ ...newItem, unit_of_measure: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  />
                </div>
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Min Threshold</label>
                  <input
                    required
                    type="number"
                    min="1"
                    value={newItem.min_threshold_quantity}
                    onChange={(e) => setNewItem({ ...newItem, min_threshold_quantity: Number(e.target.value) })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  />
                </div>
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Description</label>
                <textarea
                  rows={2}
                  value={newItem.description}
                  onChange={(e) => setNewItem({ ...newItem, description: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                />
              </div>
              <div className="pt-2 flex justify-end gap-2 border-t">
                <button
                  type="button"
                  onClick={() => setShowAddItemModal(false)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-lg"
                >
                  Save Item
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* MODAL 2: ADD LOCATION */}
      {/* ========================================================================= */}
      {showAddLocationModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <Warehouse className="w-5 h-5 text-emerald-800" /> Add Storage Location / Warehouse
              </h3>
              <button onClick={() => setShowAddLocationModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleCreateLocation} className="space-y-3 text-xs">
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Depot Code (e.g. WH-ST-KAN-01)</label>
                <input
                  required
                  type="text"
                  value={newLocation.code}
                  onChange={(e) => setNewLocation({ ...newLocation, code: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none uppercase font-mono"
                />
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Warehouse Name</label>
                <input
                  required
                  type="text"
                  value={newLocation.name}
                  onChange={(e) => setNewLocation({ ...newLocation, name: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                />
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Administrative Tier</label>
                <select
                  value={newLocation.location_type}
                  onChange={(e) => setNewLocation({ ...newLocation, location_type: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                >
                  <option value="national">National Hub</option>
                  <option value="zonal">Zonal Storage Depot</option>
                  <option value="state">State Sanitation Depot</option>
                  <option value="lga">LGA / Area Council Store</option>
                </select>
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Physical Address</label>
                <textarea
                  rows={2}
                  value={newLocation.address}
                  onChange={(e) => setNewLocation({ ...newLocation, address: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                />
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Contact Phone</label>
                <input
                  type="text"
                  value={newLocation.contact_phone}
                  onChange={(e) => setNewLocation({ ...newLocation, contact_phone: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                />
              </div>
              <div className="pt-2 flex justify-end gap-2 border-t">
                <button
                  type="button"
                  onClick={() => setShowAddLocationModal(false)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-lg"
                >
                  Create Warehouse
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* MODAL 3: RECEIVE STOCK */}
      {/* ========================================================================= */}
      {showReceiveStockModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <PackagePlus className="w-5 h-5 text-emerald-800" /> Receive Stock Inflow
              </h3>
              <button onClick={() => setShowReceiveStockModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleReceiveStock} className="space-y-3 text-xs">
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Destination Storage Location</label>
                <select
                  required
                  value={receiveStockInput.location_id}
                  onChange={(e) => setReceiveStockInput({ ...receiveStockInput, location_id: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                >
                  <option value="">Select Warehouse...</option>
                  {locations.map((loc) => (
                    <option key={loc.id} value={loc.id}>
                      {loc.name} ({loc.code})
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Item to Receive</label>
                <select
                  required
                  value={receiveStockInput.item_id}
                  onChange={(e) => setReceiveStockInput({ ...receiveStockInput, item_id: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                >
                  <option value="">Select Item...</option>
                  {items.map((itm) => (
                    <option key={itm.id} value={itm.id}>
                      {itm.name} ({itm.sku}) - {itm.unit_of_measure}
                    </option>
                  ))}
                </select>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Received Quantity</label>
                  <input
                    required
                    type="number"
                    min="1"
                    value={receiveStockInput.quantity}
                    onChange={(e) => setReceiveStockInput({ ...receiveStockInput, quantity: Number(e.target.value) })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  />
                </div>
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Movement Type</label>
                  <select
                    value={receiveStockInput.movement_type}
                    onChange={(e) => setReceiveStockInput({ ...receiveStockInput, movement_type: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  >
                    <option value="initial_receipt">Initial Bulk Receipt</option>
                    <option value="donation_in_kind">Donation in Kind</option>
                    <option value="return">Returned Stock</option>
                  </select>
                </div>
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Notes / Manufacturer Reference</label>
                <input
                  type="text"
                  placeholder="e.g. Batch #445 from Kaduna supplier"
                  value={receiveStockInput.notes}
                  onChange={(e) => setReceiveStockInput({ ...receiveStockInput, notes: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                />
              </div>
              <div className="pt-2 flex justify-end gap-2 border-t">
                <button
                  type="button"
                  onClick={() => setShowReceiveStockModal(false)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-lg"
                >
                  Confirm & Credit Stock
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* MODAL 4: NEW MATERIAL REQUEST */}
      {/* ========================================================================= */}
      {showCreateRequestModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <FileCheck2 className="w-5 h-5 text-emerald-800" /> Submit Material Request
              </h3>
              <button onClick={() => setShowCreateRequestModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleCreateRequest} className="space-y-3 text-xs">
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Destination Warehouse</label>
                <select
                  required
                  value={newRequestInput.destination_location_id}
                  onChange={(e) => setNewRequestInput({ ...newRequestInput, destination_location_id: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                >
                  <option value="">Select Destination...</option>
                  {locations.map((loc) => (
                    <option key={loc.id} value={loc.id}>
                      {loc.name} ({loc.location_type.toUpperCase()})
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Item to Request</label>
                <select
                  required
                  value={newRequestInput.item_id}
                  onChange={(e) => setNewRequestInput({ ...newRequestInput, item_id: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                >
                  <option value="">Select Item...</option>
                  {items.map((itm) => (
                    <option key={itm.id} value={itm.id}>
                      {itm.name} ({itm.sku})
                    </option>
                  ))}
                </select>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Requested Quantity</label>
                  <input
                    required
                    type="number"
                    min="1"
                    value={newRequestInput.quantity}
                    onChange={(e) => setNewRequestInput({ ...newRequestInput, quantity: Number(e.target.value) })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  />
                </div>
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Required By Date</label>
                  <input
                    required
                    type="date"
                    value={newRequestInput.required_date}
                    onChange={(e) => setNewRequestInput({ ...newRequestInput, required_date: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  />
                </div>
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Intended Use / Cleanup Purpose</label>
                <textarea
                  required
                  rows={2}
                  placeholder="e.g. Sanitation drive in municipal market"
                  value={newRequestInput.intended_use}
                  onChange={(e) => setNewRequestInput({ ...newRequestInput, intended_use: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                />
              </div>
              <div className="pt-2 flex justify-end gap-2 border-t">
                <button
                  type="button"
                  onClick={() => setShowCreateRequestModal(false)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-lg"
                >
                  Submit Request
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* MODAL 5: REVIEW MATERIAL REQUEST */}
      {/* ========================================================================= */}
      {selectedReviewRequest && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <ShieldCheck className="w-5 h-5 text-emerald-800" /> Review Request: {selectedReviewRequest.reference}
              </h3>
              <button onClick={() => setSelectedReviewRequest(null)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="text-xs space-y-2 bg-gray-50 p-3 rounded-lg border">
              <div><span className="font-semibold">Requester:</span> {selectedReviewRequest.requesting_user_name}</div>
              <div><span className="font-semibold">Destination:</span> {selectedReviewRequest.destination_location_name}</div>
              <div><span className="font-semibold">Intended Use:</span> {selectedReviewRequest.intended_use}</div>
            </div>
            <form onSubmit={handleReviewRequest} className="space-y-3 text-xs">
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Decision Action</label>
                <select
                  value={reviewInput.action}
                  onChange={(e) => setReviewInput({ ...reviewInput, action: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none font-bold"
                >
                  <option value="approve">Approve (Full Requested Quantity)</option>
                  <option value="reduce">Reduce Approved Quantity</option>
                  <option value="reject">Reject Request</option>
                </select>
              </div>

              {reviewInput.action === "reduce" && (
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Mandatory Reduction Reason</label>
                  <textarea
                    required
                    rows={2}
                    placeholder="Provide justification for reduction..."
                    value={reviewInput.reduction_reason}
                    onChange={(e) => setReviewInput({ ...reviewInput, reduction_reason: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  />
                </div>
              )}

              {reviewInput.action === "reject" && (
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Mandatory Rejection Reason</label>
                  <textarea
                    required
                    rows={2}
                    placeholder="Provide justification for rejection..."
                    value={reviewInput.rejection_reason}
                    onChange={(e) => setReviewInput({ ...reviewInput, rejection_reason: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  />
                </div>
              )}

              <div className="pt-2 flex justify-end gap-2 border-t">
                <button
                  type="button"
                  onClick={() => setSelectedReviewRequest(null)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-lg"
                >
                  Confirm Decision
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* MODAL 6: ALLOCATE STOCK */}
      {/* ========================================================================= */}
      {selectedAllocateRequest && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <Boxes className="w-5 h-5 text-emerald-800" /> Allocate Stock for {selectedAllocateRequest.reference}
              </h3>
              <button onClick={() => setSelectedAllocateRequest(null)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <p className="text-xs text-gray-500">
              Select the supplying warehouse. The system will acquire row-level locks and reserve the approved items immediately.
            </p>
            <form onSubmit={handleAllocateRequest} className="space-y-3 text-xs">
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Supplying Warehouse</label>
                <select
                  required
                  value={allocateInput.supplying_location_id}
                  onChange={(e) => setAllocateInput({ supplying_location_id: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                >
                  <option value="">Select Supplying Warehouse...</option>
                  {locations.map((loc) => (
                    <option key={loc.id} value={loc.id}>
                      {loc.name} ({loc.location_type.toUpperCase()})
                    </option>
                  ))}
                </select>
              </div>
              <div className="pt-2 flex justify-end gap-2 border-t">
                <button
                  type="button"
                  onClick={() => setSelectedAllocateRequest(null)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-emerald-950 hover:bg-black text-white font-bold rounded-lg"
                >
                  Reserve Stock
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* MODAL 7: CREATE & DISPATCH TRANSFER */}
      {/* ========================================================================= */}
      {showDispatchModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 shadow-xl space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <Truck className="w-5 h-5 text-emerald-800" /> Create & Dispatch Shipment
              </h3>
              <button onClick={() => setShowDispatchModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleCreateAndDispatchTransfer} className="space-y-3 text-xs">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Source Warehouse</label>
                  <select
                    required
                    value={dispatchInput.source_location_id}
                    onChange={(e) => setDispatchInput({ ...dispatchInput, source_location_id: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  >
                    <option value="">Select Origin...</option>
                    {locations.map((loc) => (
                      <option key={loc.id} value={loc.id}>
                        {loc.name}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Destination Warehouse</label>
                  <select
                    required
                    value={dispatchInput.destination_location_id}
                    onChange={(e) => setDispatchInput({ ...dispatchInput, destination_location_id: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  >
                    <option value="">Select Destination...</option>
                    {locations.map((loc) => (
                      <option key={loc.id} value={loc.id}>
                        {loc.name}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Item to Transfer</label>
                  <select
                    required
                    value={dispatchInput.item_id}
                    onChange={(e) => setDispatchInput({ ...dispatchInput, item_id: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  >
                    <option value="">Select Item...</option>
                    {items.map((itm) => (
                      <option key={itm.id} value={itm.id}>
                        {itm.name} ({itm.sku})
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Dispatched Quantity</label>
                  <input
                    required
                    type="number"
                    min="1"
                    value={dispatchInput.quantity}
                    onChange={(e) => setDispatchInput({ ...dispatchInput, quantity: Number(e.target.value) })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Carrier Name</label>
                  <input
                    type="text"
                    placeholder="e.g. GIG Logistics"
                    value={dispatchInput.carrier_name}
                    onChange={(e) => setDispatchInput({ ...dispatchInput, carrier_name: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                  />
                </div>
                <div>
                  <label className="font-semibold text-gray-700 block mb-1">Tracking Number</label>
                  <input
                    type="text"
                    placeholder="e.g. TRK-77889"
                    value={dispatchInput.tracking_number}
                    onChange={(e) => setDispatchInput({ ...dispatchInput, tracking_number: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none font-mono"
                  />
                </div>
              </div>

              <div className="pt-2 flex justify-end gap-2 border-t">
                <button
                  type="button"
                  onClick={() => setShowDispatchModal(false)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-lg"
                >
                  Dispatch Shipment
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* MODAL 8: CONFIRM RECEIPT (WITH DISCREPANCY REPORTING) */}
      {/* ========================================================================= */}
      {selectedReceiveTransfer && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <CheckCircle2 className="w-5 h-5 text-emerald-800" /> Confirm Receipt: {selectedReceiveTransfer.reference}
              </h3>
              <button onClick={() => setSelectedReceiveTransfer(null)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <p className="text-xs text-gray-500">
              Verify the physical quantity delivered. Only the confirmed received quantity will be credited to destination warehouse on-hand stock.
            </p>
            <form onSubmit={handleConfirmReceipt} className="space-y-4 text-xs">
              <div className="space-y-3 bg-gray-50 p-3 rounded-lg border">
                {transferItemsForReceipt.map((itm) => {
                  const recv = receiptQuantities[itm.item_id] ?? itm.dispatched_quantity;
                  const diff = itm.dispatched_quantity - recv;
                  return (
                    <div key={itm.id} className="space-y-1">
                      <div className="flex justify-between font-bold text-gray-900">
                        <span>{itm.item_name}</span>
                        <span className="text-gray-500">Dispatched: {itm.dispatched_quantity} {itm.unit_of_measure}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <label className="text-gray-600">Received:</label>
                        <input
                          type="number"
                          min="0"
                          max={itm.dispatched_quantity}
                          value={recv}
                          onChange={(e) =>
                            setReceiptQuantities({
                              ...receiptQuantities,
                              [itm.item_id]: Number(e.target.value),
                            })
                          }
                          className="w-24 px-2 py-1 border rounded text-right font-bold outline-none"
                        />
                        {diff > 0 && (
                          <span className="text-rose-700 font-bold">(-{diff} missing)</span>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>

              {Object.entries(receiptQuantities).some(([k, v]) => {
                const itm = transferItemsForReceipt.find(i => i.item_id === k);
                return itm && v < itm.dispatched_quantity;
              }) && (
                <div className="bg-amber-50 p-3 rounded-lg border border-amber-200 space-y-2">
                  <div className="font-bold text-amber-900 flex items-center gap-1.5">
                    <AlertTriangle className="w-4 h-4 text-amber-600" /> Discrepancy Detected
                  </div>
                  <div>
                    <label className="font-semibold text-gray-700 block mb-1">Discrepancy Reason</label>
                    <select
                      value={receiptDiscrepancyReason}
                      onChange={(e) => setReceiptDiscrepancyReason(e.target.value)}
                      className="w-full px-3 py-1.5 border rounded outline-none"
                    >
                      <option value="missing_in_transit">Missing in Transit</option>
                      <option value="damaged">Damaged Goods</option>
                      <option value="expired">Expired Stock</option>
                      <option value="wrong_item">Incorrect Item Specification</option>
                    </select>
                  </div>
                  <div>
                    <label className="font-semibold text-gray-700 block mb-1">Supporting Photo / Report URL</label>
                    <input
                      type="url"
                      placeholder="https://..."
                      value={receiptEvidenceURL}
                      onChange={(e) => setReceiptEvidenceURL(e.target.value)}
                      className="w-full px-3 py-1.5 border rounded outline-none"
                    />
                  </div>
                </div>
              )}

              <div className="pt-2 flex justify-end gap-2 border-t">
                <button
                  type="button"
                  onClick={() => setSelectedReceiveTransfer(null)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-lg"
                >
                  Confirm & Post to Ledger
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* MODAL 9: RESOLVE DISCREPANCY */}
      {/* ========================================================================= */}
      {selectedResolveDiscrepancy && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <AlertTriangle className="w-5 h-5 text-rose-700" /> Resolve Shipment Discrepancy
              </h3>
              <button onClick={() => setSelectedResolveDiscrepancy(null)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="text-xs space-y-1 bg-gray-50 p-3 rounded-lg border">
              <div><span className="font-semibold">Transfer:</span> {selectedResolveDiscrepancy.transfer_reference}</div>
              <div><span className="font-semibold">Item:</span> {selectedResolveDiscrepancy.item_name}</div>
              <div><span className="font-semibold text-rose-700">Missing Quantity:</span> -{selectedResolveDiscrepancy.difference_quantity}</div>
            </div>
            <form onSubmit={handleResolveDiscrepancy} className="space-y-3 text-xs">
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Resolution Strategy</label>
                <select
                  value={resolveDiscrepancyInput.resolution}
                  onChange={(e) => setResolveDiscrepancyInput({ ...resolveDiscrepancyInput, resolution: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                >
                  <option value="write_off">Approved Loss Write-Off (Append Compensating Ledger Entry)</option>
                  <option value="returned">Goods Recovered / Returned to Stock</option>
                </select>
              </div>
              <div>
                <label className="font-semibold text-gray-700 block mb-1">Resolution Investigation Notes</label>
                <textarea
                  required
                  rows={3}
                  placeholder="Detail the investigation outcome and approval authority..."
                  value={resolveDiscrepancyInput.notes}
                  onChange={(e) => setResolveDiscrepancyInput({ ...resolveDiscrepancyInput, notes: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:ring-1 focus:ring-emerald-700 outline-none"
                />
              </div>
              <div className="pt-2 flex justify-end gap-2 border-t">
                <button
                  type="button"
                  onClick={() => setSelectedResolveDiscrepancy(null)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-rose-700 hover:bg-rose-800 text-white font-bold rounded-lg"
                >
                  Resolve Discrepancy
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* MODAL 10: WAREHOUSE BALANCES DRAWER */}
      {/* ========================================================================= */}
      {viewWarehouseBalances && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-2xl max-w-2xl w-full p-6 shadow-xl space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <Warehouse className="w-5 h-5 text-emerald-800" /> Current Stock Balances: {viewWarehouseBalances.name} ({viewWarehouseBalances.code})
              </h3>
              <button onClick={() => setViewWarehouseBalances(null)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="overflow-x-auto max-h-96">
              <table className="w-full text-left text-xs text-gray-700">
                <thead className="bg-gray-50 text-gray-500 uppercase tracking-wider text-[11px] border-b">
                  <tr>
                    <th className="px-4 py-2.5">SKU</th>
                    <th className="px-4 py-2.5">Item Name</th>
                    <th className="px-4 py-2.5">On-Hand</th>
                    <th className="px-4 py-2.5">Reserved</th>
                    <th className="px-4 py-2.5">Available to Allocate</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {balances.filter(b => b.location_id === viewWarehouseBalances.id).length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-4 py-6 text-center text-gray-400">
                        No inventory stocked at this warehouse yet.
                      </td>
                    </tr>
                  ) : (
                    balances.filter(b => b.location_id === viewWarehouseBalances.id).map((b) => (
                      <tr key={b.id} className="hover:bg-gray-50 transition">
                        <td className="px-4 py-3 font-mono font-bold text-gray-900">{b.sku}</td>
                        <td className="px-4 py-3 font-semibold">{b.item_name}</td>
                        <td className="px-4 py-3 font-bold font-mono">{b.on_hand_quantity} {b.unit_of_measure}</td>
                        <td className="px-4 py-3 font-mono text-amber-700">{b.reserved_quantity} {b.unit_of_measure}</td>
                        <td className="px-4 py-3 font-extrabold font-mono text-emerald-800">
                          {b.available_quantity} {b.unit_of_measure}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
            <div className="pt-2 flex justify-end border-t">
              <button
                onClick={() => setViewWarehouseBalances(null)}
                className="px-4 py-2 bg-gray-900 text-white font-bold rounded-lg text-xs"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
