package catalog

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/TCGStore/TCGCatalogService/gen/catalogpb"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxBatchIDs     = 100
)

type Server struct {
	catalogpb.UnimplementedCatalogServiceServer
	repo *Repository
}

func NewServer(repo *Repository) *Server {
	return &Server{repo: repo}
}

func (s *Server) GetProduct(ctx context.Context, req *catalogpb.GetProductRequest) (*catalogpb.Product, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	doc, err := s.repo.FindByID(ctx, req.GetId(), req.GetIncludeUnpriced())
	if errors.Is(err, errNotFound) {
		return nil, status.Errorf(codes.NotFound, "product %q not found", req.GetId())
	}
	if err != nil {
		return nil, internalError("get product", err)
	}
	return toProto(doc), nil
}

func (s *Server) GetProducts(ctx context.Context, req *catalogpb.GetProductsRequest) (*catalogpb.GetProductsResponse, error) {
	if len(req.GetIds()) > maxBatchIDs {
		return nil, status.Errorf(codes.InvalidArgument, "at most %d ids per request", maxBatchIDs)
	}
	if len(req.GetIds()) == 0 {
		return &catalogpb.GetProductsResponse{}, nil
	}

	docs, err := s.repo.FindByIDs(ctx, req.GetIds(), req.GetIncludeUnpriced())
	if err != nil {
		return nil, internalError("get products", err)
	}
	return &catalogpb.GetProductsResponse{Products: toProtoList(docs)}, nil
}

func (s *Server) SearchProducts(ctx context.Context, req *catalogpb.SearchProductsRequest) (*catalogpb.SearchProductsResponse, error) {
	if req.GetMinPriceCents() < 0 || req.GetMaxPriceCents() < 0 {
		return nil, status.Error(codes.InvalidArgument, "prices can't be negative")
	}
	page, pageSize, err := normalizePage(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	docs, total, err := s.repo.Search(ctx, buildSearchFilter(req), page, pageSize)
	if err != nil {
		return nil, internalError("search products", err)
	}
	return &catalogpb.SearchProductsResponse{Products: toProtoList(docs), TotalCount: int32(total)}, nil
}

func (s *Server) SetProductPrice(ctx context.Context, req *catalogpb.SetProductPriceRequest) (*catalogpb.Product, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if req.PriceCents != nil && req.GetPriceCents() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "price_cents must be positive; leave it unset to remove the price")
	}

	doc, err := s.repo.FindByID(ctx, req.GetId(), true)
	if errors.Is(err, errNotFound) {
		return nil, status.Errorf(codes.NotFound, "product %q not found", req.GetId())
	}
	if err != nil {
		return nil, internalError("set product price", err)
	}
	// The next import would overwrite a manual card price anyway.
	if doc.Card != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%q is a card: card prices come from the pokemontcg.io import", req.GetId())
	}

	doc, err = s.repo.SetPrice(ctx, req.GetId(), req.PriceCents)
	if err != nil {
		return nil, internalError("set product price", err)
	}
	return toProto(doc), nil
}

func normalizePage(page, pageSize int32) (int64, int64, error) {
	if page < 0 || pageSize < 0 {
		return 0, 0, status.Error(codes.InvalidArgument, "page and page_size can't be negative")
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return int64(page), int64(pageSize), nil
}

func toProtoList(docs []productDoc) []*catalogpb.Product {
	products := make([]*catalogpb.Product, len(docs))
	for i := range docs {
		products[i] = toProto(&docs[i])
	}
	return products
}

// internalError logs the real cause and hides it from the caller.
func internalError(op string, err error) error {
	slog.Error(op+" failed", "error", err)
	return status.Error(codes.Internal, "internal error")
}
