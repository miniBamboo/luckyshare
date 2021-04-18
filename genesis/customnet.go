// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package genesis

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"

	"github.com/miniBamboo/luckyshare/luckyshare"
	sharer "github.com/miniBamboo/luckyshare/sharer"
	"github.com/miniBamboo/luckyshare/state"
	"github.com/miniBamboo/luckyshare/tx"
	"github.com/miniBamboo/luckyshare/vm"
)

// CustomGenesis is user customized genesis
type CustomGenesis struct {
	LaunchTime uint64                 `json:"launchTime"`
	GasLimit   uint64                 `json:"gaslimit"`
	ExtraData  string                 `json:"extraData"`
	Accounts   []Account              `json:"accounts"`
	Authority  []Authority            `json:"authority"`
	Params     Params                 `json:"params"`
	Executor   Executor               `json:"executor"`
	ForkConfig *luckyshare.ForkConfig `json:"forkConfig"`
}

// NewCustomNet create custom network genesis.
func NewCustomNet(gen *CustomGenesis) (*Genesis, error) {
	launchTime := gen.LaunchTime

	if gen.GasLimit == 0 {
		gen.GasLimit = luckyshare.InitialGasLimit
	}
	var executor luckyshare.Address
	if gen.Params.ExecutorAddress != nil {
		executor = *gen.Params.ExecutorAddress
	} else {
		executor = sharer.Executor.Address
	}

	builder := new(Builder).
		Timestamp(launchTime).
		GasLimit(gen.GasLimit).
		State(func(state *state.State) error {
			// alloc precompiled contracts
			for addr := range vm.PrecompiledContractsByzantium {
				if err := state.SetCode(luckyshare.Address(addr), emptyRuntimeBytecode); err != nil {
					return err
				}
			}

			// alloc sharer contracts
			if err := state.SetCode(sharer.Authority.Address, sharer.Authority.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(sharer.Energy.Address, sharer.Energy.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(sharer.Extension.Address, sharer.Extension.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(sharer.Params.Address, sharer.Params.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(sharer.Prototype.Address, sharer.Prototype.RuntimeBytecodes()); err != nil {
				return err
			}

			if len(gen.Executor.Approvers) > 0 {
				if err := state.SetCode(sharer.Executor.Address, sharer.Executor.RuntimeBytecodes()); err != nil {
					return err
				}
			}

			tokenSupply := &big.Int{}
			energySupply := &big.Int{}
			for _, a := range gen.Accounts {
				if b := (*big.Int)(a.Balance); b != nil {
					if b.Sign() < 0 {
						return fmt.Errorf("%s: balance must be a non-negative integer", a.Address)
					}
					tokenSupply.Add(tokenSupply, b)
					if err := state.SetBalance(a.Address, b); err != nil {
						return err
					}
					if err := state.SetEnergy(a.Address, &big.Int{}, launchTime); err != nil {
						return err
					}
				}
				if e := (*big.Int)(a.Energy); e != nil {
					if e.Sign() < 0 {
						return fmt.Errorf("%s: energy must be a non-negative integer", a.Address)
					}
					energySupply.Add(energySupply, e)
					if err := state.SetEnergy(a.Address, e, launchTime); err != nil {
						return err
					}
				}
				if len(a.Code) > 0 {
					code, err := hexutil.Decode(a.Code)
					if err != nil {
						return fmt.Errorf("invalid contract code for address: %s", a.Address)
					}
					if err := state.SetCode(a.Address, code); err != nil {
						return err
					}
				}
				if len(a.Storage) > 0 {
					for k, v := range a.Storage {
						state.SetStorage(a.Address, luckyshare.MustParseBytes32(k), v)
					}
				}
			}

			return sharer.Energy.Native(state, launchTime).SetInitialSupply(tokenSupply, energySupply)
		})

	///// initialize sharer contracts

	// initialize params
	bgp := (*big.Int)(gen.Params.BaseGasPrice)
	if bgp != nil {
		if bgp.Sign() < 0 {
			return nil, errors.New("baseGasPrice must be a non-negative integer")
		}
	} else {
		bgp = luckyshare.InitialBaseGasPrice
	}

	r := (*big.Int)(gen.Params.RewardRatio)
	if r != nil {
		if r.Sign() < 0 {
			return nil, errors.New("rewardRatio must be a non-negative integer")
		}
	} else {
		r = luckyshare.InitialRewardRatio
	}

	e := (*big.Int)(gen.Params.ProposerEndorsement)
	if e != nil {
		if e.Sign() < 0 {
			return nil, errors.New("proposerEndorsement must a non-negative integer")
		}
	} else {
		e = luckyshare.InitialProposerEndorsement
	}

	data := mustEncodeInput(sharer.Params.ABI, "set", luckyshare.KeyExecutorAddress, new(big.Int).SetBytes(executor[:]))
	builder.Call(tx.NewClause(&sharer.Params.Address).WithData(data), luckyshare.Address{})

	data = mustEncodeInput(sharer.Params.ABI, "set", luckyshare.KeyRewardRatio, r)
	builder.Call(tx.NewClause(&sharer.Params.Address).WithData(data), executor)

	data = mustEncodeInput(sharer.Params.ABI, "set", luckyshare.KeyBaseGasPrice, bgp)
	builder.Call(tx.NewClause(&sharer.Params.Address).WithData(data), executor)

	data = mustEncodeInput(sharer.Params.ABI, "set", luckyshare.KeyProposerEndorsement, e)
	builder.Call(tx.NewClause(&sharer.Params.Address).WithData(data), executor)

	if len(gen.Authority) == 0 {
		return nil, errors.New("at least one authority node")
	}
	// add initial authority nodes
	for _, anode := range gen.Authority {
		data := mustEncodeInput(sharer.Authority.ABI, "add", anode.MasterAddress, anode.EndorsorAddress, anode.Identity)
		builder.Call(tx.NewClause(&sharer.Authority.Address).WithData(data), executor)
	}

	if len(gen.Executor.Approvers) > 0 {
		// add initial approvers
		for _, approver := range gen.Executor.Approvers {
			data := mustEncodeInput(sharer.Executor.ABI, "addApprover", approver.Address, approver.Identity)
			builder.Call(tx.NewClause(&sharer.Executor.Address).WithData(data), executor)
		}
	}

	if len(gen.ExtraData) > 0 {
		var extra [28]byte
		copy(extra[:], gen.ExtraData)
		builder.ExtraData(extra)
	}

	id, err := builder.ComputeID()
	if err != nil {
		panic(err)
	}
	return &Genesis{builder, id, "customnet"}, nil
}

// Account is the account will set to the genesis block
type Account struct {
	Address luckyshare.Address            `json:"address"`
	Balance *hexOrDecimal256              `json:"balance"`
	Energy  *hexOrDecimal256              `json:"energy"`
	Code    string                        `json:"code"`
	Storage map[string]luckyshare.Bytes32 `json:"storage"`
}

// Authority is the authority node info
type Authority struct {
	MasterAddress   luckyshare.Address `json:"masterAddress"`
	EndorsorAddress luckyshare.Address `json:"endorsorAddress"`
	Identity        luckyshare.Bytes32 `json:"identity"`
}

// Executor is the params for executor info
type Executor struct {
	Approvers []Approver `json:"approvers"`
}

// Approver is the approver info for executor contract
type Approver struct {
	Address  luckyshare.Address `json:"address"`
	Identity luckyshare.Bytes32 `json:"identity"`
}

// Params means the chain params for params contract
type Params struct {
	RewardRatio         *hexOrDecimal256    `json:"rewardRatio"`
	BaseGasPrice        *hexOrDecimal256    `json:"baseGasPrice"`
	ProposerEndorsement *hexOrDecimal256    `json:"proposerEndorsement"`
	ExecutorAddress     *luckyshare.Address `json:"executorAddress"`
}

// hexOrDecimal256 marshals big.Int as hex or decimal.
// Copied from go-ethereum/common/math and implement json. Marshaler
type hexOrDecimal256 big.Int

// UnmarshalJSON implements the json.Unmarshaler interface.
func (i *hexOrDecimal256) UnmarshalJSON(input []byte) error {
	var hex string
	if err := json.Unmarshal(input, &hex); err != nil {
		if err = (*big.Int)(i).UnmarshalJSON(input); err != nil {
			return err
		}
		return nil
	}
	bigint, ok := math.ParseBig256(hex)
	if !ok {
		return fmt.Errorf("invalid hex or decimal integer %q", input)
	}
	*i = hexOrDecimal256(*bigint)
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (i *hexOrDecimal256) MarshalJSON() ([]byte, error) {
	return (*math.HexOrDecimal256)(i).MarshalText()
}
