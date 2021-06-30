package actorstate_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/chain/types"
	init_ "github.com/filecoin-project/sentinel-visor/chain/actors/builtin/init"
	sa0builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	sa0init "github.com/filecoin-project/specs-actors/actors/builtin/init"
	tutils "github.com/filecoin-project/specs-actors/support/testing"
	sa2builtin "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	sa2init "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	sa3builtin "github.com/filecoin-project/specs-actors/v3/actors/builtin"
	sa3init "github.com/filecoin-project/specs-actors/v3/actors/builtin/init"
	sa4builtin "github.com/filecoin-project/specs-actors/v4/actors/builtin"
	sa4init "github.com/filecoin-project/specs-actors/v4/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/v5/actors/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	initmodel "github.com/filecoin-project/sentinel-visor/model/actors/init_"
	"github.com/filecoin-project/sentinel-visor/tasks/actorstate"
)

func mapWithBuiltinAddresses() map[string]string {
	out := map[string]string{}
	// add the builtin addresses.
	for _, builtinAddress := range []address.Address{builtin.SystemActorAddr, builtin.InitActorAddr,
		builtin.RewardActorAddr, builtin.CronActorAddr, builtin.StoragePowerActorAddr, builtin.StorageMarketActorAddr,
		builtin.VerifiedRegistryActorAddr, builtin.BurntFundsActorAddr} {
		out[builtinAddress.String()] = builtinAddress.String()
	}
	return out
}

func TestInitExtractorV0(t *testing.T) {
	ctx := context.Background()

	mapi := NewMockAPI(t)

	state := mapi.mustCreateEmptyInitStateV0()
	minerAddr := tutils.NewIDAddr(t, 1234)

	t.Run("genesis init state extraction", func(t *testing.T) {
		// return map keyed on stringified id address and value stringified pk Address.
		addToInitActor := func(state *sa0init.State, numAddrs int) map[string]string {
			out := mapWithBuiltinAddresses()
			for i := 0; i < numAddrs; i++ {
				addr := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", i))
				idAddr, err := state.MapAddressToNewID(mapi.store, addr)
				require.NoError(t, err)
				out[idAddr.String()] = addr.String()
			}
			return out
		}
		// add 2 addresses in the init actor.
		addrMap := addToInitActor(state, 2)

		// persist state
		stateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		stateTs := mapi.fakeTipset(minerAddr, 1, WithHeight(0)) // genesis
		mapi.setActor(stateTs.Key(), init_.Address, &types.Actor{Code: sa0builtin.InitActorCodeID, Head: stateCid})

		info := actorstate.ActorInfo{
			Epoch:           1, // parent state is genesis
			Actor:           types.Actor{Code: sa0builtin.InitActorCodeID, Head: stateCid},
			Address:         init_.Address,
			TipSet:          stateTs,
			ParentStateRoot: stateTs.ParentState(),
			ParentTipSet:    mapi.fakeTipset(minerAddr, 2, WithHeight(1)),
		}

		ex := actorstate.InitExtractor{}
		res, err := ex.Extract(ctx, info, mapi)
		require.NoError(t, err)

		is, ok := res.(initmodel.IdAddressList)
		require.True(t, ok)
		require.NotNil(t, is)

		numAddresses := 10
		assert.Len(t, is, numAddresses)
		for i := 0; i < numAddresses; i++ {
			assert.EqualValues(t, addrMap[is[i].ID], is[i].Address)
		}
	})

	t.Run("init state extraction with new address and modified address", func(t *testing.T) {
		// setup the base state with an address in it, we will modify it in the following state.
		pkAddrToMod := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", 1))
		idAddrToMod, err := state.MapAddressToNewID(mapi.store, pkAddrToMod)
		require.NoError(t, err)

		// persist base state.
		baseStateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		baseTs := mapi.fakeTipset(minerAddr, 1)
		mapi.setActor(baseTs.Key(), init_.Address, &types.Actor{Code: sa0builtin.InitActorCodeID, Head: baseStateCid})

		// setup following state.
		// add a new address.
		pkNewAddr := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", 2))
		idNewAddr, err := state.MapAddressToNewID(mapi.store, pkNewAddr)
		require.NoError(t, err)

		// modify an existing address
		idAddrAfterMod, err := state.MapAddressToNewID(mapi.store, pkAddrToMod)
		require.NoError(t, err)
		// sanity check the address receieved a new ID address
		require.NotEqual(t, idAddrToMod, idAddrAfterMod)

		// persist new state.
		stateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		stateTs := mapi.fakeTipset(minerAddr, 1)
		mapi.setActor(stateTs.Key(), init_.Address, &types.Actor{Code: sa0builtin.InitActorCodeID, Head: stateCid})

		info := actorstate.ActorInfo{
			Epoch:           2,
			Actor:           types.Actor{Code: sa0builtin.InitActorCodeID, Head: stateCid},
			Address:         init_.Address,
			ParentTipSet:    baseTs,
			TipSet:          stateTs,
			ParentStateRoot: baseStateCid,
		}

		ex := actorstate.InitExtractor{}
		res, err := ex.Extract(ctx, info, mapi)
		require.NoError(t, err)

		is, ok := res.(initmodel.IdAddressList)
		require.True(t, ok)
		require.NotNil(t, is)

		assert.Len(t, is, 2)
		assert.EqualValues(t, idNewAddr.String(), is[0].ID)
		assert.EqualValues(t, pkNewAddr.String(), is[0].Address)
		assert.EqualValues(t, 2, is[0].Height)
		assert.EqualValues(t, idAddrAfterMod.String(), is[1].ID)
		assert.EqualValues(t, pkAddrToMod.String(), is[1].Address)
		assert.EqualValues(t, 2, is[1].Height)
	})
}

func TestInitExtractorV2(t *testing.T) {
	ctx := context.Background()

	mapi := NewMockAPI(t)

	state := mapi.mustCreateEmptyInitStateV2()
	minerAddr := tutils.NewIDAddr(t, 1234)

	t.Run("genesis init state extraction", func(t *testing.T) {
		// return map keyed on stringified id address and value stringified pk Address.
		addToInitActor := func(state *sa2init.State, numAddrs int) map[string]string {
			out := mapWithBuiltinAddresses()
			for i := 0; i < numAddrs; i++ {
				addr := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", i))
				idAddr, err := state.MapAddressToNewID(mapi.store, addr)
				require.NoError(t, err)
				out[idAddr.String()] = addr.String()
			}
			return out
		}
		// add 2 addresses in the init actor.
		addrMap := addToInitActor(state, 2)

		// persist state
		stateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		stateTs := mapi.fakeTipset(minerAddr, 1, WithHeight(0)) // genesis
		mapi.setActor(stateTs.Key(), init_.Address, &types.Actor{Code: sa2builtin.InitActorCodeID, Head: stateCid})

		info := actorstate.ActorInfo{
			Epoch:           1, // genesis
			Actor:           types.Actor{Code: sa2builtin.InitActorCodeID, Head: stateCid},
			Address:         init_.Address,
			TipSet:          stateTs,
			ParentStateRoot: stateTs.ParentState(),
			ParentTipSet:    mapi.fakeTipset(minerAddr, 2, WithHeight(1)),
		}

		ex := actorstate.InitExtractor{}
		res, err := ex.Extract(ctx, info, mapi)
		require.NoError(t, err)

		is, ok := res.(initmodel.IdAddressList)
		require.True(t, ok)
		require.NotNil(t, is)

		numAddresses := 10
		assert.Len(t, is, numAddresses)
		for i := 0; i < numAddresses; i++ {
			assert.EqualValues(t, addrMap[is[i].ID], is[i].Address)
		}
	})

	t.Run("init state extraction with new address and modified address", func(t *testing.T) {
		// setup the base state with an address in it, we will modify it in the following state.
		pkAddrToMod := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", 1))
		idAddrToMod, err := state.MapAddressToNewID(mapi.store, pkAddrToMod)
		require.NoError(t, err)

		// persist base state.
		baseStateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		baseTs := mapi.fakeTipset(minerAddr, 1)
		mapi.setActor(baseTs.Key(), init_.Address, &types.Actor{Code: sa2builtin.InitActorCodeID, Head: baseStateCid})

		// setup following state.
		// add a new address.
		pkNewAddr := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", 2))
		idNewAddr, err := state.MapAddressToNewID(mapi.store, pkNewAddr)
		require.NoError(t, err)

		// modify an existing address
		idAddrAfterMod, err := state.MapAddressToNewID(mapi.store, pkAddrToMod)
		require.NoError(t, err)
		// sanity check the address receieved a new ID address
		require.NotEqual(t, idAddrToMod, idAddrAfterMod)

		// persist new state.
		stateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		stateTs := mapi.fakeTipset(minerAddr, 1)
		mapi.setActor(stateTs.Key(), init_.Address, &types.Actor{Code: sa2builtin.InitActorCodeID, Head: stateCid})

		info := actorstate.ActorInfo{
			Epoch:           2,
			Actor:           types.Actor{Code: sa2builtin.InitActorCodeID, Head: stateCid},
			Address:         init_.Address,
			ParentTipSet:    baseTs,
			TipSet:          stateTs,
			ParentStateRoot: baseStateCid,
		}

		ex := actorstate.InitExtractor{}
		res, err := ex.Extract(ctx, info, mapi)
		require.NoError(t, err)

		is, ok := res.(initmodel.IdAddressList)
		require.True(t, ok)
		require.NotNil(t, is)

		assert.Len(t, is, 2)
		assert.EqualValues(t, idNewAddr.String(), is[0].ID)
		assert.EqualValues(t, pkNewAddr.String(), is[0].Address)
		assert.EqualValues(t, 2, is[0].Height)
		assert.EqualValues(t, idAddrAfterMod.String(), is[1].ID)
		assert.EqualValues(t, pkAddrToMod.String(), is[1].Address)
		assert.EqualValues(t, 2, is[1].Height)
	})
}

func TestInitExtractorV3(t *testing.T) {
	ctx := context.Background()

	mapi := NewMockAPI(t)

	state := mapi.mustCreateEmptyInitStateV3()
	minerAddr := tutils.NewIDAddr(t, 1234)

	t.Run("genesis init state extraction", func(t *testing.T) {
		// return map keyed on stringified id address and value stringified pk Address.
		addToInitActor := func(state *sa3init.State, numAddrs int) map[string]string {
			out := mapWithBuiltinAddresses()
			for i := 0; i < numAddrs; i++ {
				addr := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", i))
				idAddr, err := state.MapAddressToNewID(mapi.store, addr)
				require.NoError(t, err)
				out[idAddr.String()] = addr.String()
			}
			return out
		}
		// add 2 addresses in the init actor.
		addrMap := addToInitActor(state, 2)

		// persist state
		stateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		stateTs := mapi.fakeTipset(minerAddr, 1, WithHeight(0)) // genesis
		mapi.setActor(stateTs.Key(), init_.Address, &types.Actor{Code: sa3builtin.InitActorCodeID, Head: stateCid})

		info := actorstate.ActorInfo{
			Epoch:           1, // genesis
			Actor:           types.Actor{Code: sa3builtin.InitActorCodeID, Head: stateCid},
			Address:         init_.Address,
			TipSet:          stateTs,
			ParentStateRoot: stateTs.ParentState(),
			ParentTipSet:    mapi.fakeTipset(minerAddr, 2, WithHeight(1)),
		}

		ex := actorstate.InitExtractor{}
		res, err := ex.Extract(ctx, info, mapi)
		require.NoError(t, err)

		is, ok := res.(initmodel.IdAddressList)
		require.True(t, ok)
		require.NotNil(t, is)

		numAddresses := 10
		assert.Len(t, is, numAddresses)
		for i := 0; i < numAddresses; i++ {
			assert.EqualValues(t, addrMap[is[i].ID], is[i].Address)
		}
	})

	t.Run("init state extraction with new address and modified address", func(t *testing.T) {
		// setup the base state with an address in it, we will modify it in the following state.
		pkAddrToMod := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", 1))
		idAddrToMod, err := state.MapAddressToNewID(mapi.store, pkAddrToMod)
		require.NoError(t, err)

		// persist base state.
		baseStateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		baseTs := mapi.fakeTipset(minerAddr, 1)
		mapi.setActor(baseTs.Key(), init_.Address, &types.Actor{Code: sa3builtin.InitActorCodeID, Head: baseStateCid})

		// setup following state.
		// add a new address.
		pkNewAddr := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", 2))
		idNewAddr, err := state.MapAddressToNewID(mapi.store, pkNewAddr)
		require.NoError(t, err)

		// modify an existing address
		idAddrAfterMod, err := state.MapAddressToNewID(mapi.store, pkAddrToMod)
		require.NoError(t, err)
		// sanity check the address receieved a new ID address
		require.NotEqual(t, idAddrToMod, idAddrAfterMod)

		// persist new state.
		stateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		stateTs := mapi.fakeTipset(minerAddr, 1)
		mapi.setActor(stateTs.Key(), init_.Address, &types.Actor{Code: sa3builtin.InitActorCodeID, Head: stateCid})

		info := actorstate.ActorInfo{
			Epoch:           2,
			Actor:           types.Actor{Code: sa3builtin.InitActorCodeID, Head: stateCid},
			Address:         init_.Address,
			ParentTipSet:    baseTs,
			TipSet:          stateTs,
			ParentStateRoot: baseStateCid,
		}

		ex := actorstate.InitExtractor{}
		res, err := ex.Extract(ctx, info, mapi)
		require.NoError(t, err)

		is, ok := res.(initmodel.IdAddressList)
		require.True(t, ok)
		require.NotNil(t, is)

		assert.Len(t, is, 2)
		assert.EqualValues(t, idNewAddr.String(), is[0].ID)
		assert.EqualValues(t, pkNewAddr.String(), is[0].Address)
		assert.EqualValues(t, 2, is[0].Height)
		assert.EqualValues(t, idAddrAfterMod.String(), is[1].ID)
		assert.EqualValues(t, pkAddrToMod.String(), is[1].Address)
		assert.EqualValues(t, 2, is[1].Height)
	})
}

func TestInitExtractorV4(t *testing.T) {
	ctx := context.Background()

	mapi := NewMockAPI(t)

	state := mapi.mustCreateEmptyInitStateV4()
	minerAddr := tutils.NewIDAddr(t, 1234)

	t.Run("genesis init state extraction", func(t *testing.T) {
		// return map keyed on stringified id address and value stringified pk Address.
		addToInitActor := func(state *sa4init.State, numAddrs int) map[string]string {
			out := mapWithBuiltinAddresses()
			for i := 0; i < numAddrs; i++ {
				addr := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", i))
				idAddr, err := state.MapAddressToNewID(mapi.store, addr)
				require.NoError(t, err)
				out[idAddr.String()] = addr.String()
			}
			return out
		}
		// add 2 addresses in the init actor.
		addrMap := addToInitActor(state, 2)

		// persist state
		stateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		stateTs := mapi.fakeTipset(minerAddr, 1, WithHeight(0)) // genesis
		mapi.setActor(stateTs.Key(), init_.Address, &types.Actor{Code: sa4builtin.InitActorCodeID, Head: stateCid})

		info := actorstate.ActorInfo{
			Epoch:           1, // genesis
			Actor:           types.Actor{Code: sa4builtin.InitActorCodeID, Head: stateCid},
			Address:         init_.Address,
			TipSet:          stateTs,
			ParentStateRoot: stateTs.ParentState(),
			ParentTipSet:    mapi.fakeTipset(minerAddr, 2, WithHeight(1)),
		}

		ex := actorstate.InitExtractor{}
		res, err := ex.Extract(ctx, info, mapi)
		require.NoError(t, err)

		is, ok := res.(initmodel.IdAddressList)
		require.True(t, ok)
		require.NotNil(t, is)

		numAddresses := 10
		assert.Len(t, is, numAddresses)
		for i := 0; i < numAddresses; i++ {
			assert.EqualValues(t, addrMap[is[i].ID], is[i].Address)
		}
	})

	t.Run("init state extraction with new address and modified address", func(t *testing.T) {
		// setup the base state with an address in it, we will modify it in the following state.
		pkAddrToMod := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", 1))
		idAddrToMod, err := state.MapAddressToNewID(mapi.store, pkAddrToMod)
		require.NoError(t, err)

		// persist base state.
		baseStateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		baseTs := mapi.fakeTipset(minerAddr, 1)
		mapi.setActor(baseTs.Key(), init_.Address, &types.Actor{Code: sa4builtin.InitActorCodeID, Head: baseStateCid})

		// setup following state.
		// add a new address.
		pkNewAddr := tutils.NewSECP256K1Addr(t, fmt.Sprintf("%d", 2))
		idNewAddr, err := state.MapAddressToNewID(mapi.store, pkNewAddr)
		require.NoError(t, err)

		// modify an existing address
		idAddrAfterMod, err := state.MapAddressToNewID(mapi.store, pkAddrToMod)
		require.NoError(t, err)
		// sanity check the address receieved a new ID address
		require.NotEqual(t, idAddrToMod, idAddrAfterMod)

		// persist new state.
		stateCid, err := mapi.Store().Put(ctx, state)
		require.NoError(t, err)
		stateTs := mapi.fakeTipset(minerAddr, 1)
		mapi.setActor(stateTs.Key(), init_.Address, &types.Actor{Code: sa4builtin.InitActorCodeID, Head: stateCid})

		info := actorstate.ActorInfo{
			Epoch:           2,
			Actor:           types.Actor{Code: sa4builtin.InitActorCodeID, Head: stateCid},
			Address:         init_.Address,
			ParentTipSet:    baseTs,
			TipSet:          stateTs,
			ParentStateRoot: baseStateCid,
		}

		ex := actorstate.InitExtractor{}
		res, err := ex.Extract(ctx, info, mapi)
		require.NoError(t, err)

		is, ok := res.(initmodel.IdAddressList)
		require.True(t, ok)
		require.NotNil(t, is)

		assert.Len(t, is, 2)
		assert.EqualValues(t, idNewAddr.String(), is[0].ID)
		assert.EqualValues(t, pkNewAddr.String(), is[0].Address)
		assert.EqualValues(t, 2, is[0].Height)
		assert.EqualValues(t, idAddrAfterMod.String(), is[1].ID)
		assert.EqualValues(t, pkAddrToMod.String(), is[1].Address)
		assert.EqualValues(t, 2, is[1].Height)
	})
}
