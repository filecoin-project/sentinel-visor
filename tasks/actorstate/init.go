package actorstate

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"golang.org/x/xerrors"

	init_ "github.com/filecoin-project/sentinel-visor/chain/actors/builtin/init"
	"github.com/filecoin-project/sentinel-visor/model"
	initmodel "github.com/filecoin-project/sentinel-visor/model/actors/init_"
)

// was services/processor/tasks/init/init_actor.go

// InitExtractor extracts init actor state
type InitExtractor struct{}

func init() {
	for _, c := range init_.AllCodes() {
		Register(c, InitExtractor{})
	}
}

func (InitExtractor) Extract(ctx context.Context, a ActorInfo, node ActorStateAPI) (model.Persistable, error) {
	// genesis state.
	if a.Epoch == 1 {
		initActorState, err := init_.Load(node.Store(), &a.Actor)
		if err != nil {
			return nil, err
		}

		out := initmodel.IdAddressList{}
		for _, builtinAddress := range []address.Address{builtin.SystemActorAddr, builtin.InitActorAddr,
			builtin.RewardActorAddr, builtin.CronActorAddr, builtin.StoragePowerActorAddr, builtin.StorageMarketActorAddr,
			builtin.VerifiedRegistryActorAddr, builtin.BurntFundsActorAddr} {
			out = append(out, &initmodel.IdAddress{
				Height:    0,
				ID:        builtinAddress.String(),
				Address:   builtinAddress.String(),
				StateRoot: a.ParentTipSet.ParentState().String(),
			})
		}
		if err := initActorState.ForEachActor(func(id abi.ActorID, addr address.Address) error {
			idAddr, err := address.NewIDAddress(uint64(id))
			if err != nil {
				return err
			}
			out = append(out, &initmodel.IdAddress{
				Height:    int64(a.Epoch),
				ID:        idAddr.String(),
				Address:   addr.String(),
				StateRoot: a.ParentStateRoot.String(),
			})
			return nil
		}); err != nil {
			return nil, err
		}
		return out, nil
	}
	prevActor, err := node.StateGetActor(ctx, a.Address, a.ParentTipSet.Key())
	if err != nil {
		return nil, xerrors.Errorf("loading previous init actor: %w", err)
	}

	prevState, err := init_.Load(node.Store(), prevActor)
	if err != nil {
		return nil, xerrors.Errorf("loading previous init actor state: %w", err)
	}

	curState, err := init_.Load(node.Store(), &a.Actor)
	if err != nil {
		return nil, xerrors.Errorf("loading current init actor state: %w", err)
	}

	addressChanges, err := init_.DiffAddressMap(ctx, node.Store(), prevState, curState)
	if err != nil {
		return nil, xerrors.Errorf("diffing init actor state: %w", err)
	}

	out := make(initmodel.IdAddressList, 0, len(addressChanges.Added)+len(addressChanges.Modified))
	for _, newAddr := range addressChanges.Added {
		out = append(out, &initmodel.IdAddress{
			Height:    int64(a.Epoch),
			StateRoot: a.ParentStateRoot.String(),
			ID:        newAddr.ID.String(),
			Address:   newAddr.PK.String(),
		})
	}
	for _, modAddr := range addressChanges.Modified {
		out = append(out, &initmodel.IdAddress{
			Height:    int64(a.Epoch),
			StateRoot: a.ParentStateRoot.String(),
			ID:        modAddr.To.ID.String(),
			Address:   modAddr.To.PK.String(),
		})
	}

	return out, nil
}
