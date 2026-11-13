package catalog

import (
	"encoding/json"
	"testing"

	"github.com/TCGStore/TCGCatalogService/internal/pokemontcg"
)

var set151 = pokemontcg.Set{ID: "sv3pt5", Name: "151", PtcgoCode: "MEW", PrintedTotal: 165}

func apiCard(t *testing.T, raw string) pokemontcg.Card {
	t.Helper()
	var card pokemontcg.Card
	if err := json.Unmarshal([]byte(raw), &card); err != nil {
		t.Fatal(err)
	}
	return card
}

func TestCardsFromAPI(t *testing.T) {
	charizard := apiCard(t, `{
		"name": "Charizard ex", "number": "6", "rarity": "Double Rare", "supertype": "Pokémon",
		"types": ["Fire"], "images": {"small": "https://images.pokemontcg.io/sv3pt5/6.png"},
		"tcgplayer": {"prices": {"holofoil": {"market": 24.987, "mid": 30}}}
	}`)
	noSales := apiCard(t, `{"name": "Rare Candy", "number": "191", "supertype": "Trainer"}`)

	docs, unpriced := cardsFromAPI(set151, []pokemontcg.Card{charizard, noSales})

	if unpriced != 1 {
		t.Errorf("unpriced = %d, want 1", unpriced)
	}
	got := docs[0]
	if got.ID != "prd_mew_006_en_nm" || got.SetCode != "MEW" || got.Card.Number != "006/165" {
		t.Errorf("id/set/number = %s %s %s", got.ID, got.SetCode, got.Card.Number)
	}
	if got.PriceCents == nil || *got.PriceCents != 2499 {
		t.Errorf("price = %v, want 2499", got.PriceCents)
	}
	if got.Card.Supertype != "POKEMON" || got.Card.Condition != "NEAR_MINT" || got.Language != "en" {
		t.Errorf("card = %+v", got.Card)
	}
	if docs[1].PriceCents != nil || docs[1].Card.Supertype != "TRAINER" {
		t.Errorf("card without sales = %+v", docs[1])
	}
}

func TestMarketPrice(t *testing.T) {
	tests := []struct {
		name   string
		prices string
		want   int64 // 0 = no price
	}{
		{"normal print before reverse holo", `{"reverseHolofoil": {"market": 0.5}, "normal": {"market": 0.55}}`, 55},
		{"mid when there's no market price", `{"normal": {"mid": 1.2}}`, 120},
		{"zero is no price", `{"normal": {"market": 0}}`, 0},
		{"only reverse holo", `{"reverseHolofoil": {"market": 3}}`, 300},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marketPriceCents(apiCard(t, `{"tcgplayer": {"prices": `+tt.prices+`}}`))
			if tt.want == 0 {
				if got != nil {
					t.Errorf("got %d, want no price", *got)
				}
				return
			}
			if got == nil || *got != tt.want {
				t.Errorf("got %v, want %d", got, tt.want)
			}
		})
	}
}

func TestPadNumber(t *testing.T) {
	for in, want := range map[string]string{"6": "006", "151": "151", "TG01": "TG01"} {
		if got := padNumber(in); got != want {
			t.Errorf("padNumber(%q) = %q, want %q", in, got, want)
		}
	}
}
