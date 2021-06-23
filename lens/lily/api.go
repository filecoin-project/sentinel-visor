package lily

import (
	"context"
	"time"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/sentinel-visor/schedule"
)

type LilyAPI interface {
	// NOTE: when adding daemon methods here, don't forget to add to the implementation to LilyAPIStruct too

	AuthVerify(ctx context.Context, token string) ([]auth.Permission, error)

	LilyWatch(ctx context.Context, cfg *LilyWatchConfig) (schedule.JobID, error)
	LilyWalk(ctx context.Context, cfg *LilyWalkConfig) (schedule.JobID, error)
	LilyObserve(ctx context.Context, cfg *LilyObserveConfig) (schedule.JobID, error)

	LilyJobStart(ctx context.Context, ID schedule.JobID) error
	LilyJobStop(ctx context.Context, ID schedule.JobID) error
	LilyJobList(ctx context.Context) ([]schedule.JobResult, error)

	// SyncState returns the current status of the chain sync system.
	SyncState(context.Context) (*api.SyncState, error) //perm:read

	ChainHead(context.Context) (*types.TipSet, error) //perm:read

	// trigger graceful shutdown
	Shutdown(context.Context) error

	// LogList returns a list of loggers
	LogList(context.Context) ([]string, error)         //perm:write
	LogSetLevel(context.Context, string, string) error //perm:write

	// ID returns peerID of libp2p node backing this API
	ID(context.Context) (peer.ID, error) //perm:read
	NetAutoNatStatus(ctx context.Context) (i api.NatInfo, err error)
	NetPeers(context.Context) ([]peer.AddrInfo, error)
	NetAddrsListen(context.Context) (peer.AddrInfo, error)
	NetPubsubScores(context.Context) ([]api.PubsubScore, error)
	NetAgentVersion(ctx context.Context, p peer.ID) (string, error)
	NetPeerInfo(context.Context, peer.ID) (*api.ExtendedPeerInfo, error)
}

type LilyWatchConfig struct {
	Name                string
	Tasks               []string
	Window              time.Duration
	Confidence          int
	RestartOnFailure    bool
	RestartOnCompletion bool
	RestartDelay        time.Duration
	Storage             string // name of storage system to use, may be empty
}

type LilyWalkConfig struct {
	From                int64
	To                  int64
	Name                string
	Tasks               []string
	Window              time.Duration
	RestartOnFailure    bool
	RestartOnCompletion bool
	RestartDelay        time.Duration
	Storage             string // name of storage system to use, may be empty
}

type LilyObserveConfig struct {
	Name                string
	Tasks               []string
	Interval            time.Duration
	RestartOnFailure    bool
	RestartOnCompletion bool
	RestartDelay        time.Duration
	Storage             string // name of storage system to use, may be empty
}
