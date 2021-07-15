package testutil

import (
	"bytes"
	"context"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	itestkit "github.com/filecoin-project/lotus/itests/kit"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	cid "github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/lens"
)

type APIOpener struct {
	node itestkit.TestFullNode
}

func NewAPIOpener(node itestkit.TestFullNode) *APIOpener {
	return &APIOpener{node: node}
}

func (o *APIOpener) Open(ctx context.Context) (lens.API, lens.APICloser, error) {
	return &APIWrapper{
		TestFullNode: o.node,
		ctx:          ctx,
	}, lens.APICloser(func() {}), nil
}

type APIWrapper struct {
	itestkit.TestFullNode
	ctx context.Context
}

func (aw *APIWrapper) Store() adt.Store {
	return aw
}

func (aw *APIWrapper) GetExecutedAndBlockMessagesForTipset(ctx context.Context, ts, pts *types.TipSet) (*lens.TipSetMessages, error) {
	return nil, xerrors.Errorf("GetExecutedAndBlockMessagesForTipset is not implemented")
}

func (aw *APIWrapper) GetMessageExecutionsForTipSet(ctx context.Context, ts, pts *types.TipSet) ([]*lens.MessageExecution, error) {
	return nil, xerrors.Errorf("GetMessageExecutionsForTipSet is not implemented")
}

func (aw *APIWrapper) Get(ctx context.Context, c cid.Cid, out interface{}) error {
	cu, ok := out.(cbg.CBORUnmarshaler)
	if !ok {
		return xerrors.Errorf("out parameter does not implement CBORUnmarshaler")
	}

	// miss :(
	raw, err := aw.ChainReadObj(ctx, c)
	if err != nil {
		return xerrors.Errorf("read obj: %w", err)
	}

	if err := cu.UnmarshalCBOR(bytes.NewReader(raw)); err != nil {
		return xerrors.Errorf("unmarshal obj: %w", err)
	}

	return nil
}

func (aw *APIWrapper) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	return cid.Undef, xerrors.Errorf("put is not implemented")
}

func (aw *APIWrapper) Context() context.Context {
	return aw.ctx
}

func (aw *APIWrapper) StateGetReceipt(ctx context.Context, msg cid.Cid, from types.TipSetKey) (*types.MessageReceipt, error) {
	ml, err := aw.StateSearchMsg(ctx, from, msg, api.LookbackNoLimit, true)
	if err != nil {
		return nil, err
	}

	if ml == nil {
		return nil, nil
	}

	return &ml.Receipt, nil
}
