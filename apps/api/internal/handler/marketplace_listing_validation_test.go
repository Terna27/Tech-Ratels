package handler

import "testing"

func TestMarketplacePublishListingValidation(t *testing.T) {
 goodDescription:="Durable, corrosion-resistant steel gardening shovel. Includes a 90 cm handle, reinforced blade, and brand-new condition."
 tests:=[]struct{
  name string; title string; description string; category string; sku string; images []string; stock int32; wantError bool
 }{
  {"complete listing","Steel Shovel",goodDescription,"tools","SHOV-001",[]string{"https://assets.example.org/shovel.jpg"},20,false},
  {"missing title","",goodDescription,"tools","SHOV-001",[]string{"https://assets.example.org/shovel.jpg"},20,true},
  {"weak description","Steel Shovel","Shovel","tools","SHOV-001",[]string{"https://assets.example.org/shovel.jpg"},20,true},
  {"missing photo","Steel Shovel",goodDescription,"tools","SHOV-001",nil,20,true},
  {"missing SKU","Steel Shovel",goodDescription,"tools","",[]string{"https://assets.example.org/shovel.jpg"},20,true},
  {"negative stock","Steel Shovel",goodDescription,"tools","SHOV-001",[]string{"https://assets.example.org/shovel.jpg"},-2,true},
 }
 for _,tt:=range tests{t.Run(tt.name,func(t *testing.T){
  err:=validateMarketplacePublishListing(tt.title,tt.description,tt.category,tt.sku,tt.images,tt.stock)
  if (err!=nil)!=tt.wantError{t.Fatalf("validation error = %v; wantError=%t",err,tt.wantError)}
 })}
}
