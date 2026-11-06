"use client";
import {useState} from 'react';
import {apiRequest} from '@/lib/api';
import {SocialMember,friendActions,relationshipPath} from '@/lib/community';
export default function MemberConnection({member,onChange}:{member:SocialMember;onChange:()=>Promise<void>}){
 const [busy,setBusy]=useState(false),[error,setError]=useState('');
 const act=async(path:string,method:string)=>{setBusy(true);setError('');try{await apiRequest(path,{method});await onChange();}catch(e:any){setError(e.message||'Could not update connection.');}finally{setBusy(false);}};
 const base=relationshipPath(member.reference);
 return <div className="space-y-2"><div className="flex flex-wrap gap-2">
 <button type="button" aria-pressed={member.following} disabled={busy} onClick={()=>void act(base+'/follow',member.following?'DELETE':'POST')} className="rounded-full bg-emerald-800 text-white px-4 py-2 text-sm font-semibold disabled:opacity-50">{member.following?'Following · Unfollow':'Follow'}</button>
 {friendActions(member.friendship).map(a=><button type="button" key={a.label} disabled={busy} onClick={()=>{if(a.label==='Remove friend'&&!window.confirm('Remove this friend?'))return;void act(base+'/friend'+(a.action?'/'+a.action:''),a.method);}} className="rounded-full border border-emerald-800 text-emerald-900 px-4 py-2 text-sm font-semibold disabled:opacity-50">{a.label}</button>)}
 </div>{member.friendship==='friends'&&<p className="text-sm text-emerald-800">You are friends</p>}{error&&<p role="alert" className="text-sm text-red-700">{error}</p>}</div>;
}
