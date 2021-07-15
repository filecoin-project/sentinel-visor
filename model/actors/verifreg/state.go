package verifreg

import (
	"context"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
	"go.opencensus.io/tag"
)

const (
	Added   = "ADDED"
	Removed = "REMOVED"
)

type VerifiedRegistryVerifierEvent struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	StateRoot string `pg:",pk,notnull"`
	Address   string `pg:",pk,notnull"`
	Event     string `pg:",pk,notnull,type:verified_registry_event_type"`
}

type VerifiedRegistryVerifierEventList []*VerifiedRegistryVerifierEvent

func (v *VerifiedRegistryVerifierEvent) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "verified_registry_verifier_event"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, v)
}

func (v VerifiedRegistryVerifierEventList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "verified_registry_verifier_event_list"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, v)
}

type VerifiedRegistryClientEvent struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	StateRoot string `pg:",pk,notnull"`
	Address   string `pg:",pk,notnull"`
	Event     string `pg:",pk,notnull,type:verified_registry_event_type"`
}

type VerifiedRegistryClientEventList []*VerifiedRegistryClientEvent

func (v *VerifiedRegistryClientEvent) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "verified_registry_client_event"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, v)
}

func (v VerifiedRegistryClientEventList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "verified_registry_client_event_list"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, v)
}

type VerifiedRegistryVerifier struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	StateRoot string `pg:",pk,notnull"`
	Address   string `pg:",pk,notnull"`

	DataCap string `pg:"type:numeric,notnull"`
}

type VerifiedRegistryVerifiersList []*VerifiedRegistryVerifier

func (v *VerifiedRegistryVerifier) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "verified_registry_verifier"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, v)
}

func (v VerifiedRegistryVerifiersList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "verified_registry_verifier_list"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, v)
}

type VerifiedRegistryVerifiedClient struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	StateRoot string `pg:",pk,notnull"`
	Address   string `pg:",pk,notnull"`

	DataCap string `pg:"type:numeric,notnull"`
}

type VerifiedRegistryVerifiedClientsList []*VerifiedRegistryVerifiedClient

func (v *VerifiedRegistryVerifiedClient) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "verified_registry_verified_client"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, v)
}

func (v VerifiedRegistryVerifiedClientsList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "verified_registry_verified_client_list"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, v)
}

var _ model.Persistable = (*VerifiedRegistryVerifierEvent)(nil)
var _ model.Persistable = (*VerifiedRegistryVerifierEventList)(nil)
var _ model.Persistable = (*VerifiedRegistryClientEvent)(nil)
var _ model.Persistable = (*VerifiedRegistryClientEventList)(nil)
var _ model.Persistable = (*VerifiedRegistryVerifier)(nil)
var _ model.Persistable = (*VerifiedRegistryVerifiersList)(nil)
var _ model.Persistable = (*VerifiedRegistryVerifiedClient)(nil)
var _ model.Persistable = (*VerifiedRegistryVerifiedClientsList)(nil)
