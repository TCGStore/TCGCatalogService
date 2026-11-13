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

	"github.com/TCGStore/TCGCatalogService/gen/catalogpb"
)

var errNotFound = errors.New("product not found")

// A product without a price has no priceCents field at all (see productDoc).
var (
	priced   = bson.M{"$exists": true}
	unpriced = bson.M{"$exists": false}
)

type Repository struct {
	products *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{products: db.Collection("products")}
}

func (r *Repository) FindByID(ctx context.Context, id string, includeUnpriced bool) (*productDoc, error) {
	filter := bson.M{"_id": id}
	if !includeUnpriced {
		filter["priceCents"] = priced
	}
	var doc productDoc
	err := r.products.FindOne(ctx, filter).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errNotFound
	}
	return &doc, err
}

func (r *Repository) FindByIDs(ctx context.Context, ids []string, includeUnpriced bool) ([]productDoc, error) {
	filter := bson.M{"_id": bson.M{"$in": ids}}
	if !includeUnpriced {
		filter["priceCents"] = priced
	}
	cursor, err := r.products.Find(ctx, filter)
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

// Seed inserts the products that don't exist yet. Existing ones are left alone, so
// running it again doesn't undo prices set by the staff.
func (r *Repository) Seed(ctx context.Context, productsJSON []byte) (int, error) {
	var docs []productDoc
	if err := json.Unmarshal(productsJSON, &docs); err != nil {
		return 0, fmt.Errorf("parse seed data: %w", err)
	}
	inserted := 0
	for _, doc := range docs {
		_, err := r.products.InsertOne(ctx, doc)
		if mongo.IsDuplicateKeyError(err) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("insert %s: %w", doc.ID, err)
		}
		inserted++
	}
	return inserted, nil
}

// SetPrice sets or, with nil, removes the price. It returns errNotFound if the
// product doesn't exist.
func (r *Repository) SetPrice(ctx context.Context, id string, priceCents *int64) (*productDoc, error) {
	update := bson.M{"$unset": bson.M{"priceCents": ""}}
	if priceCents != nil {
		update = bson.M{"$set": bson.M{"priceCents": *priceCents}}
	}
	var doc productDoc
	err := r.products.FindOneAndUpdate(ctx, bson.M{"_id": id}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errNotFound
	}
	return &doc, err
}

// upsertCards writes imported cards in one round trip. Each card replaces the
// stored one entirely, so a card that lost its market price becomes hidden.
func (r *Repository) upsertCards(ctx context.Context, docs []productDoc) error {
	if len(docs) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, len(docs))
	for i, doc := range docs {
		models[i] = mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": doc.ID}).
			SetReplacement(doc).
			SetUpsert(true)
	}
	_, err := r.products.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
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

	switch {
	case req.GetUnpricedOnly():
		// A price range can't match a product without a price, so it's ignored here.
		filter["priceCents"] = unpriced
		return filter
	case !req.GetIncludeUnpriced():
		filter["priceCents"] = priced
	}

	price := bson.M{}
	if req.MinPriceCents != nil {
		price["$gte"] = req.GetMinPriceCents()
	}
	if req.MaxPriceCents != nil {
		price["$lte"] = req.GetMaxPriceCents()
	}
	if len(price) > 0 {
		// A range already leaves out products without a price.
		filter["priceCents"] = price
	}

	return filter
}
