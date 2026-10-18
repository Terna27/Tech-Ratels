"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  DollarSign,
  Download,
  Printer,
  PlusCircle,
  ShieldCheck,
  Filter,
  CheckCircle2,
  AlertTriangle,
  RefreshCw,
  ArrowLeft,
  FileText,
  Lock,
  UserCheck,
  X,
  CreditCard,
  Building,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest, API_BASE } from "@/lib/api";
import {
  FinancialFund,
  FinancialTransaction,
  FinancialStatement,
  FinancialAuthorization,
  User as UserModel,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

export default function AdminFinancePage() {
  const router = useRouter();
  const { user, isCoordinator, isSuperAdmin, isLoading } = useAuth();

  const [statement, setStatement] = useState<FinancialStatement | null>(null);
  const [funds, setFunds] = useState<FinancialFund[]>([]);
  const [authorizations, setAuthorizations] = useState<FinancialAuthorization[]>([]);
  const [members, setMembers] = useState<UserModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [unauthorized, setUnauthorized] = useState(false);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  // Filters
  const [selectedFund, setSelectedFund] = useState("");
  const [selectedType, setSelectedType] = useState("");
  const [selectedStatus, setSelectedStatus] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");

  // Modals
  const [selectedVoucher, setSelectedVoucher] = useState<FinancialTransaction | null>(null);
  const [showRecordModal, setShowRecordModal] = useState(false);
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [showReverseModal, setShowReverseModal] = useState<FinancialTransaction | null>(null);

  // Record Form state
  const [recType, setRecType] = useState("confirmed_donation");
  const [recAmountNaira, setRecAmountNaira] = useState("");
  const [recSourceFund, setRecSourceFund] = useState("");
  const [recDestFund, setRecDestFund] = useState("foundation_donation_fund");
  const [recProviderRef, setRecProviderRef] = useState("");
  const [recNotes, setRecNotes] = useState("");
  const [submittingRec, setSubmittingRec] = useState(false);

  // Reverse Form state
  const [reverseReason, setReverseReason] = useState("");
  const [submittingReverse, setSubmittingReverse] = useState(false);

  // Auth Form state
  const [authUserId, setAuthUserId] = useState("");
  const [authLevel, setAuthLevel] = useState<"viewer" | "submitter">("viewer");
  const [authNotes, setAuthNotes] = useState("");
  const [submittingAuth, setSubmittingAuth] = useState(false);

  const loadFinanceData = async () => {
    setLoading(true);
    setUnauthorized(false);
    try {
      // 1. Fetch available funds
      const fundsData = await apiRequest<FinancialFund[]>("/finance/funds");
      setFunds(fundsData || []);

      // 2. Fetch statement with current filters
      const params = new URLSearchParams();
      if (selectedFund) params.append("fund_id", selectedFund);
      if (selectedType) params.append("transaction_type", selectedType);
      if (selectedStatus) params.append("status", selectedStatus);
      if (startDate) params.append("start_date", new Date(startDate).toISOString());
      if (endDate) params.append("end_date", new Date(endDate).toISOString());

      const queryStr = params.toString() ? `?${params.toString()}` : "";
      const stmtData = await apiRequest<FinancialStatement>(`/finance/statement${queryStr}`);
      setStatement(stmtData);

      // 3. If super admin, fetch authorizations and members
      if (isSuperAdmin) {
        try {
          const [auths, mems] = await Promise.all([
            apiRequest<FinancialAuthorization[]>("/finance/authorizations"),
            apiRequest<UserModel[]>("/members?limit=50"),
          ]);
          setAuthorizations(auths || []);
          setMembers(mems || []);
        } catch {
          // Non-critical if authorizations fetch fails
        }
      }
    } catch (err: any) {
      if (err.status === 403 || err.code === "FINANCIAL_UNAUTHORIZED" || err.code === "FORBIDDEN") {
        setUnauthorized(true);
      } else {
        setFeedback({ type: "error", message: err.message || "Failed to load financial records" });
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/login?redirect=/admin/finance");
      return;
    }
    if (user) {
      loadFinanceData();
    }
  }, [user, isLoading, selectedFund, selectedType, selectedStatus, startDate, endDate]);

  const handleRecordTransaction = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmittingRec(true);
    setFeedback(null);

    const naira = parseFloat(recAmountNaira);
    if (isNaN(naira) || naira <= 0) {
      setFeedback({ type: "error", message: "Please enter a valid amount greater than zero." });
      setSubmittingRec(false);
      return;
    }
    const minorUnits = Math.round(naira * 100);

    try {
      await apiRequest("/finance/transactions", {
        method: "POST",
        body: JSON.stringify({
          transaction_type: recType,
          amount: minorUnits,
          currency: "NGN",
          source_fund_id: recSourceFund || undefined,
          destination_fund_id: recDestFund || undefined,
          provider_reference: recProviderRef || undefined,
          notes: recNotes || undefined,
          auto_post: isSuperAdmin,
        }),
      });

      setFeedback({ type: "success", message: "Financial record logged with balanced double-entry ledger entries!" });
      setShowRecordModal(false);
      setRecAmountNaira("");
      setRecNotes("");
      setRecProviderRef("");
      await loadFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to record transaction" });
    } finally {
      setSubmittingRec(false);
    }
  };

  const handleReverseTransaction = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!showReverseModal) return;
    setSubmittingReverse(true);
    setFeedback(null);

    try {
      await apiRequest(`/finance/transactions/${showReverseModal.id}/reverse`, {
        method: "POST",
        body: JSON.stringify({ reason: reverseReason }),
      });

      setFeedback({ type: "success", message: `Transaction ${showReverseModal.reference} reversed. Balanced adjustment entries posted.` });
      setShowReverseModal(null);
      setReverseReason("");
      await loadFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to reverse transaction" });
    } finally {
      setSubmittingReverse(false);
    }
  };

  const handleAuthorizeStaff = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmittingAuth(true);
    setFeedback(null);

    try {
      await apiRequest("/finance/authorizations", {
        method: "POST",
        body: JSON.stringify({
          user_id: authUserId,
          access_level: authLevel,
          notes: authNotes || undefined,
        }),
      });

      setFeedback({ type: "success", message: "Staff member authorized for financial records viewing." });
      setShowAuthModal(false);
      setAuthNotes("");
      await loadFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to authorize staff" });
    } finally {
      setSubmittingAuth(false);
    }
  };

  const handleRevokeAuth = async (authId: string) => {
    if (!confirm("Are you sure you want to revoke this user's financial clearance?")) return;
    try {
      await apiRequest(`/finance/authorizations/${authId}`, { method: "DELETE" });
      setFeedback({ type: "success", message: "Financial clearance revoked immediately." });
      await loadFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to revoke clearance" });
    }
  };

  const handleExportCSV = () => {
    const params = new URLSearchParams();
    if (selectedFund) params.append("fund_id", selectedFund);
    if (selectedType) params.append("transaction_type", selectedType);
    if (selectedStatus) params.append("status", selectedStatus);
    if (startDate) params.append("start_date", new Date(startDate).toISOString());
    if (endDate) params.append("end_date", new Date(endDate).toISOString());

    const token = sessionStorage.getItem("ratels_auth_token");
    const downloadUrl = `${API_BASE}/finance/export?${params.toString()}`;

    // Direct authenticated fetch and download
    fetch(downloadUrl, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => res.blob())
      .then((blob) => {
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `ratels_financial_report_${new Date().toISOString().split("T")[0]}.csv`;
        document.body.appendChild(a);
        a.click();
        a.remove();
      })
      .catch((err) => {
        alert("Failed to export CSV: " + err.message);
      });
  };

  const formatKobo = (kobo: number) => {
    const naira = kobo / 100;
    return new Intl.NumberFormat("en-NG", {
      style: "currency",
      currency: "NGN",
      minimumFractionDigits: 2,
    }).format(naira);
  };

  if (isLoading || loading) return <Spinner text="Loading financial records..." />;

  if (unauthorized) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-16 text-center space-y-6">
        <div className="w-16 h-16 mx-auto bg-amber-100 text-amber-800 rounded-full flex items-center justify-center">
          <Lock className="w-8 h-8" />
        </div>
        <h1 className="text-2xl font-extrabold text-gray-900">
          Financial Documentation Restricted
        </h1>
        <p className="text-gray-600 max-w-lg mx-auto">
          Access to fund movements, general ledger records, and financial statements is strictly restricted. Only the Super Administrator or staff members with explicit financial clearance granted by the Super Administrator may access this documentation.
        </p>
        <div className="pt-4">
          <Link
            href="/admin"
            className="inline-flex items-center gap-2 px-5 py-2.5 bg-gray-900 text-white rounded-xl text-sm font-semibold hover:bg-gray-800 transition"
          >
            <ArrowLeft className="w-4 h-4" />
            Return to Admin Workspace
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8 print:p-0">
      {/* Non-print Top Navigation */}
      <div className="flex items-center justify-between no-print">
        <Link
          href="/admin"
          className="inline-flex items-center text-xs font-semibold text-gray-500 hover:text-gray-900"
        >
          <ArrowLeft className="w-4 h-4 mr-1" />
          Back to Admin Workspace
        </Link>
        <div className="flex items-center gap-2">
          {isSuperAdmin && (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 bg-amber-100 text-amber-900 border border-amber-300 rounded-full text-xs font-bold">
              <ShieldCheck className="w-3.5 h-3.5 text-amber-700" />
              Super Admin Authority
            </span>
          )}
        </div>
      </div>

      {/* Official Header for Screen and Print */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b pb-6 print:border-b-2 print:border-black print:pb-4">
        <div>
          <div className="flex items-center gap-2">
            <span className="text-xs uppercase font-extrabold tracking-wider text-emerald-800">
              Martins Vincent Otse Foundation / Nigerian Ratels Movement
            </span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight mt-1">
            Financial Ledger & Fund Movement Statement
          </h1>
          <p className="text-xs text-gray-600 mt-1">
            Official immutable double-entry accounting records, segregated funds, and cash commitments.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2 no-print">
          <button
            type="button"
            onClick={() => window.print()}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-white border border-gray-300 hover:bg-gray-50 text-gray-800 rounded-xl text-xs font-bold shadow-sm transition"
          >
            <Printer className="w-4 h-4 text-gray-600" />
            Print Official Statement
          </button>
          <button
            type="button"
            onClick={handleExportCSV}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-white border border-gray-300 hover:bg-gray-50 text-gray-800 rounded-xl text-xs font-bold shadow-sm transition"
          >
            <Download className="w-4 h-4 text-emerald-700" />
            Export CSV
          </button>
          {isSuperAdmin && (
            <>
              <button
                type="button"
                onClick={() => setShowAuthModal(true)}
                className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-amber-700 hover:bg-amber-800 text-white rounded-xl text-xs font-bold shadow-sm transition"
              >
                <UserCheck className="w-4 h-4" />
                Staff Clearances ({authorizations.filter((a) => a.status === "active").length})
              </button>
              <button
                type="button"
                onClick={() => setShowRecordModal(true)}
                className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl text-xs font-bold shadow-sm transition"
              >
                <PlusCircle className="w-4 h-4" />
                Record Movement
              </button>
            </>
          )}
        </div>
      </div>

      {feedback && <Alert type={feedback.type}>{feedback.message}</Alert>}

      {/* Financial Statement Summary Cards (Reconciled from DB) */}
      {statement && (
        <div className="space-y-4">
          <div className="flex items-center justify-between text-xs">
            <span className="font-bold text-gray-700 flex items-center gap-1.5">
              <CheckCircle2 className="w-4 h-4 text-emerald-600" />
              Reconciliation Status:{" "}
              <span className="uppercase text-emerald-800 font-extrabold bg-emerald-50 px-2 py-0.5 rounded border border-emerald-200">
                {statement.reconciliation_status}
              </span>
            </span>
            <span className="text-gray-500 font-mono text-[11px]">
              Ref: {statement.report_reference} • Generated: {new Date(statement.generated_at).toLocaleString()}
            </span>
          </div>

          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1 print:border-gray-400">
              <span className="text-xs text-gray-500 font-medium">Opening Balance</span>
              <div className="text-xl font-extrabold text-gray-900">{formatKobo(statement.opening_balance)}</div>
              <span className="text-[10px] text-gray-400">Prior period ledger baseline</span>
            </div>

            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1 print:border-gray-400">
              <span className="text-xs text-emerald-700 font-medium">Confirmed Inflows (External)</span>
              <div className="text-xl font-extrabold text-emerald-900">{formatKobo(statement.confirmed_inflows)}</div>
              <span className="text-[10px] text-emerald-600 font-medium">External income only</span>
            </div>

            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-1 print:border-gray-400">
              <span className="text-xs text-red-700 font-medium">Disbursements & Expenses</span>
              <div className="text-xl font-extrabold text-red-900">{formatKobo(statement.confirmed_outflows)}</div>
              <span className="text-[10px] text-gray-400">Approved project & ops spending</span>
            </div>

            <div className="bg-emerald-950 text-white p-4 rounded-xl border border-emerald-800 shadow-sm space-y-1 print:bg-white print:text-black print:border-black">
              <span className="text-xs text-emerald-300 font-medium print:text-black">Net Closing Balance</span>
              <div className="text-xl font-extrabold text-white print:text-black">{formatKobo(statement.closing_balance)}</div>
              <span className="text-[10px] text-emerald-400 print:text-gray-600">Reconciled double-entry total</span>
            </div>
          </div>

          {/* Cash vs Commitments vs Available-to-Spend Breakdown */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 bg-gray-50 p-4 rounded-xl border border-gray-200 print:bg-white print:border-gray-400">
            <div className="space-y-0.5">
              <span className="text-xs text-gray-500 font-medium">Physical Cash In Ledger</span>
              <div className="text-base font-bold text-gray-900">{formatKobo(statement.cash_balance)}</div>
            </div>
            <div className="space-y-0.5">
              <span className="text-xs text-amber-700 font-medium">Pending Commitments</span>
              <div className="text-base font-bold text-amber-900">{formatKobo(statement.commitments)}</div>
            </div>
            <div className="space-y-0.5">
              <span className="text-xs text-emerald-700 font-medium">Net Available to Spend</span>
              <div className="text-base font-bold text-emerald-900">{formatKobo(statement.available_to_spend)}</div>
            </div>
          </div>
        </div>
      )}

      {/* Filter Toolbar (No-print) */}
      <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-3 no-print">
        <div className="flex items-center gap-2 text-xs font-bold text-gray-700">
          <Filter className="w-4 h-4 text-emerald-700" />
          Filter Transactions & Funds
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-5 gap-3 text-xs">
          <div>
            <label className="block text-gray-600 mb-1 font-medium">Segregated Fund</label>
            <select
              value={selectedFund}
              onChange={(e) => setSelectedFund(e.target.value)}
              className="w-full border border-gray-300 rounded-lg p-2 bg-white"
            >
              <option value="">All Funds</option>
              {funds.map((f) => (
                <option key={f.id} value={f.id}>
                  {f.name}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-gray-600 mb-1 font-medium">Transaction Type</label>
            <select
              value={selectedType}
              onChange={(e) => setSelectedType(e.target.value)}
              className="w-full border border-gray-300 rounded-lg p-2 bg-white"
            >
              <option value="">All Types</option>
              <option value="confirmed_donation">Confirmed Donation</option>
              <option value="project_allocation">Project Allocation</option>
              <option value="internal_transfer">Internal Transfer</option>
              <option value="disbursement">Disbursement</option>
              <option value="expense">Operational Expense</option>
              <option value="provider_fee">Provider Fee</option>
              <option value="refund">Refund</option>
              <option value="reversal">Adjustment Reversal</option>
              <option value="bank_settlement">Bank Settlement</option>
            </select>
          </div>

          <div>
            <label className="block text-gray-600 mb-1 font-medium">Status</label>
            <select
              value={selectedStatus}
              onChange={(e) => setSelectedStatus(e.target.value)}
              className="w-full border border-gray-300 rounded-lg p-2 bg-white"
            >
              <option value="">All Statuses</option>
              <option value="posted">Posted (Final)</option>
              <option value="pending">Pending Approval</option>
              <option value="reversed">Reversed</option>
            </select>
          </div>

          <div>
            <label className="block text-gray-600 mb-1 font-medium">Start Date</label>
            <input
              type="date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              className="w-full border border-gray-300 rounded-lg p-2 bg-white"
            />
          </div>

          <div>
            <label className="block text-gray-600 mb-1 font-medium">End Date</label>
            <input
              type="date"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              className="w-full border border-gray-300 rounded-lg p-2 bg-white"
            />
          </div>
        </div>
      </div>

      {/* Transactions Table */}
      <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm print:border-none print:shadow-none">
        <div className="p-4 border-b border-gray-200 bg-gray-50/50 flex justify-between items-center text-xs print:hidden">
          <span className="font-semibold text-gray-700">Posted Fund Movements & Transactions</span>
          <span className="text-gray-500">{statement?.transactions.length || 0} records</span>
        </div>

        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 text-xs">
            <thead className="bg-gray-50 text-gray-600 font-semibold uppercase text-[11px] print:bg-transparent">
              <tr>
                <th className="px-4 py-3 text-left">Reference / Date</th>
                <th className="px-4 py-3 text-left">Type</th>
                <th className="px-4 py-3 text-left">Fund Scope</th>
                <th className="px-4 py-3 text-right">Amount (NGN)</th>
                <th className="px-4 py-3 text-center">External Income</th>
                <th className="px-4 py-3 text-center">Status</th>
                <th className="px-4 py-3 text-left">Approver</th>
                <th className="px-4 py-3 text-right no-print">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {statement?.transactions.map((tx) => (
                <tr key={tx.id} className="hover:bg-gray-50/60 transition">
                  <td className="px-4 py-3 whitespace-nowrap">
                    <button
                      type="button"
                      onClick={() => setSelectedVoucher(tx)}
                      className="font-bold text-emerald-800 hover:underline block text-left"
                    >
                      {tx.reference}
                    </button>
                    <span className="text-[10px] text-gray-400">
                      {new Date(tx.occurred_at).toLocaleDateString()} {new Date(tx.occurred_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </span>
                  </td>

                  <td className="px-4 py-3 whitespace-nowrap">
                    <span className="font-medium text-gray-800 capitalize">
                      {tx.transaction_type.replace(/_/g, " ")}
                    </span>
                  </td>

                  <td className="px-4 py-3 whitespace-nowrap text-gray-600">
                    <div>{tx.destination_fund_name || tx.source_fund_name || "General Fund"}</div>
                    {tx.provider_reference && (
                      <span className="text-[10px] font-mono text-gray-400">Ext: {tx.provider_reference}</span>
                    )}
                  </td>

                  <td className="px-4 py-3 whitespace-nowrap text-right font-bold text-gray-900 font-mono">
                    {formatKobo(tx.amount)}
                  </td>

                  <td className="px-4 py-3 whitespace-nowrap text-center">
                    {tx.is_external_income ? (
                      <span className="px-2 py-0.5 bg-emerald-100 text-emerald-800 rounded-full font-bold text-[10px]">
                        YES (Income)
                      </span>
                    ) : (
                      <span className="px-2 py-0.5 bg-gray-100 text-gray-600 rounded-full text-[10px]">
                        NO (Internal/Settlement)
                      </span>
                    )}
                  </td>

                  <td className="px-4 py-3 whitespace-nowrap text-center">
                    <Badge
                      variant={
                        tx.status === "posted"
                          ? "success"
                          : tx.status === "pending"
                          ? "warning"
                          : "danger"
                      }
                      size="sm"
                    >
                      {tx.status.toUpperCase()}
                    </Badge>
                  </td>

                  <td className="px-4 py-3 whitespace-nowrap text-gray-600">
                    <div>{tx.approved_by_name || "System Automated"}</div>
                    {tx.approved_at && (
                      <span className="text-[10px] text-gray-400">
                        {new Date(tx.approved_at).toLocaleDateString()}
                      </span>
                    )}
                  </td>

                  <td className="px-4 py-3 whitespace-nowrap text-right no-print">
                    <div className="flex items-center justify-end gap-1.5">
                      <button
                        type="button"
                        onClick={() => setSelectedVoucher(tx)}
                        className="px-2.5 py-1 bg-gray-100 hover:bg-gray-200 text-gray-700 rounded text-[11px] font-semibold transition"
                      >
                        Voucher
                      </button>
                      {isSuperAdmin && tx.status === "posted" && !tx.is_reversal && (
                        <button
                          type="button"
                          onClick={() => setShowReverseModal(tx)}
                          className="px-2 py-1 bg-red-50 hover:bg-red-100 text-red-700 rounded text-[11px] font-semibold transition"
                        >
                          Reverse
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Printable Sign-off Block (Visible in Print) */}
      <div className="print-only pt-12 space-y-8">
        <div className="grid grid-cols-2 gap-12 text-xs">
          <div className="border-t border-black pt-2">
            <span className="font-bold block">Approved by Super Administrator</span>
            <span className="text-gray-600 block">Name: Chief VeryDarkMan / Martins Vincent Otse Foundation</span>
            <span className="text-gray-600 block">Signature: __________________________</span>
            <span className="text-gray-600 block">Date: {new Date().toLocaleDateString()}</span>
          </div>
          <div className="border-t border-black pt-2">
            <span className="font-bold block">Internal Audit & Reconciliation</span>
            <span className="text-gray-600 block">Reconciliation Status: BALANCED & VERIFIED</span>
            <span className="text-gray-600 block">Signature: __________________________</span>
            <span className="text-gray-600 block">Date: {new Date().toLocaleDateString()}</span>
          </div>
        </div>
      </div>

      {/* Printable Payment Voucher Modal */}
      {selectedVoucher && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 print:p-0 print:static print:bg-transparent">
          <div className="bg-white rounded-2xl max-w-xl w-full p-6 space-y-6 shadow-2xl border border-gray-200 print:shadow-none print:border-black print:max-w-none">
            <div className="flex justify-between items-start border-b pb-4">
              <div>
                <span className="text-xs font-bold text-emerald-800 uppercase tracking-wider block">
                  Official Movement Receipt & Voucher
                </span>
                <h2 className="text-xl font-extrabold text-gray-900 mt-0.5">
                  Reference: {selectedVoucher.reference}
                </h2>
              </div>
              <button
                type="button"
                onClick={() => setSelectedVoucher(null)}
                className="p-1 text-gray-400 hover:text-gray-600 no-print"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="space-y-3 text-xs">
              <div className="flex justify-between py-1.5 border-b border-gray-100">
                <span className="text-gray-500 font-medium">Transaction Type</span>
                <span className="font-bold capitalize text-gray-900">{selectedVoucher.transaction_type.replace(/_/g, " ")}</span>
              </div>
              <div className="flex justify-between py-1.5 border-b border-gray-100">
                <span className="text-gray-500 font-medium">Date & Time</span>
                <span className="font-mono text-gray-900">{new Date(selectedVoucher.occurred_at).toLocaleString()}</span>
              </div>
              <div className="flex justify-between py-1.5 border-b border-gray-100">
                <span className="text-gray-500 font-medium">Amount In Minor Units</span>
                <span className="font-mono text-gray-900">{selectedVoucher.amount.toLocaleString()} Kobo</span>
              </div>
              <div className="flex justify-between py-2 border-b-2 border-emerald-800 bg-emerald-50 px-2 rounded">
                <span className="text-emerald-950 font-bold">Total Amount (NGN)</span>
                <span className="font-mono font-extrabold text-base text-emerald-950">{formatKobo(selectedVoucher.amount)}</span>
              </div>
              <div className="flex justify-between py-1.5 border-b border-gray-100">
                <span className="text-gray-500 font-medium">Destination Fund</span>
                <span className="font-medium text-gray-900">{selectedVoucher.destination_fund_name || "General Fund"}</span>
              </div>
              {selectedVoucher.source_fund_name && (
                <div className="flex justify-between py-1.5 border-b border-gray-100">
                  <span className="text-gray-500 font-medium">Source Fund</span>
                  <span className="font-medium text-gray-900">{selectedVoucher.source_fund_name}</span>
                </div>
              )}
              {selectedVoucher.notes && (
                <div className="py-2 border-b border-gray-100">
                  <span className="text-gray-500 block mb-1">Notes / Description</span>
                  <p className="text-gray-800 bg-gray-50 p-2 rounded text-xs">{selectedVoucher.notes}</p>
                </div>
              )}
              <div className="flex justify-between py-1.5">
                <span className="text-gray-500 font-medium">Approved / Posted By</span>
                <span className="font-semibold text-gray-900">{selectedVoucher.approved_by_name || "Super Admin Authority"}</span>
              </div>
            </div>

            <div className="flex justify-end gap-3 pt-4 border-t no-print">
              <button
                type="button"
                onClick={() => window.print()}
                className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl text-xs font-bold transition flex items-center gap-1.5"
              >
                <Printer className="w-4 h-4" />
                Print Voucher
              </button>
              <button
                type="button"
                onClick={() => setSelectedVoucher(null)}
                className="px-4 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-xl text-xs font-semibold"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Record Movement Modal (Super Admin only) */}
      {showRecordModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 space-y-4 shadow-xl border border-gray-200">
            <div className="flex justify-between items-center border-b pb-3">
              <h2 className="text-base font-bold text-gray-900">Record Verified Fund Movement</h2>
              <button onClick={() => setShowRecordModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleRecordTransaction} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Movement Type *</label>
                <select
                  value={recType}
                  onChange={(e) => setRecType(e.target.value)}
                  className="w-full border border-gray-300 rounded-lg p-2 bg-white"
                >
                  <option value="confirmed_donation">Confirmed Donation (External Inflow)</option>
                  <option value="project_allocation">Project Allocation (Internal Transfer)</option>
                  <option value="internal_transfer">General Fund Transfer (Internal)</option>
                  <option value="disbursement">Disbursement / Project Payment</option>
                  <option value="expense">Operational Expense</option>
                  <option value="provider_fee">Provider / Gateway Fee</option>
                  <option value="bank_settlement">Bank Gateway Settlement</option>
                </select>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Amount in Naira (NGN) *</label>
                <input
                  type="number"
                  step="0.01"
                  required
                  value={recAmountNaira}
                  onChange={(e) => setRecAmountNaira(e.target.value)}
                  placeholder="e.g. 50000.00"
                  className="w-full border border-gray-300 rounded-lg p-2 font-mono text-sm"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Source Fund</label>
                  <select
                    value={recSourceFund}
                    onChange={(e) => setRecSourceFund(e.target.value)}
                    className="w-full border border-gray-300 rounded-lg p-2 bg-white"
                  >
                    <option value="">None (External Inflow)</option>
                    {funds.map((f) => (
                      <option key={f.id} value={f.id}>
                        {f.name}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Destination Fund *</label>
                  <select
                    value={recDestFund}
                    onChange={(e) => setRecDestFund(e.target.value)}
                    className="w-full border border-gray-300 rounded-lg p-2 bg-white"
                  >
                    <option value="">None (External Outflow)</option>
                    {funds.map((f) => (
                      <option key={f.id} value={f.id}>
                        {f.name}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">External Provider Reference</label>
                <input
                  type="text"
                  value={recProviderRef}
                  onChange={(e) => setRecProviderRef(e.target.value)}
                  placeholder="e.g. BANK-TRF-009182 / PAYSTACK-TX"
                  className="w-full border border-gray-300 rounded-lg p-2 font-mono"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Description / Purpose *</label>
                <textarea
                  required
                  rows={2}
                  value={recNotes}
                  onChange={(e) => setRecNotes(e.target.value)}
                  placeholder="State the verifiable reason and approved purpose..."
                  className="w-full border border-gray-300 rounded-lg p-2"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowRecordModal(false)}
                  className="px-4 py-2 border rounded-xl font-semibold text-gray-700 hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingRec}
                  className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl font-bold transition"
                >
                  {submittingRec ? "Posting..." : "Post Movement"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Reverse Transaction Modal (Super Admin only) */}
      {showReverseModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 space-y-4 shadow-xl border border-gray-200">
            <div className="flex justify-between items-center border-b pb-3">
              <h2 className="text-base font-bold text-gray-900 flex items-center gap-1.5 text-red-700">
                <AlertTriangle className="w-5 h-5" />
                Post Linked Reversal Entry
              </h2>
              <button onClick={() => setShowReverseModal(null)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>

            <p className="text-xs text-gray-600">
              Posted financial transactions cannot be edited or deleted. Reversing this record will post a linked, inverted double-entry adjustment.
            </p>

            <form onSubmit={handleReverseTransaction} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Audit Reason for Reversal *</label>
                <textarea
                  required
                  minLength={5}
                  rows={3}
                  value={reverseReason}
                  onChange={(e) => setReverseReason(e.target.value)}
                  placeholder="State the formal reason for reversing this posted transaction..."
                  className="w-full border border-gray-300 rounded-lg p-2"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowReverseModal(null)}
                  className="px-4 py-2 border rounded-xl font-semibold text-gray-700 hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingReverse}
                  className="px-5 py-2 bg-red-700 hover:bg-red-800 text-white rounded-xl font-bold transition"
                >
                  {submittingReverse ? "Reversing..." : "Confirm Reversal"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Staff Authorizations Modal (Super Admin only) */}
      {showAuthModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white rounded-2xl max-w-2xl w-full p-6 space-y-5 shadow-xl border border-gray-200">
            <div className="flex justify-between items-center border-b pb-3">
              <div>
                <h2 className="text-base font-bold text-gray-900">Financial Access Authorizations</h2>
                <p className="text-xs text-gray-500">Only the Super Admin may authorize staff to inspect financial documentation.</p>
              </div>
              <button onClick={() => setShowAuthModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Grant Form */}
            <form onSubmit={handleAuthorizeStaff} className="p-4 bg-gray-50 rounded-xl space-y-3 text-xs border border-gray-200">
              <span className="font-bold text-gray-800 block">Grant New Clearance</span>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block font-medium text-gray-700 mb-1">Select Member / Officer *</label>
                  <select
                    required
                    value={authUserId}
                    onChange={(e) => setAuthUserId(e.target.value)}
                    className="w-full border border-gray-300 rounded-lg p-2 bg-white"
                  >
                    <option value="">Select registered member...</option>
                    {members.map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.display_name} ({m.membership_reference})
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block font-medium text-gray-700 mb-1">Clearance Level *</label>
                  <select
                    value={authLevel}
                    onChange={(e) => setAuthLevel(e.target.value as any)}
                    className="w-full border border-gray-300 rounded-lg p-2 bg-white"
                  >
                    <option value="viewer">Viewer (Read-only financial statements)</option>
                    <option value="submitter">Submitter (Can submit claims & view)</option>
                  </select>
                </div>
              </div>

              <div>
                <label className="block font-medium text-gray-700 mb-1">Authorization Purpose / Notes</label>
                <input
                  type="text"
                  value={authNotes}
                  onChange={(e) => setAuthNotes(e.target.value)}
                  placeholder="e.g. Q4 Internal Audit / Finance Committee"
                  className="w-full border border-gray-300 rounded-lg p-2 bg-white"
                />
              </div>

              <div className="flex justify-end pt-1">
                <button
                  type="submit"
                  disabled={submittingAuth}
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-lg font-bold"
                >
                  {submittingAuth ? "Granting..." : "Grant Clearance"}
                </button>
              </div>
            </form>

            {/* Active Authorizations List */}
            <div className="space-y-2 text-xs">
              <span className="font-bold text-gray-800 block">Currently Authorized Staff</span>
              <div className="max-h-60 overflow-y-auto divide-y divide-gray-100 border rounded-xl">
                {authorizations.length === 0 ? (
                  <div className="p-4 text-center text-gray-500">No external staff currently authorized.</div>
                ) : (
                  authorizations.map((a) => (
                    <div key={a.id} className="p-3 flex items-center justify-between hover:bg-gray-50">
                      <div>
                        <span className="font-bold text-gray-900 block">{a.user_name || a.user_id}</span>
                        <span className="text-[10px] text-gray-500 block">
                          Level: <span className="font-semibold uppercase">{a.access_level}</span> • Status:{" "}
                          <span className={a.status === "active" ? "text-emerald-700 font-bold" : "text-gray-400"}>
                            {a.status}
                          </span>
                        </span>
                        {a.notes && <span className="text-[10px] text-gray-400 italic block">{a.notes}</span>}
                      </div>
                      {a.status === "active" && (
                        <button
                          type="button"
                          onClick={() => handleRevokeAuth(a.id)}
                          className="px-2.5 py-1 bg-red-50 hover:bg-red-100 text-red-700 rounded text-[11px] font-semibold"
                        >
                          Revoke
                        </button>
                      )}
                    </div>
                  ))
                )}
              </div>
            </div>

            <div className="flex justify-end pt-2 border-t">
              <button
                type="button"
                onClick={() => setShowAuthModal(false)}
                className="px-4 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-xl font-semibold"
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
