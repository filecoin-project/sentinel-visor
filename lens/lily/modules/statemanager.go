package modules

import (
	"context"
	"sync"

	"github.com/filecoin-project/lotus/chain/stmgr"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/vm"
	"github.com/filecoin-project/lotus/node/modules/helpers"
	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"
	"golang.org/x/xerrors"
)

var log = logging.Logger("modules/statemanager")

var ExecutionTraceNotFound = xerrors.Errorf("failed to find execution trace")

func StateManager(lmctx helpers.MetricsCtx, lc fx.Lifecycle, cs *store.ChainStore, us stmgr.UpgradeSchedule, em stmgr.ExecMonitor) (*stmgr.StateManager, error) {
	sm, err := stmgr.NewStateManagerWithUpgradeScheduleAndMonitor(cs, us, em)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStart: sm.Start,
		OnStop:  sm.Stop,
	})
	return sm, nil
}

var _ stmgr.ExecMonitor = (*BufferedExecMonitor)(nil)

func NewBufferedExecMonitor() *BufferedExecMonitor {
	// this only errors when a negative size is supplied...y u no accept unsigned ints :(
	cache, err := lru.New(64)
	if err != nil {
		panic(err)
	}
	return &BufferedExecMonitor{
		cache: cache,
	}
}

type BufferedExecMonitor struct {
	cacheMu sync.Mutex
	cache   *lru.Cache
}

type BufferedExecution struct {
	TipSet   *types.TipSet
	Mcid     cid.Cid
	Msg      *types.Message
	Ret      *vm.ApplyRet
	Implicit bool
}

func (b *BufferedExecMonitor) MessageApplied(ctx context.Context, ts *types.TipSet, mcid cid.Cid, msg *types.Message, ret *vm.ApplyRet, implicit bool) error {
	execution := &BufferedExecution{
		TipSet:   ts,
		Mcid:     mcid,
		Msg:      msg,
		Ret:      ret,
		Implicit: implicit,
	}

	b.cacheMu.Lock()
	defer b.cacheMu.Unlock()

	// if this is the first tipset we have seen a message applied for add it to the cache and bail.
	found := b.cache.Contains(ts.Key())
	if !found {
		b.cache.Add(ts.Key(), []*BufferedExecution{execution})
		return nil
	}
	// otherwise append to the current list of executions for this tipset.
	v, _ := b.cache.Get(ts.Key())
	exe := v.([]*BufferedExecution)
	exe = append(exe, execution)
	evicted := b.cache.Add(ts.Key(), exe)
	// TODO it would be nice to know if we have extracted the buffered execution for this tipset already, maybe not important
	if evicted {
		log.Debugw("Evicting tipset from buffered exec monitor", "ts", ts.Key())
	}

	return nil
}

// So long as we are are always driving this method with tipsets we get from HeadEvents then we should always find a tipset in here.
func (b *BufferedExecMonitor) ExecutionFor(ts *types.TipSet) ([]*BufferedExecution, error) {
	log.Infow("execution for", "ts", ts.String())
	b.cacheMu.Lock()
	defer b.cacheMu.Unlock()

	exe, found := b.cache.Get(ts.Key())
	if !found {
		log.Errorw("failed to find execution", "tipset", ts.String())
		return nil, ExecutionTraceNotFound
	}
	return exe.([]*BufferedExecution), nil
}
