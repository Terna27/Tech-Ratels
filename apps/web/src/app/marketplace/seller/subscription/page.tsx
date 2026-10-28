"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { formatKobo } from "@/lib/store-subscriptions";

export default function SellerSubscription() {
 const { user, isLoading } = useAuth();
 const [data, setData] = useState<any>(null);
 const [error, setError] = useState("");
 const [busy, setBusy] = useState(false);
 const [store, setStore] = useState<any>(null);
 async function load() { try { const b = await apiRequest<any>("/marketplace/seller/business"); setStore(b); setData(await apiRequest(`/marketplace/seller/businesses/${b.id}/subscription/`)); } catch (e: any) { setError(e.message); } }
 useEffect(() => { if(user) void load(); }, [user]);
 async function verify(reference: string) {setBusy(true);setError("");try {await apiRequest(`/marketplace/subscriptions/payments/${encodeURIComponent(reference)}/verify`,{method:"POST"});await load();} catch(e:any){setError(e.message);}finally{setBusy(false);}}
 async function retryCheckout(reference: string) {setBusy(true);setError("");try {const d=await apiRequest<any>(`/marketplace/subscriptions/payments/${encodeURIComponent(reference)}/retry-checkout`,{method:"POST"});if(d.checkout_url)window.location.assign(d.checkout_url);}catch(e:any){setError(e.message);}finally{setBusy(false);}}
 async function cancel() {setBusy(true);setError("");try{await apiRequest(`/marketplace/seller/businesses/${store.id}/subscription/cancel`,{method:"POST"});await load();}catch(e:any){setError(e.message);}finally{setBusy(false);}}
 if(isLoading) return <p className="p-6">Loading…</p>;
 if(!user) return <main className="p-6"><Link href="/login">Sign in to manage your store subscription</Link></main>;
 return <main className="max-w-5xl mx-auto p-6 space-y-6"><Link className="text-emerald-700" href="/marketplace/seller">← Seller dashboard</Link><h1 className="text-3xl font-bold">Store subscription</h1>
 {error && <p role="alert" className="p-4 bg-red-50 text-red-700 rounded-lg">{error}</p>}
 {!data ? <Link href="/marketplace/seller">Create or load your store</Link> : <>
 <section className="border rounded-2xl p-6 bg-white space-y-3"><h2 className="text-2xl font-semibold">{data.store_name}</h2><p>Status: <strong>{data.status}</strong>{!data.enabled && " · Subscription enforcement is disabled"}</p><p>Plan: {data.current?.plan || "No paid term"}</p><p>Start: {data.current ? new Date(data.current.starts_at).toLocaleString() : "—"}</p><p>Expiry: {data.current ? new Date(data.current.expires_at).toLocaleString() : "—"} · {data.remaining_days} days remaining</p><p>Products: {data.product_usage} / {data.product_limit ?? (data.current ? "Unlimited" : "No paid limit assigned")}</p>{data.grace_until && <p>Existing-store grace ends {new Date(data.grace_until).toLocaleString()}</p>}
 <div className="flex flex-wrap gap-3"><Link href="/marketplace/subscriptions" className="bg-emerald-800 text-white px-4 py-3 rounded-lg">Subscribe / renew / change plan</Link><button disabled={busy||data.status==="cancelled"||data.status==="suspended"} onClick={cancel} className="border px-4 py-3 rounded-lg disabled:opacity-50">Cancel subscription</button></div><p className="text-sm text-gray-600">Cancellation retains paid access through expiry. No automatic charges. Plan changes apply after the current term without proration. Fulfilment of existing paid orders remains available.</p></section>
 {data.scheduled.length>0 && <section className="border rounded-xl p-5"><h2 className="font-bold">Scheduled paid terms</h2>{data.scheduled.map((t:any)=><p key={t.reference}>{t.plan} · {new Date(t.starts_at).toLocaleDateString()} – {new Date(t.expires_at).toLocaleDateString()}</p>)}</section>}
 <section className="bg-white border rounded-xl overflow-x-auto"><h2 className="text-xl font-bold p-5">Payment history</h2><table className="w-full text-left text-sm"><thead><tr>{["Date","Plan","Amount","Status","Actions"].map(h=><th key={h} className="p-3">{h}</th>)}</tr></thead><tbody>{data.payments.map((p:any)=><tr key={p.reference} className="border-t"><td className="p-3">{new Date(p.created_at).toLocaleDateString()}</td><td>{p.plan} / {p.interval}</td><td>{formatKobo(p.amount_kobo)}</td><td>{p.status}</td><td className="p-3"><button className="underline mr-3" disabled={busy} onClick={()=>verify(p.reference)}>Verify / refresh</button>{p.status==="pending" && (p.checkout_url ? <a href={p.checkout_url} className="underline">Resume checkout</a> : <button className="underline" disabled={busy} onClick={()=>retryCheckout(p.reference)}>Retry checkout setup</button>)}</td></tr>)}</tbody></table>{!data.payments.length&&<p className="p-5">No subscription payments yet.</p>}</section>
 </>}
 </main>;
}
