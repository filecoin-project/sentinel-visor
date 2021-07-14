package gap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/sentinel-visor/chain"
	"github.com/filecoin-project/sentinel-visor/lens"
	"github.com/filecoin-project/sentinel-visor/model/visor"
	"github.com/filecoin-project/sentinel-visor/storage"
	"github.com/go-pg/pg/v10"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"
)

var log = logging.Logger("visor/gap")

type GapFiller struct {
	DB     *storage.Database
	opener lens.APIOpener
	tasks  []string
}

func NewGapFiller(o lens.APIOpener, db *storage.Database, tasks []string) *GapFiller {
	return &GapFiller{
		DB:     db,
		opener: o,
		tasks:  tasks,
	}
}

func (g *GapFiller) Run(ctx context.Context) error {
	gaps, err := g.queryGaps(ctx)
	if err != nil {
		return err
	}
	fillLog := log.With("type", "fill")
	fillLog.Infow("run", "count", len(gaps))

	filledGaps := make([]*visor.GapReport, len(gaps))
	for idx, gap := range gaps {
		// TODO we could optimize here by collecting all gaps at same height and launching a single instance of the walker for them
		indexer, err := chain.NewTipSetIndexer(g.opener, g.DB, 0, fmt.Sprintf("gapfill_%d", time.Now().UTC().Unix()), []string{gap.Task})
		if err != nil {
			gap.Status = err.Error()
			log.Errorw("fill failed", "height", gap.Height, "error", err.Error())

		} else {
			walker := chain.NewWalker(indexer, g.opener, gap.Height, gap.Height)
			if err := walker.Run(ctx); err != nil {
				gap.Status = err.Error()
				log.Errorw("fill failed", "height", gap.Height, "error", err.Error())
			} else {
				gap.Status = "FILLED"
				fillLog.Infow("fill success", "height", gap.Height, "remaining", len(gaps)-idx)
			}
		}

		filledGaps[idx] = gap
	}
	return g.updateGaps(ctx, gaps)
}

func (g *GapFiller) queryGaps(ctx context.Context) ([]*visor.GapReport, error) {
	var out []*visor.GapReport
	if len(g.tasks) != 0 {
		if err := g.DB.AsORM().Model(&out).
			Order("height desc").
			Where("status = ?", "GAP").
			Where("task = ANY (?)", pg.Array(g.tasks)).
			Select(); err != nil {
			return nil, xerrors.Errorf("querying gap reports: %w", err)
		}
	} else {
		if err := g.DB.AsORM().Model(&out).
			Order("height desc").
			Where("status = ?", "GAP").
			Select(); err != nil {
			return nil, xerrors.Errorf("querying gap reports: %w", err)
		}
	}
	return out, nil
}

func (g *GapFiller) updateGaps(ctx context.Context, gaps visor.GapReportList) error {
	if len(gaps) == 0 {
		return nil
	}
	_, err := g.DB.AsORM().Model(&gaps).WherePK().Update()
	return err
}

func tipSetKeyFromString(tss string) (types.TipSetKey, error) {
	// remove the brackets, eg:
	// From: {bafy2bzacec6trnklsegcoiean7fov4k4rnlxgnhpcoa7w2rfuecy7pp4vrhyu,bafy2bzacecs57ibm2ynmt2rdkmyuz4cclztvxjcq465wqymcu3zmcgvf4h4su}
	// To: bafy2bzacec6trnklsegcoiean7fov4k4rnlxgnhpcoa7w2rfuecy7pp4vrhyu,bafy2bzacecs57ibm2ynmt2rdkmyuz4cclztvxjcq465wqymcu3zmcgvf4h4su
	tss = strings.TrimLeft(tss, "{")
	tss = strings.TrimRight(tss, "}")
	// tokenize
	// From: bafy2bzacec6trnklsegcoiean7fov4k4rnlxgnhpcoa7w2rfuecy7pp4vrhyu,bafy2bzacecs57ibm2ynmt2rdkmyuz4cclztvxjcq465wqymcu3zmcgvf4h4su
	// To:
	// tss[0] = bafy2bzacec6trnklsegcoiean7fov4k4rnlxgnhpcoa7w2rfuecy7pp4vrhyu
	// tss[1] = bafy2bzacecs57ibm2ynmt2rdkmyuz4cclztvxjcq465wqymcu3zmcgvf4h4su
	tokens := strings.Split(tss, ",")
	cids := make([]cid.Cid, len(tokens))
	var err error
	for idx, token := range tokens {
		cids[idx], err = cid.Decode(token)
		if err != nil {
			return types.EmptyTSK, err
		}
	}
	return types.NewTipSetKey(cids...), nil
}
