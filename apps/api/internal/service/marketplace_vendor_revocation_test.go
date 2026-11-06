package service_test

import (
 "context"
 "fmt"
 "testing"
 "time"

 "ratels-api/internal/service"
)

func TestMarketplaceSuperAdminExclusiveVendorRevocation(t *testing.T) {
 env:=setupTestEnv(t)
 defer env.Pool.Close()
 ctx:=context.Background()
 nonce:=time.Now().UnixNano()
 admin:=createRegisteredUser(t,env,fmt.Sprintf("revoke_admin_%d",nonce),"super_admin")
 seller,business,product:=helperCreateSellerWithProduct(t,env,ctx,fmt.Sprintf("revoke_seller_%d",nonce),"Revocation Seller","Before Revocation",10,admin)
 regular:=createRegisteredUser(t,env,fmt.Sprintf("revoke_member_%d",nonce),"member")
 coordinator:=createRegisteredUser(t,env,fmt.Sprintf("revoke_coordinator_%d",nonce),"state_coordinator")
 if _,err:=env.MarketplaceService.RevokeBusiness(ctx,regular,business.ID,"Compliance violation verified");err!=service.ErrScopeUnauthorized {
  t.Fatalf("member unexpectedly revoked vendor: %v",err)
 }
 if _,err:=env.MarketplaceService.RevokeBusiness(ctx,coordinator,business.ID,"Compliance violation verified");err!=service.ErrScopeUnauthorized {
  t.Fatalf("state coordinator unexpectedly revoked vendor: %v",err)
 }
 if _,err:=env.MarketplaceService.RevokeBusiness(ctx,admin,business.ID,"short");err==nil {
  t.Fatal("missing adequate reason was accepted")
 }
 revoked,err:=env.MarketplaceService.RevokeBusiness(ctx,admin,business.ID,"Verified serious marketplace policy violation")
 if err!=nil||revoked.Status!="revoked"||revoked.IsVerified {
  t.Fatalf("revocation failed: vendor=%+v, error=%v",revoked,err)
 }
 _,err=env.MarketplaceService.CreateProduct(ctx,seller,service.CreateProductInput{
  BusinessID:business.ID,Title:"After Revocation",PriceKobo:10000,StockQuantity:1,
 })
 if err!=service.ErrBusinessNotApproved{t.Fatalf("revoked vendor could publish: %v",err)}
 if _,err:=env.MarketplaceService.ListProductReviews(ctx,product.ID);err!=service.ErrProductNotFound {
  t.Fatalf("revoked vendor product reviews should not be public: %v",err)
 }
}
