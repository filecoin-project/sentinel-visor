package messages

import (
	"context"
	"fmt"

	"github.com/go-pg/pg/v10"
)

type Message struct {
	Cid string `pg:",pk,notnull"`

	From       string `pg:",notnull"`
	To         string `pg:",notnull"`
	Value      string `pg:",notnull"`
	GasFeeCap  string `pg:",notnull"`
	GasPremium string `pg:",notnull"`

	GasLimit  int64  `pg:",use_zero"`
	SizeBytes int    `pg:",use_zero"`
	Nonce     uint64 `pg:",use_zero"`
	Method    uint64 `pg:",use_zero"`

	Params []byte
}

func (m *Message) PersistWithTx(ctx context.Context, tx *pg.Tx) error {
	if _, err := tx.ModelContext(ctx, m).
		OnConflict("do nothing").
		Insert(); err != nil {
		return fmt.Errorf("persisting message: %w", err)
	}
	return nil
}

type Messages []*Message

func (ms Messages) PersistWithTx(ctx context.Context, tx *pg.Tx) error {
	for _, m := range ms {
		if err := m.PersistWithTx(ctx, tx); err != nil {
			return err
		}
	}
	return nil
}
