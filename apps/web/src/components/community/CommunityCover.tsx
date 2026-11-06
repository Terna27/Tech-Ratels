"use client";
import {useEffect,useState} from 'react';
import {SocialMember,relationshipPath} from '@/lib/community';

export default function CommunityCover({member}:{member:SocialMember}){
 const [failed,setFailed]=useState(false);
 useEffect(()=>setFailed(false),[member.reference,member.has_cover]);
 return <div className="aspect-[3/1] overflow-hidden bg-gradient-to-r from-slate-200 to-slate-100">
 {member.has_cover&&!failed&&<img src={'/api/v1'+relationshipPath(member.reference)+'/cover'} alt={member.name+' cover photograph'}
 className="w-full h-full object-cover" onError={()=>setFailed(true)}
 style={{objectPosition:`${member.cover_x}% ${member.cover_y}%`,transform:`scale(${member.cover_zoom/100})`,transformOrigin:`${member.cover_x}% ${member.cover_y}%`}}/>}
 </div>;
}
