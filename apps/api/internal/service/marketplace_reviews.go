package service

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "strings"
 "time"

 "ratels-api/internal/db"
 "ratels-api/internal/security"
 "ratels-api/internal/storage"
)

var (
 ErrReviewIneligible = errors.New("only buyers with a paid and delivered order may review this product")
 ErrReviewNotFound = errors.New("review not found")
 ErrReviewInvalid = errors.New("invalid review")
)

type MarketplaceReview struct {
 ID string `json:"id"`
 ProductID string `json:"product_id"`
 BuyerID string `json:"-"`
 BuyerName string `json:"buyer_name"`
 Rating int `json:"rating"`
 Comment string `json:"comment"`
 Images []string `json:"images"`
 VendorResponse *string `json:"vendor_response"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}

type ProductReviewsResponse struct {
 Reviews []MarketplaceReview `json:"reviews"`
 Count int `json:"count"`
 AverageRating float64 `json:"average_rating"`
}

type SubmitReviewInput struct {
 Rating int `json:"rating"`
 Comment string `json:"comment"`
 Images []string `json:"images"`
}

func (s *MarketplaceService) eligibleReviewOrder(ctx context.Context, buyerID, productID string) (string, error) {
 var orderID string
 err := s.repo.Pool().QueryRow(ctx, `
 SELECT o.id FROM marketplace_orders o
 JOIN marketplace_order_items i ON i.order_id=o.id
 WHERE o.buyer_user_id=$1 AND i.product_id=$2 AND o.payment_status IN ('paid','partially_refunded')
 AND o.order_status NOT IN ('cancelled','refunded','requires_reconciliation')
 AND (o.order_status='delivered' OR o.fulfilment_status='delivered'
 OR EXISTS (SELECT 1 FROM marketplace_shipments sh WHERE sh.order_id=o.id
 AND sh.business_id=i.business_id AND sh.status='delivered'))
 ORDER BY o.created_at DESC LIMIT 1
 `, buyerID, productID).Scan(&orderID)
 if err != nil { return "", ErrReviewIneligible }
 return orderID,nil
}

func (s *MarketplaceService) ValidateReviewImages(userID, productID string, images []string) error {
 if len(images)>5 {return fmt.Errorf("%w: maximum five images",ErrReviewInvalid)}
 prefix := s.storage.GetPublicURL("marketplace/reviews/"+userID+"/"+productID+"/")
 if prefix=="" {return ErrReviewInvalid}
 for _,img := range images {
  if len(img)>1024 || !strings.HasPrefix(img,prefix) || len(img)<=len(prefix) {return fmt.Errorf("%w: image must belong to this customer and product",ErrReviewInvalid)}
 }
 return nil
}

func (s *MarketplaceService) ListProductReviews(ctx context.Context, productID string) (*ProductReviewsResponse,error) {
 if _,err:=s.GetProductByID(ctx,productID);err!=nil{return nil,err}
 result:=&ProductReviewsResponse{Reviews:[]MarketplaceReview{}}
 err:=s.repo.Pool().QueryRow(ctx, `
 SELECT count(*),COALESCE(avg(rating),0)::float8
 FROM marketplace_product_reviews WHERE product_id=$1 AND status='published'
 `,productID).Scan(&result.Count,&result.AverageRating)
 if err!=nil{return nil,err}
 rows,err:=s.repo.Pool().Query(ctx, `
 SELECT r.id,r.product_id,u.display_name,r.rating,r.comment,r.images,r.vendor_response,r.created_at,r.updated_at
 FROM marketplace_product_reviews r JOIN users u ON u.id=r.buyer_user_id
 WHERE r.product_id=$1 AND r.status='published'
 ORDER BY r.created_at DESC LIMIT 100
 `,productID)
 if err!=nil{return nil,err}
 defer rows.Close()
 for rows.Next(){
  var review MarketplaceReview
  var images []byte
  if err:=rows.Scan(&review.ID,&review.ProductID,&review.BuyerName,&review.Rating,&review.Comment,&images,&review.VendorResponse,&review.CreatedAt,&review.UpdatedAt);err!=nil{return nil,err}
  if err:=json.Unmarshal(images,&review.Images);err!=nil{return nil,err}
  result.Reviews=append(result.Reviews,review)
 }
 return result,rows.Err()
}

func (s *MarketplaceService) SubmitProductReview(ctx context.Context,user *db.User, productID string,input SubmitReviewInput) (string,error) {
 if user==nil{return "",ErrReviewIneligible}
 if input.Rating<1||input.Rating>5||len(strings.TrimSpace(input.Comment))<10||len(input.Comment)>2000 {
  return "",ErrReviewInvalid
 }
 if err:=s.ValidateReviewImages(user.ID,productID,input.Images);err!=nil{return "",err}
 orderID,err:=s.eligibleReviewOrder(ctx,user.ID,productID);if err!=nil{return "",err}
 images,err:=json.Marshal(input.Images);if err!=nil{return "",err}
 id:=security.GenerateUUID()
 _,err=s.repo.Pool().Exec(ctx,`
 INSERT INTO marketplace_product_reviews(id,product_id,buyer_user_id,order_id,rating,comment,images)
 VALUES($1,$2,$3,$4,$5,$6,$7)
 `,id,productID,user.ID,orderID,input.Rating,strings.TrimSpace(input.Comment),images)
 if err!=nil{return "",fmt.Errorf("could not create review (only one review per product per buyer): %w",err)}
 _=s.audit.Log(ctx,&user.ID,&user.PrimaryRole,"create_marketplace_review","marketplace_review",id,nil,nil,nil,nil)
 return id,nil
}

func (s *MarketplaceService) EditProductReview(ctx context.Context,user *db.User,id string,input SubmitReviewInput) error {
 if user==nil{return ErrReviewNotFound}
 if input.Rating<1||input.Rating>5||len(strings.TrimSpace(input.Comment))<10||len(input.Comment)>2000{return ErrReviewInvalid}
 var productID string
 err:=s.repo.Pool().QueryRow(ctx,`SELECT product_id FROM marketplace_product_reviews WHERE id=$1 AND buyer_user_id=$2 AND created_at>now()-interval '30 days'`,id,user.ID).Scan(&productID)
 if err!=nil{return ErrReviewNotFound}
 if err:=s.ValidateReviewImages(user.ID,productID,input.Images);err!=nil{return err}
 images,err:=json.Marshal(input.Images);if err!=nil{return err}
 tag,err:=s.repo.Pool().Exec(ctx,`UPDATE marketplace_product_reviews SET rating=$1,comment=$2,images=$3,updated_at=now() WHERE id=$4 AND buyer_user_id=$5 AND created_at>now()-interval '30 days'`,input.Rating,strings.TrimSpace(input.Comment),images,id,user.ID)
 if err!=nil{return err};if tag.RowsAffected()==0{return ErrReviewNotFound}
 _=s.audit.Log(ctx,&user.ID,&user.PrimaryRole,"edit_marketplace_review","marketplace_review",id,nil,nil,nil,nil)
 return nil
}

func (s *MarketplaceService) ReplyToReview(ctx context.Context,user *db.User,id,reply string) error {
 if user==nil{return ErrUnauthorizedSellerAction}
 reply=strings.TrimSpace(reply)
 if len(reply)<2||len(reply)>2000{return ErrReviewInvalid}
 tag,err:=s.repo.Pool().Exec(ctx,`
 UPDATE marketplace_product_reviews r SET vendor_response=$1,vendor_response_at=now(),updated_at=now()
 FROM marketplace_products p JOIN marketplace_businesses b ON b.id=p.business_id
 WHERE r.id=$2 AND r.product_id=p.id AND b.owner_user_id=$3 AND b.status='approved' AND b.is_verified=true AND r.status='published'
 `,reply,id,user.ID)
 if err!=nil{return err};if tag.RowsAffected()==0{return ErrUnauthorizedSellerAction}
 _=s.audit.Log(ctx,&user.ID,&user.PrimaryRole,"respond_marketplace_review","marketplace_review",id,nil,nil,nil,nil)
 return nil
}

func (s *MarketplaceService) ReportReview(ctx context.Context,user *db.User,id,reason string) error {
 if user==nil{return ErrReviewInvalid}
 reason=strings.TrimSpace(reason)
 if len(reason)<5||len(reason)>500{return ErrReviewInvalid}
 tag,err:=s.repo.Pool().Exec(ctx,`
 INSERT INTO marketplace_review_reports(id,review_id,reporter_user_id,reason)
 SELECT $1,id,$2,$3 FROM marketplace_product_reviews WHERE id=$4 AND status='published'
 ON CONFLICT(review_id,reporter_user_id) DO NOTHING
 `,security.GenerateUUID(),user.ID,reason,id)
 if err!=nil{return err}
 if tag.RowsAffected()==0{return ErrReviewNotFound}
 return nil
}

func (s *MarketplaceService) ModerateReview(ctx context.Context,admin *db.User,id,status string) error {
 if admin==nil||!s.isSuperAdminUser(admin,s.getActiveAppointments(ctx,admin.ID)){return ErrScopeUnauthorized}
 if status!="published"&&status!="hidden"{return ErrReviewInvalid}
 tag,err:=s.repo.Pool().Exec(ctx,`UPDATE marketplace_product_reviews SET status=$1,updated_at=now() WHERE id=$2`,status,id)
 if err!=nil{return err};if tag.RowsAffected()==0{return ErrReviewNotFound}
 _=s.audit.Log(ctx,&admin.ID,&admin.PrimaryRole,"moderate_marketplace_review","marketplace_review",id,nil,nil,&status,nil)
 return nil
}

func (s *MarketplaceService) UploadReviewImage(ctx context.Context,user *db.User,productID string,r io.Reader,maxSize int64) (*UploadedImageResult,error) {
 if user==nil{return nil,ErrReviewIneligible}
 if _,err:=s.eligibleReviewOrder(ctx,user.ID,productID);err!=nil{return nil,err}
 if !s.storage.IsConfigured(){return nil,storage.ErrStorageUnconfigured}
 data,meta,err:=storage.InspectAndValidateImage(r,maxSize)
 if err!=nil{return nil,err}
 key:=storage.GenerateReviewImageKey(user.ID,productID,meta.Format)
 publicURL,err:=s.storage.UploadObject(ctx,key,meta.ContentType,bytes.NewReader(data),meta.Size)
 if err!=nil{return nil,err}
 return &UploadedImageResult{ObjectKey:key,PublicURL:publicURL,Width:meta.Width,Height:meta.Height,Format:meta.Format,ContentType:meta.ContentType,Size:meta.Size},nil
}

type MarketplaceReviewReport struct {
 ID string `json:"id"`
 ReviewID string `json:"review_id"`
 ProductID string `json:"product_id"`
 Comment string `json:"comment"`
 Rating int `json:"rating"`
 ReviewStatus string `json:"review_status"`
 Reason string `json:"reason"`
 CreatedAt time.Time `json:"created_at"`
}

func (s *MarketplaceService) ListReportedReviews(ctx context.Context, admin *db.User) ([]MarketplaceReviewReport, error) {
 if admin==nil || !s.isSuperAdminUser(admin,s.getActiveAppointments(ctx,admin.ID)){return nil,ErrScopeUnauthorized}
 rows,err:=s.repo.Pool().Query(ctx,`
 SELECT reports.id, reviews.id, reviews.product_id, reviews.comment, reviews.rating, reviews.status, reports.reason, reports.created_at
 FROM marketplace_review_reports reports JOIN marketplace_product_reviews reviews ON reviews.id=reports.review_id
 ORDER BY reports.created_at DESC LIMIT 100
 `)
 if err!=nil{return nil,err}
 defer rows.Close()
 result:=[]MarketplaceReviewReport{}
 for rows.Next(){
  var item MarketplaceReviewReport
  if err:=rows.Scan(&item.ID,&item.ReviewID,&item.ProductID,&item.Comment,&item.Rating,&item.ReviewStatus,&item.Reason,&item.CreatedAt);err!=nil{return nil,err}
  result=append(result,item)
 }
 return result,rows.Err()
}
