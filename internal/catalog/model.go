package catalog

// productDoc is how a product is stored in MongoDB. Enums are stored as their
// short names ("CARD", "NEAR_MINT") so the collection stays readable.
type productDoc struct {
	ID          string   `bson:"_id" json:"id"`
	Name        string   `bson:"name" json:"name"`
	Type        string   `bson:"type" json:"type"`
	PriceCents  *int64   `bson:"priceCents,omitempty" json:"priceCents,omitempty"` // nil = hidden from customers
	ImageURL    string   `bson:"imageUrl,omitempty" json:"imageUrl,omitempty"`
	Description string   `bson:"description,omitempty" json:"description,omitempty"`
	SetCode     string   `bson:"setCode" json:"setCode"`
	SetName     string   `bson:"setName" json:"setName"`
	Language    string   `bson:"language" json:"language"`
	Card        *cardDoc `bson:"card,omitempty" json:"card,omitempty"`
	// Card listings made with CreateProduct (another condition, language or grading).
	// The import never writes their ids, so the staff can price and edit them.
	CreatedByStaff bool `bson:"createdByStaff,omitempty" json:"createdByStaff,omitempty"`
}

// imported reports whether the pokemontcg.io import owns the product: it replaces the
// whole document on every run, so manual changes would be lost.
func (d *productDoc) imported() bool {
	return d.Card != nil && !d.CreatedByStaff
}

type cardDoc struct {
	Number    string      `bson:"number" json:"number"`
	Rarity    string      `bson:"rarity" json:"rarity"`
	Supertype string      `bson:"supertype" json:"supertype"`
	Types     []string    `bson:"types,omitempty" json:"types,omitempty"`
	Condition string      `bson:"condition" json:"condition"`
	Grading   *gradingDoc `bson:"grading,omitempty" json:"grading,omitempty"`
}

type gradingDoc struct {
	Company string `bson:"company" json:"company"`
	Grade   string `bson:"grade" json:"grade"`
}
