package service_test

import (
 "context"
 "fmt"
 "testing"
 "time"

 "ratels-api/internal/service"
)

func TestMarketplaceReviewVerifiedPurchaseAndModeration(t *testing.T) {
 env:=setupTestEnv(t)
 defer env.Pool.Close()
 ctx:=context.Background()
 nonce:=time.Now().UnixNano()
 admin:=createRegisteredUser(t,env,fmt.Sprintf("review_admin_%d",nonce),"super_admin")
 _,_,product:=helperCreateSellerWithProduct(t,env,ctx,fmt.Sprintf("review_seller_%d",nonce),"Review Vendor","Review Product",25,admin)
 buyer:=createRegisteredUser(t,env,fmt.Sprintf("review_buyer_%d",nonce),"member")
 reviewer:=service.SubmitReviewInput{Rating:5,Comment:"Excellent product that arrived exactly as described."}

 if _,err:=env.MarketplaceService.SubmitProductReview(ctx,buyer,product.ID,reviewer);err!=service.ErrReviewIneligible {
  t.Fatalf("expected non-buyer review denial, got %v",err)
 }
 order,err:=env.MarketplaceService.CreateOrder(ctx,buyer,service.CheckoutInput{
  Items:[]service.CartItemInput{{ProductID:product.ID,Quantity:1}},
  BuyerName:"Test Buyer",BuyerEmail:"review-buyer@example.org",BuyerPhone:"+2348012345678",
  DeliveryType:"delivery",ShippingAddress:"10 Main Street, Lagos",
 })
 if err!=nil {t.Fatalf("create test order: %v",err)}
 if _,err:=env.MarketplaceService.SubmitProductReview(ctx,buyer,product.ID,reviewer);err!=service.ErrReviewIneligible {
  t.Fatalf("unpaid order must not qualify for review, got %v",err)
 }
 if _,err:=env.MarketplaceService.ConfirmPayment(ctx,order.Payment.Reference);err!=nil {
  t.Fatalf("confirm sandbox test payment: %v",err)
 }
 if _,err:=env.Pool.Exec(ctx,"UPDATE marketplace_orders SET payment_status='paid',order_status='delivered',fulfilment_status='delivered' WHERE id=$1",order.Order.ID);err!=nil {
  t.Fatalf("mark test delivery complete: %v",err)
 }
 reviewID,err:=env.MarketplaceService.SubmitProductReview(ctx,buyer,product.ID,reviewer)
 if err!=nil{t.Fatalf("verified buyer should be able to post: %v",err)}
 if _,err:=env.MarketplaceService.SubmitProductReview(ctx,buyer,product.ID,reviewer);err==nil{
  t.Fatal("duplicate review was accepted")
 }
 reviews,err:=env.MarketplaceService.ListProductReviews(ctx,product.ID)
 if err!=nil||reviews.Count!=1||reviews.AverageRating!=5{
  t.Fatalf("unexpected verified review summary: %+v, err %v",reviews,err)
 }
 if err:=env.MarketplaceService.ModerateReview(ctx,buyer,reviewID,"hidden");err!=service.ErrScopeUnauthorized {
  t.Fatalf("ordinary member moderated review: %v",err)
 }
 if err:=env.MarketplaceService.ModerateReview(ctx,admin,reviewID,"hidden");err!=nil{
  t.Fatalf("Super Admin cannot moderate review: %v",err)
 }
 if err:=env.MarketplaceService.EditProductReview(ctx,buyer,reviewID,service.SubmitReviewInput{Rating:4,Comment:"Still good, updated after more use."});err!=nil{
  t.Fatalf("buyer could not edit own review: %v",err)
 }
 reviews,err=env.MarketplaceService.ListProductReviews(ctx,product.ID)
 if err!=nil||reviews.Count!=0 {t.Fatalf("hidden review became visible after buyer edit: %+v, err %v",reviews,err)}
 if err:=env.MarketplaceService.ModerateReview(ctx,admin,reviewID,"published");err!=nil {
  t.Fatalf("Super Admin cannot restore review: %v",err)
 }
 reviews,err=env.MarketplaceService.ListProductReviews(ctx,product.ID)
 if err!=nil||reviews.Count!=1||reviews.AverageRating!=4 {t.Fatalf("review restore failed: %+v, err %v",reviews,err)}
 for _,sort := range []string{"newest","oldest","highest","lowest","with_photos"} {
  page,err:=env.MarketplaceService.ListProductReviewsPage(ctx,product.ID,sort,1,20)
  if err!=nil || page.Count!=1 || len(page.Reviews)!=1 || page.HasMore {
   t.Fatalf("review sorting %s returned unexpected page %+v error %v",sort,page,err)
  }
 }
 page2,err:=env.MarketplaceService.ListProductReviewsPage(ctx,product.ID,"newest",2,20)
 if err!=nil || page2.Count!=1 || len(page2.Reviews)!=0 {
  t.Fatalf("review second page unexpected: %+v error %v",page2,err)
 }
 if _,err:=env.MarketplaceService.ListProductReviewsPage(ctx,product.ID,"rating DESC; DROP TABLE users",1,20);err!=service.ErrReviewInvalid{
  t.Fatalf("unsafe sort was accepted: %v",err)
 }
 if _,err:=env.MarketplaceService.ListProductReviewsPage(ctx,product.ID,"newest",0,20);err!=service.ErrReviewInvalid{
  t.Fatalf("invalid page was accepted: %v",err)
 }
}
