package lily

import (
	"context"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/v0api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/sentinel-visor/lens"
	"github.com/filecoin-project/sentinel-visor/schedule"
)

var log = logging.Logger("visor/lens/lily")

var _ LilyAPI = (*LilyAPIStruct)(nil)

type LilyAPIStruct struct {
	// authentication
	// TODO: avoid importing CommonStruct, split out into separate visor structs
	v0api.CommonStruct

	Internal struct {
		Store                                func() adt.Store                                                                  `perm:"read"`
		GetExecutedAndBlockMessagesForTipset func(context.Context, *types.TipSet, *types.TipSet) (*lens.TipSetMessages, error) `perm:"read"`

		LilyWatch   func(context.Context, *LilyWatchConfig) (schedule.JobID, error)   `perm:"read"`
		LilyWalk    func(context.Context, *LilyWalkConfig) (schedule.JobID, error)    `perm:"read"`
		LilyObserve func(context.Context, *LilyObserveConfig) (schedule.JobID, error) `perm:"read"`

		LilyJobStart func(ctx context.Context, ID schedule.JobID) error      `perm:"read"`
		LilyJobStop  func(ctx context.Context, ID schedule.JobID) error      `perm:"read"`
		LilyJobList  func(ctx context.Context) ([]schedule.JobResult, error) `perm:"read"`

		Shutdown func(context.Context) error `perm:"read"`

		SyncState func(ctx context.Context) (*api.SyncState, error) `perm:"read"`
		ChainHead func(context.Context) (*types.TipSet, error)      `perm:"read"`

		LogList     func(context.Context) ([]string, error)     `perm:"read"`
		LogSetLevel func(context.Context, string, string) error `perm:"read"`

		ID               func(context.Context) (peer.ID, error)                        `perm:"read"`
		NetAutoNatStatus func(context.Context) (api.NatInfo, error)                    `perm:"read"`
		NetPeers         func(context.Context) ([]peer.AddrInfo, error)                `perm:"read"`
		NetAddrsListen   func(context.Context) (peer.AddrInfo, error)                  `perm:"read"`
		NetPubsubScores  func(context.Context) ([]api.PubsubScore, error)              `perm:"read"`
		NetAgentVersion  func(ctx context.Context, p peer.ID) (string, error)          `perm:"read"`
		NetPeerInfo      func(context.Context, peer.ID) (*api.ExtendedPeerInfo, error) `perm:"read"`
	}
}

func (s *LilyAPIStruct) Store() adt.Store {
	return s.Internal.Store()
}

func (s *LilyAPIStruct) LilyWatch(ctx context.Context, cfg *LilyWatchConfig) (schedule.JobID, error) {
	return s.Internal.LilyWatch(ctx, cfg)
}

func (s *LilyAPIStruct) LilyWalk(ctx context.Context, cfg *LilyWalkConfig) (schedule.JobID, error) {
	return s.Internal.LilyWalk(ctx, cfg)
}

func (s *LilyAPIStruct) LilyObserve(ctx context.Context, cfg *LilyObserveConfig) (schedule.JobID, error) {
	return s.Internal.LilyObserve(ctx, cfg)
}

func (s *LilyAPIStruct) LilyJobStart(ctx context.Context, ID schedule.JobID) error {
	return s.Internal.LilyJobStart(ctx, ID)
}

func (s *LilyAPIStruct) LilyJobStop(ctx context.Context, ID schedule.JobID) error {
	return s.Internal.LilyJobStop(ctx, ID)
}

func (s *LilyAPIStruct) LilyJobList(ctx context.Context) ([]schedule.JobResult, error) {
	return s.Internal.LilyJobList(ctx)
}

func (s *LilyAPIStruct) Shutdown(ctx context.Context) error {
	return s.Internal.Shutdown(ctx)
}

func (s *LilyAPIStruct) GetExecutedAndBlockMessagesForTipset(ctx context.Context, ts, pts *types.TipSet) (*lens.TipSetMessages, error) {
	return s.Internal.GetExecutedAndBlockMessagesForTipset(ctx, ts, pts)
}

func (s *LilyAPIStruct) SyncState(ctx context.Context) (*api.SyncState, error) {
	return s.Internal.SyncState(ctx)
}

func (s *LilyAPIStruct) ChainHead(ctx context.Context) (*types.TipSet, error) {
	return s.Internal.ChainHead(ctx)
}

func (s *LilyAPIStruct) ID(ctx context.Context) (peer.ID, error) {
	return s.Internal.ID(ctx)
}

func (s *LilyAPIStruct) LogList(ctx context.Context) ([]string, error) {
	return s.Internal.LogList(ctx)
}

func (s *LilyAPIStruct) LogSetLevel(ctx context.Context, subsystem, level string) error {
	return s.Internal.LogSetLevel(ctx, subsystem, level)
}

func (s *LilyAPIStruct) NetAutoNatStatus(ctx context.Context) (api.NatInfo, error) {
	return s.Internal.NetAutoNatStatus(ctx)
}

func (s *LilyAPIStruct) NetPeers(ctx context.Context) ([]peer.AddrInfo, error) {
	return s.Internal.NetPeers(ctx)
}

func (s *LilyAPIStruct) NetAddrsListen(ctx context.Context) (peer.AddrInfo, error) {
	return s.Internal.NetAddrsListen(ctx)
}

func (s *LilyAPIStruct) NetPubsubScores(ctx context.Context) ([]api.PubsubScore, error) {
	return s.Internal.NetPubsubScores(ctx)
}

func (s *LilyAPIStruct) NetAgentVersion(ctx context.Context, p peer.ID) (string, error) {
	return s.Internal.NetAgentVersion(ctx, p)
}

func (s *LilyAPIStruct) NetPeerInfo(ctx context.Context, p peer.ID) (*api.ExtendedPeerInfo, error) {
	return s.Internal.NetPeerInfo(ctx, p)
}
