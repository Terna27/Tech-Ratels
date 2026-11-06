import test from 'node:test';
import assert from 'node:assert/strict';
import {apiRequest, setCsrfToken} from '../src/lib/api.ts';

test('photo uploads report progress and retain CSRF and cookie authentication',async t=>{
 const original=globalThis.XMLHttpRequest;
 t.after(()=>{globalThis.XMLHttpRequest=original;setCsrfToken(null);});
 let request;
 globalThis.XMLHttpRequest=class {
  constructor(){request=this;this.upload={};this.headers={};}
  open(method,url){this.method=method;this.url=url;}
  setRequestHeader(name,value){this.headers[name]=value;}
  getResponseHeader(){return 'application/json';}
  send(body){this.body=body;this.upload.onprogress({lengthComputable:true,loaded:5,total:10});this.status=200;this.responseText='{"has_cover":true}';this.onload();}
 };
 setCsrfToken('test-csrf');const progress=[];const body=new FormData();body.append('file',new Blob(['photo']),'photo.jpg');
 const result=await apiRequest('/members/me/media/cover',{method:'POST',body,onUploadProgress:p=>progress.push(p)});
 assert.equal(result.has_cover,true);assert.deepEqual(progress,[50]);
 assert.equal(request.withCredentials,true);assert.equal(request.headers['X-CSRF-Token'],'test-csrf');
 assert.equal(request.headers['Content-Type'],undefined);assert.equal(request.body,body);
});

test('upload validation failures keep the server message',async t=>{
 const original=globalThis.XMLHttpRequest;t.after(()=>globalThis.XMLHttpRequest=original);
 globalThis.XMLHttpRequest=class {
  constructor(){this.upload={};}open(){}setRequestHeader(){}getResponseHeader(){return 'application/json';}
  send(){this.status=400;this.responseText=JSON.stringify({error:{code:'INVALID_IMAGE',message:'Image is too small'}});this.onload();}
 };
 await assert.rejects(apiRequest('/members/me/media/avatar',{method:'POST',body:new FormData(),onUploadProgress:()=>{}}),e=>e.status===400&&e.code==='INVALID_IMAGE'&&e.message==='Image is too small');
});
