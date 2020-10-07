package messages

import (
	"context"

	"github.com/go-pg/pg/v10"
	"go.opentelemetry.io/otel/api/global"
)

type MessageTaskResult struct {
	Messages      Messages
	BlockMessages BlockMessages
	Receipts      Receipts
}

func (mtr *MessageTaskResult) Persist(ctx context.Context, db *pg.DB) error {
	return db.RunInTransaction(ctx, func(tx *pg.Tx) error {
		return mtr.PersistWithTx(ctx, tx)
	})

}

func (mtr *MessageTaskResult) PersistWithTx(ctx context.Context, tx *pg.Tx) error {
	ctx, span := global.Tracer("").Start(ctx, "MessageTaskResult.PersistWithTx")
	defer span.End()

	if err := mtr.Messages.PersistWithTx(ctx, tx); err != nil {
		return err
	}
	if err := mtr.BlockMessages.PersistWithTx(ctx, tx); err != nil {
		return err
	}
	if err := mtr.Receipts.PersistWithTx(ctx, tx); err != nil {
		return err
	}

	return nil
}
