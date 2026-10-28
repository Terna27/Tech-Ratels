"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { apiRequest } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { BillingInterval, formatKobo, StorePlan, storePrice } from "@/lib/store-subscriptions";

export default function StorePricing() {
 const { user } = useAuth();
 const [interval, setInterval] = useState<BillingInterval>("monthly");
 const [store, setStore] = useState<any>(null);
 const [enabled, setEnabled] = useState(false);
 const [busy, setBusy] = useState(false);
 const [error, setError] = useState("");
 const [keys] = useState<Record<string,string>>({});
 useEffect(() => { apiRequest<any>("/marketplace/subscriptions/plans").then(d => setEnabled(d.checkout_enabled)).catch(e => setError(e.message)); }, []);
 useEffect(() => { if (user) apiRequest<any>("/marketplace/seller/business").then(setStore).catch(() => setStore(null)); }, [user]);
 async function subscribe(plan: StorePlan) {
  if (!store) return;
  setBusy(true); setError("");
  const choice = plan + interval;
  keys[choice] ||= crypto.randomUUID();
  try { const d = await apiRequest<any>(`/marketplace/seller/businesses/${store.id}/subscription/checkout`, { method: "POST", body: JSON.stringify({ plan, interval, idempotency_key: keys[choice] }) }); if (d.checkout_url) window.location.assign(d.checkout_url); else setError("Checkout is being reconciled. Check your subscription dashboard."); }
  catch (e: any) { setError(e.message); } finally { setBusy(false); }
 }
 return <main className="max-w-5xl mx-auto p-6 space-y-6">
  <Link href="/marketplace" className="text-emerald-700">← Marketplace</Link>
  <header className="text-center space-y-3"><p className="text-sm font-semibold text-emerald-700">RATELS MARKETPLACE</p><h1 className="text-3xl font-bold">A home for your business</h1><p>Choose your store plan. Pay securely with Paystack. Renew manually, with no automatic charges.</p></header>
  <div className="flex justify-center gap-2">{(["monthly","yearly"] as const).map(v => <button key={v} onClick={() => setInterval(v)} aria-pressed={interval===v} className={`px-5 py-3 rounded-lg border ${interval===v ? "bg-emerald-800 text-white" : "bg-white"}`}>{v === "monthly" ? "Monthly" : "Yearly · save 2 months"}</button>)}</div>
  {!enabled && <p role="status" className="bg-amber-50 rounded-lg p-4">Subscription checkout is currently unavailable. Existing marketplace access follows the rollout policy.</p>}
  {error && <p role="alert" className="text-red-700 bg-red-50 p-4 rounded-lg">{error}</p>}
  {store && <p className="text-center">Store: <strong>{store.name}</strong> · <Link className="underline" href="/marketplace/seller/subscription">Manage subscription</Link></p>}
  <div className="grid md:grid-cols-2 gap-6">{(["basic","business"] as const).map(plan => <article key={plan} className="rounded-2xl border bg-white shadow-sm p-7 space-y-5">
   <h2 className="text-2xl font-bold">{plan === "basic" ? "Basic Store" : "Business Store"}</h2>
   <p><strong className="text-4xl">{formatKobo(storePrice(plan,interval))}</strong> / {interval === "monthly" ? "month" : "year"}</p>
   <ul className="space-y-3">{(plan === "basic" ? ["30 draft or published products", "Storefront access", "Product and order management", "Standard support"] : ["Unlimited products", "Storefront, product and order management", "Sales analytics", "Featured-store eligibility", "Priority support"]).map(f => <li key={f}>✓ {f}</li>)}</ul>
   {user ? store ? <button disabled={!enabled||busy} onClick={() => subscribe(plan)} className="w-full bg-emerald-800 text-white rounded-lg p-3 disabled:opacity-50">{busy ? "Preparing checkout…" : "Subscribe"}</button> : <Link href="/marketplace/seller" className="block text-center bg-emerald-800 text-white rounded-lg p-3">Create your store</Link> : <Link href="/login" className="block text-center bg-emerald-800 text-white rounded-lg p-3">Sign in to subscribe</Link>}
  </article>)}</div>
  <p className="text-sm text-gray-600">Store approval remains required. Draft and published products count toward the limit; archived products do not. Changes and renewals start after your current paid term, at full price with no proration. A queued Basic term reserves the 30-product limit. Existing stores receive 30 days of grace from first enforcement. Paid orders can still be fulfilled after expiry.</p>
 </main>;
}
