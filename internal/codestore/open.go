package codestore

import (
	"context"

	"github.com/ponchione/sirtopham/internal/codeintel"
	"github.com/ponchione/sirtopham/internal/vectorstore"
)

func Open(ctx context.Context, path string) (codeintel.Store, error) {
	return vectorstore.NewStore(ctx, path)
}
