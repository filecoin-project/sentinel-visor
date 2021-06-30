package actorstate

import (
	"bytes"
	"context"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/sentinel-visor/chain/actors/adt"
	maddr "github.com/multiformats/go-multiaddr"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/label"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/chain/types"
	miner "github.com/filecoin-project/sentinel-visor/chain/actors/builtin/miner"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
	minermodel "github.com/filecoin-project/sentinel-visor/model/actors/miner"
)

// was services/processor/tasks/miner/miner.go

// StorageMinerExtractor extracts miner actor state
type StorageMinerExtractor struct{}

func init() {
	for _, c := range miner.AllCodes() {
		Register(c, StorageMinerExtractor{})
	}
}

func ModelExtractors(m model.Persistable) (func(ctx context.Context, ectx *MinerStateExtractionContext) (model.Persistable, error), error) {
	switch m.(type) {
	case *minermodel.MinerInfo:
		return ExtractMinerInfo, nil
	case *minermodel.MinerFeeDebt:
		return ExtractMinerFeeDebt, nil
	case *minermodel.MinerSectorInfo:
		return ExtractMinerSectorInfo, nil
	case *minermodel.MinerLockedFund:
		return ExtractMinerLockedFunds, nil
	case *minermodel.MinerSectorDeal:
		return ExtractMinerSectorDeals, nil
	case *minermodel.MinerSectorEvent:
		return ExtractMinerSectorEvents, nil
	case *minermodel.MinerPreCommitInfo:
		return ExtractMinerPreCommitInfo, nil
	case *minermodel.MinerCurrentDeadlineInfo:
		return ExtractMinerCurrentDeadlineInfo, nil
	case *minermodel.MinerSectorPost:
		return ExtractMinerPoSts, nil
	default:
		return nil, xerrors.Errorf("unrecognized model: %T", m)
	}
}

func (m StorageMinerExtractor) Extract(ctx context.Context, a ActorInfo, node ActorStateAPI) (model.Persistable, error) {
	ctx, span := global.Tracer("").Start(ctx, "StorageMinerExtractor")
	if span.IsRecording() {
		span.SetAttributes(label.String("actor", a.Address.String()))
	}
	defer span.End()

	stop := metrics.Timer(ctx, metrics.ProcessingDuration)
	defer stop()

	ec, err := NewMinerStateExtractionContext(ctx, a, node)
	if err != nil {
		return nil, xerrors.Errorf("creating miner state extraction context: %w", err)
	}

	var out model.PersistableList
	for _, m := range a.Models {
		// look up the extraction method required to produce this model.
		extF, err := ModelExtractors(m)
		if err != nil {
			return nil, err
		}
		// execute and collect data.
		data, err := extF(ctx, ec)
		if err != nil {
			return nil, err
		}
		out = append(out, data)
	}
	return out, nil
}

func NewMinerStateExtractionContext(ctx context.Context, a ActorInfo, node ActorStateAPI) (*MinerStateExtractionContext, error) {
	ctx, span := global.Tracer("").Start(ctx, "NewMinerExtractionContext")
	defer span.End()

	curState, err := miner.Load(node.Store(), &a.Actor)
	if err != nil {
		return nil, xerrors.Errorf("loading current miner state: %w", err)
	}

	prevTipset := a.TipSet
	prevState := curState
	if a.Epoch != 1 {
		prevTipset = a.ParentTipSet

		prevActor, err := node.StateGetActor(ctx, a.Address, a.ParentTipSet.Key())
		if err != nil {
			// if the actor exists in the current state and not in the parent state then the
			// actor was created in the current state.
			if err == types.ErrActorNotFound {
				return &MinerStateExtractionContext{
					PrevState: prevState,
					PrevTs:    prevTipset,
					CurrActor: &a.Actor,
					CurrState: curState,
					CurrTs:    a.TipSet,
				}, nil
			}
			return nil, xerrors.Errorf("loading previous miner %s at tipset %s epoch %d: %w", a.Address, a.ParentTipSet.Key(), a.Epoch, err)
		}

		prevState, err = miner.Load(node.Store(), prevActor)
		if err != nil {
			return nil, xerrors.Errorf("loading previous miner actor state: %w", err)
		}
	}

	return &MinerStateExtractionContext{
		PrevState: prevState,
		PrevTs:    prevTipset,
		CurrActor: &a.Actor,
		CurrState: curState,
		CurrTs:    a.TipSet,
		Address:   a.Address,
		Store:     node.Store(),
		API:       node,
		cache:     NewDiffCache(),
	}, nil
}

type MinerStateExtractionContext struct {
	PrevState miner.State
	PrevTs    *types.TipSet

	CurrState miner.State
	CurrTs    *types.TipSet

	CurrActor *types.Actor
	Address   address.Address

	Store adt.Store
	API   ActorStateAPI

	cache *diffCache
}

func NewDiffCache() *diffCache {
	return &diffCache{cache: make(map[diffType]interface{})}
}

type diffCache struct {
	cacheMu sync.Mutex
	cache   map[diffType]interface{}
}

type diffType string

const (
	PreCommitDiff diffType = "PRECOMMIT"
	SectorDiff    diffType = "SECTOR"
)

func (d *diffCache) Put(diff diffType, result interface{}) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	d.cache[diff] = result
}

func (d *diffCache) Get(diffType diffType) (interface{}, bool) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	result, found := d.cache[diffType]
	return result, found
}

func (m *MinerStateExtractionContext) HasPreviousState() bool {
	return !(m.CurrTs.Height() == 1 || m.PrevState == m.CurrState)
}

func ExtractMinerInfo(ctx context.Context, ec *MinerStateExtractionContext) (model.Persistable, error) {
	_, span := global.Tracer("").Start(ctx, "ExtractMinerInfo")
	defer span.End()
	if !ec.HasPreviousState() {
		// means this miner was created in this tipset or genesis special case
	} else if changed, err := ec.CurrState.MinerInfoChanged(ec.PrevState); err != nil {
		return nil, err
	} else if !changed {
		return nil, nil
	}
	// miner info has changed.

	newInfo, err := ec.CurrState.Info()
	if err != nil {
		return nil, err
	}

	var newWorker string
	if newInfo.NewWorker != address.Undef {
		newWorker = newInfo.NewWorker.String()
	}

	var newCtrlAddresses []string
	for _, addr := range newInfo.ControlAddresses {
		newCtrlAddresses = append(newCtrlAddresses, addr.String())
	}

	// best effort to decode, we have no control over what miners put in this field, its just bytes.
	var newMultiAddrs []string
	for _, addr := range newInfo.Multiaddrs {
		newMaddr, err := maddr.NewMultiaddrBytes(addr)
		if err == nil {
			newMultiAddrs = append(newMultiAddrs, newMaddr.String())
		} else {
			log.Debugw("failed to decode miner multiaddr", "miner", ec.Address, "multiaddress", addr, "error", err)
		}
	}
	mi := &minermodel.MinerInfo{
		Height:                  int64(ec.CurrTs.Height()),
		MinerID:                 ec.Address.String(),
		StateRoot:               ec.CurrTs.ParentState().String(),
		OwnerID:                 newInfo.Owner.String(),
		WorkerID:                newInfo.Worker.String(),
		NewWorker:               newWorker,
		WorkerChangeEpoch:       int64(newInfo.WorkerChangeEpoch),
		ConsensusFaultedElapsed: int64(newInfo.ConsensusFaultElapsed),
		ControlAddresses:        newCtrlAddresses,
		MultiAddresses:          newMultiAddrs,
		SectorSize:              uint64(newInfo.SectorSize),
	}

	if newInfo.PeerId != nil {
		mi.PeerID = newInfo.PeerId.String()
	}

	return mi, nil
}

func ExtractMinerLockedFunds(ctx context.Context, ec *MinerStateExtractionContext) (model.Persistable, error) {
	_, span := global.Tracer("").Start(ctx, "ExtractMinerLockedFunds")
	defer span.End()
	currLocked, err := ec.CurrState.LockedFunds()
	if err != nil {
		return nil, xerrors.Errorf("loading current miner locked funds: %w", err)
	}
	if ec.HasPreviousState() {
		prevLocked, err := ec.PrevState.LockedFunds()
		if err != nil {
			return nil, xerrors.Errorf("loading previous miner locked funds: %w", err)
		}
		if prevLocked == currLocked {
			return nil, nil
		}
	}
	// funds changed

	return &minermodel.MinerLockedFund{
		Height:            int64(ec.CurrTs.Height()),
		MinerID:           ec.Address.String(),
		StateRoot:         ec.CurrTs.ParentState().String(),
		LockedFunds:       currLocked.VestingFunds.String(),
		InitialPledge:     currLocked.InitialPledgeRequirement.String(),
		PreCommitDeposits: currLocked.PreCommitDeposits.String(),
	}, nil
}

func ExtractMinerFeeDebt(ctx context.Context, ec *MinerStateExtractionContext) (model.Persistable, error) {
	_, span := global.Tracer("").Start(ctx, "ExtractMinerFeeDebt")
	defer span.End()
	currDebt, err := ec.CurrState.FeeDebt()
	if err != nil {
		return nil, xerrors.Errorf("loading current miner fee debt: %w", err)
	}

	if ec.HasPreviousState() {
		prevDebt, err := ec.PrevState.FeeDebt()
		if err != nil {
			return nil, xerrors.Errorf("loading previous miner fee debt: %w", err)
		}
		if prevDebt == currDebt {
			return nil, nil
		}
	}
	// debt changed

	return &minermodel.MinerFeeDebt{
		Height:    int64(ec.CurrTs.Height()),
		MinerID:   ec.Address.String(),
		StateRoot: ec.CurrTs.ParentState().String(),
		FeeDebt:   currDebt.String(),
	}, nil
}

func ExtractMinerCurrentDeadlineInfo(ctx context.Context, ec *MinerStateExtractionContext) (model.Persistable, error) {
	_, span := global.Tracer("").Start(ctx, "ExtractMinerDeadlineInfo")
	defer span.End()
	currDeadlineInfo, err := ec.CurrState.DeadlineInfo(ec.CurrTs.Height())
	if err != nil {
		return nil, err
	}

	if ec.HasPreviousState() {
		prevDeadlineInfo, err := ec.PrevState.DeadlineInfo(ec.CurrTs.Height())
		if err != nil {
			return nil, err
		}
		if prevDeadlineInfo == currDeadlineInfo {
			return nil, nil
		}
	}

	return &minermodel.MinerCurrentDeadlineInfo{
		Height:        int64(ec.CurrTs.Height()),
		MinerID:       ec.Address.String(),
		StateRoot:     ec.CurrTs.ParentState().String(),
		DeadlineIndex: currDeadlineInfo.Index,
		PeriodStart:   int64(currDeadlineInfo.PeriodStart),
		Open:          int64(currDeadlineInfo.Open),
		Close:         int64(currDeadlineInfo.Close),
		Challenge:     int64(currDeadlineInfo.Challenge),
		FaultCutoff:   int64(currDeadlineInfo.FaultCutoff),
	}, nil
}

func getPreCommitDiff(ctx context.Context, ec *MinerStateExtractionContext) (*miner.PreCommitChanges, error) {
	preCommitChanges := new(miner.PreCommitChanges)
	result, found := ec.cache.Get(PreCommitDiff)
	if !found {
		var err error
		preCommitChanges, err = miner.DiffPreCommits(ctx, ec.Store, ec.PrevState, ec.CurrState)
		if err != nil {
			return nil, err
		}
		ec.cache.Put(PreCommitDiff, preCommitChanges)
	} else {
		// a nil diff is a valid result, we want to keep this as to avoid rediffing to get nil
		if result == nil {
			return preCommitChanges, nil
		}
		preCommitChanges = result.(*miner.PreCommitChanges)
	}
	return preCommitChanges, nil
}

func getSectorDiff(ctx context.Context, ec *MinerStateExtractionContext) (*miner.SectorChanges, error) {
	sectorChanges := new(miner.SectorChanges)
	result, found := ec.cache.Get(SectorDiff)
	if !found {
		var err error
		sectorChanges, err = miner.DiffSectors(ctx, ec.Store, ec.PrevState, ec.CurrState)
		if err != nil {
			return nil, err
		}
		ec.cache.Put(SectorDiff, sectorChanges)
	} else {
		// a nil diff is a valid result, we want to keep this as to avoid rediffing to get nil
		if result == nil {
			return sectorChanges, nil
		}
		sectorChanges = result.(*miner.SectorChanges)
	}
	return sectorChanges, nil
}

func ExtractMinerPreCommitInfo(ctx context.Context, ec *MinerStateExtractionContext) (model.Persistable, error) {
	if !ec.HasPreviousState() {
		return nil, nil
	}

	preCommitChanges, err := getPreCommitDiff(ctx, ec)
	if err != nil {
		return nil, err
	}

	preCommitModel := minermodel.MinerPreCommitInfoList{}
	for _, added := range preCommitChanges.Added {
		pcm := &minermodel.MinerPreCommitInfo{
			Height:                 int64(ec.CurrTs.Height()),
			MinerID:                ec.Address.String(),
			SectorID:               uint64(added.Info.SectorNumber),
			StateRoot:              ec.CurrTs.ParentState().String(),
			SealedCID:              added.Info.SealedCID.String(),
			SealRandEpoch:          int64(added.Info.SealRandEpoch),
			ExpirationEpoch:        int64(added.Info.Expiration),
			PreCommitDeposit:       added.PreCommitDeposit.String(),
			PreCommitEpoch:         int64(added.PreCommitEpoch),
			DealWeight:             added.DealWeight.String(),
			VerifiedDealWeight:     added.VerifiedDealWeight.String(),
			IsReplaceCapacity:      added.Info.ReplaceCapacity,
			ReplaceSectorDeadline:  added.Info.ReplaceSectorDeadline,
			ReplaceSectorPartition: added.Info.ReplaceSectorPartition,
			ReplaceSectorNumber:    uint64(added.Info.ReplaceSectorNumber),
		}
		preCommitModel = append(preCommitModel, pcm)
	}
	return preCommitModel, nil
}

func ExtractMinerSectorInfo(ctx context.Context, ec *MinerStateExtractionContext) (model.Persistable, error) {
	sectorChanges := new(miner.SectorChanges)
	sectorModel := minermodel.MinerSectorInfoList{}
	if !ec.HasPreviousState() {
		msectors, err := ec.CurrState.LoadSectors(nil)
		if err != nil {
			return nil, err
		}

		sectorChanges.Added = make([]miner.SectorOnChainInfo, len(msectors))
		for idx, sector := range msectors {
			sectorChanges.Added[idx] = *sector
		}
	} else {
		var err error
		sectorChanges, err = getSectorDiff(ctx, ec)
		if err != nil {
			return nil, err
		}
	}

	for _, added := range sectorChanges.Added {
		sm := &minermodel.MinerSectorInfo{
			Height:                int64(ec.CurrTs.Height()),
			MinerID:               ec.Address.String(),
			SectorID:              uint64(added.SectorNumber),
			StateRoot:             ec.CurrTs.ParentState().String(),
			SealedCID:             added.SealedCID.String(),
			ActivationEpoch:       int64(added.Activation),
			ExpirationEpoch:       int64(added.Expiration),
			DealWeight:            added.DealWeight.String(),
			VerifiedDealWeight:    added.VerifiedDealWeight.String(),
			InitialPledge:         added.InitialPledge.String(),
			ExpectedDayReward:     added.ExpectedDayReward.String(),
			ExpectedStoragePledge: added.ExpectedStoragePledge.String(),
		}
		sectorModel = append(sectorModel, sm)
	}

	// do the same for extended sectors, since they have a new deadline
	for _, extended := range sectorChanges.Extended {
		sm := &minermodel.MinerSectorInfo{
			Height:                int64(ec.CurrTs.Height()),
			MinerID:               ec.Address.String(),
			SectorID:              uint64(extended.To.SectorNumber),
			StateRoot:             ec.CurrTs.ParentState().String(),
			SealedCID:             extended.To.SealedCID.String(),
			ActivationEpoch:       int64(extended.To.Activation),
			ExpirationEpoch:       int64(extended.To.Expiration),
			DealWeight:            extended.To.DealWeight.String(),
			VerifiedDealWeight:    extended.To.VerifiedDealWeight.String(),
			InitialPledge:         extended.To.InitialPledge.String(),
			ExpectedDayReward:     extended.To.ExpectedDayReward.String(),
			ExpectedStoragePledge: extended.To.ExpectedStoragePledge.String(),
		}
		sectorModel = append(sectorModel, sm)
	}
	return sectorModel, nil
}

// TODO(frrist): this isn't optimized for perf, in the above method, ExtractMinerSectorInfo, we also call DiffSectors, need to reuse or cache results.
func ExtractMinerSectorDeals(ctx context.Context, ec *MinerStateExtractionContext) (model.Persistable, error) {
	sectorChanges := new(miner.SectorChanges)
	sectorDealsModel := minermodel.MinerSectorDealList{}
	if !ec.HasPreviousState() {
		msectors, err := ec.CurrState.LoadSectors(nil)
		if err != nil {
			return nil, err
		}

		sectorChanges.Added = make([]miner.SectorOnChainInfo, len(msectors))
		for idx, sector := range msectors {
			sectorChanges.Added[idx] = *sector
			for _, dealID := range sector.DealIDs {
				sectorDealsModel = append(sectorDealsModel, &minermodel.MinerSectorDeal{
					Height:   int64(ec.CurrTs.Height()),
					MinerID:  ec.Address.String(),
					SectorID: uint64(sector.SectorNumber),
					DealID:   uint64(dealID),
				})
			}
		}
	} else {
		var err error
		sectorChanges, err = getSectorDiff(ctx, ec)
		if err != nil {
			return nil, err
		}
	}

	for _, newSector := range sectorChanges.Added {
		for _, dealID := range newSector.DealIDs {
			sectorDealsModel = append(sectorDealsModel, &minermodel.MinerSectorDeal{
				Height:   int64(ec.CurrTs.Height()),
				MinerID:  ec.Address.String(),
				SectorID: uint64(newSector.SectorNumber),
				DealID:   uint64(dealID),
			})
		}
	}
	return sectorDealsModel, nil
}

// TODO(frrist): redundant diffing calls here
func ExtractMinerSectorEvents(ctx context.Context, ec *MinerStateExtractionContext) (model.Persistable, error) {
	sectorChanges := new(miner.SectorChanges)
	preCommitChanges := new(miner.PreCommitChanges)
	if !ec.HasPreviousState() {
		msectors, err := ec.CurrState.LoadSectors(nil)
		if err != nil {
			return nil, err
		}

		sectorChanges.Added = make([]miner.SectorOnChainInfo, len(msectors))
		for idx, sector := range msectors {
			sectorChanges.Added[idx] = *sector
		}
	} else {
		var err error
		sectorChanges, err = getSectorDiff(ctx, ec)
		if err != nil {
			return nil, xerrors.Errorf("diffing miner sectors: %w", err)
		}
		preCommitChanges, err = getPreCommitDiff(ctx, ec)
		if err != nil {
			return nil, err
		}
	}
	return extractMinerSectorEvents(ctx, ec, sectorChanges, preCommitChanges)
}

func ExtractMinerPoSts(ctx context.Context, ec *MinerStateExtractionContext) (model.Persistable, error) {
	ctx, span := global.Tracer("").Start(ctx, "ExtractMinerPoSts")
	defer span.End()
	// short circuit genesis state, no PoSt messages in genesis blocks.
	if !ec.HasPreviousState() {
		return nil, nil
	}
	addr := ec.Address.String()
	posts := make(minermodel.MinerSectorPostList, 0)
	block := ec.CurrTs.Cids()[0]
	msgs, err := ec.API.ChainGetParentMessages(ctx, block)
	if err != nil {
		return nil, xerrors.Errorf("diffing miner posts: %v", err)
	}

	var partitions map[uint64]miner.Partition
	loadPartitions := func(state miner.State, epoch abi.ChainEpoch) (map[uint64]miner.Partition, error) {
		info, err := state.DeadlineInfo(epoch)
		if err != nil {
			return nil, err
		}
		dline, err := state.LoadDeadline(info.Index)
		if err != nil {
			return nil, err
		}
		pmap := make(map[uint64]miner.Partition)
		if err := dline.ForEachPartition(func(idx uint64, p miner.Partition) error {
			pmap[idx] = p
			return nil
		}); err != nil {
			return nil, err
		}
		return pmap, nil
	}

	processPostMsg := func(msg *types.Message) error {
		sectors := make([]uint64, 0)
		rcpt, err := ec.API.StateGetReceipt(ctx, msg.Cid(), ec.CurrTs.Key())
		if err != nil {
			return err
		}
		if rcpt == nil || rcpt.ExitCode.IsError() {
			return nil
		}
		params := miner.SubmitWindowedPoStParams{}
		if err := params.UnmarshalCBOR(bytes.NewBuffer(msg.Params)); err != nil {
			return err
		}

		// use previous miner state and tipset state since we are using parent messages
		if partitions == nil {
			partitions, err = loadPartitions(ec.PrevState, ec.PrevTs.Height())
			if err != nil {
				return err
			}
		}

		for _, p := range params.Partitions {
			all, err := partitions[p.Index].AllSectors()
			if err != nil {
				return err
			}
			proven, err := bitfield.SubtractBitField(all, p.Skipped)
			if err != nil {
				return err
			}

			if err := proven.ForEach(func(sector uint64) error {
				sectors = append(sectors, sector)
				return nil
			}); err != nil {
				return err
			}
		}

		for _, s := range sectors {
			posts = append(posts, &minermodel.MinerSectorPost{
				Height:         int64(ec.PrevTs.Height()),
				MinerID:        addr,
				SectorID:       s,
				PostMessageCID: msg.Cid().String(),
			})
		}
		return nil
	}

	for _, msg := range msgs {
		if msg.Message.To == ec.Address && msg.Message.Method == 5 /* miner.SubmitWindowedPoSt */ {
			if err := processPostMsg(msg.Message); err != nil {
				return nil, err
			}
		}
	}
	return posts, nil
}

func extractMinerSectorEvents(ctx context.Context, ec *MinerStateExtractionContext, sc *miner.SectorChanges, pc *miner.PreCommitChanges) (minermodel.MinerSectorEventList, error) {
	ctx, span := global.Tracer("").Start(ctx, "extractMinerSectorEvents")
	defer span.End()

	ps, err := extractMinerPartitionsDiff(ctx, ec)
	if err != nil {
		return nil, xerrors.Errorf("extracting miner partition diff: %w", err)
	}

	out := minermodel.MinerSectorEventList{}
	sectorAdds := make(map[abi.SectorNumber]miner.SectorOnChainInfo)

	// if there were changes made to the miners partition lists
	if ps != nil {
		// build an index of removed sector expiration's for comparison below.

		removedSectors, err := ec.CurrState.LoadSectors(&ps.Removed)
		if err != nil {
			return nil, xerrors.Errorf("fetching miners removed sectors: %w", err)
		}
		rmExpireIndex := make(map[uint64]abi.ChainEpoch)
		for _, rm := range removedSectors {
			rmExpireIndex[uint64(rm.SectorNumber)] = rm.Expiration
		}

		// track terminated and expired sectors
		if err := ps.Removed.ForEach(func(u uint64) error {
			event := minermodel.SectorTerminated
			expiration := rmExpireIndex[u]
			if expiration == ec.CurrTs.Height() {
				event = minermodel.SectorExpired
			}
			out = append(out, &minermodel.MinerSectorEvent{
				Height:    int64(ec.CurrTs.Height()),
				MinerID:   ec.Address.String(),
				StateRoot: ec.CurrTs.ParentState().String(),
				SectorID:  u,
				Event:     event,
			})
			return nil
		}); err != nil {
			return nil, xerrors.Errorf("walking miners removed sectors: %w", err)
		}

		// track recovering sectors
		if err := ps.Recovering.ForEach(func(u uint64) error {
			out = append(out, &minermodel.MinerSectorEvent{
				Height:    int64(ec.CurrTs.Height()),
				MinerID:   ec.Address.String(),
				StateRoot: ec.CurrTs.ParentState().String(),
				SectorID:  u,
				Event:     minermodel.SectorRecovering,
			})
			return nil
		}); err != nil {
			return nil, xerrors.Errorf("walking miners recovering sectors: %w", err)
		}

		// track faulted sectors
		if err := ps.Faulted.ForEach(func(u uint64) error {
			out = append(out, &minermodel.MinerSectorEvent{
				Height:    int64(ec.CurrTs.Height()),
				MinerID:   ec.Address.String(),
				StateRoot: ec.CurrTs.ParentState().String(),
				SectorID:  u,
				Event:     minermodel.SectorFaulted,
			})
			return nil
		}); err != nil {
			return nil, xerrors.Errorf("walking miners faulted sectors: %w", err)
		}

		// track recovered sectors
		if err := ps.Recovered.ForEach(func(u uint64) error {
			out = append(out, &minermodel.MinerSectorEvent{
				Height:    int64(ec.CurrTs.Height()),
				MinerID:   ec.Address.String(),
				StateRoot: ec.CurrTs.ParentState().String(),
				SectorID:  u,
				Event:     minermodel.SectorRecovered,
			})
			return nil
		}); err != nil {
			return nil, xerrors.Errorf("walking miners recovered sectors: %w", err)
		}
	}

	// if there were changes made to the miners sectors list
	if sc != nil {
		// track sector add and commit-capacity add
		for _, add := range sc.Added {
			event := minermodel.SectorAdded
			if len(add.DealIDs) == 0 {
				event = minermodel.CommitCapacityAdded
			}
			out = append(out, &minermodel.MinerSectorEvent{
				Height:    int64(ec.CurrTs.Height()),
				MinerID:   ec.Address.String(),
				StateRoot: ec.CurrTs.ParentState().String(),
				SectorID:  uint64(add.SectorNumber),
				Event:     event,
			})
			sectorAdds[add.SectorNumber] = add
		}

		// track sector extensions
		for _, mod := range sc.Extended {
			out = append(out, &minermodel.MinerSectorEvent{
				Height:    int64(ec.CurrTs.Height()),
				MinerID:   ec.Address.String(),
				StateRoot: ec.CurrTs.ParentState().String(),
				SectorID:  uint64(mod.To.SectorNumber),
				Event:     minermodel.SectorExtended,
			})
		}

	}

	// if there were changes made to the miners precommit list
	if pc != nil {
		// track precommit addition
		for _, add := range pc.Added {
			out = append(out, &minermodel.MinerSectorEvent{
				Height:    int64(ec.CurrTs.Height()),
				MinerID:   ec.Address.String(),
				StateRoot: ec.CurrTs.ParentState().String(),
				SectorID:  uint64(add.Info.SectorNumber),
				Event:     minermodel.PreCommitAdded,
			})
		}
	}

	return out, nil
}

// PartitionStatus contains bitfileds of sectorID's that are removed, faulted, recovered and recovering.
type PartitionStatus struct {
	Removed    bitfield.BitField
	Faulted    bitfield.BitField
	Recovering bitfield.BitField
	Recovered  bitfield.BitField
}

func extractMinerPartitionsDiff(ctx context.Context, ec *MinerStateExtractionContext) (*PartitionStatus, error) {
	_, span := global.Tracer("").Start(ctx, "extractMinerPartitionDiff") // nolint: ineffassign,staticcheck
	defer span.End()

	// short circuit genesis state.
	if !ec.HasPreviousState() {
		return nil, nil
	}

	dlDiff, err := miner.DiffDeadlines(ec.PrevState, ec.CurrState)
	if err != nil {
		return nil, err
	}

	if dlDiff == nil {
		return nil, nil
	}

	removed := bitfield.New()
	faulted := bitfield.New()
	recovered := bitfield.New()
	recovering := bitfield.New()

	for _, deadline := range dlDiff {
		for _, partition := range deadline {
			removed, err = bitfield.MergeBitFields(removed, partition.Removed)
			if err != nil {
				return nil, err
			}
			faulted, err = bitfield.MergeBitFields(faulted, partition.Faulted)
			if err != nil {
				return nil, err
			}
			recovered, err = bitfield.MergeBitFields(recovered, partition.Recovered)
			if err != nil {
				return nil, err
			}
			recovering, err = bitfield.MergeBitFields(recovering, partition.Recovering)
			if err != nil {
				return nil, err
			}
		}
	}
	return &PartitionStatus{
		Removed:    removed,
		Faulted:    faulted,
		Recovering: recovering,
		Recovered:  recovered,
	}, nil
}
