// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.13.0

package postgres

import (
	"context"
)

type Querier interface {
	CreateTransfer(ctx context.Context, arg CreateTransferParams) (Transfer, error)
	DiscardTransfer(ctx context.Context, id int64) (int64, error)
	GetTransferStatusByID(ctx context.Context, id int64) (ValidStatus, error)
	ListUserTransfersByEmail(ctx context.Context, arg ListUserTransfersByEmailParams) ([]Transfer, error)
	ListUserTransfersByID(ctx context.Context, arg ListUserTransfersByIDParams) ([]Transfer, error)
	UpdateTransferStatus(ctx context.Context, arg UpdateTransferStatusParams) (int64, error)
}

var _ Querier = (*Queries)(nil)
