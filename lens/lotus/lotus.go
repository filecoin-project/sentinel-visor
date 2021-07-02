package lotus

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/api/v0api"
	"github.com/filecoin-project/lotus/node/repo"
	lru "github.com/hashicorp/golang-lru"
	"github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/lens"
)

type APIOpener struct {
	cache   *lru.ARCCache // cache shared across all instances of the api
	addr    string
	headers http.Header
}

func NewAPIOpener(cctx *cli.Context, cacheSize int) (*APIOpener, lens.APICloser, error) {
	ac, err := lru.NewARC(cacheSize)
	if err != nil {
		return nil, nil, xerrors.Errorf("new arc cache: %w", err)
	}

	var rawaddr, rawtoken string

	if cctx.IsSet("lens-lotus-api") {
		tokenMaddr := cctx.String("lens-lotus-api")
		toks := strings.Split(tokenMaddr, ":")
		if len(toks) != 2 {
			return nil, nil, fmt.Errorf("invalid api tokens, expected <token>:<maddr>, got: %s", tokenMaddr)
		}

		rawtoken = toks[0]
		rawaddr = toks[1]
	} else if cctx.IsSet("lens-repo") {
		repoPath := cctx.String("lens-repo")
		p, err := homedir.Expand(repoPath)
		if err != nil {
			return nil, nil, xerrors.Errorf("expand home dir (%s): %w", repoPath, err)
		}

		r, err := repo.NewFS(p)
		if err != nil {
			return nil, nil, xerrors.Errorf("open repo at path: %s; %w", p, err)
		}

		ma, err := r.APIEndpoint()
		if err != nil {
			return nil, nil, xerrors.Errorf("api endpoint: %w", err)
		}

		token, err := r.APIToken()
		if err != nil {
			return nil, nil, xerrors.Errorf("api token: %w", err)
		}

		rawaddr = ma.String()
		rawtoken = string(token)
	} else {
		return nil, nil, xerrors.Errorf("cannot connect to lotus api: missing --lens-lotus-api or --lens-repo flags")
	}

	parsedAddr, err := ma.NewMultiaddr(rawaddr)
	if err != nil {
		return nil, nil, xerrors.Errorf("parse listen address: %w", err)
	}

	_, addr, err := manet.DialArgs(parsedAddr)
	if err != nil {
		return nil, nil, xerrors.Errorf("dial multiaddress: %w", err)
	}

	o := &APIOpener{
		cache:   ac,
		addr:    apiURI(addr),
		headers: apiHeaders(rawtoken),
	}

	return o, lens.APICloser(func() {}), nil
}

func (o *APIOpener) Open(ctx context.Context) (lens.API, lens.APICloser, error) {
	apiv1, closer, err := client.NewFullNodeRPCV1(ctx, o.addr, o.headers)
	if err != nil {
		return nil, nil, xerrors.Errorf("new full node rpc: %w", err)
	}

	api := &v0api.WrapperV1Full{FullNode: apiv1}

	cacheStore, err := NewCacheCtxStore(ctx, api, o.cache)
	if err != nil {
		if closer != nil {
			closer()
		}
		return nil, nil, xerrors.Errorf("new cache store: %w", err)
	}

	lensAPI := NewAPIWrapper(api, cacheStore)

	return lensAPI, lens.APICloser(closer), nil
}

func (o *APIOpener) Daemonized() bool {
	return false
}

func apiURI(addr string) string {
	return "ws://" + addr + "/rpc/v0"
}

func apiHeaders(token string) http.Header {
	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+token)
	return headers
}
