package actorstate

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/chain/types"
	verifregmodel "github.com/filecoin-project/sentinel-visor/model/actors/verifreg"
	sa0builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	sa2builtin "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	sa3builtin "github.com/filecoin-project/specs-actors/v3/actors/builtin"
	sa4builtin "github.com/filecoin-project/specs-actors/v4/actors/builtin"
	sa5builtin "github.com/filecoin-project/specs-actors/v5/actors/builtin"
	"go.opentelemetry.io/otel/api/global"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/chain/actors/adt"
	"github.com/filecoin-project/sentinel-visor/chain/actors/builtin/verifreg"
	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

type VerifiedRegistryExtractor struct{}

func init() {
	Register(sa0builtin.VerifiedRegistryActorCodeID, VerifiedRegistryExtractor{})
	Register(sa2builtin.VerifiedRegistryActorCodeID, VerifiedRegistryExtractor{})
	Register(sa3builtin.VerifiedRegistryActorCodeID, VerifiedRegistryExtractor{})
	Register(sa4builtin.VerifiedRegistryActorCodeID, VerifiedRegistryExtractor{})
	Register(sa5builtin.VerifiedRegistryActorCodeID, VerifiedRegistryExtractor{})
}

type VerifiedRegistryExtractionContext struct {
	PrevState, CurrState verifreg.State
	PrevTs, CurrTs       *types.TipSet

	Store adt.Store
}

func (v *VerifiedRegistryExtractionContext) HasPreviousState() bool {
	return !(v.CurrTs.Height() == 1 || v.PrevState == v.CurrState)
}

func NewVerifiedRegistryExtractorContext(ctx context.Context, a ActorInfo, node ActorStateAPI) (*VerifiedRegistryExtractionContext, error) {
	curState, err := verifreg.Load(node.Store(), &a.Actor)
	if err != nil {
		return nil, xerrors.Errorf("loading current verified registry state: %w", err)
	}

	prevState := curState
	if a.Epoch != 0 {
		prevActor, err := node.StateGetActor(ctx, a.Address, a.ParentTipSet.Key())
		if err != nil {
			// if the actor exists in the current state and not in the parent state then the
			// actor was created in the current state.
			if err == types.ErrActorNotFound {
				return &VerifiedRegistryExtractionContext{
					PrevState: prevState,
					CurrState: curState,
					PrevTs:    a.ParentTipSet,
					CurrTs:    a.TipSet,
					Store:     node.Store(),
				}, nil
			}
			return nil, xerrors.Errorf("loading previous verified registry actor at tipset %s epoch %d: %w", a.ParentTipSet.Key(), a.Epoch, err)
		}

		prevState, err = verifreg.Load(node.Store(), prevActor)
		if err != nil {
			return nil, xerrors.Errorf("loading previous verified registry state: %w", err)
		}
	}
	return &VerifiedRegistryExtractionContext{
		PrevState: prevState,
		CurrState: curState,
		PrevTs:    a.ParentTipSet,
		CurrTs:    a.TipSet,
		Store:     node.Store(),
	}, nil
}

func (VerifiedRegistryExtractor) Extract(ctx context.Context, a ActorInfo, node ActorStateAPI) (model.Persistable, error) {
	ctx, span := global.Tracer("").Start(ctx, "VerifiedRegistryExtractor")
	defer span.End()

	stop := metrics.Timer(ctx, metrics.ProcessingDuration)
	defer stop()

	ec, err := NewVerifiedRegistryExtractorContext(ctx, a, node)
	if err != nil {
		return nil, err
	}

	verifiers, verifierEvents, err := ExtractVerifiers(ctx, ec)
	if err != nil {
		return nil, err
	}

	clients, clientsEvents, err := ExtractVerifiedClients(ctx, ec)
	if err != nil {
		return nil, err
	}

	return model.PersistableList{
		verifiers,
		verifierEvents,
		clients,
		clientsEvents,
	}, nil
}

func ExtractVerifiers(ctx context.Context, ec *VerifiedRegistryExtractionContext) (verifregmodel.VerifiedRegistryVerifiersList, verifregmodel.VerifiedRegistryVerifierEventList, error) {
	var verifiers verifregmodel.VerifiedRegistryVerifiersList
	var events verifregmodel.VerifiedRegistryVerifierEventList
	// if this is the genesis state extract whatever state it has, there is noting to diff against
	if !ec.HasPreviousState() {
		if err := ec.CurrState.ForEachVerifier(func(addr address.Address, dcap abi.StoragePower) error {
			verifiers = append(verifiers, &verifregmodel.VerifiedRegistryVerifier{
				Height:    int64(ec.CurrTs.Height()),
				StateRoot: ec.CurrTs.ParentState().String(),
				Address:   addr.String(),
				DataCap:   dcap.String(),
			})
			events = append(events, &verifregmodel.VerifiedRegistryVerifierEvent{
				Height:    int64(ec.CurrTs.Height()),
				StateRoot: ec.CurrTs.ParentState().String(),
				Event:     verifregmodel.Added,
				Address:   addr.String(),
			})
			return nil
		}); err != nil {
			return nil, nil, err
		}
		return verifiers, events, nil
	}

	changes, err := verifreg.DiffVerifiers(ctx, ec.Store, ec.PrevState, ec.CurrState)
	if err != nil {
		return nil, nil, xerrors.Errorf("diffing verifier registry verifiers: %w", err)
	}

	// a new verifier was added
	for _, change := range changes.Added {
		verifiers = append(verifiers, &verifregmodel.VerifiedRegistryVerifier{
			Height:    int64(ec.CurrTs.Height()),
			StateRoot: ec.CurrTs.ParentState().String(),
			Address:   change.Address.String(),
			DataCap:   change.DataCap.String(),
		})
		events = append(events, &verifregmodel.VerifiedRegistryVerifierEvent{
			Height:    int64(ec.CurrTs.Height()),
			StateRoot: ec.CurrTs.ParentState().String(),
			Event:     verifregmodel.Added,
			Address:   change.Address.String(),
		})
	}
	// a verifier was removed
	for _, change := range changes.Removed {
		events = append(events, &verifregmodel.VerifiedRegistryVerifierEvent{
			Height:    int64(ec.CurrTs.Height()),
			StateRoot: ec.CurrTs.ParentState().String(),
			Event:     verifregmodel.Removed,
			Address:   change.Address.String(),
		})
	}
	// an existing verifier's DataCap changed
	for _, change := range changes.Modified {
		verifiers = append(verifiers, &verifregmodel.VerifiedRegistryVerifier{
			Height:    int64(ec.CurrTs.Height()),
			StateRoot: ec.CurrTs.ParentState().String(),
			Address:   change.After.Address.String(),
			DataCap:   change.After.DataCap.String(),
		})
	}
	return verifiers, events, nil
}

func ExtractVerifiedClients(ctx context.Context, ec *VerifiedRegistryExtractionContext) (verifregmodel.VerifiedRegistryVerifiedClientsList, verifregmodel.VerifiedRegistryClientEventList, error) {
	var clients verifregmodel.VerifiedRegistryVerifiedClientsList
	var events verifregmodel.VerifiedRegistryClientEventList
	// if this is the genesis state extract whatever state it has, there is noting to diff against
	if !ec.HasPreviousState() {
		if err := ec.CurrState.ForEachClient(func(addr address.Address, dcap abi.StoragePower) error {
			clients = append(clients, &verifregmodel.VerifiedRegistryVerifiedClient{
				Height:    int64(ec.CurrTs.Height()),
				StateRoot: ec.CurrTs.ParentState().String(),
				Address:   addr.String(),
				DataCap:   dcap.String(),
			})
			events = append(events, &verifregmodel.VerifiedRegistryClientEvent{
				Height:    int64(ec.CurrTs.Height()),
				StateRoot: ec.CurrTs.ParentState().String(),
				Event:     verifregmodel.Added,
				Address:   addr.String(),
			})
			return nil
		}); err != nil {
			return nil, nil, err
		}
		return clients, events, nil
	}

	changes, err := verifreg.DiffVerifiedClients(ctx, ec.Store, ec.PrevState, ec.CurrState)
	if err != nil {
		return nil, nil, xerrors.Errorf("diffing verifier registry clients: %w", err)
	}

	for _, change := range changes.Added {
		clients = append(clients, &verifregmodel.VerifiedRegistryVerifiedClient{
			Height:    int64(ec.CurrTs.Height()),
			StateRoot: ec.CurrTs.ParentState().String(),
			Address:   change.Address.String(),
			DataCap:   change.DataCap.String(),
		})
		events = append(events, &verifregmodel.VerifiedRegistryClientEvent{
			Height:    int64(ec.CurrTs.Height()),
			StateRoot: ec.CurrTs.ParentState().String(),
			Event:     verifregmodel.Added,
			Address:   change.Address.String(),
		})
	}
	for _, change := range changes.Modified {
		clients = append(clients, &verifregmodel.VerifiedRegistryVerifiedClient{
			Height:    int64(ec.CurrTs.Height()),
			StateRoot: ec.CurrTs.ParentState().String(),
			Address:   change.After.Address.String(),
			DataCap:   change.After.DataCap.String(),
		})
	}
	for _, change := range changes.Removed {
		events = append(events, &verifregmodel.VerifiedRegistryClientEvent{
			Height:    int64(ec.CurrTs.Height()),
			StateRoot: ec.CurrTs.ParentState().String(),
			Event:     verifregmodel.Removed,
			Address:   change.Address.String(),
		})
	}
	return clients, events, nil
}
