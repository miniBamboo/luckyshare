// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package genesis

import (
	"crypto/ecdsa"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/miniBamboo/luckyshare/luckyshare"
	sharer "github.com/miniBamboo/luckyshare/sharer"
	"github.com/miniBamboo/luckyshare/state"
	"github.com/miniBamboo/luckyshare/tx"
	"github.com/miniBamboo/luckyshare/vm"
)

// DevAccount account for development.
type DevAccount struct {
	Address    luckyshare.Address
	PrivateKey *ecdsa.PrivateKey
}

var devAccounts atomic.Value

// DevAccounts returns pre-alloced accounts for solo mode.
func DevAccounts() []DevAccount {
	if accs := devAccounts.Load(); accs != nil {
		return accs.([]DevAccount)
	}

	var accs []DevAccount
	privKeys := []string{
		"dce1443bd2ef0c2631adc1c67e5c93f13dc23a41c18b536effbbdcbcdb96fb65",
		"321d6443bc6177273b5abf54210fe806d451d6b7973bccc2384ef78bbcd0bf51",
		"2d7c882bad2a01105e36dda3646693bc1aaaa45b0ed63fb0ce23c060294f3af2",
		"593537225b037191d322c3b1df585fb1e5100811b71a6f7fc7e29cca1333483e",
		"ca7b25fc980c759df5f3ce17a3d881d6e19a38e651fc4315fc08917edab41058",
		"88d2d80b12b92feaa0da6d62309463d20408157723f2d7e799b6a74ead9a673b",
		"fbb9e7ba5fe9969a71c6599052237b91adeb1e5fc0c96727b66e56ff5d02f9d0",
		"547fb081e73dc2e22b4aae5c60e2970b008ac4fc3073aebc27d41ace9c4f53e9",
		"c8c53657e41a8d669349fc287f57457bd746cb1fcfc38cf94d235deb2cfca81b",
		"87e0eba9c86c494d98353800571089f316740b0cb84c9a7cdf2fe5c9997c7966",
	}
	for _, str := range privKeys {
		pk, err := crypto.HexToECDSA(str)
		if err != nil {
			panic(err)
		}
		addr := crypto.PubkeyToAddress(pk.PublicKey)
		accs = append(accs, DevAccount{luckyshare.Address(addr), pk})
	}
	devAccounts.Store(accs)
	return accs
}

// NewDevnet create genesis for solo mode.
func NewDevnet() *Genesis {
	launchTime := uint64(1526400000) // 'Wed May 16 2018 00:00:00 GMT+0800 (CST)'

	executor := DevAccounts()[0].Address
	soloBlockSigner := DevAccounts()[0]

	builder := new(Builder).
		GasLimit(luckyshare.InitialGasLimit).
		Timestamp(launchTime).
		State(func(state *state.State) error {
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

			tokenSupply := &big.Int{}
			energySupply := &big.Int{}
			for _, a := range DevAccounts() {
				bal, _ := new(big.Int).SetString("1000000000000000000000000000", 10)
				if err := state.SetBalance(a.Address, bal); err != nil {
					return err
				}
				if err := state.SetEnergy(a.Address, bal, launchTime); err != nil {
					return err
				}
				tokenSupply.Add(tokenSupply, bal)
				energySupply.Add(energySupply, bal)
			}
			return sharer.Energy.Native(state, launchTime).SetInitialSupply(tokenSupply, energySupply)
		}).
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
		Call(
			tx.NewClause(&sharer.Authority.Address).WithData(mustEncodeInput(sharer.Authority.ABI, "add", soloBlockSigner.Address, soloBlockSigner.Address, luckyshare.BytesToBytes32([]byte("Solo Block Signer")))),
			executor)

	id, err := builder.ComputeID()
	if err != nil {
		panic(err)
	}

	return &Genesis{builder, id, "devnet"}
}
