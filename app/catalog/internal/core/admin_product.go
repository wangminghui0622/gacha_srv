package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	AdminProductTitleMaxLength = 255
	AdminProductCodeMaxLength  = 64
	AdminProductCoverMaxLength = 512
	SaleTypeInStock            = "IN_STOCK"
	SaleTypePreOrder           = "PREORDER"
)

// BuildAdminProduct 校验后台商品页的创建/编辑字段。
func BuildAdminProduct(title, productCode, saleType, coverURL string, costAmountMinor, saleAmountMinor int64) (string, string, string, string, error) {
	title = strings.TrimSpace(title)
	productCode = strings.TrimSpace(productCode)
	saleType = strings.TrimSpace(saleType)
	coverURL = strings.TrimSpace(coverURL)
	if title == "" {
		return "", "", "", "", fmt.Errorf("商品名称必填")
	}
	if utf8.RuneCountInString(title) > AdminProductTitleMaxLength {
		return "", "", "", "", fmt.Errorf("商品名称不能超过 %d 个字符", AdminProductTitleMaxLength)
	}
	if strings.ContainsAny(title, "\r\n\t") {
		return "", "", "", "", fmt.Errorf("商品名称不能包含换行或制表符")
	}
	if productCode != "" {
		if len(productCode) > AdminProductCodeMaxLength {
			return "", "", "", "", fmt.Errorf("商品编码不能超过 %d 个字符", AdminProductCodeMaxLength)
		}
		if strings.ContainsAny(productCode, " \r\n\t") {
			return "", "", "", "", fmt.Errorf("商品编码不能包含空格或换行")
		}
	}
	if saleType == "" {
		saleType = SaleTypeInStock
	}
	if saleType != SaleTypeInStock && saleType != SaleTypePreOrder {
		return "", "", "", "", fmt.Errorf("销售类型无效")
	}
	if costAmountMinor < 0 || saleAmountMinor < 0 {
		return "", "", "", "", fmt.Errorf("成本价和售价不能为负数")
	}
	if len(coverURL) > AdminProductCoverMaxLength {
		return "", "", "", "", fmt.Errorf("封面图地址不能超过 %d 个字符", AdminProductCoverMaxLength)
	}
	return title, productCode, saleType, coverURL, nil
}
