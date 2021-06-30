package chain

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/sentinel-visor/chain/actors/builtin"
	"github.com/filecoin-project/sentinel-visor/model/registry"

	itestkit "github.com/filecoin-project/lotus/itests/kit"
	"github.com/go-pg/pg/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/sentinel-visor/model/blocks"
	"github.com/filecoin-project/sentinel-visor/storage"
	"github.com/filecoin-project/sentinel-visor/testutil"
)

func TestWalker(t *testing.T) {
	if testing.Short() {
		t.Skip("short testing requested")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	db, cleanup, err := testutil.WaitForExclusiveDatabase(ctx, t)
	require.NoError(t, err)
	defer func() { require.NoError(t, cleanup()) }()

	t.Logf("truncating database tables")
	err = truncateBlockTables(t, db)
	require.NoError(t, err, "truncating tables")

	t.Logf("preparing chain")
	nodes, sn := itestkit.RPCMockMinerBuilder(t, itestkit.OneFull, itestkit.OneMiner)

	node := nodes[0]
	opener := testutil.NewAPIOpener(node)

	openedAPI, _, _ := opener.Open(ctx)

	bm := itestkit.NewBlockMiner(t, sn[0])
	bm.MineUntilBlock(ctx, node, nil)

	head, err := node.ChainHead(ctx)
	require.NoError(t, err, "chain head")

	t.Logf("collecting chain blocks")
	bhs, err := collectBlockHeaders(openedAPI, head)
	require.NoError(t, err, "collect chain blocks")

	cids := bhs.Cids()
	rounds := bhs.Rounds()

	strg, err := storage.NewDatabaseFromDB(ctx, db, "public")
	require.NoError(t, err, "NewDatabaseFromDB")

	tsIndexer, err := NewTipSetIndexer(opener, strg, builtin.EpochDurationSeconds*time.Second, t.Name(), []string{registry.BlocksTask})
	require.NoError(t, err, "NewTipSetIndexer")
	t.Logf("initializing indexer")
	idx := NewWalker(tsIndexer, opener, 0, int64(head.Height()))

	t.Logf("indexing chain")
	err = idx.WalkChain(ctx, openedAPI, head)
	require.NoError(t, err, "WalkChain")

	// TODO NewTipSetIndexer runs its processors in their own go routines (started when TipSet() is called)
	// this causes this test to behave nondeterministicly so we sleep here to ensure all async jobs
	// have completed before asserting results
	time.Sleep(time.Second * 3)

	t.Run("block_headers", func(t *testing.T) {
		var count int
		_, err := db.QueryOne(pg.Scan(&count), `SELECT COUNT(*) FROM block_headers`)
		require.NoError(t, err)
		assert.Equal(t, len(cids), count)

		var m *blocks.BlockHeader
		for _, cid := range cids {
			exists, err := db.Model(m).Where("cid = ?", cid).Exists()
			require.NoError(t, err)
			assert.True(t, exists, "cid: %s", cid)
		}
	})

	t.Run("block_parents", func(t *testing.T) {
		var count int
		_, err := db.QueryOne(pg.Scan(&count), `SELECT COUNT(*) FROM block_parents`)
		require.NoError(t, err)
		assert.Equal(t, len(cids), count)

		var m *blocks.BlockParent
		for _, cid := range cids {
			exists, err := db.Model(m).Where("block = ?", cid).Exists()
			require.NoError(t, err)
			assert.True(t, exists, "block: %s", cid)
		}
	})

	t.Run("drand_block_entries", func(t *testing.T) {
		var count int
		_, err := db.QueryOne(pg.Scan(&count), `SELECT COUNT(*) FROM drand_block_entries`)
		require.NoError(t, err)
		assert.Equal(t, len(rounds), count)

		var m *blocks.DrandBlockEntrie
		for _, round := range rounds {
			exists, err := db.Model(m).Where("round = ?", round).Exists()
			require.NoError(t, err)
			assert.True(t, exists, "round: %d", round)
		}
	})
}
