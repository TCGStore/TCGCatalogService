package catalog

import (
	"strings"

	"github.com/TCGStore/TCGCatalogService/gen/catalogpb"
)

const (
	productTypePrefix   = "PRODUCT_TYPE_"
	supertypePrefix     = "CARD_SUPERTYPE_"
	cardConditionPrefix = "CARD_CONDITION_"
)

// shortName turns "PRODUCT_TYPE_CARD" into "CARD".
func shortName(protoName, prefix string) string {
	return strings.TrimPrefix(protoName, prefix)
}

// enumValue turns "CARD" back into the proto value; unknown names map to 0 (UNSPECIFIED).
func enumValue(short, prefix string, values map[string]int32) int32 {
	return values[prefix+short]
}

func toProto(d *productDoc) *catalogpb.Product {
	p := &catalogpb.Product{
		Id:          d.ID,
		Name:        d.Name,
		Type:        catalogpb.ProductType(enumValue(d.Type, productTypePrefix, catalogpb.ProductType_value)),
		PriceCents:  d.PriceCents,
		ImageUrl:    d.ImageURL,
		Description: d.Description,
	}

	if d.Card == nil {
		p.Details = &catalogpb.Product_Sealed{Sealed: &catalogpb.SealedDetails{
			SetCode:  d.SetCode,
			SetName:  d.SetName,
			Language: d.Language,
		}}
		return p
	}

	card := &catalogpb.CardDetails{
		SetCode:   d.SetCode,
		SetName:   d.SetName,
		Number:    d.Card.Number,
		Rarity:    d.Card.Rarity,
		Language:  d.Language,
		Supertype: catalogpb.CardSupertype(enumValue(d.Card.Supertype, supertypePrefix, catalogpb.CardSupertype_value)),
		Types:     d.Card.Types,
		Condition: catalogpb.CardCondition(enumValue(d.Card.Condition, cardConditionPrefix, catalogpb.CardCondition_value)),
	}
	if d.Card.Grading != nil {
		card.Grading = &catalogpb.Grading{Company: d.Card.Grading.Company, Grade: d.Card.Grading.Grade}
	}
	p.Details = &catalogpb.Product_Card{Card: card}
	return p
}
