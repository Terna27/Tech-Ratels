package handler

import (
 "errors"
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
