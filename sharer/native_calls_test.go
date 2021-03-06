// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package sharer_test

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/miniBamboo/luckyshare/abi"
	"github.com/miniBamboo/luckyshare/block"
	"github.com/miniBamboo/luckyshare/chain"
	"github.com/miniBamboo/luckyshare/genesis"
	"github.com/miniBamboo/luckyshare/luckyshare"
	"github.com/miniBamboo/luckyshare/muxdb"
	"github.com/miniBamboo/luckyshare/runtime"
	sharer "github.com/miniBamboo/luckyshare/sharer"
	"github.com/miniBamboo/luckyshare/state"
	"github.com/miniBamboo/luckyshare/tx"
	"github.com/miniBamboo/luckyshare/xenv"
	"github.com/stretchr/testify/assert"
)

var errReverted = errors.New("evm: execution reverted")

type ctest struct {
	rt         *runtime.Runtime
	abi        *abi.ABI
	to, caller luckyshare.Address
}

type ccase struct {
	rt         *runtime.Runtime
	abi        *abi.ABI
	to, caller luckyshare.Address
	name       string
	args       []interface{}
	events     tx.Events
	provedWork *big.Int
	txID       luckyshare.Bytes32
	blockRef   tx.BlockRef
	gasPayer   luckyshare.Address
	expiration uint32

	output *[]interface{}
	vmerr  error
}

func (c *ctest) Case(name string, args ...interface{}) *ccase {
	return &ccase{
		rt:     c.rt,
		abi:    c.abi,
		to:     c.to,
		caller: c.caller,
		name:   name,
		args:   args,
	}
}

func (c *ccase) To(to luckyshare.Address) *ccase {
	c.to = to
	return c
}

func (c *ccase) Caller(caller luckyshare.Address) *ccase {
	c.caller = caller
	return c
}

func (c *ccase) ProvedWork(provedWork *big.Int) *ccase {
	c.provedWork = provedWork
	return c
}

func (c *ccase) TxID(txID luckyshare.Bytes32) *ccase {
	c.txID = txID
	return c
}

func (c *ccase) BlockRef(blockRef tx.BlockRef) *ccase {
	c.blockRef = blockRef
	return c
}

func (c *ccase) GasPayer(gasPayer luckyshare.Address) *ccase {
	c.gasPayer = gasPayer
	return c
}

func (c *ccase) Expiration(expiration uint32) *ccase {
	c.expiration = expiration
	return c
}
func (c *ccase) ShouldVMError(err error) *ccase {
	c.vmerr = err
	return c
}

func (c *ccase) ShouldLog(events ...*tx.Event) *ccase {
	c.events = events
	return c
}

func (c *ccase) ShouldOutput(outputs ...interface{}) *ccase {
	c.output = &outputs
	return c
}

func (c *ccase) Assert(t *testing.T) *ccase {
	method, ok := c.abi.MethodByName(c.name)
	assert.True(t, ok, "should have method")

	constant := method.Const()
	stage, err := c.rt.State().Stage()
	assert.Nil(t, err, "should stage state")
	stateRoot := stage.Hash()

	data, err := method.EncodeInput(c.args...)
	assert.Nil(t, err, "should encode input")

	exec, _ := c.rt.PrepareClause(tx.NewClause(&c.to).WithData(data),
		0, math.MaxUint64, &xenv.TransactionContext{
			ID:         c.txID,
			Origin:     c.caller,
			GasPrice:   &big.Int{},
			GasPayer:   c.gasPayer,
			ProvedWork: c.provedWork,
			BlockRef:   c.blockRef,
			Expiration: c.expiration})
	vmout, _, err := exec()
	assert.Nil(t, err)
	if constant || vmout.VMErr != nil {
		stage, err := c.rt.State().Stage()
		assert.Nil(t, err, "should stage state")
		newStateRoot := stage.Hash()
		assert.Equal(t, stateRoot, newStateRoot)
	}
	if c.vmerr != nil {
		assert.Equal(t, c.vmerr, vmout.VMErr)
	} else {
		assert.Nil(t, vmout.VMErr)
	}

	if c.output != nil {
		out, err := method.EncodeOutput((*c.output)...)
		assert.Nil(t, err, "should encode output")
		assert.Equal(t, out, vmout.Data, "should match output")
	}

	if len(c.events) > 0 {
		for _, ev := range c.events {
			found := func() bool {
				for _, outEv := range vmout.Events {
					if reflect.DeepEqual(ev, outEv) {
						return true
					}
				}
				return false
			}()
			assert.True(t, found, "event should appear")
		}
	}

	c.output = nil
	c.vmerr = nil
	c.events = nil

	return c
}

func buildGenesis(db *muxdb.MuxDB, proc func(state *state.State) error) *block.Block {
	blk, _, _, _ := new(genesis.Builder).
		Timestamp(uint64(time.Now().Unix())).
		State(proc).
		Build(state.NewStater(db))
	return blk
}

func TestParamsNative(t *testing.T) {
	executor := luckyshare.BytesToAddress([]byte("e"))
	db := muxdb.NewMem()
	b0 := buildGenesis(db, func(state *state.State) error {
		state.SetCode(sharer.Params.Address, sharer.Params.RuntimeBytecodes())
		sharer.Params.Native(state).Set(luckyshare.KeyExecutorAddress, new(big.Int).SetBytes(executor[:]))
		return nil
	})
	repo, _ := chain.NewRepository(db, b0)
	st := state.New(db, b0.Header().StateRoot())
	chain := repo.NewChain(b0.Header().ID())

	rt := runtime.New(chain, st, &xenv.BlockContext{}, luckyshare.NoFork)

	test := &ctest{
		rt:  rt,
		abi: sharer.Params.ABI,
		to:  sharer.Params.Address,
	}

	key := luckyshare.BytesToBytes32([]byte("key"))
	value := big.NewInt(999)
	setEvent := func(key luckyshare.Bytes32, value *big.Int) *tx.Event {
		ev, _ := sharer.Params.ABI.EventByName("Set")
		data, _ := ev.Encode(value)
		return &tx.Event{
			Address: sharer.Params.Address,
			Topics:  []luckyshare.Bytes32{ev.ID(), key},
			Data:    data,
		}
	}

	test.Case("executor").
		ShouldOutput(executor).
		Assert(t)

	test.Case("set", key, value).
		Caller(executor).
		ShouldLog(setEvent(key, value)).
		Assert(t)

	test.Case("set", key, value).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("get", key).
		ShouldOutput(value).
		Assert(t)

}

func TestAuthorityNative(t *testing.T) {
	var (
		master1   = luckyshare.BytesToAddress([]byte("master1"))
		endorsor1 = luckyshare.BytesToAddress([]byte("endorsor1"))
		identity1 = luckyshare.BytesToBytes32([]byte("identity1"))

		master2   = luckyshare.BytesToAddress([]byte("master2"))
		endorsor2 = luckyshare.BytesToAddress([]byte("endorsor2"))
		identity2 = luckyshare.BytesToBytes32([]byte("identity2"))

		master3   = luckyshare.BytesToAddress([]byte("master3"))
		endorsor3 = luckyshare.BytesToAddress([]byte("endorsor3"))
		identity3 = luckyshare.BytesToBytes32([]byte("identity3"))
		executor  = luckyshare.BytesToAddress([]byte("e"))
	)

	db := muxdb.NewMem()
	b0 := buildGenesis(db, func(state *state.State) error {
		state.SetCode(sharer.Authority.Address, sharer.Authority.RuntimeBytecodes())
		state.SetBalance(luckyshare.Address(endorsor1), luckyshare.InitialProposerEndorsement)
		state.SetCode(sharer.Params.Address, sharer.Params.RuntimeBytecodes())
		sharer.Params.Native(state).Set(luckyshare.KeyExecutorAddress, new(big.Int).SetBytes(executor[:]))
		sharer.Params.Native(state).Set(luckyshare.KeyProposerEndorsement, luckyshare.InitialProposerEndorsement)
		return nil
	})
	repo, _ := chain.NewRepository(db, b0)
	st := state.New(db, b0.Header().StateRoot())
	chain := repo.NewChain(b0.Header().ID())

	rt := runtime.New(chain, st, &xenv.BlockContext{}, luckyshare.NoFork)

	candidateEvent := func(nodeMaster luckyshare.Address, action string) *tx.Event {
		ev, _ := sharer.Authority.ABI.EventByName("Candidate")
		var b32 luckyshare.Bytes32
		copy(b32[:], action)
		data, _ := ev.Encode(b32)
		return &tx.Event{
			Address: sharer.Authority.Address,
			Topics:  []luckyshare.Bytes32{ev.ID(), luckyshare.BytesToBytes32(nodeMaster[:])},
			Data:    data,
		}
	}

	test := &ctest{
		rt:     rt,
		abi:    sharer.Authority.ABI,
		to:     sharer.Authority.Address,
		caller: executor,
	}

	test.Case("executor").
		ShouldOutput(executor).
		Assert(t)

	test.Case("first").
		ShouldOutput(luckyshare.Address{}).
		Assert(t)

	test.Case("add", master1, endorsor1, identity1).
		ShouldLog(candidateEvent(master1, "added")).
		Assert(t)

	test.Case("add", master2, endorsor2, identity2).
		ShouldLog(candidateEvent(master2, "added")).
		Assert(t)

	test.Case("add", master3, endorsor3, identity3).
		ShouldLog(candidateEvent(master3, "added")).
		Assert(t)

	test.Case("get", master1).
		ShouldOutput(true, endorsor1, identity1, true).
		Assert(t)

	test.Case("first").
		ShouldOutput(master1).
		Assert(t)

	test.Case("next", master1).
		ShouldOutput(master2).
		Assert(t)

	test.Case("next", master2).
		ShouldOutput(master3).
		Assert(t)

	test.Case("next", master3).
		ShouldOutput(luckyshare.Address{}).
		Assert(t)

	test.Case("add", master1, endorsor1, identity1).
		Caller(luckyshare.BytesToAddress([]byte("other"))).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("add", master1, endorsor1, identity1).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("revoke", master1).
		ShouldLog(candidateEvent(master1, "revoked")).
		Assert(t)

	// duped even revoked
	test.Case("add", master1, endorsor1, identity1).
		ShouldVMError(errReverted).
		Assert(t)

	// any one can revoke a candidate if out of endorsement
	st.SetBalance(endorsor2, big.NewInt(1))
	test.Case("revoke", master2).
		Caller(luckyshare.BytesToAddress([]byte("some one"))).
		Assert(t)

}

func TestEnergyNative(t *testing.T) {
	var (
		addr   = luckyshare.BytesToAddress([]byte("addr"))
		to     = luckyshare.BytesToAddress([]byte("to"))
		master = luckyshare.BytesToAddress([]byte("master"))
		eng    = big.NewInt(1000)
	)

	db := muxdb.NewMem()
	b0 := buildGenesis(db, func(state *state.State) error {
		state.SetCode(sharer.Energy.Address, sharer.Energy.RuntimeBytecodes())
		state.SetMaster(addr, master)
		return nil
	})

	repo, _ := chain.NewRepository(db, b0)
	st := state.New(db, b0.Header().StateRoot())
	chain := repo.NewChain(b0.Header().ID())

	st.SetEnergy(addr, eng, b0.Header().Timestamp())
	sharer.Energy.Native(st, b0.Header().Timestamp()).SetInitialSupply(&big.Int{}, eng)

	transferEvent := func(from, to luckyshare.Address, value *big.Int) *tx.Event {
		ev, _ := sharer.Energy.ABI.EventByName("Transfer")
		data, _ := ev.Encode(value)
		return &tx.Event{
			Address: sharer.Energy.Address,
			Topics:  []luckyshare.Bytes32{ev.ID(), luckyshare.BytesToBytes32(from[:]), luckyshare.BytesToBytes32(to[:])},
			Data:    data,
		}
	}
	approvalEvent := func(owner, spender luckyshare.Address, value *big.Int) *tx.Event {
		ev, _ := sharer.Energy.ABI.EventByName("Approval")
		data, _ := ev.Encode(value)
		return &tx.Event{
			Address: sharer.Energy.Address,
			Topics:  []luckyshare.Bytes32{ev.ID(), luckyshare.BytesToBytes32(owner[:]), luckyshare.BytesToBytes32(spender[:])},
			Data:    data,
		}
	}

	rt := runtime.New(chain, st, &xenv.BlockContext{Time: b0.Header().Timestamp()}, luckyshare.NoFork)
	test := &ctest{
		rt:     rt,
		abi:    sharer.Energy.ABI,
		to:     sharer.Energy.Address,
		caller: sharer.Energy.Address,
	}

	test.Case("name").
		ShouldOutput("Luckyshare").
		Assert(t)

	test.Case("decimals").
		ShouldOutput(uint8(18)).
		Assert(t)

	test.Case("symbol").
		ShouldOutput("VTHO").
		Assert(t)

	test.Case("totalSupply").
		ShouldOutput(eng).
		Assert(t)

	test.Case("totalBurned").
		ShouldOutput(&big.Int{}).
		Assert(t)

	test.Case("balanceOf", addr).
		ShouldOutput(eng).
		Assert(t)

	test.Case("transfer", to, big.NewInt(10)).
		Caller(addr).
		ShouldLog(transferEvent(addr, to, big.NewInt(10))).
		ShouldOutput(true).
		Assert(t)

	test.Case("transfer", to, big.NewInt(10)).
		Caller(luckyshare.BytesToAddress([]byte("some one"))).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("move", addr, to, big.NewInt(10)).
		Caller(addr).
		ShouldLog(transferEvent(addr, to, big.NewInt(10))).
		ShouldOutput(true).
		Assert(t)

	test.Case("move", addr, to, big.NewInt(10)).
		Caller(luckyshare.BytesToAddress([]byte("some one"))).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("approve", to, big.NewInt(10)).
		Caller(addr).
		ShouldLog(approvalEvent(addr, to, big.NewInt(10))).
		ShouldOutput(true).
		Assert(t)

	test.Case("allowance", addr, to).
		ShouldOutput(big.NewInt(10)).
		Assert(t)

	test.Case("transferFrom", addr, luckyshare.BytesToAddress([]byte("some one")), big.NewInt(10)).
		Caller(to).
		ShouldLog(transferEvent(addr, luckyshare.BytesToAddress([]byte("some one")), big.NewInt(10))).
		ShouldOutput(true).
		Assert(t)

	test.Case("transferFrom", addr, to, big.NewInt(10)).
		Caller(luckyshare.BytesToAddress([]byte("some one"))).
		ShouldVMError(errReverted).
		Assert(t)

}

func TestPrototypeNative(t *testing.T) {
	var (
		acc1 = luckyshare.BytesToAddress([]byte("acc1"))
		acc2 = luckyshare.BytesToAddress([]byte("acc2"))

		master    = luckyshare.BytesToAddress([]byte("master"))
		notmaster = luckyshare.BytesToAddress([]byte("notmaster"))
		user      = luckyshare.BytesToAddress([]byte("user"))
		notuser   = luckyshare.BytesToAddress([]byte("notuser"))

		credit       = big.NewInt(1000)
		recoveryRate = big.NewInt(10)
		sponsor      = luckyshare.BytesToAddress([]byte("sponsor"))
		notsponsor   = luckyshare.BytesToAddress([]byte("notsponsor"))

		key      = luckyshare.BytesToBytes32([]byte("account-key"))
		value    = luckyshare.BytesToBytes32([]byte("account-value"))
		contract luckyshare.Address
	)

	db := muxdb.NewMem()
	gene := genesis.NewDevnet()
	genesisBlock, _, _, _ := gene.Build(state.NewStater(db))
	repo, _ := chain.NewRepository(db, genesisBlock)
	st := state.New(db, genesisBlock.Header().StateRoot())
	chain := repo.NewChain(genesisBlock.Header().ID())

	st.SetStorage(luckyshare.Address(acc1), key, value)
	st.SetBalance(luckyshare.Address(acc1), big.NewInt(1))

	masterEvent := func(self, newMaster luckyshare.Address) *tx.Event {
		ev, _ := sharer.Prototype.Events().EventByName("$Master")
		data, _ := ev.Encode(newMaster)
		return &tx.Event{
			Address: self,
			Topics:  []luckyshare.Bytes32{ev.ID()},
			Data:    data,
		}
	}

	creditPlanEvent := func(self luckyshare.Address, credit, recoveryRate *big.Int) *tx.Event {
		ev, _ := sharer.Prototype.Events().EventByName("$CreditPlan")
		data, _ := ev.Encode(credit, recoveryRate)
		return &tx.Event{
			Address: self,
			Topics:  []luckyshare.Bytes32{ev.ID()},
			Data:    data,
		}
	}

	userEvent := func(self, user luckyshare.Address, action string) *tx.Event {
		ev, _ := sharer.Prototype.Events().EventByName("$User")
		var b32 luckyshare.Bytes32
		copy(b32[:], action)
		data, _ := ev.Encode(b32)
		return &tx.Event{
			Address: self,
			Topics:  []luckyshare.Bytes32{ev.ID(), luckyshare.BytesToBytes32(user.Bytes())},
			Data:    data,
		}
	}

	sponsorEvent := func(self, sponsor luckyshare.Address, action string) *tx.Event {
		ev, _ := sharer.Prototype.Events().EventByName("$Sponsor")
		var b32 luckyshare.Bytes32
		copy(b32[:], action)
		data, _ := ev.Encode(b32)
		return &tx.Event{
			Address: self,
			Topics:  []luckyshare.Bytes32{ev.ID(), luckyshare.BytesToBytes32(sponsor.Bytes())},
			Data:    data,
		}
	}

	rt := runtime.New(chain, st, &xenv.BlockContext{
		Time:   genesisBlock.Header().Timestamp(),
		Number: genesisBlock.Header().Number(),
	}, luckyshare.NoFork)

	code, _ := hex.DecodeString("60606040523415600e57600080fd5b603580601b6000396000f3006060604052600080fd00a165627a7a72305820edd8a93b651b5aac38098767f0537d9b25433278c9d155da2135efc06927fc960029")
	exec, _ := rt.PrepareClause(tx.NewClause(nil).WithData(code), 0, math.MaxUint64, &xenv.TransactionContext{
		ID:         luckyshare.Bytes32{},
		Origin:     master,
		GasPrice:   &big.Int{},
		ProvedWork: &big.Int{}})
	out, _, _ := exec()

	contract = *out.ContractAddress

	energy := big.NewInt(1000)
	st.SetEnergy(acc1, energy, genesisBlock.Header().Timestamp())

	test := &ctest{
		rt:     rt,
		abi:    sharer.Prototype.ABI,
		to:     sharer.Prototype.Address,
		caller: sharer.Prototype.Address,
	}

	test.Case("master", acc1).
		ShouldOutput(luckyshare.Address{}).
		Assert(t)

	test.Case("master", contract).
		ShouldOutput(master).
		Assert(t)

	test.Case("setMaster", acc1, acc2).
		Caller(acc1).
		ShouldOutput().
		ShouldLog(masterEvent(acc1, acc2)).
		Assert(t)

	test.Case("setMaster", acc1, acc2).
		Caller(notmaster).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("master", acc1).
		ShouldOutput(acc2).
		Assert(t)

	test.Case("hasCode", acc1).
		ShouldOutput(false).
		Assert(t)

	test.Case("hasCode", contract).
		ShouldOutput(true).
		Assert(t)

	test.Case("setCreditPlan", contract, credit, recoveryRate).
		Caller(master).
		ShouldOutput().
		ShouldLog(creditPlanEvent(contract, credit, recoveryRate)).
		Assert(t)

	test.Case("setCreditPlan", contract, credit, recoveryRate).
		Caller(notmaster).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("creditPlan", contract).
		ShouldOutput(credit, recoveryRate).
		Assert(t)

	test.Case("isUser", contract, user).
		ShouldOutput(false).
		Assert(t)

	test.Case("addUser", contract, user).
		Caller(master).
		ShouldOutput().
		ShouldLog(userEvent(contract, user, "added")).
		Assert(t)

	test.Case("addUser", contract, user).
		Caller(notmaster).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("addUser", contract, user).
		Caller(master).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("isUser", contract, user).
		ShouldOutput(true).
		Assert(t)

	test.Case("userCredit", contract, user).
		ShouldOutput(credit).
		Assert(t)

	test.Case("removeUser", contract, user).
		Caller(master).
		ShouldOutput().
		ShouldLog(userEvent(contract, user, "removed")).
		Assert(t)

	test.Case("removeUser", contract, user).
		Caller(notmaster).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("removeUser", contract, notuser).
		Caller(master).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("userCredit", contract, user).
		ShouldOutput(&big.Int{}).
		Assert(t)

	test.Case("isSponsor", contract, sponsor).
		ShouldOutput(false).
		Assert(t)

	test.Case("sponsor", contract).
		Caller(sponsor).
		ShouldOutput().
		ShouldLog(sponsorEvent(contract, sponsor, "sponsored")).
		Assert(t)

	test.Case("sponsor", contract).
		Caller(sponsor).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("isSponsor", contract, sponsor).
		ShouldOutput(true).
		Assert(t)

	test.Case("currentSponsor", contract).
		ShouldOutput(luckyshare.Address{}).
		Assert(t)

	test.Case("selectSponsor", contract, sponsor).
		Caller(master).
		ShouldOutput().
		ShouldLog(sponsorEvent(contract, sponsor, "selected")).
		Assert(t)

	test.Case("selectSponsor", contract, notsponsor).
		Caller(master).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("selectSponsor", contract, notsponsor).
		Caller(notmaster).
		ShouldVMError(errReverted).
		Assert(t)
	test.Case("currentSponsor", contract).
		ShouldOutput(sponsor).
		Assert(t)

	test.Case("unsponsor", contract).
		Caller(sponsor).
		ShouldOutput().
		Assert(t)
	test.Case("currentSponsor", contract).
		ShouldOutput(sponsor).
		Assert(t)

	test.Case("unsponsor", contract).
		Caller(sponsor).
		ShouldVMError(errReverted).
		Assert(t)

	test.Case("isSponsor", contract, sponsor).
		ShouldOutput(false).
		Assert(t)

	test.Case("storageFor", acc1, key).
		ShouldOutput(value).
		Assert(t)
	test.Case("storageFor", acc1, luckyshare.BytesToBytes32([]byte("some-key"))).
		ShouldOutput(luckyshare.Bytes32{}).
		Assert(t)

	// should be hash of rlp raw
	expected, err := st.GetStorage(sharer.Prototype.Address, luckyshare.Blake2b(contract.Bytes(), []byte("credit-plan")))
	assert.Nil(t, err)
	test.Case("storageFor", sharer.Prototype.Address, luckyshare.Blake2b(contract.Bytes(), []byte("credit-plan"))).
		ShouldOutput(expected).
		Assert(t)

	test.Case("balance", acc1, big.NewInt(0)).
		ShouldOutput(big.NewInt(1)).
		Assert(t)

	test.Case("balance", acc1, big.NewInt(100)).
		ShouldOutput(big.NewInt(0)).
		Assert(t)

	test.Case("energy", acc1, big.NewInt(0)).
		ShouldOutput(energy).
		Assert(t)

	test.Case("energy", acc1, big.NewInt(100)).
		ShouldOutput(big.NewInt(0)).
		Assert(t)
	{
		hash, _ := st.GetCodeHash(sharer.Prototype.Address)
		assert.False(t, hash.IsZero())
	}
}

func TestPrototypeNativeWithLongerBlockNumber(t *testing.T) {
	var (
		acc1 = luckyshare.BytesToAddress([]byte("acc1"))
	)

	db := muxdb.NewMem()
	gene := genesis.NewDevnet()
	genesisBlock, _, _, _ := gene.Build(state.NewStater(db))
	st := state.New(db, genesisBlock.Header().StateRoot())
	repo, _ := chain.NewRepository(db, genesisBlock)
	launchTime := genesisBlock.Header().Timestamp()

	for i := 1; i < 100; i++ {
		st.SetBalance(acc1, big.NewInt(int64(i)))
		st.SetEnergy(acc1, big.NewInt(int64(i)), launchTime+uint64(i)*10)
		stage, _ := st.Stage()
		stateRoot, _ := stage.Commit()
		b := new(block.Builder).
			ParentID(repo.BestBlock().Header().ID()).
			TotalScore(repo.BestBlock().Header().TotalScore() + 1).
			Timestamp(launchTime + uint64(i)*10).
			StateRoot(stateRoot).
			Build()
		repo.AddBlock(b, tx.Receipts{})
		repo.SetBestBlockID(b.Header().ID())
	}

	st = state.New(db, repo.BestBlock().Header().StateRoot())
	chain := repo.NewBestChain()

	rt := runtime.New(chain, st, &xenv.BlockContext{
		Number: luckyshare.MaxStateHistory + 1,
		Time:   repo.BestBlock().Header().Timestamp(),
	}, luckyshare.NoFork)

	test := &ctest{
		rt:     rt,
		abi:    sharer.Prototype.ABI,
		to:     sharer.Prototype.Address,
		caller: sharer.Prototype.Address,
	}

	test.Case("balance", acc1, big.NewInt(0)).
		ShouldOutput(big.NewInt(0)).
		Assert(t)

	test.Case("energy", acc1, big.NewInt(0)).
		ShouldOutput(big.NewInt(0)).
		Assert(t)

	test.Case("balance", acc1, big.NewInt(1)).
		ShouldOutput(big.NewInt(1)).
		Assert(t)

	test.Case("energy", acc1, big.NewInt(1)).
		ShouldOutput(big.NewInt(1)).
		Assert(t)

	test.Case("balance", acc1, big.NewInt(2)).
		ShouldOutput(big.NewInt(2)).
		Assert(t)

	test.Case("energy", acc1, big.NewInt(2)).
		ShouldOutput(big.NewInt(2)).
		Assert(t)
}

func TestPrototypeNativeWithBlockNumber(t *testing.T) {
	var (
		acc1 = luckyshare.BytesToAddress([]byte("acc1"))
	)

	db := muxdb.NewMem()
	gene := genesis.NewDevnet()
	genesisBlock, _, _, _ := gene.Build(state.NewStater(db))
	st := state.New(db, genesisBlock.Header().StateRoot())
	repo, _ := chain.NewRepository(db, genesisBlock)
	launchTime := genesisBlock.Header().Timestamp()

	for i := 1; i < 100; i++ {
		st.SetBalance(acc1, big.NewInt(int64(i)))
		st.SetEnergy(acc1, big.NewInt(int64(i)), launchTime+uint64(i)*10)
		stage, _ := st.Stage()
		stateRoot, _ := stage.Commit()
		b := new(block.Builder).
			ParentID(repo.BestBlock().Header().ID()).
			TotalScore(repo.BestBlock().Header().TotalScore() + 1).
			Timestamp(launchTime + uint64(i)*10).
			StateRoot(stateRoot).
			Build()
		repo.AddBlock(b, tx.Receipts{})
		repo.SetBestBlockID(b.Header().ID())
	}

	st = state.New(db, repo.BestBlock().Header().StateRoot())
	chain := repo.NewBestChain()

	rt := runtime.New(chain, st, &xenv.BlockContext{
		Number: repo.BestBlock().Header().Number(),
		Time:   repo.BestBlock().Header().Timestamp(),
	}, luckyshare.NoFork)

	test := &ctest{
		rt:     rt,
		abi:    sharer.Prototype.ABI,
		to:     sharer.Prototype.Address,
		caller: sharer.Prototype.Address,
	}

	test.Case("balance", acc1, big.NewInt(10)).
		ShouldOutput(big.NewInt(10)).
		Assert(t)

	test.Case("energy", acc1, big.NewInt(10)).
		ShouldOutput(big.NewInt(10)).
		Assert(t)

	test.Case("balance", acc1, big.NewInt(99)).
		ShouldOutput(big.NewInt(99)).
		Assert(t)

	test.Case("energy", acc1, big.NewInt(99)).
		ShouldOutput(big.NewInt(99)).
		Assert(t)
}

func newBlock(parent *block.Block, score uint64, timestamp uint64, privateKey *ecdsa.PrivateKey) *block.Block {
	b := new(block.Builder).ParentID(parent.Header().ID()).TotalScore(parent.Header().TotalScore() + score).Timestamp(timestamp).Build()
	sig, _ := crypto.Sign(b.Header().SigningHash().Bytes(), privateKey)
	return b.WithSignature(sig)
}

func TestExtensionNative(t *testing.T) {
	db := muxdb.NewMem()
	st := state.New(db, luckyshare.Bytes32{})
	gene := genesis.NewDevnet()
	genesisBlock, _, _, _ := gene.Build(state.NewStater(db))
	repo, _ := chain.NewRepository(db, genesisBlock)
	st.SetCode(sharer.Extension.Address, sharer.Extension.V2.RuntimeBytecodes())

	privKeys := make([]*ecdsa.PrivateKey, 2)

	for i := 0; i < 2; i++ {
		privateKey, _ := crypto.GenerateKey()
		privKeys[i] = privateKey
	}

	b0 := genesisBlock
	b1 := newBlock(b0, 123, 456, privKeys[0])
	b2 := newBlock(b1, 789, 321, privKeys[1])

	b1_singer, _ := b1.Header().Signer()
	b2_singer, _ := b2.Header().Signer()

	gasPayer := luckyshare.BytesToAddress([]byte("gasPayer"))

	err := repo.AddBlock(b1, nil)
	assert.Equal(t, err, nil)
	err = repo.AddBlock(b2, nil)
	assert.Equal(t, err, nil)

	assert.Equal(t, sharer.Extension.Address, sharer.Extension.Address)

	chain := repo.NewChain(b2.Header().ID())

	rt := runtime.New(chain, st, &xenv.BlockContext{Number: 2, Time: b2.Header().Timestamp(), TotalScore: b2.Header().TotalScore(), Signer: b2_singer}, luckyshare.NoFork)

	test := &ctest{
		rt:  rt,
		abi: sharer.Extension.V2.ABI,
		to:  sharer.Extension.Address,
	}

	test.Case("blake2b256", []byte("hello world")).
		ShouldOutput(luckyshare.Blake2b([]byte("hello world"))).
		Assert(t)

	expected, _ := sharer.Energy.Native(st, 0).TokenTotalSupply()
	test.Case("totalSupply").
		ShouldOutput(expected).
		Assert(t)

	test.Case("txBlockRef").
		BlockRef(tx.NewBlockRef(1)).
		ShouldOutput(tx.NewBlockRef(1)).
		Assert(t)

	test.Case("txExpiration").
		Expiration(100).
		ShouldOutput(big.NewInt(100)).
		Assert(t)

	test.Case("txProvedWork").
		ProvedWork(big.NewInt(1e12)).
		ShouldOutput(big.NewInt(1e12)).
		Assert(t)

	test.Case("txID").
		TxID(luckyshare.BytesToBytes32([]byte("txID"))).
		ShouldOutput(luckyshare.BytesToBytes32([]byte("txID"))).
		Assert(t)

	test.Case("blockID", big.NewInt(3)).
		ShouldOutput(luckyshare.Bytes32{}).
		Assert(t)

	test.Case("blockID", big.NewInt(2)).
		ShouldOutput(luckyshare.Bytes32{}).
		Assert(t)

	test.Case("blockID", big.NewInt(1)).
		ShouldOutput(b1.Header().ID()).
		Assert(t)

	test.Case("blockID", big.NewInt(0)).
		ShouldOutput(b0.Header().ID()).
		Assert(t)

	test.Case("blockTotalScore", big.NewInt(3)).
		ShouldOutput(uint64(0)).
		Assert(t)

	test.Case("blockTotalScore", big.NewInt(2)).
		ShouldOutput(b2.Header().TotalScore()).
		Assert(t)

	test.Case("blockTotalScore", big.NewInt(1)).
		ShouldOutput(b1.Header().TotalScore()).
		Assert(t)

	test.Case("blockTotalScore", big.NewInt(0)).
		ShouldOutput(b0.Header().TotalScore()).
		Assert(t)

	test.Case("blockTime", big.NewInt(3)).
		ShouldOutput(&big.Int{}).
		Assert(t)

	test.Case("blockTime", big.NewInt(2)).
		ShouldOutput(new(big.Int).SetUint64(b2.Header().Timestamp())).
		Assert(t)

	test.Case("blockTime", big.NewInt(1)).
		ShouldOutput(new(big.Int).SetUint64(b1.Header().Timestamp())).
		Assert(t)

	test.Case("blockTime", big.NewInt(0)).
		ShouldOutput(new(big.Int).SetUint64(b0.Header().Timestamp())).
		Assert(t)

	test.Case("blockSigner", big.NewInt(3)).
		ShouldOutput(luckyshare.Address{}).
		Assert(t)

	test.Case("blockSigner", big.NewInt(2)).
		ShouldOutput(b2_singer).
		Assert(t)

	test.Case("blockSigner", big.NewInt(1)).
		ShouldOutput(b1_singer).
		Assert(t)

	test.Case("txGasPayer").
		ShouldOutput(luckyshare.Address{}).
		Assert(t)

	test.Case("txGasPayer").
		GasPayer(gasPayer).
		ShouldOutput(gasPayer).
		Assert(t)

}
