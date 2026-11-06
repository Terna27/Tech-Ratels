import test from 'node:test';
import assert from 'node:assert/strict';
import puppeteer from 'puppeteer-core';

test('member community browser acceptance',async t=>{
 const base=process.env.TEST_WEB_URL;if(!base){t.skip('Set TEST_WEB_URL and CHROME_BIN for community browser acceptance');return;}
 const browser=await puppeteer.launch({executablePath:process.env.CHROME_BIN||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox','--disable-dev-shm-usage']});t.after(()=>browser.close());
 const page=await browser.newPage();await page.setViewport({width:390,height:844});
 const user={id:'private-jane',display_name:'Jane Member',membership_reference:'RATEL-JANE',primary_role:'member',status:'active',is_verified:true,appointments:[],profile:{bio:'Volunteer',is_public_badges:true}};
 const dan={reference:'RATEL-DAN',name:'Dan Member',bio:'A community volunteer',location:'Ikeja, Lagos',has_cover:false,avatar_x:50,avatar_y:50,avatar_zoom:100,cover_x:50,cover_y:50,cover_zoom:100,following:false,friendship:'none',followers:0,badges:['Community helper']};
 const posts=[{id:'post-dan',author_reference:dan.reference,author_name:dan.name,body:'Our cleanup was a success. <script>window.communityXSS=true</script>',visibility:'members',created_at:'2026-11-06T12:00:00Z',likes:2,liked:false,own:false}];
 let prefs={discoverable:true,share_cover:false},anonymous=false;const requests=[];
 await page.setRequestInterception(true);page.on('request',async r=>{
 const u=new URL(r.url());if(!u.pathname.startsWith('/api/v1/')){await r.continue();return;}
 requests.push({path:u.pathname,method:r.method(),body:r.postData()});let out={},status=200;
 if(u.pathname.endsWith('/members/me')){out=user;if(anonymous)status=401;}
 else if(u.pathname.endsWith('/auth/csrf'))out={csrf_token:'fixture'};
 else if(u.pathname.endsWith('/network/preferences')){if(r.method()==='PUT')prefs=JSON.parse(r.postData());out=prefs;}
 else if(u.pathname.endsWith('/network/members'))out=u.searchParams.get('q')?.includes('missing')?[]:[dan];
 else if(u.pathname.endsWith('/network/requests'))out=[{...dan,friendship:'incoming'}];
 else if(u.pathname.endsWith('/network/members/RATEL-DAN'))out=dan;
 else if(u.pathname.endsWith('/RATEL-DAN/follow')){dan.following=r.method()==='POST';dan.followers=dan.following?1:0;out={ok:true};}
 else if(u.pathname.endsWith('/RATEL-DAN/friend/accept')){dan.friendship='friends';out={ok:true};}
 else if(u.pathname.endsWith('/RATEL-DAN/friend/decline')){dan.friendship='none';out={ok:true};}
 else if(u.pathname.endsWith('/RATEL-DAN/friend')){dan.friendship=r.method()==='POST'?'outgoing':'none';out={ok:true};}
 else if(u.pathname.endsWith('/network/feed'))out=posts;
 else if(u.pathname.endsWith('/network/posts')&&r.method()==='POST'){const body=JSON.parse(r.postData());posts.unshift({id:'post-jane',author_reference:user.membership_reference,author_name:user.display_name,...body,created_at:'2026-11-06T13:00:00Z',likes:0,liked:false,own:true});status=201;out={id:'post-jane'};}
 else if(u.pathname.endsWith('/post-dan/like')){const p=posts.find(p=>p.id==='post-dan');p.liked=r.method()==='PUT';p.likes=p.liked?3:2;out={ok:true};}
 else out=[];
 await r.respond({status,contentType:'application/json',body:JSON.stringify(out)});
 });
 const clickText=async(text)=>page.evaluate(text=>{const b=[...document.querySelectorAll('button')].find(b=>b.textContent?.trim()===text);if(!b)throw new Error('Missing button '+text);b.click();},text);
 await t.test('members search and open a privacy-safe mobile profile',async()=>{
 await page.goto(base+'/member/community',{waitUntil:'networkidle0'});await clickText('Find members');await page.waitForSelector('[aria-label="Search members"]');await page.type('[aria-label="Search members"]','Dan');await clickText('Search');await page.waitForSelector('a[href="/member/people/RATEL-DAN"]');await page.click('a[href="/member/people/RATEL-DAN"]');await page.waitForFunction(()=>[...document.querySelectorAll('h1')].some(e=>e.textContent==='Dan Member'));
 const content=await page.$eval('main',e=>e.textContent);assert.ok(content.includes('Ikeja, Lagos'));assert.ok(content.includes('Community helper'));assert.ok(!content.includes('private-jane'));assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
 });
 await t.test('follow, request and cancel refresh relationship state',async()=>{
 await clickText('Follow');await page.waitForFunction(()=>[...document.querySelectorAll('button')].some(e=>e.textContent==='Following · Unfollow'));
 await clickText('Add friend');await page.waitForFunction(()=>[...document.querySelectorAll('button')].some(e=>e.textContent==='Cancel request'));
 await clickText('Cancel request');await page.waitForFunction(()=>[...document.querySelectorAll('button')].some(e=>e.textContent==='Add friend'));assert.equal(dan.friendship,'none');
 });
 await t.test('incoming requests can be accepted and posts liked or unliked',async()=>{
 dan.friendship='incoming';await page.reload({waitUntil:'networkidle0'});await clickText('Accept request');await page.waitForFunction(()=>document.body.textContent.includes('You are friends'));assert.equal(dan.friendship,'friends');
 await page.click('[aria-label="Like post by Dan Member"]');await page.waitForSelector('[aria-label="Unlike post by Dan Member"]');assert.equal(posts.find(p=>p.id==='post-dan').likes,3);
 await page.click('[aria-label="Unlike post by Dan Member"]');await page.waitForSelector('[aria-label="Like post by Dan Member"]');assert.equal(await page.evaluate(()=>window.communityXSS),undefined);
 });
 await t.test('members publish friends-only posts and control discoverability',async()=>{
 await page.goto(base+'/member/community',{waitUntil:'networkidle0'});await page.waitForSelector('#community-post');await page.type('#community-post','A friends update');await page.select('[aria-label="Post audience"]','friends');await clickText('Publish post');await page.waitForFunction(()=>document.body.textContent.includes('Post published.'));
 const req=requests.find(r=>r.path.endsWith('/network/posts')&&r.method==='POST');assert.deepEqual(JSON.parse(req.body),{body:'A friends update',visibility:'friends'});
 await clickText('Privacy');await page.waitForFunction(()=>!document.querySelector('input[type="checkbox"]')?.disabled);const checkboxes=await page.$$('input[type="checkbox"]');await checkboxes[0].click();await checkboxes[1].click();await clickText('Save community privacy');await page.waitForFunction(()=>document.body.textContent.includes('Community privacy saved.'));assert.deepEqual(prefs,{discoverable:false,share_cover:true});
 });
 await t.test('anonymous visitors must sign in',async()=>{anonymous=true;await page.goto(base+'/member/people/RATEL-DAN',{waitUntil:'networkidle0'});await page.waitForFunction(()=>document.body.textContent.includes('Sign in to view member profiles'));assert.equal(await page.$('button[aria-pressed]'),null);});
});
