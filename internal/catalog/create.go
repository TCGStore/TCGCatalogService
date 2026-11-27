package catalog

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/TCGStore/TCGCatalogService/gen/catalogpb"
)

// Short forms used in ids, matching the seed (prd_obf_box_en) and the import (prd_mew_006_en_nm).
var (
	sealedIDParts = map[catalogpb.ProductType]string{
		catalogpb.ProductType_PRODUCT_TYPE_BOOSTER_PACK:      "pack",
		catalogpb.ProductType_PRODUCT_TYPE_BOOSTER_BOX:       "box",
		catalogpb.ProductType_PRODUCT_TYPE_ELITE_TRAINER_BOX: "etb",
	}
	conditionIDParts = map[catalogpb.CardCondition]string{
		catalogpb.CardCondition_CARD_CONDITION_NEAR_MINT:         "nm",
		catalogpb.CardCondition_CARD_CONDITION_LIGHTLY_PLAYED:    "lp",
		catalogpb.CardCondition_CARD_CONDITION_MODERATELY_PLAYED: "mp",
		catalogpb.CardCondition_CARD_CONDITION_HEAVILY_PLAYED:    "hp",
		catalogpb.CardCondition_CARD_CONDITION_DAMAGED:           "dmg",
	}
	notIDChars = regexp.MustCompile(`[^a-z0-9]+`)
)

// newProductDoc validates a CreateProduct request and builds the document with its id.
// The error message goes back to the caller as INVALID_ARGUMENT.
func newProductDoc(req *catalogpb.CreateProductRequest) (*productDoc, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, errors.New("name is required")
	}
	if req.PriceCents != nil && req.GetPriceCents() <= 0 {
		return nil, errors.New("price_cents must be positive; leave it unset to create the product hidden")
	}

	doc := &productDoc{
		Name:        name,
		Type:        shortName(req.GetType().String(), productTypePrefix),
		PriceCents:  req.PriceCents,
		ImageURL:    strings.TrimSpace(req.GetImageUrl()),
		Description: strings.TrimSpace(req.GetDescription()),
	}

	switch req.GetType() {
	case catalogpb.ProductType_PRODUCT_TYPE_UNSPECIFIED:
		return nil, errors.New("type is required")
	case catalogpb.ProductType_PRODUCT_TYPE_CARD:
		card := req.GetCard()
		if card == nil {
			return nil, errors.New("a CARD needs card details")
		}
		if err := fillCard(doc, card); err != nil {
			return nil, err
		}
	default:
		sealed := req.GetSealed()
		if sealed == nil {
			return nil, errors.New("sealed products need sealed details")
		}
		doc.SetCode = strings.ToUpper(strings.TrimSpace(sealed.GetSetCode()))
		doc.SetName = strings.TrimSpace(sealed.GetSetName())
		doc.Language = strings.ToLower(strings.TrimSpace(sealed.GetLanguage()))
		if doc.SetCode == "" || doc.Language == "" {
			return nil, errors.New("set_code and language are required")
		}
		doc.ID = productID(doc.SetCode, sealedIDParts[req.GetType()], doc.Language)
	}
	return doc, nil
}

func fillCard(doc *productDoc, card *catalogpb.CardDetails) error {
	doc.SetCode = strings.ToUpper(strings.TrimSpace(card.GetSetCode()))
	doc.SetName = strings.TrimSpace(card.GetSetName())
	doc.Language = strings.ToLower(strings.TrimSpace(card.GetLanguage()))
	number := strings.TrimSpace(card.GetNumber())
	condition, ok := conditionIDParts[card.GetCondition()]
	if doc.SetCode == "" || number == "" || doc.Language == "" || !ok {
		return errors.New("cards need set_code, number, language and condition")
	}

	doc.CreatedByStaff = true
	doc.Card = &cardDoc{
		Number:    number,
		Rarity:    strings.TrimSpace(card.GetRarity()),
		Supertype: enumName(card.GetSupertype().String(), supertypePrefix),
		Types:     card.GetTypes(),
		Condition: shortName(card.GetCondition().String(), cardConditionPrefix),
	}

	// "006/165" is the 6th card of the set, like the import's prd_mew_006_en_nm.
	parts := []string{doc.SetCode, padNumber(strings.SplitN(number, "/", 2)[0]), doc.Language, condition}
	if g := card.GetGrading(); g != nil {
		company, grade := strings.TrimSpace(g.GetCompany()), strings.TrimSpace(g.GetGrade())
		if company == "" || grade == "" {
			return errors.New("grading needs company and grade")
		}
		doc.Card.Grading = &gradingDoc{Company: strings.ToUpper(company), Grade: grade}
		parts = append(parts, company+grade)
	} else if doc.Language == "en" && condition == "nm" {
		// That's the import's id: the next import would replace the listing.
		return errors.New("English Near Mint cards come from the pokemontcg.io import; create another condition, language or a graded card")
	}
	doc.ID = productID(parts...)
	return nil
}

// productID turns ("MEW", "006", "en", "nm", "BGS9.5") into "prd_mew_006_en_nm_bgs9_5".
func productID(parts ...string) string {
	for i, part := range parts {
		parts[i] = strings.Trim(notIDChars.ReplaceAllString(strings.ToLower(part), "_"), "_")
	}
	return fmt.Sprintf("prd_%s", strings.Join(parts, "_"))
}

// enumName is shortName that stores UNSPECIFIED as "" instead of "UNSPECIFIED".
func enumName(protoName, prefix string) string {
	if name := shortName(protoName, prefix); name != "UNSPECIFIED" {
		return name
	}
	return ""
}
