package registry

import (
	"github.com/filecoin-project/sentinel-visor/model"
	"golang.org/x/xerrors"
)

type Registry struct {
	Models map[string]model.PersistableList
}

func NewRegistry() *Registry {
	return &Registry{
		Models: make(map[string]model.PersistableList),
	}
}

func (r *Registry) Register(t string, m model.Persistable) {
	r.Models[t] = append(r.Models[t], m)
}

func (r *Registry) RegisteredModels() model.PersistableList {
	var out model.PersistableList
	for _, v := range r.Models {
		out = append(out, v...)
	}
	return out
}

func (r *Registry) ModelsForTask(t string) (model.PersistableList, error) {
	models, found := r.Models[t]
	if !found {
		return nil, xerrors.Errorf("no models for task: %s", string(t))
	}
	return models, nil
}

var ModelRegistry = NewRegistry()

const (
	ActorStatesRawTask      = "actorstatesraw"      // task that only extracts raw actor state
	ActorStatesPowerTask    = "actorstatespower"    // task that only extracts power actor states (but not the raw state)
	ActorStatesRewardTask   = "actorstatesreward"   // task that only extracts reward actor states (but not the raw state)
	ActorStatesMinerTask    = "actorstatesminer"    // task that only extracts miner actor states (but not the raw state)
	ActorStatesInitTask     = "actorstatesinit"     // task that only extracts init actor states (but not the raw state)
	ActorStatesMarketTask   = "actorstatesmarket"   // task that only extracts market actor states (but not the raw state)
	ActorStatesMultisigTask = "actorstatesmultisig" // task that only extracts multisig actor states (but not the raw state)
	BlocksTask              = "blocks"              // task that extracts block data
	MessagesTask            = "messages"            // task that extracts message data
	ChainEconomicsTask      = "chaineconomics"      // task that extracts chain economics data
	MultisigApprovalsTask   = "msapprovals"         // task that extracts multisig actor approvals
)
