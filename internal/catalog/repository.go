package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Javiercatalanv/ArquitecturaSistemas/services/catalog-service/gen/catalogpb"
)

var errNotFound = errors.New("product not found")

type Repository struct {
	products *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{products: db.Collection("products")}
}

func (r *Repository) FindByID(ctx context.Context, id string) (*productDoc, error) {
	var doc productDoc
	err := r.products.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errNotFound
	}
	return &doc, err
}

func (r *Repository) FindByIDs(ctx context.Context, ids []string) ([]productDoc, error) {
	cursor, err := r.products.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	var docs []productDoc
	err = cursor.All(ctx, &docs)
	return docs, err
}

func (r *Repository) Search(ctx context.Context, filter bson.M, page, pageSize int64) ([]productDoc, int64, error) {
	total, err := r.products.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "name", Value: 1}, {Key: "_id", Value: 1}}).
		SetSkip((page - 1) * pageSize).
		SetLimit(pageSize)
	cursor, err := r.products.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	var docs []productDoc
	err = cursor.All(ctx, &docs)
	return docs, total, err
}

// Seed upserts every product in a JSON array, so running it twice doesn't duplicate data.
func (r *Repository) Seed(ctx context.Context, productsJSON []byte) (int, error) {
	var docs []productDoc
	if err := json.Unmarshal(productsJSON, &docs); err != nil {
		return 0, fmt.Errorf("parse seed data: %w", err)
	}
	for _, doc := range docs {
		_, err := r.products.ReplaceOne(ctx, bson.M{"_id": doc.ID}, doc, options.Replace().SetUpsert(true))
		if err != nil {
			return 0, fmt.Errorf("upsert %s: %w", doc.ID, err)
		}
	}
	return len(docs), nil
}

func buildSearchFilter(req *catalogpb.SearchProductsRequest) bson.M {
	filter := bson.M{}

	if req.GetQuery() != "" {
		// QuoteMeta stops user input from being interpreted as a regex.
		filter["name"] = bson.M{"$regex": regexp.QuoteMeta(req.GetQuery()), "$options": "i"}
	}
	if req.GetType() != catalogpb.ProductType_PRODUCT_TYPE_UNSPECIFIED {
		filter["type"] = shortName(req.GetType().String(), productTypePrefix)
	}
	if req.GetSetCode() != "" {
		filter["setCode"] = req.GetSetCode()
	}
	if req.GetLanguage() != "" {
		filter["language"] = req.GetLanguage()
	}
	if req.GetSupertype() != catalogpb.CardSupertype_CARD_SUPERTYPE_UNSPECIFIED {
		filter["card.supertype"] = shortName(req.GetSupertype().String(), supertypePrefix)
	}
	if req.GetEnergyType() != "" {
		filter["card.types"] = req.GetEnergyType()
	}
	if req.GetCondition() != catalogpb.CardCondition_CARD_CONDITION_UNSPECIFIED {
		filter["card.condition"] = shortName(req.GetCondition().String(), cardConditionPrefix)
	}
	if req.GetGradedOnly() {
		filter["card.grading"] = bson.M{"$exists": true}
	}

	price := bson.M{}
	if req.MinPriceCents != nil {
		price["$gte"] = req.GetMinPriceCents()
	}
	if req.MaxPriceCents != nil {
		price["$lte"] = req.GetMaxPriceCents()
	}
	if len(price) > 0 {
		filter["priceCents"] = price
	}

	return filter
}
