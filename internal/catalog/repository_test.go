package catalog

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/protobuf/proto"

	"github.com/Javiercatalanv/ArquitecturaSistemas/services/catalog-service/gen/catalogpb"
)

func TestBuildSearchFilter(t *testing.T) {
	tests := []struct {
		name string
		req  *catalogpb.SearchProductsRequest
		want bson.M
	}{
		{
			name: "no filters",
			req:  &catalogpb.SearchProductsRequest{},
			want: bson.M{},
		},
		{
			name: "fire cards in english under 30 dollars",
			req: &catalogpb.SearchProductsRequest{
				Type:          catalogpb.ProductType_PRODUCT_TYPE_CARD,
				EnergyType:    "Fire",
				Language:      "en",
				Condition:     catalogpb.CardCondition_CARD_CONDITION_NEAR_MINT,
				MaxPriceCents: proto.Int64(3000),
			},
			want: bson.M{
				"type":           "CARD",
				"card.types":     "Fire",
				"language":       "en",
				"card.condition": "NEAR_MINT",
				"priceCents":     bson.M{"$lte": int64(3000)},
			},
		},
		{
			name: "zero min price is still applied",
			req:  &catalogpb.SearchProductsRequest{MinPriceCents: proto.Int64(0)},
			want: bson.M{"priceCents": bson.M{"$gte": int64(0)}},
		},
		{
			name: "query is escaped",
			req:  &catalogpb.SearchProductsRequest{Query: "ex (151)"},
			want: bson.M{"name": bson.M{"$regex": `ex \(151\)`, "$options": "i"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSearchFilter(tt.req)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToProtoMapsCardAndSealed(t *testing.T) {
	card := toProto(&productDoc{
		ID: "c1", Type: "CARD", Language: "en", SetCode: "OBF",
		Card: &cardDoc{Supertype: "POKEMON", Condition: "NEAR_MINT", Grading: &gradingDoc{Company: "PSA", Grade: "10"}},
	})
	if card.GetType() != catalogpb.ProductType_PRODUCT_TYPE_CARD {
		t.Errorf("type = %v", card.GetType())
	}
	if card.GetCard().GetCondition() != catalogpb.CardCondition_CARD_CONDITION_NEAR_MINT {
		t.Errorf("condition = %v", card.GetCard().GetCondition())
	}
	if card.GetCard().GetGrading().GetGrade() != "10" {
		t.Errorf("grading = %v", card.GetCard().GetGrading())
	}

	sealed := toProto(&productDoc{ID: "s1", Type: "BOOSTER_BOX", SetCode: "OBF", Language: "ja"})
	if sealed.GetSealed().GetLanguage() != "ja" || sealed.GetCard() != nil {
		t.Errorf("sealed details = %v", sealed.GetDetails())
	}
}
