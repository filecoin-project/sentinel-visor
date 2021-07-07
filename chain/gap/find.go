package gap

import (
	"context"
	"sort"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/sentinel-visor/chain"
	"github.com/filecoin-project/sentinel-visor/lens"
	"github.com/filecoin-project/sentinel-visor/model/visor"
	"github.com/filecoin-project/sentinel-visor/storage"
	"golang.org/x/xerrors"
)

type GapIndexer struct {
	DB     *storage.Database
	opener lens.APIOpener
}

func NewGapIndexer(o lens.APIOpener, db *storage.Database) *GapIndexer {
	return &GapIndexer{
		DB:     db,
		opener: o,
	}
}

func (g *GapIndexer) Run(ctx context.Context) error {
	node, closer, err := g.opener.Open(ctx)
	if err != nil {
		return xerrors.Errorf("open lens: %w", err)
	}
	defer func() {
		closer()
	}()

	findLog := log.With("type", "find")
	heightGaps, err := g.detectProcessingGaps(ctx, node)
	if err != nil {
		return xerrors.Errorf("detecting processing gaps: %w", err)
	}

	findLog.Infow("detected gaps in height", "count", len(heightGaps))

	skipGaps, err := g.detectSkippedTipSets(ctx, node)
	if err != nil {
		return xerrors.Errorf("detecting skipped gaps: %w", err)
	}

	findLog.Infow("detected gaps from skip", "count", len(skipGaps))

	return g.DB.PersistBatch(ctx, skipGaps, heightGaps)
}

func (g *GapIndexer) detectSkippedTipSets(ctx context.Context, node lens.API) (visor.GapReportList, error) {
	reportTime := time.Now()
	// TODO unsure how big these lists will be, will likely want some sort of pagination here with limits and a loop over it
	var skippedReports []visor.ProcessingReport
	if err := g.DB.AsORM().Model(&skippedReports).Order("height desc").Where("status = ?", visor.ProcessingStatusSkip).Select(); err != nil {
		return nil, xerrors.Errorf("query processing report skips: %w", err)
	}
	gapReport := make([]*visor.GapReport, len(skippedReports))
	for idx, r := range skippedReports {
		tsgap, err := node.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(r.Height), types.EmptyTSK)
		if err != nil {
			return nil, xerrors.Errorf("getting tipset by height %d: %w", r.Height, err)
		}
		gapReport[idx] = &visor.GapReport{
			Height:     r.Height,
			TipSet:     tsgap.Key().String(),
			Task:       r.Task,
			Status:     "GAP",
			Reporter:   "gapIndexer", // TODO does this really need a name?
			ReportedAt: reportTime,
		}
	}
	return gapReport, nil
}

func (g *GapIndexer) detectProcessingGaps(ctx context.Context, node lens.API) (visor.GapReportList, error) {
	reportTime := time.Now()
	// TODO unsure how big these lists will be, will likely want some sort of pagination here with limits and a loop over it

	// build list of all reports ordered by height, scan these to find gaps in epochs
	var reportHeights []visor.ProcessingReport
	if err := g.DB.AsORM().Model(&reportHeights).
		Order("height desc").
		DistinctOn("height").
		Select(); err != nil {
		return nil, xerrors.Errorf("query processing report heights: %w", err)
	}

	gapReport := make([]*visor.GapReport, 0, len(reportHeights))
	// walk the possible gaps and query lotus to determine if gap was a null round or missed epoch.
	for _, gap := range findEpochGaps(reportHeights) {
		gh := abi.ChainEpoch(gap)
		tsgap, err := node.ChainGetTipSetByHeight(ctx, gh, types.EmptyTSK)
		if err != nil {
			return nil, xerrors.Errorf("getting tipset by height %d: %w", gh, err)
		}
		if tsgap.Height() == gh {
			for _, task := range chain.AllTasks {
				gapReport = append(gapReport, &visor.GapReport{
					Height:     int64(tsgap.Height()),
					TipSet:     tsgap.Key().String(),
					Task:       task,
					Status:     "GAP",
					Reporter:   "gapIndexer", // TODO does this really need a name?
					ReportedAt: reportTime,
				})
			}
		}
	}
	return gapReport, nil
}

// Example:
// find the missing heights, that is where there are non-sequential heights:
// 1
// 3
// 5
// 8
// missing 2,4,6,7, need to determine if they were missed or null rounds
func findEpochGaps(prs []visor.ProcessingReport) []int64 {
	if len(prs) == 0 {
		return []int64{}
	}
	maybeGap := make([]int64, 0, len(prs))
	prev := prs[0]
	for idx := 1; idx < len(prs); idx++ {
		cur := prs[idx]
		for gap := cur.Height + 1; gap < prev.Height; gap++ {
			maybeGap = append(maybeGap, gap)
		}
		prev = cur
	}
	// check if there are any epochs before the oldest processing report we have
	for idx := int64(0); idx < prs[len(prs)-1].Height; idx++ {
		maybeGap = append(maybeGap, idx)
	}
	// sort here to optimize the calls to lotus API (caching)
	sort.Slice(maybeGap, func(i, j int) bool {
		return maybeGap[i] < maybeGap[j]
	})
	return maybeGap
}
