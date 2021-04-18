// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package sharer

import (
	"math/big"

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
		{"native_get", func(env *xenv.Environment) []interface{} {
			var key common.Hash
			env.ParseArgs(&key)

			env.UseGas(luckyshare.SloadGas)
			v, err := Params.Native(env.State()).Get(luckyshare.Bytes32(key))
			if err != nil {
				panic(err)
			}
			return []interface{}{v}
		}},
		{"native_set", func(env *xenv.Environment) []interface{} {
			var args struct {
				Key   common.Hash
				Value *big.Int
			}
			env.ParseArgs(&args)

			env.UseGas(luckyshare.SstoreSetGas)
			if err := Params.Native(env.State()).Set(luckyshare.Bytes32(args.Key), args.Value); err != nil {
				panic(err)
			}
			return nil
		}},
	}
	abi := Params.NativeABI()
	for _, def := range defines {
		if method, found := abi.MethodByName(def.name); found {
			nativeMethods[methodKey{Params.Address, method.ID()}] = &nativeMethod{
				abi: method,
				run: def.run,
			}
		} else {
			panic("method not found: " + def.name)
		}
	}
}
