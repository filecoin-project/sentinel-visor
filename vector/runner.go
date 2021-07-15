package vector

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/filecoin-project/sentinel-visor/model/actors/common"
	init_ "github.com/filecoin-project/sentinel-visor/model/actors/init"
	"github.com/filecoin-project/sentinel-visor/model/actors/market"
	"github.com/filecoin-project/sentinel-visor/model/actors/multisig"
	"github.com/filecoin-project/sentinel-visor/model/actors/power"
	"github.com/filecoin-project/sentinel-visor/model/actors/reward"
	modelchain "github.com/filecoin-project/sentinel-visor/model/chain"
	"github.com/filecoin-project/sentinel-visor/model/derived"
	"github.com/filecoin-project/sentinel-visor/model/messages"
	"github.com/filecoin-project/sentinel-visor/model/msapprovals"

	"github.com/google/go-cmp/cmp"
	"github.com/ipld/go-car"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/chain/types"

	"github.com/filecoin-project/sentinel-visor/chain"
	"github.com/filecoin-project/sentinel-visor/lens"
	"github.com/filecoin-project/sentinel-visor/lens/util"
	"github.com/filecoin-project/sentinel-visor/model/actors/miner"
	"github.com/filecoin-project/sentinel-visor/model/blocks"
	"github.com/filecoin-project/sentinel-visor/storage"
)

type RunnerSchema struct {
	Meta   Metadata           `json:"metadata"`
	Params Parameters         `json:"parameters"`
	CAR    Base64EncodedBytes `json:"car"`
	Exp    RunnerExpected     `json:"expected"`
}

type Runner struct {
	schema RunnerSchema

	storage *storage.MemStorage
	bs      *util.ProxyingBlockstore

	opener lens.APIOpener
	closer lens.APICloser
}

func NewRunner(ctx context.Context, vectorPath string, cacheHint int) (*Runner, error) {
	fecVile, err := os.OpenFile(vectorPath, os.O_RDONLY, 0o644)
	if err != nil {
		return nil, err
	}
	var vs RunnerSchema
	if err := json.NewDecoder(fecVile).Decode(&vs); err != nil {
		return nil, err
	}
	// need to go from bytes representing a car file to a blockstore, then to a Lotus API.
	bs := blockstore.Blockstore(blockstore.NewMemorySync())

	// Read the base64-encoded CAR from the vector, and inflate the gzip.
	buf := bytes.NewReader(vs.CAR)
	r, err := gzip.NewReader(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to inflate gzipped CAR: %s", err)
	}
	defer r.Close() // nolint: errcheck

	// put the thing in the blockstore.
	// TODO: the entire car file is now in memory, this is an unrealistic expectation, adjust at some point.
	carHeader, err := car.LoadCar(bs, r)
	if err != nil {
		return nil, fmt.Errorf("failed to load state tree car from test vector: %s", err)
	}

	cacheDB := util.NewCachingStore(bs)

	h := func(ctx context.Context, lookback int) (*types.TipSetKey, error) {
		tsk := types.NewTipSetKey(carHeader.Roots...)
		return &tsk, nil
	}

	opener, closer, err := util.NewAPIOpener(ctx, cacheDB, h, cacheHint)
	if err != nil {
		return nil, err
	}

	return &Runner{
		schema:  vs,
		storage: storage.NewMemStorageLatest(),
		bs:      cacheDB,
		opener:  opener,
		closer:  closer,
	}, nil
}

func (r *Runner) Run(ctx context.Context) error {
	var opt []chain.TipSetIndexerOpt
	if len(r.schema.Params.AddressFilter) > 0 {
		opt = append(opt, chain.AddressFilterOpt(chain.NewAddressFilter(r.schema.Params.AddressFilter)))
	}
	tsIndexer, err := chain.NewTipSetIndexer(r.opener, r.storage, 0, "run_vector", r.schema.Params.Tasks, opt...)
	if err != nil {
		return xerrors.Errorf("setup indexer: %w", err)
	}

	if err := chain.NewWalker(tsIndexer, r.opener, r.schema.Params.From, r.schema.Params.To).Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func (r *Runner) Validate(ctx context.Context) error {
	actual := r.storage.Data
	expected := r.schema.Exp.Models

	var errStr []string
	for expTable, expData := range expected {
		if expTable == "visor_processing_reports" {
			continue
		}

		actData, ok := actual[expTable]
		if !ok {
			return xerrors.Errorf("Missing Table: %s", expTable)
		}

		diff, err := modelTypeFromTable(expTable, expData, actData)
		if err != nil {
			return err
		}

		if diff != "" {
			log.Errorf("Validate Model %s: Failed\n", expTable)
			fmt.Println(diff)
			errStr = append(errStr, fmt.Sprintf("failed to validate model: %s\n", expTable))
		} else {
			log.Infof("Validate Model %s: Passed\n", expTable)
		}
	}
	if len(errStr) > 0 {
		return xerrors.Errorf("validation failed: %s", errStr)
	}
	return nil
}

func (r *Runner) Reset() {
	r.storage = storage.NewMemStorageLatest()
	r.bs.ResetMetrics()
}

func (r *Runner) BlockstoreGetCount() int64 {
	return r.bs.GetCount()
}

func modelTypeFromTable(tableName string, expected json.RawMessage, actual []interface{}) (string, error) {
	// TODO: something with reflection someday
	switch tableName {
	default:
		return "", xerrors.Errorf("validation no implemented for model table %s", tableName)

	case "block_headers":
		var expType blocks.BlockHeaders
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType blocks.BlockHeaders
		for _, raw := range actual {
			act, ok := raw.(*blocks.BlockHeader)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "block_parents":
		var expType blocks.BlockParents
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType blocks.BlockParents
		for _, raw := range actual {
			act, ok := raw.(*blocks.BlockParent)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "drand_block_entries":
		var expType blocks.DrandBlockEntries
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType blocks.DrandBlockEntries
		for _, raw := range actual {
			act, ok := raw.(*blocks.DrandBlockEntrie)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "derived_gas_outputs":
		var expType derived.GasOutputsList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType derived.GasOutputsList
		for _, raw := range actual {
			act, ok := raw.(*derived.GasOutputs)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType, cmpopts.IgnoreUnexported(derived.GasOutputs{})), nil
	case "receipts":
		var expType messages.Receipts
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType messages.Receipts
		for _, raw := range actual {
			act, ok := raw.(*messages.Receipt)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "parsed_messages":
		var expType messages.ParsedMessages
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType messages.ParsedMessages
		for _, raw := range actual {
			act, ok := raw.(*messages.ParsedMessage)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "block_messages":
		var expType messages.BlockMessages
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType messages.BlockMessages
		for _, raw := range actual {
			act, ok := raw.(*messages.BlockMessage)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "message_gas_economy":
		var expType []*messages.MessageGasEconomy
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType []*messages.MessageGasEconomy
		for _, raw := range actual {
			act, ok := raw.(*messages.MessageGasEconomy)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType, cmpopts.IgnoreUnexported(messages.MessageGasEconomy{})), nil
	case "messages":
		var expType messages.Messages
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType messages.Messages
		for _, raw := range actual {
			act, ok := raw.(*messages.Message)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "miner_current_deadline_infos":
		var expType miner.MinerCurrentDeadlineInfoList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType miner.MinerCurrentDeadlineInfoList
		for _, raw := range actual {
			act, ok := raw.(*miner.MinerCurrentDeadlineInfo)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "miner_fee_debts":
		var expType miner.MinerFeeDebtList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType miner.MinerFeeDebtList
		for _, raw := range actual {
			act, ok := raw.(*miner.MinerFeeDebt)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "miner_locked_funds":
		var expType miner.MinerLockedFundsList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType miner.MinerLockedFundsList
		for _, raw := range actual {
			act, ok := raw.(*miner.MinerLockedFund)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "miner_pre_commit_infos":
		var expType miner.MinerPreCommitInfoList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType miner.MinerPreCommitInfoList
		for _, raw := range actual {
			act, ok := raw.(*miner.MinerPreCommitInfo)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		sort.Slice(actType, func(i, j int) bool {
			return actType[i].SectorID < actType[j].SectorID
		})
		sort.Slice(expType, func(i, j int) bool {
			return expType[i].SectorID < expType[j].SectorID
		})
		return cmp.Diff(actType, expType), nil
	case "miner_sector_events":
		var expType miner.MinerSectorEventList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType miner.MinerSectorEventList
		for _, raw := range actual {
			act, ok := raw.(*miner.MinerSectorEvent)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		sort.Slice(actType, func(i, j int) bool {
			return actType[i].SectorID < actType[j].SectorID
		})
		sort.Slice(expType, func(i, j int) bool {
			return expType[i].SectorID < expType[j].SectorID
		})
		return cmp.Diff(actType, expType), nil
	case "miner_sector_infos":
		var expType miner.MinerSectorInfoList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType miner.MinerSectorInfoList
		for _, raw := range actual {
			act, ok := raw.(*miner.MinerSectorInfo)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		sort.Slice(actType, func(i, j int) bool {
			return actType[i].SectorID < actType[j].SectorID
		})
		sort.Slice(expType, func(i, j int) bool {
			return expType[i].SectorID < expType[j].SectorID
		})
		return cmp.Diff(actType, expType), nil
	case "miner_infos":
		var expType miner.MinerInfoList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType miner.MinerInfoList
		for _, raw := range actual {
			act, ok := raw.(*miner.MinerInfo)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "miner_sector_posts":
		var expType miner.MinerSectorPostList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType miner.MinerSectorPostList
		for _, raw := range actual {
			act, ok := raw.(*miner.MinerSectorPost)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "miner_sector_deals":
		var expType miner.MinerSectorDealList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType miner.MinerSectorDealList
		for _, raw := range actual {
			act, ok := raw.(*miner.MinerSectorDeal)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "market_deal_proposals":
		var expType market.MarketDealProposals
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType market.MarketDealProposals
		for _, raw := range actual {
			act, ok := raw.(*market.MarketDealProposal)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "market_deal_states":
		var expType market.MarketDealStates
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType market.MarketDealStates
		for _, raw := range actual {
			act, ok := raw.(*market.MarketDealState)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "multisig_transactions":
		var expType multisig.MultisigTransactionList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType multisig.MultisigTransactionList
		for _, raw := range actual {
			act, ok := raw.(*multisig.MultisigTransaction)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "chain_powers":
		var expType power.ChainPowerList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType power.ChainPowerList
		for _, raw := range actual {
			act, ok := raw.(*power.ChainPower)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "power_actor_claims":
		var expType power.PowerActorClaimList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType power.PowerActorClaimList
		for _, raw := range actual {
			act, ok := raw.(*power.PowerActorClaim)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		sort.Slice(actType, func(i, j int) bool {
			return actType[i].MinerID < actType[j].MinerID
		})
		sort.Slice(expType, func(i, j int) bool {
			return expType[i].MinerID < expType[j].MinerID
		})
		return cmp.Diff(actType, expType), nil
	case "chain_rewards":
		var expType []*reward.ChainReward
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType []*reward.ChainReward
		for _, raw := range actual {
			act, ok := raw.(*reward.ChainReward)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "actors":
		var expType common.ActorList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType common.ActorList
		for _, raw := range actual {
			act, ok := raw.(*common.Actor)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "actor_states":
		var expType common.ActorStateList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType common.ActorStateList
		for _, raw := range actual {
			act, ok := raw.(*common.ActorState)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType), nil
	case "id_addresses":
		var expType init_.IdAddressList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType init_.IdAddressList
		for _, raw := range actual {
			act, ok := raw.(*init_.IdAddress)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		sort.Slice(actType, func(i, j int) bool {
			return actType[i].ID < actType[j].ID
		})
		sort.Slice(expType, func(i, j int) bool {
			return expType[i].ID < expType[j].ID
		})
		return cmp.Diff(actType, expType), nil
	case "chain_economics":
		var expType modelchain.ChainEconomicsList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType modelchain.ChainEconomicsList
		for _, raw := range actual {
			act, ok := raw.(*modelchain.ChainEconomics)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType, cmpopts.IgnoreUnexported(modelchain.ChainEconomics{})), nil
	case "multisig_approvals":
		var expType msapprovals.MultisigApprovalList
		if err := json.Unmarshal(expected, &expType); err != nil {
			return "", err
		}

		var actType msapprovals.MultisigApprovalList
		for _, raw := range actual {
			act, ok := raw.(*msapprovals.MultisigApproval)
			if !ok {
				panic("developer error")
			}
			actType = append(actType, act)
		}
		return cmp.Diff(actType, expType, cmpopts.IgnoreUnexported(msapprovals.MultisigApproval{})), nil
	}
}
