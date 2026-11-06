"use client";
import {useCallback,useEffect,useState} from 'react';
import {useParams} from 'next/navigation';
import Link from 'next/link';
import {useAuth} from '@/lib/auth-context';
import {apiRequest} from '@/lib/api';
import {SocialMember,relationshipPath} from '@/lib/community';
import MemberConnection from '@/components/community/MemberConnection';
import CommunityFeed from '@/components/community/CommunityFeed';
import CommunityCover from '@/components/community/CommunityCover';
export default function MemberCommunityProfile(){
 const params=useParams<{reference:string}>(),reference=params.reference;
 const {user,isLoading}=useAuth();const [member,setMember]=useState<SocialMember|null>(null),[error,setError]=useState('');
 const load=useCallback(async()=>{setMember(await apiRequest<SocialMember>(relationshipPath(reference)));},[reference]);
 useEffect(()=>{if(!user)return;let active=true;setMember(null);setError('');apiRequest<SocialMember>(relationshipPath(reference)).then(m=>{if(active)setMember(m);}).catch(e=>{if(active)setError(e.message);});return()=>{active=false;};},[reference,user]);
 if(isLoading)return <p className="p-8">Loading profile…</p>;
 if(!user)return <main className="p-8"><Link href={'/login?redirect='+encodeURIComponent('/member/people/'+encodeURIComponent(reference))}>Sign in to view member profiles</Link></main>;
 return <main className="max-w-4xl mx-auto px-4 py-8 space-y-6"><Link href="/member/community" className="text-emerald-800 underline">Back to community</Link>{error&&<p role="alert" className="border rounded-xl p-6 text-red-700">{error}</p>}{!error&&!member&&<p role="status">Loading member…</p>}{member&&<><article className="rounded-2xl border bg-white overflow-hidden"><CommunityCover member={member}/><div className="px-5 pb-6 space-y-4"><div className="relative -mt-12 w-24 h-24 sm:w-32 sm:h-32 sm:-mt-16 rounded-full border-4 border-white bg-emerald-100 overflow-hidden grid place-items-center text-3xl font-bold text-emerald-900">{member.avatar_url?<img src={member.avatar_url} alt={member.name+' profile photograph'} referrerPolicy="no-referrer" className="w-full h-full object-cover" style={{objectPosition:`${member.avatar_x}% ${member.avatar_y}%`,transform:`scale(${member.avatar_zoom/100})`,transformOrigin:`${member.avatar_x}% ${member.avatar_y}%`}}/>:member.name.split(/\s+/).slice(0,2).map(s=>s[0]).join('')}</div><div><h1 className="text-3xl font-bold break-words">{member.name}</h1><p className="text-sm font-mono text-gray-500 break-all">{member.reference}</p><p className="text-sm text-gray-600 mt-1">{member.location}</p><p className="text-sm text-emerald-800 mt-2">{member.followers} followers</p></div><p className="whitespace-pre-wrap break-words text-gray-700">{member.bio}</p>{member.reference!==user.membership_reference?<MemberConnection member={member} onChange={load}/>:<Link href="/member/profile" className="inline-block rounded-full border px-4 py-2">Edit my profile</Link>}{member.badges.length>0&&<section aria-label="Public badges"><h2 className="font-semibold mb-2">Public badges</h2><ul className="flex flex-wrap gap-2">{member.badges.map((b,i)=><li key={i} className="bg-emerald-50 text-emerald-900 border rounded-full px-3 py-1 text-sm">{b}</li>)}</ul></section>}</div></article><CommunityFeed key={reference+member.friendship} reference={reference} canPost={member.reference===user.membership_reference}/></>}</main>;
}
