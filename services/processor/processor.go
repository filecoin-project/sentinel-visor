package processor

import (
	"context"
	"github.com/filecoin-project/visor/services/indexer"
	"strings"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	types "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/visor/storage"

	"github.com/gocraft/work"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel/api/trace"
)

func NewProcessor(db *storage.Database, n api.FullNode) *Processor {
	// TODO I don't like how these are buried in here.
	p := NewPublisher(db)
	s := NewScheduler(n, p)
	return &Processor{
		storage:   db,
		node:      n,
		scheduler: s,
		log:       logging.Logger("processor"),
	}
}

type Processor struct {
	storage *storage.Database
	node    api.FullNode

	scheduler *Scheduler

	log    *logging.ZapEventLogger
	tracer trace.Tracer

	// we will want to spcial case the processing of the genesis state.
	genesis *types.TipSet

	batchSize int

	pool        *work.WorkerPool
	tipsetQueue *work.Enqueuer
}

func (p *Processor) InitHandler(ctx context.Context, batchSize int) error {
	if err := logging.SetLogLevel("*", "debug"); err != nil {
		return err
	}

	gen, err := p.node.ChainGetGenesis(ctx)
	if err != nil {
		return err
	}

	p.genesis = gen
	p.batchSize = batchSize

	p.scheduler.Start()

	p.log.Infow("initialized processor", "genesis", gen.String())
	return nil
}

func (p *Processor) Start(ctx context.Context) {
	p.log.Info("starting processor")
	go func() {
		for {
			select {
			case <-ctx.Done():
				p.log.Info("stopping processor")
				return
			default:
				blksToProcess, err := p.collectBlocksToProcess(ctx, p.batchSize)
				if err != nil {
					panic(err)
				}

				if len(blksToProcess) == 0 {
					time.Sleep(time.Second * 30)
					continue
				}

				actorChanges, err := p.collectActorChanges(ctx, blksToProcess)
				if err != nil {
					panic(err)
				}

				if err := p.scheduler.Dispatch(actorChanges); err != nil {
					panic(err)
				}
			}
		}
	}()
}

func (p *Processor) collectActorChanges(ctx context.Context, blks []*types.BlockHeader) (map[types.TipSetKey][]indexer.ActorInfo, error) {
	out := make(map[types.TipSetKey][]indexer.ActorInfo)
	for _, blk := range blks {
		pts, err := p.node.ChainGetTipSet(ctx, types.NewTipSetKey(blk.Parents...))
		if err != nil {
			return nil, err
		}

		changes, err := p.node.StateChangedActors(ctx, pts.ParentState(), blk.ParentStateRoot)
		if err != nil {
			return nil, err
		}

		for str, act := range changes {
			addr, err := address.NewFromString(str)
			if err != nil {
				return nil, err
			}

			_, err = p.node.StateGetActor(ctx, addr, pts.Key())
			if err != nil {
				if strings.Contains(err.Error(), "actor not found") {
					// TODO consider tracking deleted actors
					continue
				}
				return nil, err
			}
			_, err = p.node.StateGetActor(ctx, addr, pts.Parents())
			if err != nil {
				if strings.Contains(err.Error(), "actor not found") {
					// TODO consider tracking deleted actors
					continue
				}
				return nil, err
			}

			// TODO track null rounds
			out[pts.Key()] = append(out[pts.Key()], indexer.ActorInfo{
				Actor:           act,
				Address:         addr,
				TipSet:          pts.Key(),
				ParentTipset:    pts.Parents(),
				ParentStateRoot: pts.ParentState(),
			})
		}
	}
	return out, nil
}

func (p *Processor) collectBlocksToProcess(ctx context.Context, batch int) ([]*types.BlockHeader, error) {
	blks, err := p.storage.CollectAndMarkBlocksAsProcessing(ctx, batch)
	if err != nil {
		return nil, err
	}

	out := make([]*types.BlockHeader, len(blks))
	for idx, blk := range blks {
		blkCid, err := cid.Decode(blk.Cid)
		if err != nil {
			return nil, err
		}
		header, err := p.node.ChainGetBlock(ctx, blkCid)
		if err != nil {
			return nil, err
		}
		out[idx] = header
	}
	return out, nil
}
