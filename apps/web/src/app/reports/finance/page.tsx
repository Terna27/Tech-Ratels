"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  DollarSign,
  TrendingUp,
  PieChart,
  ShieldCheck,
  ArrowLeft,
  Lock,
  Calendar,
  AlertCircle,
  FileCheck2,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { PublicFinanceData, State } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

export default function FinanceReportPage() {
  const [data, setData] = useState<PublicFinanceData | null>(null);
  const [states, setStates] = useState<State[]>([]);
  const [selectedState, setSelectedState] = useState<string>("");
  const [selectedPeriod, setSelectedPeriod] = useState<string>("All Time");
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    async function loadStates() {
      try {
        const res = await apiRequest<State[]>("/geography/states");
        if (res) setStates(res);
      } catch (e) {
        console.error(e);
      }
    }
    loadStates();
  }, []);

  useEffect(() => {
    async function loadData() {
      setLoading(true);
      try {
        const qp = new URLSearchParams();
        if (selectedState) qp.set("state_id", selectedState);
        if (selectedPeriod && selectedPeriod !== "All Time") qp.set("period", selectedPeriod);
        const res = await apiRequest<PublicFinanceData>(`/public/reports/finance?${qp.toString()}`);
        if (res) setData(res);
      } catch (err) {
        console.error("Failed loading finance report", err);
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, [selectedState, selectedPeriod]);

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Back button and breadcrumb */}
      <div className="flex items-center gap-2 text-xs text-gray-500 font-medium">
        <Link href="/reports" className="flex items-center gap-1 hover:text-emerald-800 transition">
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>All Public Reports</span>
        </Link>
        <span>/</span>
        <span className="text-gray-900 font-bold">Financial Transparency</span>
      </div>

      {/* Header */}
      <div className="bg-emerald-950 text-white rounded-3xl p-6 sm:p-8 shadow-sm border border-emerald-800 flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div className="space-y-1 max-w-xl">
          <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-emerald-800 text-emerald-300 text-xs font-semibold">
            <DollarSign className="w-3.5 h-3.5" />
            <span>Authoritative Double-Entry General Ledger</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold">Public Financial Accountability Reports</h1>
          <p className="text-xs text-emerald-200 leading-relaxed">
            All published figures are strictly derived from verified transactions, bank statement reconciliations,
            and Super Admin authorized payment vouchers. Calculated in integer kobo to eliminate rounding anomalies.
          </p>
        </div>

        {/* Filter Controls */}
        <div className="flex flex-wrap items-center gap-2 bg-emerald-900/80 p-2 rounded-xl border border-emerald-700/60 text-xs">
          <select
            value={selectedState}
            onChange={(e) => setSelectedState(e.target.value)}
            className="rounded-lg bg-emerald-950 border-emerald-700 text-white text-xs py-1.5 px-2.5 focus:ring-emerald-500"
          >
            <option value="">All 36 States + FCT</option>
            {states.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>

          <select
            value={selectedPeriod}
            onChange={(e) => setSelectedPeriod(e.target.value)}
            className="rounded-lg bg-emerald-950 border-emerald-700 text-white text-xs py-1.5 px-2.5 focus:ring-emerald-500"
          >
            <option value="All Time">All Time</option>
            <option value="2026">2026 Annual</option>
            <option value="2026-10">October 2026</option>
            <option value="2026-09">September 2026</option>
            <option value="2025">2025 Annual</option>
          </select>
        </div>
      </div>

      {loading ? (
        <Spinner text="Loading financial transparency metrics..." />
      ) : (
        <>
          {/* Key Financial Cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Approved Income</span>
              <div className="text-2xl font-extrabold text-emerald-900">
                {data?.approved_income_formatted || "₦0.00"}
              </div>
              <span className="text-[10px] text-gray-400">Reconciled external inflows</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Approved Expenditure</span>
              <div className="text-2xl font-extrabold text-gray-900">
                {data?.approved_expenditure_formatted || "₦0.00"}
              </div>
              <span className="text-[10px] text-gray-400">Authorized payment vouchers</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Net Surplus / (Deficit)</span>
              <div className="text-2xl font-extrabold text-emerald-800">
                {data?.net_surplus_deficit_formatted || "₦0.00"}
              </div>
              <span className="text-[10px] text-gray-400">Operating fund balance</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Budget vs Actual Variance</span>
              <div className="text-2xl font-extrabold text-gray-900">
                {data?.budget_variance_formatted || "₦0.00"}
              </div>
              <span className="text-[10px] text-emerald-700 font-semibold">
                Budget: {data?.total_approved_budget_formatted || "₦0.00"}
              </span>
            </div>
          </div>

          {/* Spending by Governance Category */}
          <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm p-6 space-y-4">
            <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
              <PieChart className="w-4 h-4 text-emerald-800" />
              <span>Expenditure Breakdown by Governance Category</span>
            </h2>

            {!data?.spending_categories || data.spending_categories.length === 0 ? (
              <div className="p-8 text-center text-xs text-gray-500">
                No categorized expenditures recorded for this period/scope.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-gray-50 text-gray-600 font-semibold uppercase tracking-wider border-b border-gray-200">
                    <tr>
                      <th className="py-3 px-4">Expense Category</th>
                      <th className="py-3 px-4">Voucher Count</th>
                      <th className="py-3 px-4">Amount (NGN)</th>
                      <th className="py-3 px-4">Share of Total</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {data.spending_categories.map((cat) => {
                      const sharePct =
                        data.approved_expenditure_kobo > 0
                          ? ((cat.total_kobo / data.approved_expenditure_kobo) * 100).toFixed(1)
                          : "0.0";
                      return (
                        <tr key={cat.category} className="hover:bg-gray-50/50 transition">
                          <td className="py-3.5 px-4 font-bold text-gray-900">{cat.category_label}</td>
                          <td className="py-3.5 px-4 font-mono text-gray-600">{cat.expense_count}</td>
                          <td className="py-3.5 px-4 font-mono font-bold text-emerald-950">
                            {cat.amount_formatted}
                          </td>
                          <td className="py-3.5 px-4">
                            <div className="flex items-center gap-2">
                              <div className="w-20 bg-gray-200 rounded-full h-1.5 overflow-hidden">
                                <div
                                  className="bg-emerald-700 h-1.5 rounded-full"
                                  style={{ width: `${Math.min(parseFloat(sharePct), 100)}%` }}
                                />
                              </div>
                              <span className="font-mono text-[11px] text-gray-600">{sharePct}%</span>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* Governance & Anti-Corruption Shield Banner */}
          <div className="bg-gray-50 rounded-2xl border border-gray-200 p-6 space-y-3">
            <div className="flex items-center gap-2 text-emerald-900">
              <Lock className="w-5 h-5 text-emerald-800" />
              <h3 className="font-bold text-sm">Financial Safeguards & Information Protection</h3>
            </div>
            <p className="text-xs text-gray-600 leading-relaxed">
              In accordance with banking confidentiality and governance policies, raw account numbers,
              private vendor details, individual donor records, and unverified invoices are strictly protected from
              public disclosure. Only reconciled totals authorized by Super Admin clearance are eligible for publication.
            </p>
          </div>
        </>
      )}
    </div>
  );
}
