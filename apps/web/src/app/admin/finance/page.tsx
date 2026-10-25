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
  TrendingUp,
  Wallet,
  Send,
  Layers,
  Eye,
  Check,
  Ban,
  UploadCloud,
  FileSpreadsheet,
  Coins,
  Clock,
  ChevronRight,
  ShieldAlert,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest, API_BASE } from "@/lib/api";
import {
  FinancialFund,
  FinancialTransaction,
  FinancialStatement,
  FinancialAuthorization,
  FinancialDashboardSummary,
  FinancialExpense,
  FinancialBudget,
  FinancialBudgetRevision,
  FinancialAllocation,
  FinancialDonation,
  FinancialBankAccount,
  FinancialReconciliationPeriod,
  FinancialStatementItem,
  FinancialDocument,
  User as UserModel,
  Zone,
  State,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

type FinanceTab =
  | "overview"
  | "income"
  | "expenses"
  | "budgets"
  | "allocations"
  | "donations"
  | "bank_accounts"
  | "reconciliation"
  | "documents"
  | "audit";

export default function AdminFinancePage() {
  const router = useRouter();
  const { user, isSuperAdmin, isLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<FinanceTab>("overview");
  const [loading, setLoading] = useState(true);
  const [unauthorized, setUnauthorized] = useState(false);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  // Authoritative Dashboard Data
  const [summary, setSummary] = useState<FinancialDashboardSummary | null>(null);
  const [funds, setFunds] = useState<FinancialFund[]>([]);
  const [statement, setStatement] = useState<FinancialStatement | null>(null);
  const [expenses, setExpenses] = useState<FinancialExpense[]>([]);
  const [budgets, setBudgets] = useState<FinancialBudget[]>([]);
  const [allocations, setAllocations] = useState<FinancialAllocation[]>([]);
  const [donations, setDonations] = useState<FinancialDonation[]>([]);
  const [bankAccounts, setBankAccounts] = useState<FinancialBankAccount[]>([]);
  const [reconciliationPeriods, setReconciliationPeriods] = useState<FinancialReconciliationPeriod[]>([]);
  const [authorizations, setAuthorizations] = useState<FinancialAuthorization[]>([]);
  const [members, setMembers] = useState<UserModel[]>([]);
  const [zones, setZones] = useState<Zone[]>([]);
  const [states, setStates] = useState<State[]>([]);

  // Modals & Drawers
  const [showIncomeModal, setShowIncomeModal] = useState(false);
  const [showExpenseModal, setShowExpenseModal] = useState(false);
  const [showReviewModal, setShowReviewModal] = useState<FinancialExpense | null>(null);
  const [showPayExpenseModal, setShowPayExpenseModal] = useState<FinancialExpense | null>(null);
  const [showBudgetModal, setShowBudgetModal] = useState(false);
  const [showRevisionModal, setShowRevisionModal] = useState<FinancialBudget | null>(null);
  const [showAllocationModal, setShowAllocationModal] = useState(false);
  const [showBankModal, setShowBankModal] = useState(false);
  const [showReconcileModal, setShowReconcileModal] = useState(false);
  const [showCSVModal, setShowCSVModal] = useState<FinancialReconciliationPeriod | null>(null);
  const [showPrintModal, setShowPrintModal] = useState<FinancialDocument | null>(null);
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [showReverseModal, setShowReverseModal] = useState<FinancialTransaction | null>(null);

  // Form State: Income
  const [incomeCategory, setIncomeCategory] = useState("member_contribution");
  const [incomeAmountNaira, setIncomeAmountNaira] = useState("");
  const [incomeMethod, setIncomeMethod] = useState("bank_transfer");
  const [incomeSource, setIncomeSource] = useState("");
  const [incomeFundId, setIncomeFundId] = useState("general_operations_fund");
  const [incomeNotes, setIncomeNotes] = useState("");
  const [submittingIncome, setSubmittingIncome] = useState(false);

  // Form State: Expense
  const [expTitle, setExpTitle] = useState("");
  const [expCategory, setExpCategory] = useState("cleanup_materials");
  const [expDescription, setExpDescription] = useState("");
  const [expAmountNaira, setExpAmountNaira] = useState("");
  const [expBeneficiary, setExpBeneficiary] = useState("");
  const [expBank, setExpBank] = useState("");
  const [expAccountNum, setExpAccountNum] = useState("");
  const [expStateId, setExpStateId] = useState("");
  const [expBudgetId, setExpBudgetId] = useState("");
  const [submittingExpense, setSubmittingExpense] = useState(false);

  // Form State: Review Expense
  const [reviewDecision, setReviewDecision] = useState<"under_review" | "approved" | "rejected">("approved");
  const [reviewReason, setReviewReason] = useState("");
  const [submittingReview, setSubmittingReview] = useState(false);

  // Form State: Pay Expense
  const [payReference, setPayReference] = useState("");
  const [payFundId, setPayFundId] = useState("general_operations_fund");
  const [submittingPay, setSubmittingPay] = useState(false);

  // Form State: Budget Proposal
  const [bgtTitle, setBgtTitle] = useState("");
  const [bgtScope, setBgtScope] = useState<"national" | "zone" | "state" | "community_project">("state");
  const [bgtStateId, setBgtStateId] = useState("");
  const [bgtPeriodType, setBgtPeriodType] = useState("quarterly");
  const [bgtPeriodLabel, setBgtPeriodLabel] = useState("2026-Q4");
  const [bgtAmountNaira, setBgtAmountNaira] = useState("");
  const [bgtNotes, setBgtNotes] = useState("");
  const [submittingBudget, setSubmittingBudget] = useState(false);

  // Form State: Budget Revision
  const [revAmountNaira, setRevAmountNaira] = useState("");
  const [revReason, setRevReason] = useState("");
  const [submittingRevision, setSubmittingRevision] = useState(false);

  // Form State: Allocation
  const [allocSourceScope, setAllocSourceScope] = useState("national");
  const [allocDestScope, setAllocDestScope] = useState("state");
  const [allocDestStateId, setAllocDestStateId] = useState("");
  const [allocSourceFund, setAllocSourceFund] = useState("general_operations_fund");
  const [allocDestFund, setAllocDestFund] = useState("general_operations_fund");
  const [allocAmountNaira, setAllocAmountNaira] = useState("");
  const [allocPurpose, setAllocPurpose] = useState("");
  const [submittingAlloc, setSubmittingAlloc] = useState(false);

  // Form State: Bank Account
  const [bankName, setBankName] = useState("");
  const [acctName, setAcctName] = useState("");
  const [acctNumber, setAcctNumber] = useState("");
  const [bankOrgType, setBankOrgType] = useState<"ratels" | "foundation">("ratels");
  const [bankIsPrimary, setBankIsPrimary] = useState(false);
  const [submittingBank, setSubmittingBank] = useState(false);

  // Form State: Reconciliation Period
  const [reconBankId, setReconBankId] = useState("");
  const [reconStart, setReconStart] = useState("");
  const [reconEnd, setReconEnd] = useState("");
  const [reconOpeningNaira, setReconOpeningNaira] = useState("");
  const [reconClosingNaira, setReconClosingNaira] = useState("");
  const [submittingRecon, setSubmittingRecon] = useState(false);

  // Form State: CSV Statement
  const [csvContent, setCsvContent] = useState("");
  const [submittingCSV, setSubmittingCSV] = useState(false);

  // Form State: Reversal
  const [reverseReason, setReverseReason] = useState("");
  const [submittingReverse, setSubmittingReverse] = useState(false);

  // Form State: Auth Delegation
  const [authUserId, setAuthUserId] = useState("");
  const [authLevel, setAuthLevel] = useState<"viewer" | "submitter" | "finance_officer">("finance_officer");
  const [authNotes, setAuthNotes] = useState("");
  const [submittingAuth, setSubmittingAuth] = useState(false);

  const formatNaira = (kobo: number) => {
    return new Intl.NumberFormat("en-NG", {
      style: "currency",
      currency: "NGN",
      minimumFractionDigits: 2,
    }).format(kobo / 100);
  };

  const loadAllFinanceData = async () => {
    setLoading(true);
    setUnauthorized(false);
    try {
      // 1. Authoritative Dashboard Summary
      const summaryData = await apiRequest<FinancialDashboardSummary>("/finance/dashboard/summary");
      setSummary(summaryData);

      // 2. Funds & Ledger Statement
      const [fundsData, stmtData] = await Promise.all([
        apiRequest<FinancialFund[]>("/finance/funds"),
        apiRequest<FinancialStatement>("/finance/statement"),
      ]);
      setFunds(fundsData || []);
      setStatement(stmtData);

      // 3. Expenses
      try {
        const expData = await apiRequest<FinancialExpense[]>("/finance/expenses");
        setExpenses(expData || []);
      } catch {
        setExpenses([]);
      }

      // 4. Budgets
      try {
        const bgtData = await apiRequest<FinancialBudget[]>("/finance/budgets");
        setBudgets(bgtData || []);
      } catch {
        setBudgets([]);
      }

      // 5. Allocations
      try {
        const allocData = await apiRequest<FinancialAllocation[]>("/finance/allocations");
        setAllocations(allocData || []);
      } catch {
        setAllocations([]);
      }

      // 6. Donations
      try {
        const donData = await apiRequest<FinancialDonation[]>("/finance/donations");
        setDonations(donData || []);
      } catch {
        setDonations([]);
      }

      // 7. Bank Accounts
      try {
        const bankData = await apiRequest<FinancialBankAccount[]>("/finance/bank-accounts");
        setBankAccounts(bankData || []);
      } catch {
        setBankAccounts([]);
      }

      // 8. Reconciliation Periods
      try {
        const reconData = await apiRequest<FinancialReconciliationPeriod[]>("/finance/reconciliations");
        setReconciliationPeriods(reconData || []);
      } catch {
        setReconciliationPeriods([]);
      }

      // 9. Geography helpers
      try {
        const [zData, sData] = await Promise.all([
          apiRequest<Zone[]>("/geography/zones"),
          apiRequest<State[]>("/geography/states"),
        ]);
        setZones(zData || []);
        setStates(sData || []);
      } catch {}

      // 10. Super Admin privileged data (authorizations & members)
      if (isSuperAdmin) {
        try {
          const [auths, mems] = await Promise.all([
            apiRequest<FinancialAuthorization[]>("/finance/authorizations"),
            apiRequest<UserModel[]>("/members?limit=100"),
          ]);
          setAuthorizations(auths || []);
          setMembers(mems || []);
        } catch {}
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
      loadAllFinanceData();
    }
  }, [user, isLoading]);

  // Actions
  const handleRecordIncome = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmittingIncome(true);
    setFeedback(null);
    const amountKobo = Math.round(parseFloat(incomeAmountNaira) * 100);
    try {
      await apiRequest("/finance/income", {
        method: "POST",
        body: JSON.stringify({
          income_category: incomeCategory,
          amount_kobo: amountKobo,
          currency: "NGN",
          payment_method: incomeMethod,
          funding_source: incomeSource,
          destination_fund_id: incomeFundId,
          notes: incomeNotes || undefined,
        }),
      });
      setFeedback({ type: "success", message: "Income recorded successfully and submitted for verification." });
      setShowIncomeModal(false);
      setIncomeAmountNaira("");
      setIncomeSource("");
      setIncomeNotes("");
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to record income" });
    } finally {
      setSubmittingIncome(false);
    }
  };

  const handleVerifyIncome = async (txId: string, decision: "verified" | "rejected") => {
    setFeedback(null);
    try {
      await apiRequest(`/finance/income/${txId}/verify`, {
        method: "POST",
        body: JSON.stringify({ decision, notes: `Actioned by ${user?.display_name}` }),
      });
      setFeedback({ type: "success", message: `Income transaction ${decision === "verified" ? "verified and posted to ledger" : "rejected"}.` });
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to action income verification" });
    }
  };

  const handleCreateExpense = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmittingExpense(true);
    setFeedback(null);
    const amountKobo = Math.round(parseFloat(expAmountNaira) * 100);
    try {
      await apiRequest("/finance/expenses", {
        method: "POST",
        body: JSON.stringify({
          title: expTitle,
          category: expCategory,
          description: expDescription,
          amount_kobo: amountKobo,
          currency: "NGN",
          beneficiary_name: expBeneficiary,
          beneficiary_bank_name: expBank || undefined,
          beneficiary_account_number: expAccountNum || undefined,
          state_id: expStateId || undefined,
          budget_id: expBudgetId || undefined,
        }),
      });
      setFeedback({ type: "success", message: "Expense request submitted for verification and review." });
      setShowExpenseModal(false);
      setExpTitle("");
      setExpDescription("");
      setExpAmountNaira("");
      setExpBeneficiary("");
      setExpBank("");
      setExpAccountNum("");
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to submit expense" });
    } finally {
      setSubmittingExpense(false);
    }
  };

  const handleReviewExpense = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!showReviewModal) return;
    setSubmittingReview(true);
    setFeedback(null);
    try {
      await apiRequest(`/finance/expenses/${showReviewModal.id}/review`, {
        method: "POST",
        body: JSON.stringify({
          decision: reviewDecision,
          rejection_reason: reviewDecision === "rejected" ? reviewReason : undefined,
        }),
      });
      setFeedback({ type: "success", message: `Expense request marked as ${reviewDecision}.` });
      setShowReviewModal(null);
      setReviewReason("");
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to review expense" });
    } finally {
      setSubmittingReview(false);
    }
  };

  const handleAuthorizeExpense = async (expId: string) => {
    setFeedback(null);
    try {
      await apiRequest(`/finance/expenses/${expId}/authorize`, {
        method: "POST",
        body: JSON.stringify({ notes: "Authorized for disbursement" }),
      });
      setFeedback({ type: "success", message: "Expense authorized for disbursement." });
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to authorize expense" });
    }
  };

  const handlePayExpense = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!showPayExpenseModal) return;
    setSubmittingPay(true);
    setFeedback(null);
    try {
      await apiRequest(`/finance/expenses/${showPayExpenseModal.id}/pay`, {
        method: "POST",
        body: JSON.stringify({
          payment_reference: payReference || `PAY-DISB-${Date.now()}`,
        }),
      });
      setFeedback({ type: "success", message: "Disbursement recorded and posted to double-entry ledger!" });
      setShowPayExpenseModal(null);
      setPayReference("");
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to record payment" });
    } finally {
      setSubmittingPay(false);
    }
  };

  const handleCreateBudget = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmittingBudget(true);
    setFeedback(null);
    const amountKobo = Math.round(parseFloat(bgtAmountNaira) * 100);
    try {
      await apiRequest("/finance/budgets", {
        method: "POST",
        body: JSON.stringify({
          title: bgtTitle,
          scope_type: bgtScope,
          state_id: bgtStateId || undefined,
          period_type: bgtPeriodType,
          period_label: bgtPeriodLabel,
          proposed_amount_kobo: amountKobo,
          notes: bgtNotes || undefined,
        }),
      });
      setFeedback({ type: "success", message: "Budget proposal created successfully." });
      setShowBudgetModal(false);
      setBgtTitle("");
      setBgtAmountNaira("");
      setBgtNotes("");
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to create budget" });
    } finally {
      setSubmittingBudget(false);
    }
  };

  const handleApproveBudget = async (budgetId: string) => {
    setFeedback(null);
    try {
      await apiRequest(`/finance/budgets/${budgetId}/approve`, {
        method: "POST",
      });
      setFeedback({ type: "success", message: "Budget approved by Super Admin authority." });
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to approve budget" });
    }
  };

  const handleRequestRevision = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!showRevisionModal) return;
    setSubmittingRevision(true);
    setFeedback(null);
    const amountKobo = Math.round(parseFloat(revAmountNaira) * 100);
    try {
      await apiRequest(`/finance/budgets/${showRevisionModal.id}/revisions`, {
        method: "POST",
        body: JSON.stringify({
          revised_amount_kobo: amountKobo,
          reason: revReason,
        }),
      });
      setFeedback({ type: "success", message: "Budget revision requested." });
      setShowRevisionModal(null);
      setRevAmountNaira("");
      setRevReason("");
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to request budget revision" });
    } finally {
      setSubmittingRevision(false);
    }
  };

  const handleCreateAllocation = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmittingAlloc(true);
    setFeedback(null);
    const amountKobo = Math.round(parseFloat(allocAmountNaira) * 100);
    try {
      await apiRequest("/finance/allocations", {
        method: "POST",
        body: JSON.stringify({
          source_type: allocSourceScope,
          destination_type: allocDestScope,
          destination_id: allocDestStateId || undefined,
          source_fund_id: allocSourceFund,
          destination_fund_id: allocDestFund,
          amount_kobo: amountKobo,
          currency: "NGN",
          purpose: allocPurpose,
          approval_reference: `ALLOC-AUTH-${Date.now()}`,
        }),
      });
      setFeedback({ type: "success", message: "Controlled fund allocation recorded atomically (internal transfers do not inflate income)." });
      setShowAllocationModal(false);
      setAllocAmountNaira("");
      setAllocPurpose("");
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to create fund allocation" });
    } finally {
      setSubmittingAlloc(false);
    }
  };

  const handleRegisterBank = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmittingBank(true);
    setFeedback(null);
    try {
      await apiRequest("/finance/bank-accounts", {
        method: "POST",
        body: JSON.stringify({
          bank_name: bankName,
          account_name: acctName,
          account_number: acctNumber,
          currency: "NGN",
          org_type: bankOrgType,
          is_primary: bankIsPrimary,
        }),
      });
      setFeedback({ type: "success", message: "Official bank account registered." });
      setShowBankModal(false);
      setBankName("");
      setAcctName("");
      setAcctNumber("");
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to register bank account" });
    } finally {
      setSubmittingBank(false);
    }
  };

  const handleActivateBank = async (bankId: string) => {
    setFeedback(null);
    try {
      await apiRequest(`/finance/bank-accounts/${bankId}/activate`, {
        method: "POST",
      });
      setFeedback({ type: "success", message: "Bank account officially activated by Super Admin." });
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to activate bank account" });
    }
  };

  const handleCreateReconciliation = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmittingRecon(true);
    setFeedback(null);
    const openingKobo = Math.round(parseFloat(reconOpeningNaira) * 100);
    const closingKobo = Math.round(parseFloat(reconClosingNaira) * 100);
    try {
      await apiRequest("/finance/reconciliations", {
        method: "POST",
        body: JSON.stringify({
          bank_account_id: reconBankId,
          period_start: new Date(reconStart).toISOString(),
          period_end: new Date(reconEnd).toISOString(),
          opening_balance_kobo: openingKobo,
          closing_balance_kobo: closingKobo,
          statement_closing_balance_kobo: closingKobo,
        }),
      });
      setFeedback({ type: "success", message: "Reconciliation period opened." });
      setShowReconcileModal(false);
      setReconOpeningNaira("");
      setReconClosingNaira("");
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to open reconciliation period" });
    } finally {
      setSubmittingRecon(false);
    }
  };

  const handleImportCSV = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!showCSVModal) return;
    setSubmittingCSV(true);
    setFeedback(null);
    try {
      const res = await apiRequest<{ imported_count: number }>(`/finance/reconciliations/${showCSVModal.id}/import-csv`, {
        method: "POST",
        body: csvContent,
      });
      setFeedback({ type: "success", message: `Successfully imported statement items. Duplicate detection active.` });
      setShowCSVModal(null);
      setCsvContent("");
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to import CSV statement" });
    } finally {
      setSubmittingCSV(false);
    }
  };

  const handleAutoMatch = async (periodId: string) => {
    setFeedback(null);
    try {
      const res = await apiRequest<{ matched_count: number }>(`/finance/reconciliations/${periodId}/auto-match`, {
        method: "POST",
      });
      setFeedback({ type: "success", message: `Auto-matching executed. Transactions reconciled.` });
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to auto-match" });
    }
  };

  const handleCertifyReconciliation = async (periodId: string) => {
    setFeedback(null);
    try {
      await apiRequest(`/finance/reconciliations/${periodId}/certify`, {
        method: "POST",
        body: JSON.stringify({ notes: "Certified official bank statement matching" }),
      });
      setFeedback({ type: "success", message: "Reconciliation certified with Super Admin signature." });
      await loadAllFinanceData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to certify reconciliation" });
    }
  };

  const handlePrintDocument = async (docType: string, entityId: string) => {
    setFeedback(null);
    try {
      const doc = await apiRequest<FinancialDocument>(`/finance/documents/${docType}/${entityId}`);
      setShowPrintModal(doc);
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to generate printable document" });
    }
  };

  if (unauthorized) {
    return (
      <div className="min-h-screen bg-slate-900 text-white flex items-center justify-center p-6">
        <div className="max-w-md w-full bg-slate-800 border border-slate-700 rounded-xl p-8 text-center space-y-6">
          <div className="w-16 h-16 bg-red-500/10 text-red-400 rounded-full flex items-center justify-center mx-auto border border-red-500/20">
            <ShieldAlert className="w-8 h-8" />
          </div>
          <div>
            <h2 className="text-xl font-bold text-white mb-2">Restricted Financial Jurisdiction</h2>
            <p className="text-sm text-slate-400">
              Access to the Ratels Financial Ledger and accountability systems requires Super Admin authority or explicitly delegated clearance.
            </p>
          </div>
          <div className="pt-2">
            <Link
              href="/admin"
              className="inline-flex items-center justify-center px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium transition-colors w-full"
            >
              <ArrowLeft className="w-4 h-4 mr-2" /> Return to Admin Panel
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 p-6 md:p-8">
      {/* Header */}
      <div className="max-w-7xl mx-auto space-y-8">
        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b border-slate-800 pb-6">
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-emerald-400 text-sm font-semibold tracking-wider uppercase">
              <ShieldCheck className="w-4 h-4" /> Official Ratels Financial Oversight
            </div>
            <h1 className="text-3xl font-bold text-white tracking-tight flex items-center gap-3">
              Finance & Accountability System
            </h1>
            <p className="text-sm text-slate-400">
              Transparent income, expenditures, budgets, project allocations, and bank reconciliation.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <button
              onClick={loadAllFinanceData}
              disabled={loading}
              className="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-200 rounded-lg text-sm font-medium flex items-center gap-2 transition"
            >
              <RefreshCw className={`w-4 h-4 ${loading ? "animate-spin" : ""}`} /> Refresh
            </button>
            <button
              onClick={() => setShowIncomeModal(true)}
              className="px-3.5 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium flex items-center gap-2 shadow-lg shadow-emerald-900/30 transition"
            >
              <PlusCircle className="w-4 h-4" /> Record Income
            </button>
            <button
              onClick={() => setShowExpenseModal(true)}
              className="px-3.5 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium flex items-center gap-2 shadow-lg shadow-blue-900/30 transition"
            >
              <DollarSign className="w-4 h-4" /> Request Expense
            </button>
          </div>
        </div>

        {/* Feedback Alert */}
        {feedback && (
          <Alert
            type={feedback.type}
            className="mb-4"
          >
            {feedback.message}
          </Alert>
        )}

        {/* 10-Tab Navigation */}
        <div className="border-b border-slate-800 overflow-x-auto scrollbar-none">
          <nav className="flex space-x-2 pb-px" aria-label="Tabs">
            {[
              { id: "overview", label: "Overview", icon: TrendingUp },
              { id: "income", label: "Income & Inflows", icon: Coins },
              { id: "expenses", label: "Expenses & Vouchers", icon: DollarSign },
              { id: "budgets", label: "Budgets & Revisions", icon: Wallet },
              { id: "allocations", label: "Fund Allocations", icon: Send },
              { id: "donations", label: "Donations & MOF", icon: Layers },
              { id: "bank_accounts", label: "Bank Accounts", icon: Building },
              { id: "reconciliation", label: "Reconciliation", icon: FileSpreadsheet },
              { id: "documents", label: "Documentation", icon: FileText },
              { id: "audit", label: "Governance & Audit", icon: ShieldCheck },
            ].map((tab) => {
              const Icon = tab.icon;
              const isActive = activeTab === tab.id;
              return (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id as FinanceTab)}
                  className={`flex items-center gap-2 px-4 py-3 text-sm font-medium rounded-t-lg transition-colors whitespace-nowrap border-b-2 ${
                    isActive
                      ? "border-emerald-500 text-emerald-400 bg-slate-900/80"
                      : "border-transparent text-slate-400 hover:text-slate-200 hover:bg-slate-900/40"
                  }`}
                >
                  <Icon className="w-4 h-4" />
                  {tab.label}
                </button>
              );
            })}
          </nav>
        </div>

        {/* Tab 1: Overview */}
        {activeTab === "overview" && (
          <div className="space-y-8">
            {/* Top Stat Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm space-y-2">
                <div className="flex items-center justify-between text-slate-400 text-xs font-semibold uppercase tracking-wider">
                  <span>Verified Inflows</span>
                  <Coins className="w-4 h-4 text-emerald-400" />
                </div>
                <div className="text-2xl font-bold text-emerald-400">
                  {summary ? formatNaira(summary.total_verified_income_kobo) : "₦0.00"}
                </div>
                <div className="text-xs text-slate-400">Total verified external receipts</div>
              </div>

              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm space-y-2">
                <div className="flex items-center justify-between text-slate-400 text-xs font-semibold uppercase tracking-wider">
                  <span>Verified Outflows</span>
                  <DollarSign className="w-4 h-4 text-rose-400" />
                </div>
                <div className="text-2xl font-bold text-rose-400">
                  {summary ? formatNaira(summary.total_verified_expenditure_kobo) : "₦0.00"}
                </div>
                <div className="text-xs text-slate-400">Authorized disbursements paid</div>
              </div>

              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm space-y-2">
                <div className="flex items-center justify-between text-slate-400 text-xs font-semibold uppercase tracking-wider">
                  <span>Cash Balance</span>
                  <Wallet className="w-4 h-4 text-blue-400" />
                </div>
                <div className="text-2xl font-bold text-blue-400">
                  {summary ? formatNaira(summary.available_cash_balance_kobo) : "₦0.00"}
                </div>
                <div className="text-xs text-slate-400">Net verified cash on hand</div>
              </div>

              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm space-y-2">
                <div className="flex items-center justify-between text-slate-400 text-xs font-semibold uppercase tracking-wider">
                  <span>Spendable Available</span>
                  <ShieldCheck className="w-4 h-4 text-amber-400" />
                </div>
                <div className="text-2xl font-bold text-amber-400">
                  {summary ? formatNaira(summary.available_to_spend_kobo) : "₦0.00"}
                </div>
                <div className="text-xs text-slate-400">
                  Cash minus {summary ? formatNaira(summary.outstanding_commitments_kobo) : "₦0.00"} commitments
                </div>
              </div>
            </div>

            {/* Sub-Metrics & Trends */}
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
              {/* Trends Card */}
              <div className="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-xl p-6 space-y-5">
                <div className="flex items-center justify-between border-b border-slate-800 pb-3">
                  <h3 className="font-semibold text-white flex items-center gap-2">
                    <TrendingUp className="w-4 h-4 text-emerald-400" /> Six-Month Financial Velocity
                  </h3>
                  <span className="text-xs text-slate-400">Authoritative double-entry ledger totals</span>
                </div>
                <div className="space-y-4">
                  {summary?.monthly_trends && summary.monthly_trends.length > 0 ? (
                    summary.monthly_trends.map((item, idx) => (
                      <div key={idx} className="space-y-1.5">
                        <div className="flex justify-between text-xs font-medium">
                          <span className="text-slate-300">{item.month_label}</span>
                          <span className="text-slate-400">
                            In: <strong className="text-emerald-400">{formatNaira(item.income_kobo)}</strong> | Out:{" "}
                            <strong className="text-rose-400">{formatNaira(item.expenditure_kobo)}</strong>
                          </span>
                        </div>
                        <div className="h-2.5 bg-slate-800 rounded-full flex overflow-hidden">
                          <div
                            className="bg-emerald-500 h-full"
                            style={{
                              width: `${Math.min(100, Math.max(5, (item.income_kobo / (item.income_kobo + item.expenditure_kobo || 1)) * 100))}%`,
                            }}
                          />
                          <div
                            className="bg-rose-500 h-full"
                            style={{
                              width: `${Math.min(100, Math.max(5, (item.expenditure_kobo / (item.income_kobo + item.expenditure_kobo || 1)) * 100))}%`,
                            }}
                          />
                        </div>
                      </div>
                    ))
                  ) : (
                    <div className="text-sm text-slate-400 py-6 text-center">No historical monthly data recorded yet.</div>
                  )}
                </div>
              </div>

              {/* Fund Balances Card */}
              <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 space-y-4">
                <div className="flex items-center justify-between border-b border-slate-800 pb-3">
                  <h3 className="font-semibold text-white flex items-center gap-2">
                    <Building className="w-4 h-4 text-blue-400" /> Dedicated Fund Balances
                  </h3>
                  <span className="text-xs text-slate-400">Live Ledgers</span>
                </div>
                <div className="space-y-3">
                  {summary?.fund_balances && summary.fund_balances.length > 0 ? (
                    summary.fund_balances.map((fund) => (
                      <div key={fund.fund_id} className="p-3 bg-slate-950/60 rounded-lg border border-slate-800/80 flex justify-between items-center">
                        <div>
                          <div className="text-sm font-medium text-white">{fund.fund_name}</div>
                          <div className="text-xs text-slate-500 font-mono">{fund.fund_id}</div>
                        </div>
                        <div className="text-sm font-semibold text-emerald-400">{formatNaira(fund.balance_kobo)}</div>
                      </div>
                    ))
                  ) : (
                    <div className="text-sm text-slate-400 py-4 text-center">Funds loading...</div>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Tab 2: Income Management */}
        {activeTab === "income" && (
          <div className="space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-slate-900 p-4 rounded-xl border border-slate-800">
              <div>
                <h3 className="font-semibold text-white">Income Ledger Records</h3>
                <p className="text-xs text-slate-400">
                  Track member dues, verified grants, donations, and commercial inflows. Manual records require explicit clearance.
                </p>
              </div>
              <button
                onClick={() => setShowIncomeModal(true)}
                className="px-3.5 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium flex items-center gap-2 transition"
              >
                <PlusCircle className="w-4 h-4" /> Record New Income
              </button>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm text-slate-300">
                  <thead className="bg-slate-950 text-slate-400 text-xs uppercase tracking-wider border-b border-slate-800">
                    <tr>
                      <th className="p-4">Reference</th>
                      <th className="p-4">Category</th>
                      <th className="p-4">Funding Source</th>
                      <th className="p-4">Amount</th>
                      <th className="p-4">Status</th>
                      <th className="p-4">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {statement?.transactions && statement.transactions.length > 0 ? (
                      statement.transactions
                        .filter((tx) => tx.is_external_income || tx.transaction_type.startsWith("income_") || tx.transaction_type === "confirmed_donation")
                        .map((tx) => (
                          <tr key={tx.id} className="hover:bg-slate-800/40 transition">
                            <td className="p-4 font-mono text-xs text-white">{tx.reference}</td>
                            <td className="p-4 capitalize">{tx.transaction_type.replace("income_", "").replace("_", " ")}</td>
                            <td className="p-4 text-slate-400">{tx.destination_fund_name || "General Fund"}</td>
                            <td className="p-4 font-semibold text-emerald-400">{formatNaira(tx.amount)}</td>
                            <td className="p-4">
                              <Badge variant={tx.status === "posted" ? "success" : tx.status === "reversed" ? "danger" : "warning"}>
                                {tx.status}
                              </Badge>
                            </td>
                            <td className="p-4 flex items-center gap-2">
                              {tx.status === "pending" && isSuperAdmin && (
                                <button
                                  onClick={() => handleVerifyIncome(tx.id, "verified")}
                                  className="px-2.5 py-1 bg-emerald-600/20 text-emerald-400 border border-emerald-500/30 rounded text-xs hover:bg-emerald-600/30 transition"
                                >
                                  Verify & Post
                                </button>
                              )}
                              <button
                                onClick={() => handlePrintDocument("income_receipt", tx.id)}
                                className="px-2.5 py-1 bg-slate-800 text-slate-300 border border-slate-700 rounded text-xs hover:bg-slate-700 transition flex items-center gap-1"
                              >
                                <Printer className="w-3.5 h-3.5" /> Receipt
                              </button>
                            </td>
                          </tr>
                        ))
                    ) : (
                      <tr>
                        <td colSpan={6} className="p-8 text-center text-slate-400">
                          No income records matching filters.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {/* Tab 3: Expenses */}
        {activeTab === "expenses" && (
          <div className="space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-slate-900 p-4 rounded-xl border border-slate-800">
              <div>
                <h3 className="font-semibold text-white">Expenditure Requests & Disbursements</h3>
                <p className="text-xs text-slate-400">
                  Lifecycle: Draft → Submitted → Reviewed → Authorized → Paid → Reconciled. Anti-self-approval enforced.
                </p>
              </div>
              <button
                onClick={() => setShowExpenseModal(true)}
                className="px-3.5 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium flex items-center gap-2 transition"
              >
                <DollarSign className="w-4 h-4" /> Propose Expenditure
              </button>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm text-slate-300">
                  <thead className="bg-slate-950 text-slate-400 text-xs uppercase tracking-wider border-b border-slate-800">
                    <tr>
                      <th className="p-4">Reference</th>
                      <th className="p-4">Title & Category</th>
                      <th className="p-4">Beneficiary</th>
                      <th className="p-4">Amount</th>
                      <th className="p-4">Status</th>
                      <th className="p-4">Workflow Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {expenses && expenses.length > 0 ? (
                      expenses.map((exp) => (
                        <tr key={exp.id} className="hover:bg-slate-800/40 transition">
                          <td className="p-4 font-mono text-xs text-white">{exp.reference}</td>
                          <td className="p-4">
                            <div className="font-medium text-white">{exp.title}</div>
                            <div className="text-xs text-slate-400 capitalize">{exp.category.replace("_", " ")}</div>
                          </td>
                          <td className="p-4 text-slate-300">
                            <div>{exp.beneficiary_name}</div>
                            {exp.beneficiary_bank_name && (
                              <div className="text-xs text-slate-500">
                                {exp.beneficiary_bank_name} • {exp.beneficiary_account_number}
                              </div>
                            )}
                          </td>
                          <td className="p-4 font-semibold text-rose-400">{formatNaira(exp.amount_kobo)}</td>
                          <td className="p-4">
                            <Badge
                              variant={
                                exp.status === "paid"
                                  ? "success"
                                  : exp.status === "authorized_for_payment"
                                  ? "info"
                                  : exp.status === "approved"
                                  ? "warning"
                                  : exp.status === "rejected"
                                  ? "danger"
                                  : "neutral"
                              }
                            >
                              {exp.status.replace("_", " ")}
                            </Badge>
                          </td>
                          <td className="p-4 flex items-center gap-2">
                            {exp.status === "submitted" && (
                              <button
                                onClick={() => setShowReviewModal(exp)}
                                className="px-2.5 py-1 bg-amber-600/20 text-amber-400 border border-amber-500/30 rounded text-xs hover:bg-amber-600/30 transition"
                              >
                                Review
                              </button>
                            )}
                            {exp.status === "approved" && isSuperAdmin && (
                              <button
                                onClick={() => handleAuthorizeExpense(exp.id)}
                                className="px-2.5 py-1 bg-blue-600/20 text-blue-400 border border-blue-500/30 rounded text-xs hover:bg-blue-600/30 transition"
                              >
                                Authorize
                              </button>
                            )}
                            {(exp.status === "authorized_for_payment" || exp.status === "approved") && isSuperAdmin && (
                              <button
                                onClick={() => setShowPayExpenseModal(exp)}
                                className="px-2.5 py-1 bg-emerald-600 text-white rounded text-xs hover:bg-emerald-500 transition"
                              >
                                Record Payment
                              </button>
                            )}
                            {exp.status === "paid" && (
                              <button
                                onClick={() => handlePrintDocument("expense_voucher", exp.id)}
                                className="px-2.5 py-1 bg-slate-800 text-slate-300 border border-slate-700 rounded text-xs hover:bg-slate-700 transition flex items-center gap-1"
                              >
                                <Printer className="w-3.5 h-3.5" /> Voucher
                              </button>
                            )}
                          </td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={6} className="p-8 text-center text-slate-400">
                          No expense requests submitted yet.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {/* Tab 4: Budgets */}
        {activeTab === "budgets" && (
          <div className="space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-slate-900 p-4 rounded-xl border border-slate-800">
              <div>
                <h3 className="font-semibold text-white">Organizational Budgets & Spending Controls</h3>
                <p className="text-xs text-slate-400">
                  Track allocated, committed, and spent ceilings across National, Zonal, State, and Project levels.
                </p>
              </div>
              <button
                onClick={() => setShowBudgetModal(true)}
                className="px-3.5 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium flex items-center gap-2 transition"
              >
                <PlusCircle className="w-4 h-4" /> Propose New Budget
              </button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {budgets && budgets.length > 0 ? (
                budgets.map((bgt) => {
                  const spentPct = bgt.approved_amount_kobo > 0 ? Math.round((bgt.spent_amount_kobo / bgt.approved_amount_kobo) * 100) : 0;
                  const remainingKobo = bgt.approved_amount_kobo - bgt.spent_amount_kobo - bgt.committed_amount_kobo;
                  return (
                    <div key={bgt.id} className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
                      <div className="flex justify-between items-start">
                        <div>
                          <Badge variant={bgt.status === "approved" ? "success" : "warning"}>{bgt.status}</Badge>
                          <h4 className="text-base font-bold text-white mt-1.5">{bgt.title}</h4>
                          <div className="text-xs text-slate-400">
                            Scope: <strong className="capitalize text-slate-300">{bgt.scope_type}</strong> • Period:{" "}
                            <span className="font-mono text-emerald-400">{bgt.period_label}</span>
                          </div>
                        </div>
                        <span className="text-xs font-mono text-slate-500">{bgt.reference}</span>
                      </div>

                      <div className="space-y-1.5">
                        <div className="flex justify-between text-xs">
                          <span className="text-slate-400">Approved Budget</span>
                          <span className="font-bold text-white">{formatNaira(bgt.approved_amount_kobo || bgt.proposed_amount_kobo)}</span>
                        </div>
                        <div className="flex justify-between text-xs">
                          <span className="text-slate-400">Spent to Date</span>
                          <span className="text-rose-400 font-medium">{formatNaira(bgt.spent_amount_kobo)}</span>
                        </div>
                        <div className="flex justify-between text-xs">
                          <span className="text-slate-400">Committed (In-Flight)</span>
                          <span className="text-amber-400 font-medium">{formatNaira(bgt.committed_amount_kobo)}</span>
                        </div>
                        <div className="flex justify-between text-xs pt-1 border-t border-slate-800 font-semibold">
                          <span className="text-slate-300">Remaining Available</span>
                          <span className={remainingKobo >= 0 ? "text-emerald-400" : "text-rose-500"}>
                            {formatNaira(remainingKobo)}
                          </span>
                        </div>
                        <div className="h-2 bg-slate-800 rounded-full overflow-hidden mt-2">
                          <div className="bg-emerald-500 h-full" style={{ width: `${Math.min(100, spentPct)}%` }} />
                        </div>
                      </div>

                      <div className="flex items-center gap-2 pt-2 border-t border-slate-800/80">
                        {bgt.status === "submitted" && isSuperAdmin && (
                          <button
                            onClick={() => handleApproveBudget(bgt.id)}
                            className="flex-1 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded text-xs font-medium transition"
                          >
                            Approve Budget
                          </button>
                        )}
                        <button
                          onClick={() => setShowRevisionModal(bgt)}
                          className="flex-1 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 rounded text-xs font-medium transition"
                        >
                          Request Revision
                        </button>
                        <button
                          onClick={() => handlePrintDocument("budget_document", bgt.id)}
                          className="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 rounded transition"
                          title="Print Budget Document"
                        >
                          <Printer className="w-4 h-4" />
                        </button>
                      </div>
                    </div>
                  );
                })
              ) : (
                <div className="col-span-full p-8 text-center text-slate-400 bg-slate-900 rounded-xl border border-slate-800">
                  No active budgets recorded.
                </div>
              )}
            </div>
          </div>
        )}

        {/* Tab 5: Allocations */}
        {activeTab === "allocations" && (
          <div className="space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-slate-900 p-4 rounded-xl border border-slate-800">
              <div>
                <h3 className="font-semibold text-white">Controlled Fund Allocations & Transfers</h3>
                <p className="text-xs text-slate-400">
                  Atomic transfers between tiers (National → Zone → State → LGA → Projects). Internal transfers never inflate organizational income.
                </p>
              </div>
              <button
                onClick={() => setShowAllocationModal(true)}
                className="px-3.5 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-sm font-medium flex items-center gap-2 transition"
              >
                <Send className="w-4 h-4" /> Disburse Allocation
              </button>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm text-slate-300">
                  <thead className="bg-slate-950 text-slate-400 text-xs uppercase tracking-wider border-b border-slate-800">
                    <tr>
                      <th className="p-4">Reference</th>
                      <th className="p-4">Transfer Route</th>
                      <th className="p-4">Purpose</th>
                      <th className="p-4">Amount</th>
                      <th className="p-4">Timestamp</th>
                      <th className="p-4">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {allocations && allocations.length > 0 ? (
                      allocations.map((alloc) => (
                        <tr key={alloc.id} className="hover:bg-slate-800/40 transition">
                          <td className="p-4 font-mono text-xs text-white">{alloc.reference}</td>
                          <td className="p-4">
                            <span className="capitalize font-semibold text-emerald-400">{alloc.source_type}</span> →{" "}
                            <span className="capitalize font-semibold text-blue-400">{alloc.destination_type}</span>
                          </td>
                          <td className="p-4 text-slate-300">{alloc.purpose}</td>
                          <td className="p-4 font-semibold text-white">{formatNaira(alloc.amount_kobo)}</td>
                          <td className="p-4 text-xs text-slate-400">{new Date(alloc.created_at).toLocaleDateString()}</td>
                          <td className="p-4">
                            <Badge variant="success">Completed</Badge>
                          </td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={6} className="p-8 text-center text-slate-400">
                          No fund allocations recorded yet.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {/* Tab 6: Donations & MOF */}
        {activeTab === "donations" && (
          <div className="space-y-6">
            <div className="bg-slate-900 p-4 rounded-xl border border-slate-800 flex justify-between items-center">
              <div>
                <h3 className="font-semibold text-white">Donations & Martinsvincent Ortse Foundation</h3>
                <p className="text-xs text-slate-400">
                  Segregated organizational accounting with donor privacy protection. Only verified payments update balances.
                </p>
              </div>
              <div className="flex gap-2 text-xs">
                <span className="px-3 py-1 bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 rounded-full font-medium">
                  Ratels Worldwide
                </span>
                <span className="px-3 py-1 bg-blue-500/10 text-blue-400 border border-blue-500/20 rounded-full font-medium">
                  Martinsvincent Ortse Foundation
                </span>
              </div>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm text-slate-300">
                  <thead className="bg-slate-950 text-slate-400 text-xs uppercase tracking-wider border-b border-slate-800">
                    <tr>
                      <th className="p-4">Reference</th>
                      <th className="p-4">Organization</th>
                      <th className="p-4">Donor</th>
                      <th className="p-4">Amount</th>
                      <th className="p-4">Status</th>
                      <th className="p-4">Gateway Reference</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {donations && donations.length > 0 ? (
                      donations.map((don) => (
                        <tr key={don.id} className="hover:bg-slate-800/40 transition">
                          <td className="p-4 font-mono text-xs text-white">{don.reference}</td>
                          <td className="p-4 capitalize">
                            <Badge variant={don.entity_type === "foundation" ? "info" : "success"}>
                              {don.entity_type === "foundation" ? "Ortse Foundation" : "Ratels Worldwide"}
                            </Badge>
                          </td>
                          <td className="p-4">
                            <span className={don.is_anonymous ? "italic text-slate-400" : "font-medium text-white"}>
                              {don.donor_name}
                            </span>
                          </td>
                          <td className="p-4 font-semibold text-emerald-400">{formatNaira(don.amount_kobo)}</td>
                          <td className="p-4">
                            <Badge variant={don.status === "verified" ? "success" : "warning"}>{don.status}</Badge>
                          </td>
                          <td className="p-4 font-mono text-xs text-slate-500">{don.payment_reference || "N/A"}</td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={6} className="p-8 text-center text-slate-400">
                          No donations recorded yet.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {/* Tab 7: Bank Accounts */}
        {activeTab === "bank_accounts" && (
          <div className="space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-slate-900 p-4 rounded-xl border border-slate-800">
              <div>
                <h3 className="font-semibold text-white">Official Bank Accounts</h3>
                <p className="text-xs text-slate-400">
                  Protected bank account numbers with masking for non-super-admins. Activation strictly reserved for Super Admin.
                </p>
              </div>
              <button
                onClick={() => setShowBankModal(true)}
                className="px-3.5 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium flex items-center gap-2 transition"
              >
                <PlusCircle className="w-4 h-4" /> Register Bank Account
              </button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {bankAccounts && bankAccounts.length > 0 ? (
                bankAccounts.map((acc) => (
                  <div key={acc.id} className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
                    <div className="flex justify-between items-start">
                      <div>
                        <div className="text-xs text-slate-400 uppercase font-semibold tracking-wider">{acc.bank_name}</div>
                        <h4 className="text-base font-bold text-white mt-0.5">{acc.account_name}</h4>
                      </div>
                      <Badge variant={acc.status === "active" ? "success" : "warning"}>{acc.status}</Badge>
                    </div>

                    <div className="p-3 bg-slate-950 rounded-lg border border-slate-800 space-y-1">
                      <div className="text-xs text-slate-500">Account Number:</div>
                      <div className="text-lg font-mono font-bold tracking-widest text-emerald-400">{acc.account_number}</div>
                    </div>

                    <div className="flex justify-between items-center text-xs text-slate-400">
                      <span>Org: <strong className="capitalize text-slate-300">{acc.org_type}</strong></span>
                      {acc.is_primary && (
                        <span className="px-2 py-0.5 bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 rounded text-xs font-semibold">
                          PRIMARY ACCOUNT
                        </span>
                      )}
                    </div>

                    {acc.status !== "active" && isSuperAdmin && (
                      <button
                        onClick={() => handleActivateBank(acc.id)}
                        className="w-full py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-medium transition"
                      >
                        Activate Bank Account
                      </button>
                    )}
                  </div>
                ))
              ) : (
                <div className="col-span-full p-8 text-center text-slate-400 bg-slate-900 rounded-xl border border-slate-800">
                  No registered bank accounts.
                </div>
              )}
            </div>
          </div>
        )}

        {/* Tab 8: Reconciliation */}
        {activeTab === "reconciliation" && (
          <div className="space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-slate-900 p-4 rounded-xl border border-slate-800">
              <div>
                <h3 className="font-semibold text-white">Bank Statement Reconciliation</h3>
                <p className="text-xs text-slate-400">
                  Import CSV bank statements with duplicate protection, match against ledger lines, and certify periods.
                </p>
              </div>
              <button
                onClick={() => setShowReconcileModal(true)}
                className="px-3.5 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium flex items-center gap-2 transition"
              >
                <PlusCircle className="w-4 h-4" /> Start Reconciliation Period
              </button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {reconciliationPeriods && reconciliationPeriods.length > 0 ? (
                reconciliationPeriods.map((period) => (
                  <div key={period.id} className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
                    <div className="flex justify-between items-start">
                      <div>
                        <Badge variant={period.status === "certified" ? "success" : "warning"}>{period.status}</Badge>
                        <h4 className="text-base font-bold text-white mt-1">
                          {new Date(period.period_start).toLocaleDateString()} — {new Date(period.period_end).toLocaleDateString()}
                        </h4>
                        <div className="text-xs text-slate-400 font-mono">Account ID: {period.bank_account_id}</div>
                      </div>
                    </div>

                    <div className="grid grid-cols-2 gap-2 text-xs">
                      <div className="p-2.5 bg-slate-950 rounded border border-slate-800">
                        <div className="text-slate-500">Opening Balance</div>
                        <div className="font-bold text-white mt-0.5">{formatNaira(period.opening_balance_kobo)}</div>
                      </div>
                      <div className="p-2.5 bg-slate-950 rounded border border-slate-800">
                        <div className="text-slate-500">Closing Balance</div>
                        <div className="font-bold text-white mt-0.5">{formatNaira(period.closing_balance_kobo)}</div>
                      </div>
                    </div>

                    <div className="flex items-center gap-2 pt-2 border-t border-slate-800">
                      {period.status !== "certified" && (
                        <>
                          <button
                            onClick={() => setShowCSVModal(period)}
                            className="flex-1 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded text-xs font-medium transition flex items-center justify-center gap-1"
                          >
                            <UploadCloud className="w-3.5 h-3.5" /> Import CSV
                          </button>
                          <button
                            onClick={() => handleAutoMatch(period.id)}
                            className="flex-1 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs font-medium transition"
                          >
                            Auto-Match
                          </button>
                          {isSuperAdmin && (
                            <button
                              onClick={() => handleCertifyReconciliation(period.id)}
                              className="flex-1 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded text-xs font-medium transition"
                            >
                              Certify
                            </button>
                          )}
                        </>
                      )}
                      {period.status === "certified" && (
                        <button
                          onClick={() => handlePrintDocument("reconciliation_statement", period.id)}
                          className="w-full py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded text-xs font-medium transition flex items-center justify-center gap-1"
                        >
                          <Printer className="w-3.5 h-3.5" /> View Certified Statement
                        </button>
                      )}
                    </div>
                  </div>
                ))
              ) : (
                <div className="col-span-full p-8 text-center text-slate-400 bg-slate-900 rounded-xl border border-slate-800">
                  No active reconciliation periods.
                </div>
              )}
            </div>
          </div>
        )}

        {/* Tab 9: Documents & Reports */}
        {activeTab === "documents" && (
          <div className="space-y-6">
            <div className="bg-slate-900 p-6 rounded-xl border border-slate-800 space-y-4">
              <h3 className="font-semibold text-white flex items-center gap-2">
                <FileText className="w-5 h-5 text-emerald-400" /> Printable Financial Documentation & Certificates
              </h3>
              <p className="text-sm text-slate-400">
                Official Ratels documentation rendered with crest, document reference, amount in words, and authorized signatory seals.
              </p>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-4">
                <div className="p-4 bg-slate-950 rounded-lg border border-slate-800 space-y-2">
                  <div className="text-white font-bold text-sm">Income Receipts</div>
                  <p className="text-xs text-slate-400">Issued for verified member contributions, grants, and external donations.</p>
                </div>
                <div className="p-4 bg-slate-950 rounded-lg border border-slate-800 space-y-2">
                  <div className="text-white font-bold text-sm">Payment Vouchers</div>
                  <p className="text-xs text-slate-400">Official disbursement vouchers with payee details, category, and approval trail.</p>
                </div>
                <div className="p-4 bg-slate-950 rounded-lg border border-slate-800 space-y-2">
                  <div className="text-white font-bold text-sm">Reconciliation Certificates</div>
                  <p className="text-xs text-slate-400">Certified periodic reconciliation statements signed by Super Admin.</p>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Tab 10: Governance & Audit */}
        {activeTab === "audit" && (
          <div className="space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-slate-900 p-4 rounded-xl border border-slate-800">
              <div>
                <h3 className="font-semibold text-white">Financial Governance & Clearance Delegation</h3>
                <p className="text-xs text-slate-400">
                  Super Admin retains exclusive nationwide authority. Staff clearance delegation is strictly tracked.
                </p>
              </div>
              {isSuperAdmin && (
                <button
                  onClick={() => setShowAuthModal(true)}
                  className="px-3.5 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium flex items-center gap-2 transition"
                >
                  <UserCheck className="w-4 h-4" /> Authorize Staff Clearance
                </button>
              )}
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm text-slate-300">
                  <thead className="bg-slate-950 text-slate-400 text-xs uppercase tracking-wider border-b border-slate-800">
                    <tr>
                      <th className="p-4">Staff Member</th>
                      <th className="p-4">Email</th>
                      <th className="p-4">Access Level</th>
                      <th className="p-4">Granted By</th>
                      <th className="p-4">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {authorizations && authorizations.length > 0 ? (
                      authorizations.map((auth) => (
                        <tr key={auth.id} className="hover:bg-slate-800/40 transition">
                          <td className="p-4 font-medium text-white">{auth.user_name || auth.user_id}</td>
                          <td className="p-4 text-slate-400">{auth.user_email || "N/A"}</td>
                          <td className="p-4">
                            <Badge variant="info">{auth.access_level}</Badge>
                          </td>
                          <td className="p-4 text-slate-400">{auth.granted_by_name || "Super Admin"}</td>
                          <td className="p-4">
                            <Badge variant={auth.status === "active" ? "success" : "danger"}>{auth.status}</Badge>
                          </td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={5} className="p-8 text-center text-slate-400">
                          No delegated financial authorizations. Nationwide oversight managed directly by Super Admin.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Modal: Record Manual Income */}
      {showIncomeModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-lg w-full p-6 space-y-4">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Record Income Entry</h3>
              <button onClick={() => setShowIncomeModal(false)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleRecordIncome} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Income Category</label>
                <select
                  value={incomeCategory}
                  onChange={(e) => setIncomeCategory(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                >
                  <option value="member_contribution">Member Contribution</option>
                  <option value="donation">External Donation</option>
                  <option value="approved_grant">Approved Grant</option>
                  <option value="sponsorship">Corporate Sponsorship</option>
                  <option value="fundraising_campaign">Fundraising Campaign</option>
                  <option value="commercial_receipt">Marketplace Sale</option>
                  <option value="other">Other Legitimate Income</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Amount (NGN)</label>
                <input
                  type="number"
                  step="0.01"
                  required
                  placeholder="50000"
                  value={incomeAmountNaira}
                  onChange={(e) => setIncomeAmountNaira(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Funding Source / Payer</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Lagos State Members Chapter"
                  value={incomeSource}
                  onChange={(e) => setIncomeSource(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Destination Fund</label>
                <select
                  value={incomeFundId}
                  onChange={(e) => setIncomeFundId(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                >
                  {funds.map((f) => (
                    <option key={f.id} value={f.id}>
                      {f.name}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Notes / Description</label>
                <textarea
                  rows={2}
                  placeholder="Additional context or bank payment details"
                  value={incomeNotes}
                  onChange={(e) => setIncomeNotes(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowIncomeModal(false)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingIncome}
                  className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm font-medium hover:bg-emerald-500 transition"
                >
                  {submittingIncome ? "Recording..." : "Record & Submit"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Propose Expenditure */}
      {showExpenseModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-lg w-full p-6 space-y-4 max-h-[90vh] overflow-y-auto">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Propose Expenditure Request</h3>
              <button onClick={() => setShowExpenseModal(false)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleCreateExpense} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Title</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Cleanup waste disposal bags & logistics"
                  value={expTitle}
                  onChange={(e) => setExpTitle(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Category</label>
                  <select
                    value={expCategory}
                    onChange={(e) => setExpCategory(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                  >
                    <option value="cleanup_materials">Cleanup Materials</option>
                    <option value="waste_collection">Waste Collection</option>
                    <option value="transportation">Transportation</option>
                    <option value="community_projects">Community Projects</option>
                    <option value="administrative">Administrative</option>
                    <option value="equipment">Equipment</option>
                    <option value="event_expenses">Event Expenses</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Amount (NGN)</label>
                  <input
                    type="number"
                    step="0.01"
                    required
                    placeholder="75000"
                    value={expAmountNaira}
                    onChange={(e) => setExpAmountNaira(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Beneficiary / Vendor</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Mama Ngozi Supplies Ltd"
                  value={expBeneficiary}
                  onChange={(e) => setExpBeneficiary(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Bank Name</label>
                  <input
                    type="text"
                    placeholder="First Bank"
                    value={expBank}
                    onChange={(e) => setExpBank(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Account Number</label>
                  <input
                    type="text"
                    placeholder="0123456789"
                    value={expAccountNum}
                    onChange={(e) => setExpAccountNum(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">State Jurisdiction</label>
                <select
                  value={expStateId}
                  onChange={(e) => setExpStateId(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                >
                  <option value="">National Scope</option>
                  {states.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Description</label>
                <textarea
                  rows={2}
                  required
                  placeholder="Justification and purpose of expenditure"
                  value={expDescription}
                  onChange={(e) => setExpDescription(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowExpenseModal(false)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingExpense}
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-500 transition"
                >
                  {submittingExpense ? "Submitting..." : "Submit Expense"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Review Expense */}
      {showReviewModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-md w-full p-6 space-y-4">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Review Expenditure Request</h3>
              <button onClick={() => setShowReviewModal(null)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-2 text-sm text-slate-300 bg-slate-950 p-3 rounded-lg border border-slate-800">
              <div>Title: <strong className="text-white">{showReviewModal.title}</strong></div>
              <div>Amount: <strong className="text-rose-400">{formatNaira(showReviewModal.amount_kobo)}</strong></div>
              <div>Beneficiary: <span className="text-white">{showReviewModal.beneficiary_name}</span></div>
            </div>
            <form onSubmit={handleReviewExpense} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Decision</label>
                <select
                  value={reviewDecision}
                  onChange={(e) => setReviewDecision(e.target.value as any)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                >
                  <option value="approved">Approve Request</option>
                  <option value="under_review">Mark Under Review</option>
                  <option value="rejected">Reject Request</option>
                </select>
              </div>

              {reviewDecision === "rejected" && (
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Rejection Reason</label>
                  <textarea
                    rows={2}
                    required
                    placeholder="Document mandatory reason for rejecting this expenditure"
                    value={reviewReason}
                    onChange={(e) => setReviewReason(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                  />
                </div>
              )}

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowReviewModal(null)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingReview}
                  className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm font-medium hover:bg-emerald-500 transition"
                >
                  {submittingReview ? "Saving..." : "Submit Review"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Pay Expense */}
      {showPayExpenseModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-md w-full p-6 space-y-4">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Record Actual Payment</h3>
              <button onClick={() => setShowPayExpenseModal(null)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-1 text-sm text-slate-300 bg-slate-950 p-3 rounded-lg border border-slate-800">
              <div>Amount to Disburse: <strong className="text-emerald-400">{formatNaira(showPayExpenseModal.amount_kobo)}</strong></div>
              <div>Payee: <span className="text-white">{showPayExpenseModal.beneficiary_name}</span></div>
            </div>
            <form onSubmit={handlePayExpense} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Bank Payment Reference</label>
                <input
                  type="text"
                  placeholder="e.g. ZENITH-TRF-998811"
                  value={payReference}
                  onChange={(e) => setPayReference(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Disbursing Fund</label>
                <select
                  value={payFundId}
                  onChange={(e) => setPayFundId(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                >
                  {funds.map((f) => (
                    <option key={f.id} value={f.id}>
                      {f.name}
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowPayExpenseModal(null)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingPay}
                  className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm font-medium hover:bg-emerald-500 transition"
                >
                  {submittingPay ? "Processing..." : "Confirm & Post Ledger"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Propose Budget */}
      {showBudgetModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-lg w-full p-6 space-y-4">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Propose New Budget</h3>
              <button onClick={() => setShowBudgetModal(false)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleCreateBudget} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Budget Title</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Lagos State Q4 Sanitation Budget"
                  value={bgtTitle}
                  onChange={(e) => setBgtTitle(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Scope Level</label>
                  <select
                    value={bgtScope}
                    onChange={(e) => setBgtScope(e.target.value as any)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                  >
                    <option value="national">National Operations</option>
                    <option value="zone">Geopolitical Zone</option>
                    <option value="state">State Operations</option>
                    <option value="community_project">Community Project</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Period Label</label>
                  <input
                    type="text"
                    required
                    placeholder="2026-Q4"
                    value={bgtPeriodLabel}
                    onChange={(e) => setBgtPeriodLabel(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                  />
                </div>
              </div>

              {bgtScope === "state" && (
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">State</label>
                  <select
                    value={bgtStateId}
                    onChange={(e) => setBgtStateId(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                  >
                    <option value="">Select State</option>
                    {states.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                </div>
              )}

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Proposed Amount (NGN)</label>
                <input
                  type="number"
                  step="0.01"
                  required
                  placeholder="5000000"
                  value={bgtAmountNaira}
                  onChange={(e) => setBgtAmountNaira(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Notes</label>
                <textarea
                  rows={2}
                  placeholder="Justification and breakdown"
                  value={bgtNotes}
                  onChange={(e) => setBgtNotes(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowBudgetModal(false)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingBudget}
                  className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm font-medium hover:bg-emerald-500 transition"
                >
                  {submittingBudget ? "Creating..." : "Propose Budget"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Request Budget Revision */}
      {showRevisionModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-md w-full p-6 space-y-4">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Request Budget Revision</h3>
              <button onClick={() => setShowRevisionModal(null)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="text-sm text-slate-300 bg-slate-950 p-3 rounded-lg border border-slate-800">
              <div>Budget: <strong className="text-white">{showRevisionModal.title}</strong></div>
              <div>Current Approved: <strong className="text-emerald-400">{formatNaira(showRevisionModal.approved_amount_kobo)}</strong></div>
            </div>
            <form onSubmit={handleRequestRevision} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Revised Amount (NGN)</label>
                <input
                  type="number"
                  step="0.01"
                  required
                  placeholder="6000000"
                  value={revAmountNaira}
                  onChange={(e) => setRevAmountNaira(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Reason / Justification</label>
                <textarea
                  rows={3}
                  required
                  placeholder="Explain why an increase or modification is required"
                  value={revReason}
                  onChange={(e) => setRevReason(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowRevisionModal(null)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingRevision}
                  className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm font-medium hover:bg-emerald-500 transition"
                >
                  {submittingRevision ? "Submitting..." : "Submit Revision"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Disburse Allocation */}
      {showAllocationModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-lg w-full p-6 space-y-4">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Disburse Controlled Allocation</h3>
              <button onClick={() => setShowAllocationModal(false)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleCreateAllocation} className="space-y-4">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Source Tier</label>
                  <select
                    value={allocSourceScope}
                    onChange={(e) => setAllocSourceScope(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                  >
                    <option value="national">National Headquarters</option>
                    <option value="zone">Geopolitical Zone</option>
                    <option value="state">State Coordinator</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Destination Tier</label>
                  <select
                    value={allocDestScope}
                    onChange={(e) => setAllocDestScope(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                  >
                    <option value="zone">Geopolitical Zone</option>
                    <option value="state">State Operations</option>
                    <option value="lga">LGA Operations</option>
                    <option value="community_project">Community Project</option>
                  </select>
                </div>
              </div>

              {allocDestScope === "state" && (
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Destination State</label>
                  <select
                    value={allocDestStateId}
                    onChange={(e) => setAllocDestStateId(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                  >
                    <option value="">Select State</option>
                    {states.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                </div>
              )}

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Amount (NGN)</label>
                <input
                  type="number"
                  step="0.01"
                  required
                  placeholder="1000000"
                  value={allocAmountNaira}
                  onChange={(e) => setAllocAmountNaira(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Purpose / Justification</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Q4 State Operations disbursement"
                  value={allocPurpose}
                  onChange={(e) => setAllocPurpose(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowAllocationModal(false)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingAlloc}
                  className="px-4 py-2 bg-purple-600 text-white rounded-lg text-sm font-medium hover:bg-purple-500 transition"
                >
                  {submittingAlloc ? "Processing..." : "Confirm & Transfer"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Register Bank Account */}
      {showBankModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-md w-full p-6 space-y-4">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Register Official Bank Account</h3>
              <button onClick={() => setShowBankModal(false)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleRegisterBank} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Bank Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Zenith Bank"
                  value={bankName}
                  onChange={(e) => setBankName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Account Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Ratels Worldwide Movement"
                  value={acctName}
                  onChange={(e) => setAcctName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Account Number</label>
                <input
                  type="text"
                  required
                  placeholder="1012345678"
                  value={acctNumber}
                  onChange={(e) => setAcctNumber(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Organization Ownership</label>
                <select
                  value={bankOrgType}
                  onChange={(e) => setBankOrgType(e.target.value as any)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                >
                  <option value="ratels">Ratels Worldwide Movement</option>
                  <option value="foundation">Martinsvincent Ortse Foundation</option>
                </select>
              </div>

              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="primaryBank"
                  checked={bankIsPrimary}
                  onChange={(e) => setBankIsPrimary(e.target.checked)}
                  className="rounded border-slate-800 text-emerald-600 focus:ring-emerald-500"
                />
                <label htmlFor="primaryBank" className="text-xs text-slate-300">
                  Mark as Primary Operational Account
                </label>
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowBankModal(false)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingBank}
                  className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm font-medium hover:bg-emerald-500 transition"
                >
                  {submittingBank ? "Registering..." : "Register Account"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Start Reconciliation */}
      {showReconcileModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-md w-full p-6 space-y-4">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Start Reconciliation Period</h3>
              <button onClick={() => setShowReconcileModal(false)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleCreateReconciliation} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Bank Account</label>
                <select
                  value={reconBankId}
                  onChange={(e) => setReconBankId(e.target.value)}
                  required
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                >
                  <option value="">Select Account</option>
                  {bankAccounts.map((b) => (
                    <option key={b.id} value={b.id}>
                      {b.bank_name} - {b.account_name} ({b.account_number})
                    </option>
                  ))}
                </select>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Start Date</label>
                  <input
                    type="date"
                    required
                    value={reconStart}
                    onChange={(e) => setReconStart(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">End Date</label>
                  <input
                    type="date"
                    required
                    value={reconEnd}
                    onChange={(e) => setReconEnd(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Opening Balance (NGN)</label>
                  <input
                    type="number"
                    step="0.01"
                    required
                    placeholder="500000"
                    value={reconOpeningNaira}
                    onChange={(e) => setReconOpeningNaira(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Closing Balance (NGN)</label>
                  <input
                    type="number"
                    step="0.01"
                    required
                    placeholder="650000"
                    value={reconClosingNaira}
                    onChange={(e) => setReconClosingNaira(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white font-mono"
                  />
                </div>
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowReconcileModal(false)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingRecon}
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-500 transition"
                >
                  {submittingRecon ? "Opening..." : "Open Period"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Import CSV Statement */}
      {showCSVModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-lg w-full p-6 space-y-4">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Import Bank Statement CSV</h3>
              <button onClick={() => setShowCSVModal(null)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="p-3 bg-blue-500/10 border border-blue-500/20 rounded-lg text-xs text-blue-300 space-y-1">
              <div><strong>Format:</strong> date, reference, amount, type, description</div>
              <div>Duplicate line items will be automatically detected and protected.</div>
            </div>
            <form onSubmit={handleImportCSV} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">CSV Content</label>
                <textarea
                  rows={6}
                  required
                  placeholder={`date,reference,amount,type,description\n2026-10-05,TXN-99881,150000,credit,Donation from public\n2026-10-12,TXN-99882,50000,debit,Refuse bags`}
                  value={csvContent}
                  onChange={(e) => setCsvContent(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-xs font-mono text-white"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowCSVModal(null)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingCSV}
                  className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm font-medium hover:bg-emerald-500 transition"
                >
                  {submittingCSV ? "Importing..." : "Import Statement"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Print Preview */}
      {showPrintModal && (
        <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex items-center justify-center p-4">
          <div className="bg-white text-slate-900 rounded-xl max-w-3xl w-full p-6 space-y-4 max-h-[90vh] overflow-y-auto shadow-2xl">
            <div className="flex justify-between items-center border-b pb-3 no-print">
              <div className="font-bold text-lg text-slate-900 flex items-center gap-2">
                <Printer className="w-5 h-5 text-emerald-600" /> Print Preview: {showPrintModal.title}
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => window.print()}
                  className="px-3.5 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-bold flex items-center gap-1.5 transition"
                >
                  <Printer className="w-3.5 h-3.5" /> Print / Save PDF
                </button>
                <button onClick={() => setShowPrintModal(null)} className="text-slate-400 hover:text-slate-700 p-1">
                  <X className="w-5 h-5" />
                </button>
              </div>
            </div>

            {/* Rendered HTML Document */}
            <div
              className="printable-document-content"
              dangerouslySetInnerHTML={{ __html: showPrintModal.html_printable_body }}
            />
          </div>
        </div>
      )}

      {/* Modal: Authorize Staff Access */}
      {showAuthModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-md w-full p-6 space-y-4">
            <div className="flex justify-between items-center border-b border-slate-800 pb-3">
              <h3 className="font-bold text-white text-lg">Authorize Financial Clearance</h3>
              <button onClick={() => setShowAuthModal(false)} className="text-slate-400 hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form
              onSubmit={async (e) => {
                e.preventDefault();
                setSubmittingAuth(true);
                try {
                  await apiRequest("/finance/authorizations", {
                    method: "POST",
                    body: JSON.stringify({
                      user_id: authUserId,
                      access_level: authLevel,
                      notes: authNotes,
                    }),
                  });
                  setFeedback({ type: "success", message: "Financial clearance granted." });
                  setShowAuthModal(false);
                  await loadAllFinanceData();
                } catch (err: any) {
                  setFeedback({ type: "error", message: err.message || "Failed to authorize clearance" });
                } finally {
                  setSubmittingAuth(false);
                }
              }}
              className="space-y-4"
            >
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Select Member / Staff</label>
                <select
                  value={authUserId}
                  onChange={(e) => setAuthUserId(e.target.value)}
                  required
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                >
                  <option value="">Select Member</option>
                  {members.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.display_name} ({m.primary_role})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Clearance Level</label>
                <select
                  value={authLevel}
                  onChange={(e) => setAuthLevel(e.target.value as any)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                >
                  <option value="finance_officer">Finance Officer (Review, Preparation & Reconciliation)</option>
                  <option value="submitter">Submitter (Can propose entries & expenses)</option>
                  <option value="viewer">Viewer (Read-only financial audit)</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Authorization Justification</label>
                <textarea
                  rows={2}
                  required
                  placeholder="e.g. Authorized for national Q4 reconciliation duty"
                  value={authNotes}
                  onChange={(e) => setAuthNotes(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-sm text-white"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowAuthModal(false)}
                  className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg text-sm font-medium hover:bg-slate-700 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingAuth}
                  className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm font-medium hover:bg-emerald-500 transition"
                >
                  {submittingAuth ? "Granting..." : "Grant Clearance"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
