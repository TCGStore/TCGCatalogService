package catalog

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/TCGStore/TCGCatalogService/internal/pokemontcg"
)

// Raw (not reverse holo) prints first: that's the listing we sell.
var priceVariantOrder = []string{"normal", "holofoil", "1stEditionNormal", "1stEditionHolofoil", "unlimitedHolofoil"}

var supertypes = map[string]string{"Pokémon": "POKEMON", "Trainer": "TRAINER", "Energy": "ENERGY"}

// ImportCards stores the cards of a set as products and returns how many of them
// had no market price (those stay hidden from customers).
func (r *Repository) ImportCards(ctx context.Context, set pokemontcg.Set, cards []pokemontcg.Card) (unpriced int, err error) {
	docs, unpriced := cardsFromAPI(set, cards)
	return unpriced, r.upsertCards(ctx, docs)
}

// The API has one price per card, so each card becomes a single English Near Mint
// listing, with the same id scheme as the sealed products (prd_mew_006_en_nm).
func cardsFromAPI(set pokemontcg.Set, cards []pokemontcg.Card) (docs []productDoc, unpriced int) {
	setCode := set.PtcgoCode
	if setCode == "" {
		setCode = strings.ToUpper(set.ID)
	}

	docs = make([]productDoc, 0, len(cards))
	for _, c := range cards {
		number := padNumber(c.Number)
		doc := productDoc{
			ID:         fmt.Sprintf("prd_%s_%s_en_nm", strings.ToLower(setCode), strings.ToLower(number)),
			Name:       c.Name,
			Type:       "CARD",
			PriceCents: marketPriceCents(c),
			ImageURL:   c.Images.Small,
			SetCode:    setCode,
			SetName:    set.Name,
			Language:   "en",
			Card: &cardDoc{
				Number:    fmt.Sprintf("%s/%03d", number, set.PrintedTotal),
				Rarity:    c.Rarity,
				Supertype: supertypes[c.Supertype],
				Types:     c.Types,
				Condition: "NEAR_MINT",
			},
		}
		if doc.PriceCents == nil {
			unpriced++
		}
		docs = append(docs, doc)
	}
	return docs, unpriced
}

// padNumber turns "6" into "006" like the printed card; "TG01" stays as is.
func padNumber(number string) string {
	if n, err := strconv.Atoi(number); err == nil {
		return fmt.Sprintf("%03d", n)
	}
	return number
}

func marketPriceCents(c pokemontcg.Card) *int64 {
	if c.TCGPlayer == nil || len(c.TCGPlayer.Prices) == 0 {
		return nil
	}

	variants := append([]string{}, priceVariantOrder...)
	others := make([]string, 0, len(c.TCGPlayer.Prices))
	for variant := range c.TCGPlayer.Prices {
		others = append(others, variant)
	}
	sort.Strings(others) // map order is random; keeps the fallback stable between imports
	variants = append(variants, others...)

	for _, variant := range variants {
		price, ok := c.TCGPlayer.Prices[variant]
		if !ok {
			continue
		}
		for _, dollars := range []*float64{price.Market, price.Mid} {
			if dollars != nil && *dollars > 0 {
				cents := int64(math.Round(*dollars * 100))
				return &cents
			}
		}
	}
	return nil
}
