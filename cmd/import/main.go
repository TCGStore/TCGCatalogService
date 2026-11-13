// Command import loads single cards and their market prices from pokemontcg.io.
// It only writes cards: sealed products come from the seed and the staff prices them.
//
//	go run ./cmd/import                  # the default sets
//	go run ./cmd/import -sets sv3pt5,sv1
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/TCGStore/TCGCatalogService/internal/catalog"
	"github.com/TCGStore/TCGCatalogService/internal/config"
	"github.com/TCGStore/TCGCatalogService/internal/pokemontcg"
)

// 151, Obsidian Flames and Scarlet & Violet. Basic energies (sve) have no market price.
const defaultSets = "sv3pt5,sv3,sv1"

func main() {
	sets := flag.String("sets", defaultSets, "comma-separated pokemontcg.io set ids")
	flag.Parse()

	if err := run(*sets); err != nil {
		slog.Error("import failed", "error", err)
		os.Exit(1)
	}
}

func run(sets string) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	client, db, err := config.ConnectMongo(ctx, cfg)
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	repo := catalog.NewRepository(db)
	api := pokemontcg.NewClient(os.Getenv("POKEMONTCG_API_KEY"))

	// A set that fails doesn't stop the others; running the import again fills it in.
	var failed []string
	for _, setID := range strings.Split(sets, ",") {
		setID = strings.TrimSpace(setID)
		if err := importSet(ctx, api, repo, setID); err != nil {
			slog.Error("set not imported", "set", setID, "error", err)
			failed = append(failed, setID)
		}
	}
	if len(failed) > 0 {
		return errors.New("sets not imported, run it again with -sets " + strings.Join(failed, ","))
	}
	return nil
}

func importSet(ctx context.Context, api *pokemontcg.Client, repo *catalog.Repository, setID string) error {
	set, err := api.GetSet(ctx, setID)
	if err != nil {
		return err
	}
	cards, err := api.GetCards(ctx, setID)
	if err != nil {
		return err
	}
	unpriced, err := repo.ImportCards(ctx, set, cards)
	if err != nil {
		return fmt.Errorf("save cards: %w", err)
	}
	slog.Info("set imported", "set", setID, "code", set.PtcgoCode, "cards", len(cards), "without_price", unpriced)
	return nil
}
