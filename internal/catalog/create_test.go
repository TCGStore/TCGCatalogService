package catalog

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/TCGStore/TCGCatalogService/gen/catalogpb"
)

func card(condition catalogpb.CardCondition, language string, grading *catalogpb.Grading) *catalogpb.CreateProductRequest {
	return &catalogpb.CreateProductRequest{
		Name: "Charizard ex",
		Type: catalogpb.ProductType_PRODUCT_TYPE_CARD,
		Details: &catalogpb.CreateProductRequest_Card{Card: &catalogpb.CardDetails{
			SetCode:   "mew",
			SetName:   "151",
			Number:    "6/165",
			Language:  language,
			Supertype: catalogpb.CardSupertype_CARD_SUPERTYPE_POKEMON,
			Types:     []string{"Fire"},
			Condition: condition,
			Grading:   grading,
		}},
	}
}

func TestNewProductDocIDs(t *testing.T) {
	tests := []struct {
		name string
		req  *catalogpb.CreateProductRequest
		want string
	}{
		{
			name: "sealed product, like the seed",
			req: &catalogpb.CreateProductRequest{
				Name:    "Surging Sparks Booster Box",
				Type:    catalogpb.ProductType_PRODUCT_TYPE_BOOSTER_BOX,
				Details: &catalogpb.CreateProductRequest_Sealed{Sealed: &catalogpb.SealedDetails{SetCode: "SSP", Language: "EN"}},
			},
			want: "prd_ssp_box_en",
		},
		{
			name: "played copy of an imported card",
			req:  card(catalogpb.CardCondition_CARD_CONDITION_LIGHTLY_PLAYED, "en", nil),
			want: "prd_mew_006_en_lp",
		},
		{
			name: "graded card with a half grade",
			req:  card(catalogpb.CardCondition_CARD_CONDITION_NEAR_MINT, "en", &catalogpb.Grading{Company: "bgs", Grade: "9.5"}),
			want: "prd_mew_006_en_nm_bgs9_5",
		},
		{
			name: "same card in Spanish",
			req:  card(catalogpb.CardCondition_CARD_CONDITION_NEAR_MINT, "es", nil),
			want: "prd_mew_006_es_nm",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := newProductDoc(tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if doc.ID != tt.want {
				t.Errorf("id = %q, want %q", doc.ID, tt.want)
			}
		})
	}
}

func TestNewProductDocKeepsCardData(t *testing.T) {
	doc, err := newProductDoc(card(catalogpb.CardCondition_CARD_CONDITION_LIGHTLY_PLAYED, "en", nil))
	if err != nil {
		t.Fatal(err)
	}
	if doc.imported() {
		t.Error("a card listing created by the staff must be editable")
	}
	if doc.SetCode != "MEW" || doc.Card.Number != "6/165" || doc.Card.Supertype != "POKEMON" || doc.Card.Condition != "LIGHTLY_PLAYED" {
		t.Errorf("unexpected card data: %+v %+v", doc, doc.Card)
	}
	if doc.PriceCents != nil {
		t.Error("without price_cents the product must start hidden")
	}
}

func TestNewProductDocRejects(t *testing.T) {
	sealedAsCard := card(catalogpb.CardCondition_CARD_CONDITION_LIGHTLY_PLAYED, "en", nil)
	sealedAsCard.Type = catalogpb.ProductType_PRODUCT_TYPE_BOOSTER_PACK

	negativePrice := card(catalogpb.CardCondition_CARD_CONDITION_LIGHTLY_PLAYED, "en", nil)
	negativePrice.PriceCents = proto.Int64(-100)

	tests := map[string]struct {
		req  *catalogpb.CreateProductRequest
		want string
	}{
		"the import's own listing":     {card(catalogpb.CardCondition_CARD_CONDITION_NEAR_MINT, "en", nil), "pokemontcg.io import"},
		"no condition":                 {card(catalogpb.CardCondition_CARD_CONDITION_UNSPECIFIED, "es", nil), "condition"},
		"grading without a grade":      {card(catalogpb.CardCondition_CARD_CONDITION_NEAR_MINT, "en", &catalogpb.Grading{Company: "PSA"}), "grading"},
		"type and details don't match": {sealedAsCard, "sealed details"},
		"negative price":               {negativePrice, "positive"},
		"no name":                      {&catalogpb.CreateProductRequest{Type: catalogpb.ProductType_PRODUCT_TYPE_BOOSTER_BOX}, "name"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newProductDoc(tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}
