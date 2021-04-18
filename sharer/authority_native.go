// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package sharer

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/miniBamboo/luckyshare/luckyshare"
	"github.com/miniBamboo/luckyshare/xenv"
)

func init() {
	defines := []struct {
		name string
		run  func(env *xenv.Environment) []interface{}
	}{
		{"native_executor", func(env *xenv.Environment) []interface{} {
			env.UseGas(luckyshare.SloadGas)

			val, err := Params.Native(env.State()).Get(luckyshare.KeyExecutorAddress)
			if err != nil {
				panic(err)
			}

			addr := luckyshare.BytesToAddress(val.Bytes())
			return []interface{}{addr}
		}},
		{"native_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				NodeMaster common.Address
				Endorsor   common.Address
				Identity   common.Hash
			}
			env.ParseArgs(&args)

			env.UseGas(luckyshare.SloadGas)
			ok, err := Authority.Native(env.State()).Add(
				luckyshare.Address(args.NodeMaster),
				luckyshare.Address(args.Endorsor),
				luckyshare.Bytes32(args.Identity))
			if err != nil {
				panic(err)
			}

			if ok {
				env.UseGas(luckyshare.SstoreSetGas)
				env.UseGas(luckyshare.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_revoke", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(luckyshare.SloadGas)
			ok, err := Authority.Native(env.State()).Revoke(luckyshare.Address(nodeMaster))
			if err != nil {
				panic(err)
			}
			if ok {
				env.UseGas(luckyshare.SstoreResetGas * 3)
			}
			return []interface{}{ok}
		}},
		{"native_get", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(luckyshare.SloadGas * 2)
			listed, endorsor, identity, active, err := Authority.Native(env.State()).Get(luckyshare.Address(nodeMaster))
			if err != nil {
				panic(err)
			}

			return []interface{}{listed, endorsor, identity, active}
		}},
		{"native_first", func(env *xenv.Environment) []interface{} {
			env.UseGas(luckyshare.SloadGas)
			nodeMaster, err := Authority.Native(env.State()).First()
			if err != nil {
				panic(err)
			}
			if nodeMaster != nil {
				return []interface{}{*nodeMaster}
			}
			return []interface{}{luckyshare.Address{}}
		}},
		{"native_next", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(luckyshare.SloadGas)
			next, err := Authority.Native(env.State()).Next(luckyshare.Address(nodeMaster))
			if err != nil {
				panic(err)
			}
			if next != nil {
				return []interface{}{*next}
			}
			return []interface{}{luckyshare.Address{}}
		}},
		{"native_isEndorsed", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(luckyshare.SloadGas * 2)
			listed, endorsor, _, _, err := Authority.Native(env.State()).Get(luckyshare.Address(nodeMaster))
			if err != nil {
				panic(err)
			}
			if !listed {
				return []interface{}{false}
			}

			env.UseGas(luckyshare.GetBalanceGas)
			bal, err := env.State().GetBalance(endorsor)
			if err != nil {
				panic(err)
			}

			env.UseGas(luckyshare.SloadGas)
			endorsement, err := Params.Native(env.State()).Get(luckyshare.KeyProposerEndorsement)
			if err != nil {
				panic(err)
			}
			return []interface{}{bal.Cmp(endorsement) >= 0}
		}},
	}
	abi := Authority.NativeABI()
	for _, def := range defines {
		if method, found := abi.MethodByName(def.name); found {
			nativeMethods[methodKey{Authority.Address, method.ID()}] = &nativeMethod{
				abi: method,
				run: def.run,
			}
		} else {
			panic("method not found: " + def.name)
		}
	}
}
