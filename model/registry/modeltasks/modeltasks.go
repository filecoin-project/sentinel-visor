package modeltasks

import (
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/model"

	"github.com/filecoin-project/sentinel-visor/model/actors/multisig"

	"github.com/filecoin-project/sentinel-visor/model/actors/reward"

	"github.com/filecoin-project/sentinel-visor/model/blocks"

	"github.com/filecoin-project/sentinel-visor/model/chain"

	"github.com/filecoin-project/sentinel-visor/model/messages"

	"github.com/filecoin-project/sentinel-visor/model/actors/common"

	"github.com/filecoin-project/sentinel-visor/model/actors/miner"

	"github.com/filecoin-project/sentinel-visor/model/actors/power"

	"github.com/filecoin-project/sentinel-visor/model/derived"

	"github.com/filecoin-project/sentinel-visor/model/msapprovals"

	"github.com/filecoin-project/sentinel-visor/model/actors/init_"

	"github.com/filecoin-project/sentinel-visor/model/actors/market"
)

func ModelForString(name string) (model.Persistable, error) {
	switch name {

	case "actor":
		return &common.Actor{}, nil

	case "actor_state":
		return &common.ActorState{}, nil

	case "id_address":
		return &init_.IdAddress{}, nil

	case "market_deal_proposal":
		return &market.MarketDealProposal{}, nil

	case "market_deal_state":
		return &market.MarketDealState{}, nil

	case "miner_current_deadline_info":
		return &miner.MinerCurrentDeadlineInfo{}, nil

	case "miner_fee_debt":
		return &miner.MinerFeeDebt{}, nil

	case "miner_locked_fund":
		return &miner.MinerLockedFund{}, nil

	case "miner_info":
		return &miner.MinerInfo{}, nil

	case "miner_pre_commit_info":
		return &miner.MinerPreCommitInfo{}, nil

	case "miner_sector_info":
		return &miner.MinerSectorInfo{}, nil

	case "miner_sector_deal":
		return &miner.MinerSectorDeal{}, nil

	case "miner_sector_event":
		return &miner.MinerSectorEvent{}, nil

	case "miner_sector_post":
		return &miner.MinerSectorPost{}, nil

	case "multisig_transaction":
		return &multisig.MultisigTransaction{}, nil

	case "chain_power":
		return &power.ChainPower{}, nil

	case "power_actor_claim":
		return &power.PowerActorClaim{}, nil

	case "chain_reward":
		return &reward.ChainReward{}, nil

	case "drand_block_entrie":
		return &blocks.DrandBlockEntrie{}, nil

	case "block_header":
		return &blocks.BlockHeader{}, nil

	case "block_parent":
		return &blocks.BlockParent{}, nil

	case "chain_economics":
		return &chain.ChainEconomics{}, nil

	case "gas_outputs":
		return &derived.GasOutputs{}, nil

	case "block_message":
		return &messages.BlockMessage{}, nil

	case "message_gas_economy":
		return &messages.MessageGasEconomy{}, nil

	case "internal_message":
		return &messages.InternalMessage{}, nil

	case "message":
		return &messages.Message{}, nil

	case "parsed_message":
		return &messages.ParsedMessage{}, nil

	case "receipt":
		return &messages.Receipt{}, nil

	case "multisig_approval":
		return &msapprovals.MultisigApproval{}, nil

	default:
		return nil, xerrors.Errorf("no model type for: %s", name)
	}
}
