"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { apiRequest } from "@/lib/api";
import { verifiedPaymentMessage } from "@/lib/store-subscriptions";
export default function SubscriptionConfirmation() {
 const [status,setStatus]=useState("pending");const [error,setError]=useState("");const [busy,setBusy]=useState(true);
 async function verify(){setBusy(true);setError("");const q=new URLSearchParams(window.location.search);const ref=q.get("reference")||q.get("trxref");if(!ref){setError("No payment reference was returned. Checkout may have been cancelled. Check payment history before trying again.");setBusy(false);return;}try{const p=await apiRequest<any>(`/marketplace/subscriptions/payments/${encodeURIComponent(ref)}/verify`,{method:"POST"});setStatus(p.status);}catch(e:any){setError(e.message);}finally{setBusy(false);}}
 useEffect(()=>{void verify();},[]);
 return <main className="max-w-xl mx-auto p-8 space-y-5"><h1 className="text-3xl font-bold">Store payment confirmation</h1><p role="status">{busy?"Verifying payment with Paystack…":error?"Payment has not been confirmed.":verifiedPaymentMessage(status)}</p>{error&&<p role="alert" className="text-red-700">{error}</p>}<p>Store approval remains separate from payment. A renewal or plan change may start after your current term.</p><button disabled={busy} onClick={verify} className="bg-emerald-800 text-white p-3 rounded-lg">Check again</button><Link className="block underline" href="/marketplace/seller/subscription">View subscription and payment history</Link></main>;
}
