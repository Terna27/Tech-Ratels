import test from 'node:test';
import assert from 'node:assert/strict';
import {cleanupIsDone} from '../src/lib/cleanup.ts';
test('cleanup deadline closes exactly at the scheduled end, including Lagos offsets',()=>{
 const event={status:'published',end_time:'2026-11-28T11:00:00+01:00'};
 const end=Date.parse('2026-11-28T10:00:00Z');
 assert.equal(cleanupIsDone(event,end-1),false);
 assert.equal(cleanupIsDone(event,end),true);
 assert.equal(cleanupIsDone(event,end+1),true);
 for(const status of ['draft','submitted','cancelled'])assert.equal(cleanupIsDone({...event,status},end+1),false);
 assert.equal(cleanupIsDone({...event,status:'completed'},end-1),true);
});
