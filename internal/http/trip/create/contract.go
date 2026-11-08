package create

import (
	"context"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/model"
)

//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package create_test -mock_names tripCreator=MockTripCreator

type tripCreator interface {
	Create(ctx context.Context, in model.Draft) (domaintrip.Trip, error)
}
