import test from 'node:test';
import assert from 'node:assert/strict';
import { storePrice, verifiedPaymentMessage } from '../src/lib/store-subscriptions.ts';
test('four subscription prices use integer kobo and annual saves two months',()=>{
 for(const [plan,monthly,yearly] of [['basic',200000,2000000],['business',500000,5000000]]){
 assert.equal(storePrice(plan,'monthly'),monthly);assert.equal(storePrice(plan,'yearly'),yearly);assert.equal(yearly,monthly*10);
 }
});
test('only successful server verification displays confirmation',()=>{
 assert.match(verifiedPaymentMessage('successful'),/Payment verified/);
 for(const state of ['pending','cancelled','unknown'])assert.doesNotMatch(verifiedPaymentMessage(state),/Payment verified/);
 assert.match(verifiedPaymentMessage('failed'),/failed/);
});
