package verifreg

import (
	"bytes"
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-hamt-ipld/v3"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/sentinel-visor/chain/actors/adt"
	"github.com/filecoin-project/sentinel-visor/chain/actors/adt/diff"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	cbg "github.com/whyrusleeping/cbor-gen"
	"go.opentelemetry.io/otel/api/global"
)

func DiffVerifiers(ctx context.Context, store adt.Store, pre, cur State) (*VerifierChanges, error) {
	ctx, span := global.Tracer("").Start(ctx, "DiffVerifiers")
	defer span.End()

	prev, err := pre.verifiers()
	if err != nil {
		return nil, err
	}

	curv, err := cur.verifiers()
	if err != nil {
		return nil, err
	}
	return diffVerifierMap(ctx, store, pre, cur, prev, curv)
}

func DiffVerifiedClients(ctx context.Context, store adt.Store, pre, cur State) (*VerifierChanges, error) {
	ctx, span := global.Tracer("").Start(ctx, "DiffVerifiedClients")
	defer span.End()

	prec, err := pre.verifiedClients()
	if err != nil {
		return nil, err
	}

	curc, err := cur.verifiedClients()
	if err != nil {
		return nil, err
	}
	return diffVerifierMap(ctx, store, pre, cur, prec, curc)
}

func diffVerifierMap(ctx context.Context, store adt.Store, pre, cur State, preM, curM adt.Map) (*VerifierChanges, error) {
	ctx, span := global.Tracer("").Start(ctx, "diffStates")
	defer span.End()

	preOpts, err := adt.MapOptsForActorCode(pre.Code())
	if err != nil {
		return nil, err
	}
	curOpts, err := adt.MapOptsForActorCode(cur.Code())
	if err != nil {
		return nil, err
	}

	diffContainer := NewVerifierDiffContainer(pre, cur)
	if mapRequiresLegacyDiffing(pre, cur, preOpts, curOpts) {
		if span.IsRecording() {
			span.SetAttribute("diff", "legacy")
		}
		if err := diff.CompareMap(preM, curM, diffContainer); err != nil {
			return nil, err
		}
		return diffContainer.Results, nil
	}
	if span.IsRecording() {
		span.SetAttribute("diff", "fast")
	}

	changes, err := diff.Hamt(ctx, preM, curM, store, store, hamt.UseHashFunction(hamt.HashFunc(preOpts.HashFunc)), hamt.UseTreeBitWidth(preOpts.Bitwidth))
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		switch change.Type {
		case hamt.Add:
			if err := diffContainer.Add(change.Key, change.After); err != nil {
				return nil, err
			}
		case hamt.Modify:
			if err := diffContainer.Modify(change.Key, change.Before, change.After); err != nil {
				return nil, err
			}
		case hamt.Remove:
			if err := diffContainer.Add(change.Key, change.Before); err != nil {
				return nil, err
			}
		}
	}

	return diffContainer.Results, nil

}

func NewVerifierDiffContainer(pre, cur State) *verifierDiffContainer {
	return &verifierDiffContainer{
		Results: new(VerifierChanges),
		pre:     pre,
		after:   cur,
	}
}

type verifierDiffContainer struct {
	Results    *VerifierChanges
	pre, after State
}

func (m *verifierDiffContainer) AsKey(key string) (abi.Keyer, error) {
	addr, err := address.NewFromBytes([]byte(key))
	if err != nil {
		return nil, err
	}
	return abi.AddrKey(addr), nil
}

func (m *verifierDiffContainer) Add(key string, val *cbg.Deferred) error {
	addr, err := address.NewFromBytes([]byte(key))
	if err != nil {
		return err
	}
	var sp abi.StoragePower
	if err := sp.UnmarshalCBOR(bytes.NewReader(val.Raw)); err != nil {
		return nil
	}
	m.Results.Added = append(m.Results.Added, VerifierInfo{
		Address: addr,
		DataCap: sp,
	})
	return nil
}

func (m *verifierDiffContainer) Modify(key string, before, after *cbg.Deferred) error {
	addr, err := address.NewFromBytes([]byte(key))
	if err != nil {
		return err
	}
	var bsp abi.StoragePower
	if err := bsp.UnmarshalCBOR(bytes.NewReader(before.Raw)); err != nil {
		return err
	}
	var asp abi.StoragePower
	if err := asp.UnmarshalCBOR(bytes.NewReader(after.Raw)); err != nil {
		return err
	}
	m.Results.Modified = append(m.Results.Modified, VerifierChange{
		Before: VerifierInfo{
			Address: addr,
			DataCap: bsp,
		},
		After: VerifierInfo{
			Address: addr,
			DataCap: asp,
		},
	})
	return nil
}

func (m *verifierDiffContainer) Remove(key string, val *cbg.Deferred) error {
	addr, err := address.NewFromBytes([]byte(key))
	if err != nil {
		return err
	}
	var sp abi.StoragePower
	if err := sp.UnmarshalCBOR(bytes.NewReader(val.Raw)); err != nil {
		return nil
	}
	m.Results.Removed = append(m.Results.Removed, VerifierInfo{
		Address: addr,
		DataCap: sp,
	})
	return nil
}

func mapRequiresLegacyDiffing(pre, cur State, pOpts, cOpts *adt.MapOpts) bool {
	// hamt/v3 cannot read hamt/v2 nodes. Their Pointers struct has changed cbor marshalers.
	if pre.Code() == builtin0.VerifiedRegistryActorCodeID {
		return true
	}
	if pre.Code() == builtin2.VerifiedRegistryActorCodeID {
		return true
	}
	if cur.Code() == builtin0.VerifiedRegistryActorCodeID {
		return true
	}
	if cur.Code() == builtin2.VerifiedRegistryActorCodeID {
		return true
	}
	// bitwidth or hashfunction differences mean legacy diffing.
	if !pOpts.Equal(cOpts) {
		return true
	}
	return false
}
