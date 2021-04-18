// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package genesis

import (
	"math/big"

	"github.com/miniBamboo/luckyshare/luckyshare"
	sharer "github.com/miniBamboo/luckyshare/sharer"
	"github.com/miniBamboo/luckyshare/state"
	"github.com/miniBamboo/luckyshare/tx"
	"github.com/miniBamboo/luckyshare/vm"
)

// NewTestnet create genesis for testnet.
func NewTestnet() *Genesis {
	launchTime := uint64(1530014400) // 'Tue Jun 26 2018 20:00:00 GMT+0800 (CST)'

	// use this address as executor instead of sharer one, for test purpose
	executor := luckyshare.MustParseAddress("0xB5A34b62b63A6f1EE99DFD30b133B657859f8d79")
	acccount0 := luckyshare.MustParseAddress("0xe59D475Abe695c7f67a8a2321f33A856B0B4c71d")

	master0 := luckyshare.MustParseAddress("0x25AE0ef84dA4a76D5a1DFE80D3789C2c46FeE30a")
	endorser0 := luckyshare.MustParseAddress("0xb4094c25f86d628fdD571Afc4077f0d0196afB48")

	builder := new(Builder).
		Timestamp(launchTime).
		GasLimit(luckyshare.InitialGasLimit).
		State(func(state *state.State) error {
			tokenSupply := new(big.Int)

			// alloc precompiled contracts
			for addr := range vm.PrecompiledContractsByzantium {
				if err := state.SetCode(luckyshare.Address(addr), emptyRuntimeBytecode); err != nil {
					return err
				}
			}

			// setup sharer contracts
			if err := state.SetCode(sharer.Authority.Address, sharer.Authority.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(sharer.Energy.Address, sharer.Energy.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(sharer.Params.Address, sharer.Params.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(sharer.Prototype.Address, sharer.Prototype.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(sharer.Extension.Address, sharer.Extension.RuntimeBytecodes()); err != nil {
				return err
			}

			// 50 billion for account0
			amount := new(big.Int).Mul(big.NewInt(1e18), big.NewInt(50*1000*1000*1000))
			if err := state.SetBalance(acccount0, amount); err != nil {
				return err
			}
			if err := state.SetEnergy(acccount0, &big.Int{}, launchTime); err != nil {
				return err
			}
			tokenSupply.Add(tokenSupply, amount)

			// 25 million for endorser0
			amount = new(big.Int).Mul(big.NewInt(1e18), big.NewInt(25*1000*1000))
			if err := state.SetBalance(endorser0, amount); err != nil {
				return err
			}
			if err := state.SetEnergy(endorser0, &big.Int{}, launchTime); err != nil {
				return err
			}
			tokenSupply.Add(tokenSupply, amount)

			return sharer.Energy.Native(state, launchTime).SetInitialSupply(tokenSupply, &big.Int{})
		}).
		// set initial params
		// use an external account as executor to manage testnet easily
		Call(
			tx.NewClause(&sharer.Params.Address).WithData(mustEncodeInput(sharer.Params.ABI, "set", luckyshare.KeyExecutorAddress, new(big.Int).SetBytes(executor[:]))),
			luckyshare.Address{}).
		Call(
			tx.NewClause(&sharer.Params.Address).WithData(mustEncodeInput(sharer.Params.ABI, "set", luckyshare.KeyRewardRatio, luckyshare.InitialRewardRatio)),
			executor).
		Call(
			tx.NewClause(&sharer.Params.Address).WithData(mustEncodeInput(sharer.Params.ABI, "set", luckyshare.KeyBaseGasPrice, luckyshare.InitialBaseGasPrice)),
			executor).
		Call(
			tx.NewClause(&sharer.Params.Address).WithData(mustEncodeInput(sharer.Params.ABI, "set", luckyshare.KeyProposerEndorsement, luckyshare.InitialProposerEndorsement)),
			executor).
		// add master0 as the initial block proposer
		Call(tx.NewClause(&sharer.Authority.Address).WithData(mustEncodeInput(sharer.Authority.ABI, "add", master0, endorser0, luckyshare.BytesToBytes32([]byte("master0")))),
			executor)

	id, err := builder.ComputeID()
	if err != nil {
		panic(err)
	}
	return &Genesis{builder, id, "testnet"}
}
