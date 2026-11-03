package create

import (
	"context"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/model"
)

//go:generate mockgen -source contract.go -destination mocks_test.go -package create_test

type tripCreator interface {
	Create(ctx context.Context, in model.Draft) (domaintrip.Trip, error)
}
