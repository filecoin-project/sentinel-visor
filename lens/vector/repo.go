package vector

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/chain/stmgr"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/vm"
	"github.com/filecoin-project/lotus/journal"
	"github.com/filecoin-project/lotus/lib/ulimit"
	"github.com/filecoin-project/lotus/node/impl"
	"github.com/filecoin-project/lotus/node/impl/full"
	"github.com/filecoin-project/lotus/node/repo"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-blockservice"
	cid "github.com/ipfs/go-cid"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	car "github.com/ipld/go-car"

	"github.com/filecoin-project/sentinel-visor/lens"
	"github.com/filecoin-project/sentinel-visor/lens/util"
)

func NewAPIOpener(c *cli.Context) (*APIOpener, lens.APICloser, error) {
	capi := CaptureAPI{}

	if _, _, err := ulimit.ManageFdLimit(); err != nil {
		return nil, nil, fmt.Errorf("setting file descriptor limit: %s", err)
	}

	r, err := repo.NewFS(c.String("lens-repo"))
	if err != nil {
		return nil, nil, err
	}

	exists, err := r.Exists()
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, fmt.Errorf("lotus repo doesn't exist")
	}

	var lr repo.LockedRepo
	if c.Bool("repo-read-only") {
		lr, err = r.LockRO(repo.FullNode)
	} else {
		lr, err = r.Lock(repo.FullNode)
	}
	if err != nil {
		return nil, nil, err
	}

	sf := func() {
		lr.Close() // nolint: errcheck
	}

	bs, err := lr.Blockstore(c.Context, repo.UniversalBlockstore)
	if err != nil {
		return nil, nil, err
	}

	// wrap the repos blockstore with a tracing implementation capturing all cid's read.
	capi.tbs = NewTracingBlockstore(bs)

	mds, err := lr.Datastore(c.Context, "/metadata")
	if err != nil {
		return nil, nil, err
	}

	cs := store.NewChainStore(capi.tbs, capi.tbs, mds, vm.Syscalls(&util.FakeVerifier{}), journal.NilJournal())
	if err := cs.Load(); err != nil {
		return nil, nil, err
	}

	sm := stmgr.NewStateManager(cs)

	capi.ExposedBlockstore = bs
	capi.FullNodeAPI.ChainAPI.Chain = cs
	capi.FullNodeAPI.ChainAPI.ChainModuleAPI = &full.ChainModule{Chain: cs, ExposedBlockstore: bs}
	capi.FullNodeAPI.StateAPI.Chain = cs
	capi.FullNodeAPI.StateAPI.StateManager = sm
	capi.FullNodeAPI.StateAPI.StateModuleAPI = &full.StateModule{Chain: cs, StateManager: sm}

	capi.Context = c.Context
	capi.cacheSize = c.Int("lens-cache-hint")
	return &APIOpener{capi: &capi}, sf, nil
}

type APIOpener struct {
	capi *CaptureAPI
}

func (o *APIOpener) Open(ctx context.Context) (lens.API, lens.APICloser, error) {
	return o.capi, lens.APICloser(func() {}), nil
}
func (o *APIOpener) Daemonized() bool {
	return false
}

func (o *APIOpener) CaptureAsCAR(ctx context.Context, w io.Writer, roots ...cid.Cid) error {
	carWalkFn := func(nd format.Node) (out []*format.Link, err error) {
		for _, link := range nd.Links() {
			if _, ok := o.capi.tbs.traced[link.Cid]; !ok {
				continue
			}
			if link.Cid.Prefix().Codec == cid.FilCommitmentSealed || link.Cid.Prefix().Codec == cid.FilCommitmentUnsealed {
				continue
			}
			out = append(out, link)
		}
		return out, nil
	}

	var (
		offl    = offline.Exchange(o.capi.tbs)
		blkserv = blockservice.New(o.capi.tbs, offl)
		dserv   = merkledag.NewDAGService(blkserv)
	)

	return car.WriteCarWithWalker(ctx, dserv, roots, w, carWalkFn)
}

type CaptureAPI struct {
	impl.FullNodeAPI
	context.Context
	cacheSize int

	tbs *TracingBlockstore
}

func (c *CaptureAPI) GetMessageExecutionsForTipSet(ctx context.Context, ts, pts *types.TipSet) ([]*lens.MessageExecution, error) {
	panic("implement me")
}

func (c *CaptureAPI) Store() adt.Store {
	cachedStore := blockstore.NewBuffered(c.tbs)
	cs := cbor.NewCborStore(cachedStore)
	adtStore := adt.WrapStore(c.Context, cs)
	return adtStore
}

func (c *CaptureAPI) GetExecutedAndBlockMessagesForTipset(ctx context.Context, ts, pts *types.TipSet) (*lens.TipSetMessages, error) {
	return util.GetExecutedAndBlockMessagesForTipset(ctx, c.FullNodeAPI.ChainAPI.Chain, ts, pts)
}

func (c *CaptureAPI) StateGetReceipt(ctx context.Context, msg cid.Cid, from types.TipSetKey) (*types.MessageReceipt, error) {
	ml, err := c.StateSearchMsg(ctx, from, msg, api.LookbackNoLimit, true)
	if err != nil {
		return nil, err
	}

	if ml == nil {
		return nil, nil
	}

	return &ml.Receipt, nil
}
