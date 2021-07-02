package lotus

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/v0api"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/chain/state"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/vm"
	builtininit "github.com/filecoin-project/sentinel-visor/chain/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/lens"
	"github.com/filecoin-project/sentinel-visor/lens/util"
	"github.com/filecoin-project/sentinel-visor/metrics"
)

func NewAPIWrapper(node v0api.FullNode, store adt.Store) *APIWrapper {
	return &APIWrapper{
		FullNode: node,
		store:    store,
	}
}

var _ lens.API = &APIWrapper{}

type APIWrapper struct {
	v0api.FullNode
	store adt.Store
}

func (aw *APIWrapper) GetMessageExecutionsForTipSet(ctx context.Context, ts, pts *types.TipSet) ([]*lens.MessageExecution, error) {
	panic("implement me")
}

func (aw *APIWrapper) Store() adt.Store {
	return aw.store
}

func (aw *APIWrapper) ChainGetBlock(ctx context.Context, msg cid.Cid) (*types.BlockHeader, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.ChainGetBlock")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "ChainGetBlock"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.ChainGetBlock(ctx, msg)
}

func (aw *APIWrapper) ChainGetBlockMessages(ctx context.Context, msg cid.Cid) (*api.BlockMessages, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.ChainGetBlockMessages")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "ChainGetBlockMessages"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.ChainGetBlockMessages(ctx, msg)
}

func (aw *APIWrapper) ChainGetGenesis(ctx context.Context) (*types.TipSet, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.ChainNotify")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "ChainGetGenesis"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.ChainGetGenesis(ctx)
}

func (aw *APIWrapper) ChainGetParentMessages(ctx context.Context, bcid cid.Cid) ([]api.Message, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.ChainGetParentMessages")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "ChainGetParentMessages"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.ChainGetParentMessages(ctx, bcid)
}

func (aw *APIWrapper) StateGetReceipt(ctx context.Context, bcid cid.Cid, tsk types.TipSetKey) (*types.MessageReceipt, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.StateGetReceipt")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "StateGetReceipt"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()

	return aw.FullNode.StateGetReceipt(ctx, bcid, tsk)
}

func (aw *APIWrapper) ChainGetParentReceipts(ctx context.Context, bcid cid.Cid) ([]*types.MessageReceipt, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.ChainGetParentReceipts")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "ChainGetParentReceipts"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.ChainGetParentReceipts(ctx, bcid)
}

func (aw *APIWrapper) ChainGetTipSet(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.ChainGetTipSet")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "ChainGetTipSet"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.ChainGetTipSet(ctx, tsk)
}

func (aw *APIWrapper) ChainNotify(ctx context.Context) (<-chan []*api.HeadChange, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.ChainNotify")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "ChainNotify"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.ChainNotify(ctx)
}

func (aw *APIWrapper) ChainReadObj(ctx context.Context, obj cid.Cid) ([]byte, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.ChainReadObj")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "ChainReadObj"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.ChainReadObj(ctx, obj)
}

func (aw *APIWrapper) StateChangedActors(ctx context.Context, old cid.Cid, new cid.Cid) (map[string]types.Actor, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.StateChangedActors")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "StateChangedActors"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.StateChangedActors(ctx, old, new)
}

func (aw *APIWrapper) StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.StateGetActor")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "StateGetActor"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()

	return aw.FullNode.StateGetActor(ctx, actor, tsk)
}

func (aw *APIWrapper) StateListActors(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.StateListActors")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "StateListActors"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.StateListActors(ctx, tsk)
}

func (aw *APIWrapper) StateMarketDeals(ctx context.Context, tsk types.TipSetKey) (map[string]api.MarketDeal, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.StateMarketDeals")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "StateMarketDeals"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.StateMarketDeals(ctx, tsk)
}

func (aw *APIWrapper) StateMinerPower(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*api.MinerPower, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.StateMinerPower")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "StateMinerPower"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.StateMinerPower(ctx, addr, tsk)
}

func (aw *APIWrapper) StateReadState(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*api.ActorState, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.StateReadState")
	defer span.End()
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "StateReadState"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()
	return aw.FullNode.StateReadState(ctx, actor, tsk)
}

func (aw *APIWrapper) StateVMCirculatingSupplyInternal(ctx context.Context, tsk types.TipSetKey) (api.CirculatingSupply, error) {
	ctx, span := global.Tracer("").Start(ctx, "Lotus.StateCirculatingSupply")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "StateVMCirculatingSupplyInternal"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()

	return aw.FullNode.StateVMCirculatingSupplyInternal(ctx, tsk)
}

// GetExecutedAndBlockMessagesForTipset returns a list of messages sent as part of pts (parent) with receipts found in ts (child).
// No attempt at deduplication of messages is made. A list of blocks with their corresponding messages is also returned - it contains all messages
// in the block regardless if they were applied during the state change.
func (aw *APIWrapper) GetExecutedAndBlockMessagesForTipset(ctx context.Context, ts, pts *types.TipSet) (*lens.TipSetMessages, error) {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.API, "GetExecutedAndBlockMessagesForTipset"))
	stop := metrics.Timer(ctx, metrics.LensRequestDuration)
	defer stop()

	if !types.CidArrsEqual(ts.Parents().Cids(), pts.Cids()) {
		return nil, xerrors.Errorf("child tipset (%s) is not on the same chain as parent (%s)", ts.Key(), pts.Key())
	}

	stateTree, err := state.LoadStateTree(aw.Store(), ts.ParentState())
	if err != nil {
		return nil, xerrors.Errorf("load state tree: %w", err)
	}

	parentStateTree, err := state.LoadStateTree(aw.Store(), pts.ParentState())
	if err != nil {
		return nil, xerrors.Errorf("load parent state tree: %w", err)
	}

	initActor, err := stateTree.GetActor(builtininit.Address)
	if err != nil {
		return nil, xerrors.Errorf("getting init actor: %w", err)
	}

	initActorState, err := builtininit.Load(aw.Store(), initActor)
	if err != nil {
		return nil, xerrors.Errorf("loading init actor state: %w", err)
	}

	// Build a lookup of actor codes
	actorCodes := map[address.Address]cid.Cid{}
	if err := stateTree.ForEach(func(a address.Address, act *types.Actor) error {
		actorCodes[a] = act.Code
		return nil
	}); err != nil {
		return nil, xerrors.Errorf("iterate actors: %w", err)
	}

	getActorCode := func(a address.Address) cid.Cid {
		ra, found, err := initActorState.ResolveAddress(a)
		if err != nil || !found {
			return cid.Undef
		}

		c, ok := actorCodes[ra]
		if ok {
			return c
		}

		return cid.Undef
	}

	// Build a lookup of which block headers indexed by their cid
	blockHeaders := map[cid.Cid]*types.BlockHeader{}
	for _, bh := range pts.Blocks() {
		blockHeaders[bh.Cid()] = bh
	}

	// Build a lookup of which blocks each message appears in
	messageBlocks := map[cid.Cid][]cid.Cid{}

	for _, blkCid := range pts.Cids() {
		blkMsgs, err := aw.ChainGetBlockMessages(ctx, blkCid)
		if err != nil {
			return nil, xerrors.Errorf("get block messages: %w", err)
		}

		for _, mcid := range blkMsgs.Cids {
			messageBlocks[mcid] = append(messageBlocks[mcid], blkCid)
		}
	}

	// Get messages that were processed in the parent tipset
	msgs, err := aw.ChainGetParentMessages(ctx, ts.Cids()[0])
	if err != nil {
		return nil, xerrors.Errorf("get parent messages: %w", err)
	}

	// Get receipts for parent messages
	rcpts, err := aw.ChainGetParentReceipts(ctx, ts.Cids()[0])
	if err != nil {
		return nil, xerrors.Errorf("get parent receipts: %w", err)
	}

	if len(rcpts) != len(msgs) {
		// logic error somewhere
		return nil, xerrors.Errorf("mismatching number of receipts: got %d wanted %d", len(rcpts), len(msgs))
	}

	// Start building a list of completed message with receipt
	emsgs := make([]*lens.ExecutedMessage, 0, len(msgs))

	// Create a skeleton vm just for calling ShouldBurn
	vmi, err := vm.NewVM(ctx, &vm.VMOpts{
		StateBase:   pts.ParentState(),
		Epoch:       pts.Height(),
		Bstore:      &apiBlockstore{api: aw.FullNode}, // sadly vm wraps this to turn it back into an adt.Store
		NtwkVersion: util.DefaultNetwork.Version,
	})
	if err != nil {
		return nil, xerrors.Errorf("creating temporary vm: %w", err)
	}

	for index, m := range msgs {

		em := &lens.ExecutedMessage{
			Cid:           m.Cid,
			Height:        pts.Height(),
			Message:       m.Message,
			Receipt:       rcpts[index],
			BlockHeader:   blockHeaders[messageBlocks[m.Cid][0]],
			Blocks:        messageBlocks[m.Cid],
			Index:         uint64(index),
			FromActorCode: getActorCode(m.Message.From),
			ToActorCode:   getActorCode(m.Message.To),
		}

		burn, err := vmi.ShouldBurn(ctx, parentStateTree, m.Message, rcpts[index].ExitCode)
		if err != nil {
			return nil, xerrors.Errorf("deciding whether should burn failed: %w", err)
		}

		em.GasOutputs = vm.ComputeGasOutputs(em.Receipt.GasUsed, em.Message.GasLimit, em.BlockHeader.ParentBaseFee, em.Message.GasFeeCap, em.Message.GasPremium, burn)
		emsgs = append(emsgs, em)
	}

	blkMsgs := make([]*lens.BlockMessages, len(ts.Blocks()))
	for idx, blk := range ts.Blocks() {
		msgs, err := aw.ChainGetBlockMessages(ctx, blk.Cid())
		if err != nil {
			return nil, err
		}

		blkMsgs[idx] = &lens.BlockMessages{
			Block:        blk,
			BlsMessages:  msgs.BlsMessages,
			SecpMessages: msgs.SecpkMessages,
		}
	}

	return &lens.TipSetMessages{
		Executed: emsgs,
		Block:    blkMsgs,
	}, nil
}

var _ blockstore.Blockstore = (*apiBlockstore)(nil)

type apiBlockstore struct {
	api interface {
		ChainReadObj(context.Context, cid.Cid) ([]byte, error)
		ChainHasObj(context.Context, cid.Cid) (bool, error)
	}
}

func (a *apiBlockstore) View(c cid.Cid, callback func([]byte) error) error {
	obj, err := a.api.ChainReadObj(context.Background(), c)
	if err != nil {
		return err
	}
	return callback(obj)
}

func (a *apiBlockstore) DeleteMany(cids []cid.Cid) error {
	return xerrors.Errorf("DeleteMany not supported by apiBlockstore")
}

func (a *apiBlockstore) Get(c cid.Cid) (blocks.Block, error) {
	data, err := a.api.ChainReadObj(context.Background(), c)
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

func (a *apiBlockstore) Has(c cid.Cid) (bool, error) {
	return a.api.ChainHasObj(context.Background(), c)
}

func (a *apiBlockstore) DeleteBlock(c cid.Cid) error {
	return xerrors.Errorf("DeleteBlock not supported by apiBlockstore")
}

func (a *apiBlockstore) GetSize(c cid.Cid) (int, error) {
	data, err := a.api.ChainReadObj(context.Background(), c)
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

func (a *apiBlockstore) Put(b blocks.Block) error {
	return xerrors.Errorf("Put not supported by apiBlockstore")
}

func (a *apiBlockstore) PutMany(bs []blocks.Block) error {
	return xerrors.Errorf("PutMany not supported by apiBlockstore")
}

func (a *apiBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return nil, xerrors.Errorf("AllKeysChan not supported by apiBlockstore")
}

func (a *apiBlockstore) HashOnRead(enabled bool) {
}
