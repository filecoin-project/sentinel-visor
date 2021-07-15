package actorstate

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/sentinel-visor/chain/actors/adt"
	"go.opentelemetry.io/otel/api/global"
	"golang.org/x/xerrors"

	market "github.com/filecoin-project/sentinel-visor/chain/actors/builtin/market"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
	marketmodel "github.com/filecoin-project/sentinel-visor/model/actors/market"
)

// was services/processor/tasks/market/market.go

// StorageMarketExtractor extracts market actor state
type StorageMarketExtractor struct{}

func init() {
	for _, c := range market.AllCodes() {
		Register(c, StorageMarketExtractor{})
	}
}

type MarketStateExtractionContext struct {
	PrevState market.State
	PrevTs    *types.TipSet

	CurrActor *types.Actor
	CurrState market.State
	CurrTs    *types.TipSet

	Store adt.Store
}

func NewMarketStateExtractionContext(ctx context.Context, a ActorInfo, node ActorStateAPI) (*MarketStateExtractionContext, error) {
	curState, err := market.Load(node.Store(), &a.Actor)
	if err != nil {
		return nil, xerrors.Errorf("loading current market state: %w", err)
	}

	prevTipset := a.TipSet
	prevState := curState
	if a.Epoch != 0 {
		prevTipset = a.ParentTipSet

		prevActor, err := node.StateGetActor(ctx, a.Address, a.ParentTipSet.Key())
		if err != nil {
			return nil, xerrors.Errorf("loading previous market actor state at tipset %s epoch %d: %w", a.ParentTipSet.Key(), a.Epoch, err)
		}

		prevState, err = market.Load(node.Store(), prevActor)
		if err != nil {
			return nil, xerrors.Errorf("loading previous market actor state: %w", err)
		}

	}
	return &MarketStateExtractionContext{
		PrevState: prevState,
		PrevTs:    prevTipset,
		CurrActor: &a.Actor,
		CurrState: curState,
		CurrTs:    a.TipSet,
		Store:     node.Store(),
	}, nil
}

func (m *MarketStateExtractionContext) IsGenesis() bool {
	return m.CurrTs.Height() == 0
}

func (m StorageMarketExtractor) Extract(ctx context.Context, a ActorInfo, node ActorStateAPI) (model.Persistable, error) {
	ctx, span := global.Tracer("").Start(ctx, "StorageMarketExtractor")
	defer span.End()

	stop := metrics.Timer(ctx, metrics.ProcessingDuration)
	defer stop()

	ec, err := NewMarketStateExtractionContext(ctx, a, node)
	if err != nil {
		return nil, err
	}

	dealStateModel, err := ExtractMarketDealStates(ctx, ec)
	if err != nil {
		return nil, xerrors.Errorf("extracting market deal state changes: %w", err)
	}

	dealProposalModel, err := ExtractMarketDealProposals(ctx, ec)
	if err != nil {
		return nil, xerrors.Errorf("extracting market proposal changes: %w", err)
	}

	return &marketmodel.MarketTaskResult{
		Proposals: dealProposalModel,
		States:    dealStateModel,
	}, nil
}

func ExtractMarketDealProposals(ctx context.Context, ec *MarketStateExtractionContext) (marketmodel.MarketDealProposals, error) {
	currDealProposals, err := ec.CurrState.Proposals()
	if err != nil {
		return nil, xerrors.Errorf("loading current market deal proposals: %w:", err)
	}

	if ec.IsGenesis() {
		var out marketmodel.MarketDealProposals
		if err := currDealProposals.ForEach(func(id abi.DealID, dp market.DealProposal) error {
			out = append(out, &marketmodel.MarketDealProposal{
				Height:               int64(ec.CurrTs.Height()),
				DealID:               uint64(id),
				StateRoot:            ec.CurrTs.ParentState().String(),
				PaddedPieceSize:      uint64(dp.PieceSize),
				UnpaddedPieceSize:    uint64(dp.PieceSize.Unpadded()),
				StartEpoch:           int64(dp.StartEpoch),
				EndEpoch:             int64(dp.EndEpoch),
				ClientID:             dp.Client.String(),
				ProviderID:           dp.Provider.String(),
				ClientCollateral:     dp.ClientCollateral.String(),
				ProviderCollateral:   dp.ProviderCollateral.String(),
				StoragePricePerEpoch: dp.StoragePricePerEpoch.String(),
				PieceCID:             dp.PieceCID.String(),
				IsVerified:           dp.VerifiedDeal,
				Label:                dp.Label,
			})
			return nil
		}); err != nil {
			return nil, xerrors.Errorf("walking current deal states: %w", err)
		}
		return out, nil

	}

	changed, err := ec.CurrState.ProposalsChanged(ec.PrevState)
	if err != nil {
		return nil, xerrors.Errorf("checking for deal proposal changes: %w", err)
	}

	if !changed {
		return nil, nil
	}

	changes, err := market.DiffDealProposals(ctx, ec.Store, ec.PrevState, ec.CurrState)
	if err != nil {
		return nil, xerrors.Errorf("diffing deal states: %w", err)
	}

	out := make(marketmodel.MarketDealProposals, len(changes.Added))
	for idx, add := range changes.Added {
		out[idx] = &marketmodel.MarketDealProposal{
			Height:               int64(ec.CurrTs.Height()),
			DealID:               uint64(add.ID),
			StateRoot:            ec.CurrTs.ParentState().String(),
			PaddedPieceSize:      uint64(add.Proposal.PieceSize),
			UnpaddedPieceSize:    uint64(add.Proposal.PieceSize.Unpadded()),
			StartEpoch:           int64(add.Proposal.StartEpoch),
			EndEpoch:             int64(add.Proposal.EndEpoch),
			ClientID:             add.Proposal.Client.String(),
			ProviderID:           add.Proposal.Provider.String(),
			ClientCollateral:     add.Proposal.ClientCollateral.String(),
			ProviderCollateral:   add.Proposal.ProviderCollateral.String(),
			StoragePricePerEpoch: add.Proposal.StoragePricePerEpoch.String(),
			PieceCID:             add.Proposal.PieceCID.String(),
			IsVerified:           add.Proposal.VerifiedDeal,
			Label:                add.Proposal.Label,
		}
	}
	return out, nil
}

func ExtractMarketDealStates(ctx context.Context, ec *MarketStateExtractionContext) (marketmodel.MarketDealStates, error) {
	currDealStates, err := ec.CurrState.States()
	if err != nil {
		return nil, xerrors.Errorf("loading current market deal states: %w", err)
	}

	if ec.IsGenesis() {
		var out marketmodel.MarketDealStates
		if err := currDealStates.ForEach(func(id abi.DealID, ds market.DealState) error {
			out = append(out, &marketmodel.MarketDealState{
				Height:           int64(ec.CurrTs.Height()),
				DealID:           uint64(id),
				SectorStartEpoch: int64(ds.SectorStartEpoch),
				LastUpdateEpoch:  int64(ds.LastUpdatedEpoch),
				SlashEpoch:       int64(ds.SlashEpoch),
				StateRoot:        ec.CurrTs.ParentState().String(),
			})
			return nil
		}); err != nil {
			return nil, xerrors.Errorf("walking current deal states: %w", err)
		}
		return out, nil
	}

	changed, err := ec.CurrState.StatesChanged(ec.PrevState)
	if err != nil {
		return nil, xerrors.Errorf("checking for deal state changes: %w", err)
	}

	if !changed {
		return nil, nil
	}

	changes, err := market.DiffDealStates(ctx, ec.Store, ec.PrevState, ec.CurrState)
	if err != nil {
		return nil, xerrors.Errorf("diffing deal states: %w", err)
	}

	out := make(marketmodel.MarketDealStates, len(changes.Added)+len(changes.Modified))
	idx := 0
	for _, add := range changes.Added {
		out[idx] = &marketmodel.MarketDealState{
			Height:           int64(ec.CurrTs.Height()),
			DealID:           uint64(add.ID),
			SectorStartEpoch: int64(add.Deal.SectorStartEpoch),
			LastUpdateEpoch:  int64(add.Deal.LastUpdatedEpoch),
			SlashEpoch:       int64(add.Deal.SlashEpoch),
			StateRoot:        ec.CurrTs.ParentState().String(),
		}
		idx++
	}
	for _, mod := range changes.Modified {
		// The market actor cron updates deal's LastUpdatedEpoch on
		// every run:
		// https://github.com/filecoin-project/specs-actors/blob/0bf4d26701c5ccf295f73b0f98650f1a2dcb5d69/actors/builtin/market/market_actor.go#L577
		//
		// This is used to calculate pending paypment amounts:
		// https://github.com/filecoin-project/specs-actors/blob/0bf4d26701c5ccf295f73b0f98650f1a2dcb5d69/actors/builtin/market/market_state.go#L132-L134
		//
		// However, this is not useful information for sentinel and
		// results in a huge number of entries in the
		// market_deal_states table.  What is most useful is to know
		// when a deal was activated (first entry of the deal in the
		// table), and when a deal was slashed (SlashEpoch > 0 and possibly last entry in the table).
		//
		// Therefore, we conciously forget any "updates" that do not
		// have a positive slash epoch, as things otherwise simply
		// correspond to market actor cron ticks.
		if mod.To.SlashEpoch <= 0 {
			continue
		}
		out[idx] = &marketmodel.MarketDealState{
			Height:           int64(ec.CurrTs.Height()),
			DealID:           uint64(mod.ID),
			SectorStartEpoch: int64(mod.To.SectorStartEpoch),
			LastUpdateEpoch:  int64(mod.To.LastUpdatedEpoch),
			SlashEpoch:       int64(mod.To.SlashEpoch),
			StateRoot:        ec.CurrTs.ParentState().String(),
		}
		idx++
	}
	return out, nil
}
