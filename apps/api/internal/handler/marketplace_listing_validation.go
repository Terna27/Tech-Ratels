package handler

import (
 "errors"
 "regexp"
 "strings"
 "unicode/utf8"
)

func validateMarketplacePublishListing(title, description, category, sku string, images []string, stock int32) error {
 name:=strings.TrimSpace(title)
 desc:=strings.TrimSpace(description)
 if utf8.RuneCountInString(name)<5 || utf8.RuneCountInString(name)>200 {
  return errors.New("published products require a meaningful title between 5 and 200 characters")
 }
 if utf8.RuneCountInString(desc)<40 || utf8.RuneCountInString(desc)>5000 {
  return errors.New("published products require a detailed description of 40–5000 characters including specifications and condition")
 }
 if strings.TrimSpace(category)=="" || strings.TrimSpace(sku)=="" {
  return errors.New("published products require category and SKU")
 }
 if len(images)<1 || len(images)>5 {
  return errors.New("published products require 1–5 clear product images")
 }
 if stock<0 { return errors.New("stock cannot be negative") }
 return nil
}

var marketplaceStoredImageFilename = regexp.MustCompile(`^[a-f0-9]{32}\.(jpg|png|webp)$`)

// validateMarketplaceImageOwnership only accepts server-generated product image
// keys under the vendor's own storage prefix. External URLs are rejected when publishing.
func validateMarketplaceImageOwnership(images []string, publicPrefix string) error {
 if publicPrefix==""{return errors.New("product image storage is not configured")}
 for _,imageURL:=range images {
  if !strings.HasPrefix(imageURL,publicPrefix) || !marketplaceStoredImageFilename.MatchString(strings.TrimPrefix(imageURL,publicPrefix)) {
   return errors.New("published product images must be uploaded to the vendor's own Ratels image storage")
  }
 }
 return nil
}
