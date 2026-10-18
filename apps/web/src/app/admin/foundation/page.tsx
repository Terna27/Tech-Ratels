"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Building,
  Landmark,
  ShieldCheck,
  Lock,
  PlusCircle,
  ArrowLeft,
  FileCheck2,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { PublicFoundation } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

interface BankAccountVersion {
  id: string;
  bank_name: string;
  account_name: string;
  account_number: string;
  currency: string;
  status: string;
  verified_by_officer_1?: string;
  verified_by_officer_2?: string;
  verification_notes?: string;
}

export default function AdminFoundationPage() {
  const router = useRouter();
  const { user, isAdmin, isLoading } = useAuth();

  const [foundation, setFoundation] = useState<PublicFoundation | null>(null);
  const [bankAccounts, setBankAccounts] = useState<BankAccountVersion[]>([]);
  const [loading, setLoading] = useState(true);

  // Draft Account Form
  const [bankName, setBankName] = useState("");
  const [accountName, setAccountName] = useState("");
  const [accountNumber, setAccountNumber] = useState("");
  const [currency, setCurrency] = useState("NGN");
  const [notes, setNotes] = useState("");
  const [submittingAccount, setSubmittingAccount] = useState(false);

  // Edit Profile Form
  const [provisionalName, setProvisionalName] = useState("");
  const [missionStatement, setMissionStatement] = useState("");
  const [disclaimer, setDisclaimer] = useState("");
  const [savingProfile, setSavingProfile] = useState(false);

  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const loadData = async () => {
    try {
      const [fnd, accounts] = await Promise.all([
        apiRequest<PublicFoundation>("/foundation"),
        apiRequest<BankAccountVersion[]>("/foundation/bank-accounts"),
      ]);
      setFoundation(fnd);
      setBankAccounts(accounts || []);
      if (fnd) {
        setProvisionalName(fnd.name);
        setMissionStatement(fnd.mission_statement);
        setDisclaimer(fnd.disclaimer);
      }
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to load Foundation records" });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isLoading && (!user || !isAdmin)) {
      router.push("/login?redirect=/admin/foundation");
      return;
    }
    if (user && isAdmin) {
      loadData();
    }
  }, [user, isAdmin, isLoading, router]);

  const handleCreateBankAccount = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmittingAccount(true);
    setFeedback(null);

    try {
      await apiRequest("/foundation/bank-accounts", {
        method: "POST",
        body: JSON.stringify({
          bank_name: bankName,
          account_name: accountName,
          account_number: accountNumber,
          currency,
          verification_notes: notes,
        }),
      });

      setFeedback({
        type: "success",
        message: "Bank account version drafted! Status is 'pending_verification' awaiting secondary trustee sign-off. It remains strictly hidden from the public.",
      });
      setBankName("");
      setAccountName("");
      setAccountNumber("");
      setNotes("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to draft bank account version" });
    } finally {
      setSubmittingAccount(false);
    }
  };

  const handleUpdateProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingProfile(true);
    setFeedback(null);

    try {
      await apiRequest("/foundation", {
        method: "PUT",
        body: JSON.stringify({
          provisional_name: provisionalName,
          mission_statement: missionStatement,
          disclaimer: disclaimer,
        }),
      });

      setFeedback({ type: "success", message: "Foundation entity settings updated." });
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Profile update failed" });
    } finally {
      setSavingProfile(false);
    }
  };

  if (isLoading || loading) return <Spinner text="Loading Foundation administration..." />;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      <Link
        href="/admin"
        className="inline-flex items-center text-xs font-semibold text-gray-500 hover:text-gray-900"
      >
        <ArrowLeft className="w-4 h-4 mr-1" />
        Back to Admin Workspace
      </Link>

      <div className="space-y-2">
        <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
          Foundation Administration & Bank Verification
        </h1>
        <p className="text-xs text-gray-600">
          Manage entity settings for the Martins Vincent Otse Foundation and prepare donation accounts under dual verification controls.
        </p>
      </div>

      <Alert type="warning" title="Dual-Officer Protection Policy">
        No bank accounts can be published without secondary trustee verification. In Phase 1, all bank details remain unverified and private, and live online payment processing is disabled.
      </Alert>

      {feedback && <Alert type={feedback.type}>{feedback.message}</Alert>}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Draft Bank Account Version */}
        <div className="bg-white p-6 sm:p-8 rounded-2xl border border-gray-200 shadow-sm space-y-5">
          <div className="flex items-center gap-2">
            <Landmark className="w-5 h-5 text-emerald-700" />
            <h2 className="text-base font-bold text-gray-900">
              Submit Bank Account Version for Review
            </h2>
          </div>

          <form onSubmit={handleCreateBankAccount} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Bank Name *</label>
              <input
                type="text"
                required
                value={bankName}
                onChange={(e) => setBankName(e.target.value)}
                placeholder="e.g. First Bank of Nigeria"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Account Name *</label>
              <input
                type="text"
                required
                value={accountName}
                onChange={(e) => setAccountName(e.target.value)}
                placeholder="Must match official registered legal name exactly"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div className="sm:col-span-2">
                <label className="block text-xs font-semibold text-gray-700 mb-1">Account Number *</label>
                <input
                  type="text"
                  required
                  value={accountNumber}
                  onChange={(e) => setAccountNumber(e.target.value)}
                  placeholder="10-digit NUBAN"
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700 font-mono"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Currency *</label>
                <input
                  type="text"
                  disabled
                  value={currency}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm bg-gray-100"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Verification Document Notes</label>
              <textarea
                rows={2}
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                placeholder="Reference supporting mandate documents, CAC registration number, or trustee resolution..."
                className="w-full border border-gray-300 rounded-lg p-2.5 text-xs focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <button
              type="submit"
              disabled={submittingAccount}
              className="w-full py-2.5 px-4 rounded-xl text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 shadow-sm transition disabled:opacity-50"
            >
              {submittingAccount ? "Submitting..." : "Submit for Trustee Verification"}
            </button>
          </form>
        </div>

        {/* Entity Configuration Form */}
        <div className="bg-white p-6 sm:p-8 rounded-2xl border border-gray-200 shadow-sm space-y-5">
          <div className="flex items-center gap-2">
            <Building className="w-5 h-5 text-emerald-700" />
            <h2 className="text-base font-bold text-gray-900">Configure Foundation Identity</h2>
          </div>

          <form onSubmit={handleUpdateProfile} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Provisional Foundation Display Name *
              </label>
              <input
                type="text"
                required
                value={provisionalName}
                onChange={(e) => setProvisionalName(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Mission Statement *</label>
              <textarea
                rows={3}
                required
                value={missionStatement}
                onChange={(e) => setMissionStatement(e.target.value)}
                className="w-full border border-gray-300 rounded-lg p-2.5 text-xs focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Transparency Disclaimer *</label>
              <textarea
                rows={3}
                required
                value={disclaimer}
                onChange={(e) => setDisclaimer(e.target.value)}
                className="w-full border border-gray-300 rounded-lg p-2.5 text-xs focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <button
              type="submit"
              disabled={savingProfile}
              className="w-full py-2.5 px-4 rounded-xl text-sm font-semibold text-gray-800 bg-gray-100 hover:bg-gray-200 border border-gray-300 shadow-sm transition disabled:opacity-50"
            >
              {savingProfile ? "Saving..." : "Update Entity Settings"}
            </button>
          </form>
        </div>
      </div>

      {/* Internal Bank Accounts Table */}
      <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
        <div className="p-4 border-b border-gray-200 bg-gray-50/50 flex justify-between items-center text-xs">
          <span className="font-semibold text-gray-700">Internal Bank Account Versions (Protected)</span>
          <span className="text-gray-500">{bankAccounts.length} versions</span>
        </div>

        {bankAccounts.length === 0 ? (
          <div className="p-8 text-center text-xs text-gray-500">
            No bank accounts drafted yet. All accounts must undergo dual-officer sign-off.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 text-xs">
              <thead className="bg-gray-50 text-gray-600 font-semibold uppercase text-[11px]">
                <tr>
                  <th className="px-4 py-3 text-left">Bank</th>
                  <th className="px-4 py-3 text-left">Account Name</th>
                  <th className="px-4 py-3 text-left">Account Number</th>
                  <th className="px-4 py-3 text-left">Officer 1 (Draft)</th>
                  <th className="px-4 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {bankAccounts.map((acc) => (
                  <tr key={acc.id} className="hover:bg-gray-50/60 transition">
                    <td className="px-4 py-3.5 font-bold text-gray-900">{acc.bank_name}</td>
                    <td className="px-4 py-3.5 text-gray-700">{acc.account_name}</td>
                    <td className="px-4 py-3.5 font-mono text-gray-900">{acc.account_number}</td>
                    <td className="px-4 py-3.5 text-gray-600">{acc.verified_by_officer_1 || "-"}</td>
                    <td className="px-4 py-3.5">
                      <Badge variant="warning" size="sm">
                        {acc.status.replace("_", " ").toUpperCase()}
                      </Badge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
