package msapprovals

import (
	"context"

	"github.com/filecoin-project/sentinel-visor/model/registry"
	"go.opencensus.io/tag"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

func init() {
	registry.ModelRegistry.Register(registry.MultisigApprovalsTask, &MultisigApproval{})
}

type MultisigApproval struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName      struct{} `pg:"multisig_approvals"`
	Height         int64    `pg:",pk,notnull,use_zero"`
	StateRoot      string   `pg:",pk,notnull"`
	MultisigID     string   `pg:",pk,notnull"`
	Message        string   `pg:",pk,notnull"`       // cid of message
	Method         uint64   `pg:",notnull,use_zero"` // method number used for the approval 2=propose, 3=approve
	Approver       string   `pg:",pk,notnull"`       // address of signer that triggerd approval
	Threshold      uint64   `pg:",notnull,use_zero"`
	InitialBalance string   `pg:"type:numeric,notnull"`
	Signers        []string `pg:",notnull"`
	GasUsed        int64    `pg:",use_zero"`
	TransactionID  int64    `pg:",notnull,use_zero"`
	To             string   `pg:",use_zero"`            // address funds will move to in transaction
	Value          string   `pg:"type:numeric,notnull"` // amount of funds moved in transaction
}

func (ma *MultisigApproval) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "multisig_approvals"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, ma)
}

type MultisigApprovalList []*MultisigApproval

func (mal MultisigApprovalList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(mal) == 0 {
		return nil
	}
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "multisig_approvals"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, mal)
}
