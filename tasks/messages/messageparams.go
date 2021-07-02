package messages

import (
	"bytes"
	"fmt"

	sa0builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	sa2builtin "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	sa3builtin "github.com/filecoin-project/specs-actors/v3/actors/builtin"
	sa4builtin "github.com/filecoin-project/specs-actors/v4/actors/builtin"
	sa5builtin "github.com/filecoin-project/specs-actors/v5/actors/builtin"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"

	"github.com/filecoin-project/sentinel-visor/tasks/messages/types"
)

type methodMeta struct {
	Name string
	ipld.NodePrototype
}
type methodtable map[int64]methodMeta

var initTable = methodtable{
	2: methodMeta{"InitExecParams", types.Type.MessageParamsInitExecParams__Repr},
}

var marketTable = methodtable{
	2: methodMeta{"AddBalance", types.Type.Address__Repr},
	3: methodMeta{"WithdrawBalance", types.Type.MessageParamsMarketWithdrawBalance__Repr},
	4: methodMeta{"PublishStorageDeals", types.Type.MessageParamsMarketPublishDeals__Repr},
	5: methodMeta{"VerifyDealsForActivation", types.Type.MessageParamsMarketVerifyDeals__Repr},
	6: methodMeta{"ActivateDeals", types.Type.MessageParamsMarketActivateDeals__Repr},
	7: methodMeta{"OnMinerSectorsTerminate", types.Type.MessageParamsMarketTerminateDeals__Repr},
	8: methodMeta{"ComputeDataCommitment", types.Type.MessageParamsMarketComputeCommitment__Repr},
	9: methodMeta{"CronTick", types.Type.Any__Repr},
}

// marketV5Table is needed since param type of ComputeDataCommitment has changed in v5 actors
var marketV5Table = methodtable{
	2: methodMeta{"AddBalance", types.Type.Address__Repr},
	3: methodMeta{"WithdrawBalance", types.Type.MessageParamsMarketWithdrawBalance__Repr},
	4: methodMeta{"PublishStorageDeals", types.Type.MessageParamsMarketPublishDeals__Repr},
	5: methodMeta{"VerifyDealsForActivation", types.Type.MessageParamsMarketVerifyDeals__Repr},
	6: methodMeta{"ActivateDeals", types.Type.MessageParamsMarketActivateDeals__Repr},
	7: methodMeta{"OnMinerSectorsTerminate", types.Type.MessageParamsMarketTerminateDeals__Repr},
	8: methodMeta{"ComputeDataCommitment", types.Type.MessageParamsMarketV5ComputeCommitment__Repr},
	9: methodMeta{"CronTick", types.Type.Any__Repr},
}

var minerTable = methodtable{
	1:  methodMeta{"Constructor", types.Type.MessageParamsMinerConstructor__Repr},
	2:  methodMeta{"ControlAddresses", types.Type.Any__Repr},
	3:  methodMeta{"ChangeWorkerAddress", types.Type.MessageParamsMinerChangeAddress__Repr},
	4:  methodMeta{"ChangePeerID", types.Type.MessageParamsMinerChangePeerID__Repr},
	5:  methodMeta{"SubmitWindowedPoSt", types.Type.MessageParamsMinerSubmitWindowedPoSt__Repr},
	6:  methodMeta{"PreCommitSector", types.Type.MinerV0SectorPreCommitInfo__Repr},
	7:  methodMeta{"ProveCommitSector", types.Type.MessageParamsMinerProveCommitSector__Repr},
	8:  methodMeta{"ExtendSectorExpiration", types.Type.MessageParamsMinerExtendSectorExpiration__Repr},
	9:  methodMeta{"TerminateSectors", types.Type.MessageParamsMinerTerminateSectors__Repr},
	10: methodMeta{"DeclareFaults", types.Type.MessageParamsMinerDeclareFaults__Repr},
	11: methodMeta{"DeclareFaultsRecovered", types.Type.MessageParamsMinerDeclareFaultsRecovered__Repr},
	12: methodMeta{"OnDeferredCronEvent", types.Type.MessageParamsMinerDeferredCron__Repr},
	13: methodMeta{"CheckSectorProven", types.Type.MessageParamsMinerCheckSectorProven__Repr},
	14: methodMeta{"ApplyRewards", types.Type.ApplyRewardParams__Repr},
	15: methodMeta{"ReportConsensusFault", types.Type.MessageParamsMinerReportFault__Repr},
	16: methodMeta{"WithdrawBalance", types.Type.MessageParamsMinerWithdrawBalance__Repr},
	17: methodMeta{"ConfirmSectorProofsValid", types.Type.MessageParamsMinerConfirmSectorProofs__Repr},
	18: methodMeta{"ChangeMultiaddrs", types.Type.MessageParamsMinerChangeMultiaddrs__Repr},
	19: methodMeta{"CompactPartitions", types.Type.MessageParamsMinerCompactPartitions__Repr},
	20: methodMeta{"CompactSectorNumbers", types.Type.MessageParamsMinerCompactSectorNumbers__Repr},
	21: methodMeta{"ConfirmUpdateWorkerKey", types.Type.Any__Repr},
	22: methodMeta{"RepayDebt", types.Type.Any__Repr},
	23: methodMeta{"ChangeOwnerAddress", types.Type.Address__Repr},

	// 24 added in v4 actors
	24: methodMeta{"DisputeWindowedPoSt", types.Type.MessageParamsMinerDisputeWindowedPoSt__Repr},

	// 25 and 26 added in v5 actors
	25: methodMeta{"PreCommitSectorBatch", types.Type.MessageParamsMinerPreCommitSectorBatch__Repr},
	26: methodMeta{"ProveCommitAggregate", types.Type.MessageParamsMinerProveCommitAggregate__Repr},
}

var multisigTable = methodtable{
	1: methodMeta{"Constructor", types.Type.MessageParamsMultisigConstructor__Repr},
	2: methodMeta{"Propose", types.Type.MessageParamsMultisigPropose__Repr},
	3: methodMeta{"Approve", types.Type.MessageParamsMultisigTxnID__Repr},
	4: methodMeta{"Cancel", types.Type.MessageParamsMultisigTxnID__Repr},
	5: methodMeta{"AddSigner", types.Type.MessageParamsMultisigAddSigner__Repr},
	6: methodMeta{"RemoveSigner", types.Type.MessageParamsMultisigRemoveSigner__Repr},
	7: methodMeta{"SwapSigner", types.Type.MessageParamsMultisigSwapSigner__Repr},
	8: methodMeta{"ChangeNumApprovalsThreshold", types.Type.MessageParamsMultisigChangeThreshold__Repr},
	9: methodMeta{"LockBalance", types.Type.MessageParamsMultisigLockBalance__Repr},
}

var paychTable = methodtable{
	1: methodMeta{"Constructor", types.Type.MessageParamsPaychConstructor__Repr},
	2: methodMeta{"UpdateChannelState", types.Type.MessageParamsPaychUpdateChannelState__Repr},
	3: methodMeta{"Settle", types.Type.Any__Repr},
	4: methodMeta{"Collect", types.Type.Any__Repr},
}

var powerTable = methodtable{
	1: methodMeta{"Constructor", types.Type.Any__Repr},
	2: methodMeta{"CreateMiner", types.Type.MessageParamsPowerCreateMiner__Repr},
	3: methodMeta{"UpdateClaimedPower", types.Type.MessageParamsPowerUpdateClaimed__Repr},
	4: methodMeta{"EnrollCronEvent", types.Type.MessageParamsPowerEnrollCron__Repr},
	5: methodMeta{"OnEpochTickEnd", types.Type.Any__Repr},
	6: methodMeta{"UpdatePledgeTotal", types.Type.BigInt__Repr},
	7: methodMeta{"Nil", types.Type.Any__Repr}, // deprecated
	8: methodMeta{"SubmitPoRepForBulkVerify", types.Type.SealVerifyInfo__Repr},
	9: methodMeta{"CurrentTotalPower", types.Type.MessageParamsPowerCurrentTotal__Repr},
}

var rewardTable = methodtable{
	1: methodMeta{"Constructor", types.Type.BigInt__Repr},
	2: methodMeta{"AwardBlockRewards", types.Type.MessageParamsRewardAwardBlock__Repr},
	3: methodMeta{"ThisEpochReward", types.Type.Any__Repr},
	4: methodMeta{"UpdateNetworkKPI", types.Type.BigInt__Repr},
}

var verifregTable = methodtable{
	1: methodMeta{"Constructor", types.Type.Address__Repr},
	2: methodMeta{"AddVerifier", types.Type.MessageParamsVerifregAddVerifier__Repr},
	3: methodMeta{"RemoveVerifier", types.Type.Address__Repr},
	4: methodMeta{"AddVerifiedClient", types.Type.MessageParamsVerifregAddVerifier__Repr},
	5: methodMeta{"UseBytes", types.Type.MessageParamsVerifregUseBytes__Repr},
	6: methodMeta{"RestoreBytes", types.Type.MessageParamsVerifregUseBytes__Repr},
}

var cronTable = methodtable{
	1: methodMeta{"Constructor", types.Type.Any__Repr},
	2: methodMeta{"EpochTick", types.Type.Any__Repr},
}

// LotusType represents known types
type LotusType string

const (
	LotusTypeUnknown             LotusType = "unknown"
	AccountActorState            LotusType = "accountActor"
	CronActorState               LotusType = "cronActor"
	InitActorState               LotusType = "initActor"
	InitActorV3State             LotusType = "initActorV3"
	MarketActorState             LotusType = "storageMarketActor"
	MarketActorV2State           LotusType = "storageMarketActorV2"
	MarketActorV3State           LotusType = "storageMarketActorV3"
	MultisigActorState           LotusType = "multisigActor"
	MultisigActorV3State         LotusType = "multisigActorV3"
	PaymentChannelActorState     LotusType = "paymentChannelActor"
	PaymentChannelActorV3State   LotusType = "paymentChannelActorV3"
	RewardActorState             LotusType = "rewardActor"
	RewardActorV2State           LotusType = "rewardActorV2"
	StorageMinerActorState       LotusType = "storageMinerActor"
	StorageMinerActorV2State     LotusType = "storageMinerActorV2"
	StorageMinerActorV3State     LotusType = "storageMinerActorV3"
	StorageMinerActorV4State     LotusType = "storageMinerActorV4"
	StoragePowerActorState       LotusType = "storagePowerActor"
	StoragePowerActorV2State     LotusType = "storagePowerActorV2"
	StoragePowerActorV3State     LotusType = "storagePowerActorV3"
	VerifiedRegistryActorState   LotusType = "verifiedRegistryActor"
	VerifiedRegistryActorV3State LotusType = "verifiedRegistryActorV3"
)

var messageParamTable = map[cid.Cid]methodtable{
	sa0builtin.AccountActorCodeID:          {},
	sa0builtin.CronActorCodeID:             cronTable,
	sa0builtin.InitActorCodeID:             initTable,
	sa0builtin.MultisigActorCodeID:         multisigTable,
	sa0builtin.PaymentChannelActorCodeID:   paychTable,
	sa0builtin.RewardActorCodeID:           rewardTable,
	sa0builtin.StorageMarketActorCodeID:    marketTable,
	sa0builtin.StorageMinerActorCodeID:     minerTable,
	sa0builtin.StoragePowerActorCodeID:     powerTable,
	sa0builtin.SystemActorCodeID:           {},
	sa0builtin.VerifiedRegistryActorCodeID: verifregTable,

	// v2
	sa2builtin.AccountActorCodeID:          {},
	sa2builtin.CronActorCodeID:             cronTable,
	sa2builtin.InitActorCodeID:             initTable,
	sa2builtin.MultisigActorCodeID:         multisigTable,
	sa2builtin.PaymentChannelActorCodeID:   paychTable,
	sa2builtin.RewardActorCodeID:           rewardTable,
	sa2builtin.StorageMarketActorCodeID:    marketTable,
	sa2builtin.StorageMinerActorCodeID:     minerTable,
	sa2builtin.StoragePowerActorCodeID:     powerTable,
	sa2builtin.SystemActorCodeID:           {},
	sa2builtin.VerifiedRegistryActorCodeID: verifregTable,

	// v3
	sa3builtin.AccountActorCodeID:          {},
	sa3builtin.CronActorCodeID:             cronTable,
	sa3builtin.InitActorCodeID:             initTable,
	sa3builtin.MultisigActorCodeID:         multisigTable,
	sa3builtin.PaymentChannelActorCodeID:   paychTable,
	sa3builtin.RewardActorCodeID:           rewardTable,
	sa3builtin.StorageMarketActorCodeID:    marketTable,
	sa3builtin.StorageMinerActorCodeID:     minerTable,
	sa3builtin.StoragePowerActorCodeID:     powerTable,
	sa3builtin.SystemActorCodeID:           {},
	sa3builtin.VerifiedRegistryActorCodeID: verifregTable,

	// v4
	sa4builtin.AccountActorCodeID:          {},
	sa4builtin.CronActorCodeID:             cronTable,
	sa4builtin.InitActorCodeID:             initTable,
	sa4builtin.MultisigActorCodeID:         multisigTable,
	sa4builtin.PaymentChannelActorCodeID:   paychTable,
	sa4builtin.RewardActorCodeID:           rewardTable,
	sa4builtin.StorageMarketActorCodeID:    marketTable,
	sa4builtin.StorageMinerActorCodeID:     minerTable,
	sa4builtin.StoragePowerActorCodeID:     powerTable,
	sa4builtin.SystemActorCodeID:           {},
	sa4builtin.VerifiedRegistryActorCodeID: verifregTable,

	// v5
	sa5builtin.AccountActorCodeID:          {},
	sa5builtin.CronActorCodeID:             cronTable,
	sa5builtin.InitActorCodeID:             initTable,
	sa5builtin.MultisigActorCodeID:         multisigTable,
	sa5builtin.PaymentChannelActorCodeID:   paychTable,
	sa5builtin.RewardActorCodeID:           rewardTable,
	sa5builtin.StorageMarketActorCodeID:    marketV5Table,
	sa5builtin.StorageMinerActorCodeID:     minerTable,
	sa5builtin.StoragePowerActorCodeID:     powerTable,
	sa5builtin.SystemActorCodeID:           {},
	sa5builtin.VerifiedRegistryActorCodeID: verifregTable,
}

func ParseParams(params []byte, method int64, destType cid.Cid) (ipld.Node, string, error) {
	mthdTable, ok := messageParamTable[destType]
	if !ok {
		return nil, "", fmt.Errorf("unknown parameters for %s", destType)
	}

	proto := ipld.NodePrototype(types.Type.Any__Repr)
	name := "Unknown"
	mthd, ok := mthdTable[method]
	if ok {
		proto = mthd.NodePrototype
		name = mthd.Name
	}

	if len(params) == 0 {
		b, err := types.Type.Bytes__Repr.FromBytes(params)
		return b, name, err
	}

	builder := proto.NewBuilder()
	if err := dagcbor.Decoder(builder, bytes.NewBuffer(params)); err != nil {
		return nil, "", fmt.Errorf("cbor decode into %s (%s.%d) failed: %v", name, destType, method, err)
	}

	return builder.Build(), name, nil
}
