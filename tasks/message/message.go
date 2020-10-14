package message

import (
	"context"
	"math"
	"math/big"
	"time"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/go-pg/pg/v10"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/raulk/clock"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/label"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/lens"
	"github.com/filecoin-project/sentinel-visor/metrics"
	messagemodel "github.com/filecoin-project/sentinel-visor/model/messages"
	"github.com/filecoin-project/sentinel-visor/model/visor"
	"github.com/filecoin-project/sentinel-visor/storage"
	"github.com/filecoin-project/sentinel-visor/wait"
)

const (
	idleSleepInterval = 60 * time.Second       // time to wait if the processor runs out of blocks to process
	batchInterval     = 100 * time.Millisecond // time to wait between batches
)

var log = logging.Logger("message")

func NewMessageProcessor(d *storage.Database, node lens.API, leaseLength time.Duration, batchSize int, minHeight, maxHeight int64) *MessageProcessor {
	return &MessageProcessor{
		node:        node,
		storage:     d,
		leaseLength: leaseLength,
		batchSize:   batchSize,
		minHeight:   minHeight,
		maxHeight:   maxHeight,
		clock:       clock.New(),
	}
}

// MessageProcessor is a task that processes blocks to detect messages and persists
// their details to the database.
type MessageProcessor struct {
	node        lens.API
	storage     *storage.Database
	leaseLength time.Duration // length of time to lease work for
	batchSize   int           // number of tipsets to lease in a batch
	minHeight   int64         // limit processing to tipsets equal to or above this height
	maxHeight   int64         // limit processing to tipsets equal to or below this height
	clock       clock.Clock
}

// Run starts processing batches of tipsets and blocks until the context is done or
// an error occurs.
func (p *MessageProcessor) Run(ctx context.Context) error {
	// Loop until context is done or processing encounters a fatal error
	return wait.RepeatUntil(ctx, batchInterval, p.processBatch)
}

func (p *MessageProcessor) processBatch(ctx context.Context) (bool, error) {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.TaskType, "message"))
	ctx, span := global.Tracer("").Start(ctx, "MessageProcessor.processBatch")
	defer span.End()

	claimUntil := p.clock.Now().Add(p.leaseLength)

	// Lease some blocks to work on
	batch, err := p.storage.LeaseTipSetMessages(ctx, claimUntil, p.batchSize, p.minHeight, p.maxHeight)
	if err != nil {
		return true, err
	}

	// If we have no tipsets to work on then wait before trying again
	if len(batch) == 0 {
		sleepInterval := wait.Jitter(idleSleepInterval, 2)
		log.Debugf("no tipsets to process, waiting for %s", sleepInterval)
		time.Sleep(sleepInterval)
		return false, nil
	}

	log.Debugw("leased batch of tipsets", "count", len(batch))
	ctx, cancel := context.WithDeadline(ctx, claimUntil)
	defer cancel()

	for _, item := range batch {
		// Stop processing if we have somehow passed our own lease time
		select {
		case <-ctx.Done():
			return false, nil // Don't propagate cancelation error so we can resume processing cleanly
		default:
		}

		if err := p.processItem(ctx, item); err != nil {
			log.Errorw("failed to process tipset", "error", err.Error(), "height", item.Height)
			if err := p.storage.MarkTipSetMessagesComplete(ctx, item.TipSet, item.Height, p.clock.Now(), err.Error()); err != nil {
				log.Errorw("failed to mark tipset messages complete", "error", err.Error(), "height", item.Height)
			}
			continue
		}

		if err := p.storage.MarkTipSetMessagesComplete(ctx, item.TipSet, item.Height, p.clock.Now(), ""); err != nil {
			log.Errorw("failed to mark tipset message complete", "error", err.Error(), "height", item.Height)
		}
	}

	return false, nil
}

func (p *MessageProcessor) processItem(ctx context.Context, item *visor.ProcessingTipSet) error {
	ctx, span := global.Tracer("").Start(ctx, "MessageProcessor.processItem")
	defer span.End()
	span.SetAttributes(label.Any("height", item.Height), label.Any("tipset", item.TipSet))

	stop := metrics.Timer(ctx, metrics.ProcessingDuration)
	defer stop()

	tsk, err := item.TipSetKey()
	if err != nil {
		return xerrors.Errorf("get tipsetkey: %w", err)
	}

	ts, err := p.node.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return xerrors.Errorf("get tipset: %w", err)
	}

	if err := p.processTipSet(ctx, ts); err != nil {
		return xerrors.Errorf("process tipset: %w", err)
	}

	return nil
}

func (p *MessageProcessor) processTipSet(ctx context.Context, ts *types.TipSet) error {
	ctx, span := global.Tracer("").Start(ctx, "MessageProcessor.processTipSet")
	defer span.End()

	ll := log.With("height", int64(ts.Height()))

	blkMsgs, err := p.fetchMessages(ctx, ts)
	if err != nil {
		return xerrors.Errorf("fetch messages: %w", err)
	}

	rcts, err := p.fetchReceipts(ctx, ts)
	if err != nil {
		return xerrors.Errorf("fetch receipts: %w", err)
	}

	msgs, bmsgs, processingMsgs, econ, err := p.extractMessageModels(ctx, ts, blkMsgs)
	if err != nil {
		return xerrors.Errorf("extract message models: %w", err)
	}

	result := &messagemodel.MessageTaskResult{
		Messages:          msgs,
		BlockMessages:     bmsgs,
		Receipts:          rcts,
		MessageGasEconomy: econ,
	}

	ll.Debugw("persisting tipset", "messages", len(msgs), "block_messages", len(blkMsgs), "receipts", len(rcts))

	if err := p.storage.DB.RunInTransaction(ctx, func(tx *pg.Tx) error {
		if err := result.PersistWithTx(ctx, tx); err != nil {
			return err
		}
		if err := processingMsgs.PersistWithTx(ctx, tx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return xerrors.Errorf("persist: %w", err)
	}

	return nil
}

func (p *MessageProcessor) fetchMessages(ctx context.Context, ts *types.TipSet) (map[cid.Cid]*api.BlockMessages, error) {
	out := make(map[cid.Cid]*api.BlockMessages)
	for _, blk := range ts.Cids() {
		// Stop processing if we have somehow passed our own lease time
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		blkMsgs, err := p.node.ChainGetBlockMessages(ctx, blk)
		if err != nil {
			return nil, err
		}
		out[blk] = blkMsgs
	}
	return out, nil
}

func (p *MessageProcessor) extractMessageModels(ctx context.Context, ts *types.TipSet, blkMsgs map[cid.Cid]*api.BlockMessages) (messagemodel.Messages, messagemodel.BlockMessages, visor.ProcessingMessageList, *messagemodel.MessageGasEconomy, error) {
	msgModels := messagemodel.Messages{}
	bmsgModels := messagemodel.BlockMessages{}
	pmsgModels := visor.ProcessingMessageList{}

	msgsSeen := map[cid.Cid]struct{}{}
	totalGasLimit := int64(0)
	totalUniqGasLimit := int64(0)

	for blk, msgs := range blkMsgs {
		// Stop processing if we have somehow passed our own lease time
		select {
		case <-ctx.Done():
			return nil, nil, nil, nil, ctx.Err()
		default:
		}

		// extract all messages, vmm will include duplicate messages.
		vmm := make([]*types.Message, 0, len(msgs.Cids))
		for _, m := range msgs.BlsMessages {
			vmm = append(vmm, m)
		}
		for _, m := range msgs.SecpkMessages {
			vmm = append(vmm, &m.Message)
		}

		for _, message := range vmm {
			// record which blocks had which messages
			bmsgModels = append(bmsgModels, &messagemodel.BlockMessage{
				Block:   blk.String(),
				Message: message.Cid().String(),
			})

			totalUniqGasLimit += message.GasLimit
			if _, seen := msgsSeen[message.Cid()]; seen {
				continue
			}
			totalGasLimit += message.GasLimit

			// record this message for processing by later stages
			pmsgModels = append(pmsgModels, visor.NewProcessingMessage(message, int64(ts.Height())))

			var msgSize int
			if b, err := message.Serialize(); err == nil {
				msgSize = len(b)
			} else {
				return nil, nil, nil, nil, err
			}

			// record all unique messages
			msgModels = append(msgModels, &messagemodel.Message{
				Cid:        message.Cid().String(),
				From:       message.From.String(),
				To:         message.To.String(),
				Value:      message.Value.String(),
				GasFeeCap:  message.GasFeeCap.String(),
				GasPremium: message.GasPremium.String(),
				GasLimit:   message.GasLimit,
				SizeBytes:  msgSize,
				Nonce:      message.Nonce,
				Method:     uint64(message.Method),
				Params:     message.Params,
			})

			msgsSeen[message.Cid()] = struct{}{}
		}

	}
	newBaseFee := store.ComputeNextBaseFee(ts.Blocks()[0].ParentBaseFee, totalUniqGasLimit, len(ts.Blocks()), ts.Height())
	baseFeeRat := new(big.Rat).SetFrac(newBaseFee.Int, new(big.Int).SetUint64(build.FilecoinPrecision))
	baseFee, _ := baseFeeRat.Float64()

	baseFeeChange := new(big.Rat).SetFrac(newBaseFee.Int, ts.Blocks()[0].ParentBaseFee.Int)
	baseFeeChangeF, _ := baseFeeChange.Float64()

	return msgModels,
		bmsgModels,
		pmsgModels,
		&messagemodel.MessageGasEconomy{
			StateRoot:           ts.ParentState().String(),
			GasLimitTotal:       totalGasLimit,
			GasLimitUniqueTotal: totalUniqGasLimit,
			BaseFee:             baseFee,
			BaseFeeChangeLog:    math.Log(baseFeeChangeF) / math.Log(1.125),
			GasFillRatio:        float64(totalGasLimit) / float64(len(ts.Blocks())*build.BlockGasTarget),
			GasCapacityRatio:    float64(totalUniqGasLimit) / float64(len(ts.Blocks())*build.BlockGasTarget),
			GasWasteRatio:       float64(totalGasLimit-totalUniqGasLimit) / float64(len(ts.Blocks())*build.BlockGasTarget),
		},
		nil
}

func (p *MessageProcessor) fetchReceipts(ctx context.Context, ts *types.TipSet) (messagemodel.Receipts, error) {
	out := messagemodel.Receipts{}

	for _, blk := range ts.Cids() {
		// Stop processing if we have somehow passed our own lease time
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		recs, err := p.node.ChainGetParentReceipts(ctx, blk)
		if err != nil {
			return nil, err
		}
		msgs, err := p.node.ChainGetParentMessages(ctx, blk)
		if err != nil {
			return nil, err
		}

		for i, r := range recs {
			out = append(out, &messagemodel.Receipt{
				Message:   msgs[i].Cid.String(),
				StateRoot: ts.ParentState().String(),
				Idx:       i,
				ExitCode:  int64(r.ExitCode),
				GasUsed:   r.GasUsed,
				Return:    r.Return,
			})
		}
	}
	return out, nil
}
