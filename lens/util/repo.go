package util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-multistore"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/state"
	"github.com/filecoin-project/lotus/chain/stmgr"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/vm"
	"github.com/filecoin-project/lotus/extern/sector-storage/ffiwrapper"
	"github.com/filecoin-project/lotus/journal"
	"github.com/filecoin-project/lotus/lib/ulimit"
	marketevents "github.com/filecoin-project/lotus/markets/loggers"
	"github.com/filecoin-project/lotus/node/impl"
	"github.com/filecoin-project/lotus/node/impl/full"
	"github.com/filecoin-project/lotus/node/modules"
	"github.com/filecoin-project/lotus/node/repo"
	builtininit "github.com/filecoin-project/sentinel-visor/chain/actors/builtin/init"
	"github.com/filecoin-project/sentinel-visor/tasks/messages"
	"github.com/filecoin-project/sentinel-visor/tasks/messages/fcjson"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v5/actors/builtin"
	proof5 "github.com/filecoin-project/specs-actors/v5/actors/runtime/proof"
	"github.com/ipfs/go-cid"
	dstore "github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/lens"
)

var log = logging.Logger("lens/util")

type APIOpener struct {
	// shared instance of the repo since the opener holds an exclusive lock on it
	rapi *LensAPI
}

type HeadMthd func(ctx context.Context, lookback int) (*types.TipSetKey, error)

func NewAPIOpener(ctx context.Context, bs blockstore.Blockstore, head HeadMthd, cacheHint int) (*APIOpener, lens.APICloser, error) {
	rapi := LensAPI{}

	if _, _, err := ulimit.ManageFdLimit(); err != nil {
		return nil, nil, fmt.Errorf("setting file descriptor limit: %s", err)
	}
	r := repo.NewMemory(nil)

	lr, err := r.Lock(repo.FullNode)
	if err != nil {
		return nil, nil, err
	}

	mds, err := lr.Datastore(ctx, "/metadata")
	if err != nil {
		return nil, nil, err
	}

	cs := store.NewChainStore(bs, bs, mds, vm.Syscalls(&FakeVerifier{}), journal.NilJournal())

	const safetyLookBack = 5

	headKey, err := head(ctx, safetyLookBack)
	if err != nil {
		return nil, nil, err
	}

	headTs, err := cs.LoadTipSet(*headKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load our own chainhead: %w", err)
	}
	if err := cs.SetHead(headTs); err != nil {
		return nil, nil, fmt.Errorf("failed to set our own chainhead: %w", err)
	}

	sm := stmgr.NewStateManager(cs)

	rapi.cs = cs
	rapi.ExposedBlockstore = bs
	rapi.FullNodeAPI.ChainAPI.Chain = cs
	rapi.FullNodeAPI.ChainAPI.ChainModuleAPI = &full.ChainModule{Chain: cs, ExposedBlockstore: bs}
	rapi.FullNodeAPI.StateAPI.Chain = cs
	rapi.FullNodeAPI.StateAPI.StateManager = sm
	rapi.FullNodeAPI.StateAPI.StateModuleAPI = &full.StateModule{Chain: cs, StateManager: sm}

	sf := func() {
		lr.Close() // nolint: errcheck
	}

	genesisBlkHeader, err := modules.LoadGenesis(build.MaybeGenesis())(bs)()
	if err != nil {
		return nil, nil, err
	}
	if err := mds.Put(dstore.NewKey("0"), genesisBlkHeader.Cid().Bytes()); err != nil {
		return nil, nil, err
	}

	rapi.Context = ctx
	rapi.cacheSize = cacheHint
	return &APIOpener{rapi: &rapi}, sf, nil
}

func (o *APIOpener) Open(ctx context.Context) (lens.API, lens.APICloser, error) {
	return o.rapi, lens.APICloser(func() {}), nil
}

func (o *APIOpener) Daemonized() bool {
	return false
}

type LensAPI struct {
	impl.FullNodeAPI
	context.Context
	cacheSize int
	cs        *store.ChainStore
}

func (ra *LensAPI) GetMessageExecutionsForTipSet(ctx context.Context, ts, pts *types.TipSet) ([]*lens.MessageExecution, error) {
	panic("implement me")
}

func (ra *LensAPI) GetExecutedAndBlockMessagesForTipset(ctx context.Context, ts, pts *types.TipSet) (*lens.TipSetMessages, error) {
	return GetExecutedAndBlockMessagesForTipset(ctx, ra.cs, ts, pts)
}

func (ra *LensAPI) Store() adt.Store {
	return ra.FullNodeAPI.ChainAPI.Chain.ActorStore(ra.Context)
}

func (ra *LensAPI) ClientStartDeal(ctx context.Context, params *api.StartDealParams) (*cid.Cid, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientListDeals(ctx context.Context) ([]api.DealInfo, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientGetDealInfo(ctx context.Context, d cid.Cid) (*api.DealInfo, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientGetDealUpdates(ctx context.Context) (<-chan api.DealInfo, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientHasLocal(ctx context.Context, root cid.Cid) (bool, error) {
	return false, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientFindData(ctx context.Context, root cid.Cid, piece *cid.Cid) ([]api.QueryOffer, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientMinerQueryOffer(ctx context.Context, miner address.Address, root cid.Cid, piece *cid.Cid) (api.QueryOffer, error) {
	return api.QueryOffer{}, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientImport(ctx context.Context, ref api.FileRef) (*api.ImportRes, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientRemoveImport(ctx context.Context, importID multistore.StoreID) error {
	return fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientImportLocal(ctx context.Context, f io.Reader) (cid.Cid, error) {
	return cid.Undef, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientListImports(ctx context.Context) ([]api.Import, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientRetrieve(ctx context.Context, order api.RetrievalOrder, ref *api.FileRef) error {
	return fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientRetrieveWithEvents(ctx context.Context, order api.RetrievalOrder, ref *api.FileRef) (<-chan marketevents.RetrievalEvent, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientQueryAsk(ctx context.Context, p peer.ID, miner address.Address) (*storagemarket.StorageAsk, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientCalcCommP(ctx context.Context, inpath string) (*api.CommPRet, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientDealSize(ctx context.Context, root cid.Cid) (api.DataSize, error) {
	return api.DataSize{}, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientGenCar(ctx context.Context, ref api.FileRef, outputPath string) error {
	return fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientListDataTransfers(ctx context.Context) ([]api.DataTransferChannel, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientDataTransferUpdates(ctx context.Context) (<-chan api.DataTransferChannel, error) {
	return nil, fmt.Errorf("unsupported")
}

func (ra *LensAPI) ClientRetrieveTryRestartInsufficientFunds(ctx context.Context, paymentChannel address.Address) error {
	return fmt.Errorf("unsupported")
}

func (ra *LensAPI) StateGetReceipt(ctx context.Context, msg cid.Cid, from types.TipSetKey) (*types.MessageReceipt, error) {
	ml, err := ra.StateSearchMsg(ctx, from, msg, api.LookbackNoLimit, true)
	if err != nil {
		return nil, err
	}

	if ml == nil {
		return nil, nil
	}

	return &ml.Receipt, nil
}

// From https://github.com/ribasushi/ltsh/blob/5b0211033020570217b0ae37b50ee304566ac218/cmd/lotus-shed/deallifecycles.go#L41-L171
type FakeVerifier struct{}

var _ ffiwrapper.Verifier = (*FakeVerifier)(nil)

func (m FakeVerifier) VerifySeal(svi proof.SealVerifyInfo) (bool, error) {
	return true, nil
}

func (m FakeVerifier) VerifyWinningPoSt(ctx context.Context, info proof.WinningPoStVerifyInfo) (bool, error) {
	return true, nil
}

func (m FakeVerifier) VerifyWindowPoSt(ctx context.Context, info proof.WindowPoStVerifyInfo) (bool, error) {
	return true, nil
}

func (m FakeVerifier) GenerateWinningPoStSectorChallenge(ctx context.Context, proof abi.RegisteredPoStProof, id abi.ActorID, randomness abi.PoStRandomness, u uint64) ([]uint64, error) {
	panic("GenerateWinningPoStSectorChallenge not supported")
}

func (m FakeVerifier) VerifyAggregateSeals(aggregate proof5.AggregateSealVerifyProofAndInfos) (bool, error) {
	return true, nil
}

// GetMessagesForTipset returns a list of messages sent as part of pts (parent) with receipts found in ts (child).
// No attempt at deduplication of messages is made. A list of blocks with their corresponding messages is also returned - it contains all messages
// in the block regardless if they were applied during the state change.
func GetExecutedAndBlockMessagesForTipset(ctx context.Context, cs *store.ChainStore, ts, pts *types.TipSet) (*lens.TipSetMessages, error) {
	if !types.CidArrsEqual(ts.Parents().Cids(), pts.Cids()) {
		return nil, xerrors.Errorf("child tipset (%s) is not on the same chain as parent (%s)", ts.Key(), pts.Key())
	}

	getActorCode, err := MakeGetActorCodeFunc(ctx, cs.ActorStore(ctx), ts, pts)
	if err != nil {
		return nil, err
	}

	// Build a lookup of which blocks each message appears in
	messageBlocks := map[cid.Cid][]cid.Cid{}
	for blockIdx, bh := range pts.Blocks() {
		blscids, secpkcids, err := cs.ReadMsgMetaCids(bh.Messages)
		if err != nil {
			return nil, xerrors.Errorf("read messages for block: %w", err)
		}

		for _, c := range blscids {
			messageBlocks[c] = append(messageBlocks[c], pts.Cids()[blockIdx])
		}

		for _, c := range secpkcids {
			messageBlocks[c] = append(messageBlocks[c], pts.Cids()[blockIdx])
		}

	}

	bmsgs, err := cs.BlockMsgsForTipset(pts)
	if err != nil {
		return nil, xerrors.Errorf("block messages for tipset: %w", err)
	}

	pblocks := pts.Blocks()
	if len(bmsgs) != len(pblocks) {
		// logic error somewhere
		return nil, xerrors.Errorf("mismatching number of blocks returned from block messages, got %d wanted %d", len(bmsgs), len(pblocks))
	}

	count := 0
	for _, bm := range bmsgs {
		count += len(bm.BlsMessages) + len(bm.SecpkMessages)
	}

	// Start building a list of completed message with receipt
	emsgs := make([]*lens.ExecutedMessage, 0, count)

	// bmsgs is ordered by block
	var index uint64
	for blockIdx, bm := range bmsgs {
		for _, blsm := range bm.BlsMessages {
			msg := blsm.VMMessage()
			// if a message ran out of gas while executing this is expected.
			toCode, found := getActorCode(msg.To)
			if !found {
				log.Warnw("failed to find TO actor", "height", ts.Height().String(), "message", msg.Cid().String(), "actor", msg.To.String())
			}
			fromCode, found := getActorCode(msg.From)
			if !found {
				log.Warnw("failed to find FROM actor", "height", ts.Height().String(), "message", msg.Cid().String(), "actor", msg.To.String())
			}
			emsgs = append(emsgs, &lens.ExecutedMessage{
				Cid:           blsm.Cid(),
				Height:        pts.Height(),
				Message:       msg,
				BlockHeader:   pblocks[blockIdx],
				Blocks:        messageBlocks[blsm.Cid()],
				Index:         index,
				FromActorCode: fromCode,
				ToActorCode:   toCode,
			})
			index++
		}

		for _, secm := range bm.SecpkMessages {
			msg := secm.VMMessage()
			toCode, found := getActorCode(msg.To)
			if !found {
				log.Warnw("failed to find TO actor", "height", ts.Height().String(), "message", msg.Cid().String(), "actor", msg.To.String())
			}
			fromCode, found := getActorCode(msg.From)
			if !found {
				log.Warnw("failed to find FROM actor", "height", ts.Height().String(), "message", msg.Cid().String(), "actor", msg.To.String())
			}
			emsgs = append(emsgs, &lens.ExecutedMessage{
				Cid:           secm.Cid(),
				Height:        pts.Height(),
				Message:       secm.VMMessage(),
				BlockHeader:   pblocks[blockIdx],
				Blocks:        messageBlocks[secm.Cid()],
				Index:         index,
				FromActorCode: fromCode,
				ToActorCode:   toCode,
			})
			index++
		}

	}

	// Retrieve receipts using a block from the child tipset
	rs, err := adt.AsArray(cs.ActorStore(ctx), ts.Blocks()[0].ParentMessageReceipts)
	if err != nil {
		return nil, xerrors.Errorf("amt load: %w", err)
	}

	if rs.Length() != uint64(len(emsgs)) {
		// logic error somewhere
		return nil, xerrors.Errorf("mismatching number of receipts: got %d wanted %d", rs.Length(), len(emsgs))
	}

	// Create a skeleton vm just for calling ShouldBurn
	vmi, err := vm.NewVM(ctx, &vm.VMOpts{
		StateBase:   pts.ParentState(),
		Epoch:       pts.Height(),
		Bstore:      cs.StateBlockstore(),
		NtwkVersion: DefaultNetwork.Version,
	})
	if err != nil {
		return nil, xerrors.Errorf("creating temporary vm: %w", err)
	}

	parentStateTree, err := state.LoadStateTree(cs.ActorStore(ctx), pts.ParentState())
	if err != nil {
		return nil, xerrors.Errorf("load state tree: %w", err)
	}

	// Receipts are in same order as BlockMsgsForTipset
	for _, em := range emsgs {
		var r types.MessageReceipt
		if found, err := rs.Get(em.Index, &r); err != nil {
			return nil, err
		} else if !found {
			return nil, xerrors.Errorf("failed to find receipt %d", em.Index)
		}
		em.Receipt = &r

		burn, err := vmi.ShouldBurn(ctx, parentStateTree, em.Message, em.Receipt.ExitCode)
		if err != nil {
			return nil, xerrors.Errorf("deciding whether should burn failed: %w", err)
		}

		em.GasOutputs = vm.ComputeGasOutputs(em.Receipt.GasUsed, em.Message.GasLimit, em.BlockHeader.ParentBaseFee, em.Message.GasFeeCap, em.Message.GasPremium, burn)

	}
	blkMsgs := make([]*lens.BlockMessages, len(ts.Blocks()))
	for idx, blk := range ts.Blocks() {
		msgs, smsgs, err := cs.MessagesForBlock(blk)
		if err != nil {
			return nil, err
		}
		blkMsgs[idx] = &lens.BlockMessages{
			Block:        blk,
			BlsMessages:  msgs,
			SecpMessages: smsgs,
		}
	}

	return &lens.TipSetMessages{
		Executed: emsgs,
		Block:    blkMsgs,
	}, nil
}

func MethodAndParamsForMessage(m *types.Message, destCode cid.Cid) (string, string, error) {
	var params ipld.Node
	var method string
	var err error

	// fall back to generic cbor->json conversion.
	params, method, err = messages.ParseParams(m.Params, int64(m.Method), destCode)
	if method == "Unknown" {
		return "", "", xerrors.Errorf("unknown method for actor type %s: %d", destCode.String(), int64(m.Method))
	}
	if err != nil {
		log.Warnf("failed to parse parameters of message %s: %v", m.Cid, err)
		// this can occur when the message is not valid cbor
		return method, "", err
	}
	if params == nil {
		return method, "", nil
	}

	buf := bytes.NewBuffer(nil)
	if err := fcjson.Encoder(params, buf); err != nil {
		return "", "", xerrors.Errorf("json encode: %w", err)
	}

	encoded := string(bytes.ReplaceAll(bytes.ToValidUTF8(buf.Bytes(), []byte{}), []byte{0x00}, []byte{}))

	return method, encoded, nil

}

func ActorNameAndFamilyFromCode(c cid.Cid) (name string, family string, err error) {
	if !c.Defined() {
		return "", "", xerrors.Errorf("cannot derive actor name from undefined CID")
	}
	name = builtin.ActorNameByCode(c)
	if name == "<unknown>" {
		return "", "", xerrors.Errorf("cannot derive actor name from unknown CID: %s (maybe we need up update deps?)", c.String())
	}
	tokens := strings.Split(name, "/")
	if len(tokens) != 3 {
		return "", "", xerrors.Errorf("cannot parse actor name: %s from tokens: %s", name, tokens)
	}
	// network = tokens[0]
	// version = tokens[1]
	family = tokens[2]
	return
}

func MakeGetActorCodeFunc(ctx context.Context, store adt.Store, ts, pts *types.TipSet) (func(a address.Address) (cid.Cid, bool), error) {
	if !types.CidArrsEqual(ts.Parents().Cids(), pts.Cids()) {
		return nil, xerrors.Errorf("child tipset (%s) is not on the same chain as parent (%s)", ts.Key(), pts.Key())
	}

	stateTree, err := state.LoadStateTree(store, ts.ParentState())
	if err != nil {
		return nil, xerrors.Errorf("load state tree: %w", err)
	}

	initActor, err := stateTree.GetActor(builtininit.Address)
	if err != nil {
		return nil, xerrors.Errorf("getting init actor: %w", err)
	}

	initActorState, err := builtininit.Load(store, initActor)
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

	return func(a address.Address) (cid.Cid, bool) {
		ra, found, err := initActorState.ResolveAddress(a)
		if err != nil || !found {
			return cid.Undef, false
		}

		c, ok := actorCodes[ra]
		if ok {
			return c, true
		}

		return cid.Undef, false
	}, nil

}
