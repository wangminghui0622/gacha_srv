package logic

import (
	"encoding/json"
	"strings"

	"gacha_srv/app/catalog/catalog"
)

type adminProductExtras struct {
	TotalStock    uint64             `json:"totalStock"`
	FrozenStock   uint64             `json:"frozenStock"`
	HalfBodyImage string             `json:"halfBodyImage"`
	ThumbURL      string             `json:"thumbUrl"`
	DetailIntro   string             `json:"detailIntro"`
	DetailImages  []string           `json:"detailImages"`
	DetailParams  []adminDetailParam `json:"detailParams"`
}

type adminDetailParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func encodeAdminJSON(totalStock, frozenStock uint64, halfBodyImage, thumbURL, detailIntro string, detailImages []string, params []*catalog.ProductDetailParam) (string, error) {
	extras := adminProductExtras{
		TotalStock: totalStock, FrozenStock: frozenStock,
		HalfBodyImage: halfBodyImage, ThumbURL: thumbURL, DetailIntro: detailIntro,
		DetailImages: detailImages,
	}
	for _, p := range params {
		if p == nil {
			continue
		}
		name, value := strings.TrimSpace(p.Name), strings.TrimSpace(p.Value)
		if name == "" && value == "" {
			continue
		}
		extras.DetailParams = append(extras.DetailParams, adminDetailParam{Name: name, Value: value})
	}
	blob, err := json.Marshal(extras)
	return string(blob), err
}
