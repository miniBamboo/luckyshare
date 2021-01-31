// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/miniBamboo/luckyshare/common/stackedmap"
	"github.com/miniBamboo/luckyshare/luckyshare"
	"github.com/miniBamboo/luckyshare/muxdb"
)

const (
	// AccountTrieName is the name of account trie.
	AccountTrieName = "a"

	codeStoreName = "state.code"
)

// StorageTrieName returns the name of storage trie.
//
// Each storage trie has a unique name, which can improve IO performance.
func StorageTrieName(addressHash luckyshare.Bytes32) string {
	return "s" + string(addressHash[:16])
}

// Error is the error caused by state access failure.
type Error struct {
	cause error
}

func (e *Error) Error() string {
	return fmt.Sprintf("state: %v", e.cause)
}

// State manages the world state.
type State struct {
	db    *muxdb.MuxDB
	trie  *muxdb.Trie                          // the accounts trie reader
	cache map[luckyshare.Address]*cachedObject // cache of accounts trie
	sm    *stackedmap.StackedMap               // keeps revisions of accounts state
}

// New create state object.
func New(db *muxdb.MuxDB, root luckyshare.Bytes32) *State {
	state := State{
		db:    db,
		trie:  db.NewSecureTrie(AccountTrieName, root),
		cache: make(map[luckyshare.Address]*cachedObject),
	}

	state.sm = stackedmap.New(func(key interface{}) (interface{}, bool, error) {
		return state.cacheGetter(key)
	})
	return &state
}

// NewStater create stater object.
func (s *State) NewStater() *Stater {
	return NewStater(s.db)
}

// cacheGetter implements stackedmap.MapGetter.
func (s *State) cacheGetter(key interface{}) (value interface{}, exist bool, err error) {
	switch k := key.(type) {
	case luckyshare.Address: // get account
		obj, err := s.getCachedObject(k)
		if err != nil {
			return nil, false, err
		}
		return &obj.data, true, nil
	case codeKey: // get code
		obj, err := s.getCachedObject(luckyshare.Address(k))
		if err != nil {
			return nil, false, err
		}
		code, err := obj.GetCode()
		if err != nil {
			return nil, false, err
		}
		return code, true, nil
	case storageKey: // get storage
		obj, err := s.getCachedObject(k.addr)
		if err != nil {
			return nil, false, err
		}
		v, err := obj.GetStorage(k.key)
		if err != nil {
			return nil, false, err
		}
		return v, true, nil
	}
	panic(fmt.Errorf("unexpected key type %+v", key))
}

func (s *State) getCachedObject(addr luckyshare.Address) (*cachedObject, error) {
	if co, ok := s.cache[addr]; ok {
		return co, nil
	}
	a, err := loadAccount(s.trie, addr)
	if err != nil {
		return nil, err
	}
	co := newCachedObject(s.db, addr, a)
	s.cache[addr] = co
	return co, nil
}

// getAccount gets account by address. the returned account should not be modified.
func (s *State) getAccount(addr luckyshare.Address) (*Account, error) {
	v, _, err := s.sm.Get(addr)
	if err != nil {
		return nil, err
	}
	return v.(*Account), nil
}

// getAccountCopy get a copy of account by address.
func (s *State) getAccountCopy(addr luckyshare.Address) (Account, error) {
	acc, err := s.getAccount(addr)
	if err != nil {
		return Account{}, err
	}
	return *acc, nil
}

func (s *State) updateAccount(addr luckyshare.Address, acc *Account) {
	s.sm.Put(addr, acc)
}

// GetBalance returns balance for the given address.
func (s *State) GetBalance(addr luckyshare.Address) (*big.Int, error) {
	acc, err := s.getAccount(addr)
	if err != nil {
		return nil, &Error{err}
	}
	return acc.Balance, nil
}

// SetBalance set balance for the given address.
func (s *State) SetBalance(addr luckyshare.Address, balance *big.Int) error {
	cpy, err := s.getAccountCopy(addr)
	if err != nil {
		return &Error{err}
	}
	cpy.Balance = balance
	s.updateAccount(addr, &cpy)
	return nil
}

// GetEnergy get energy for the given address at block number specified.
func (s *State) GetEnergy(addr luckyshare.Address, blockTime uint64) (*big.Int, error) {
	acc, err := s.getAccount(addr)
	if err != nil {
		return nil, &Error{err}
	}
	return acc.CalcEnergy(blockTime), nil
}

// SetEnergy set energy at block number for the given address.
func (s *State) SetEnergy(addr luckyshare.Address, energy *big.Int, blockTime uint64) error {
	cpy, err := s.getAccountCopy(addr)
	if err != nil {
		return &Error{err}
	}
	cpy.Energy, cpy.BlockTime = energy, blockTime
	s.updateAccount(addr, &cpy)
	return nil
}

// GetMaster get master for the given address.
// Master can move energy, manage users...
func (s *State) GetMaster(addr luckyshare.Address) (luckyshare.Address, error) {
	acc, err := s.getAccount(addr)
	if err != nil {
		return luckyshare.Address{}, &Error{err}
	}
	return luckyshare.BytesToAddress(acc.Master), nil
}

// SetMaster set master for the given address.
func (s *State) SetMaster(addr luckyshare.Address, master luckyshare.Address) error {
	cpy, err := s.getAccountCopy(addr)
	if err != nil {
		return &Error{err}
	}
	if master.IsZero() {
		cpy.Master = nil
	} else {
		cpy.Master = master[:]
	}
	s.updateAccount(addr, &cpy)
	return nil
}

// GetStorage returns storage value for the given address and key.
func (s *State) GetStorage(addr luckyshare.Address, key luckyshare.Bytes32) (luckyshare.Bytes32, error) {
	raw, err := s.GetRawStorage(addr, key)
	if err != nil {
		return luckyshare.Bytes32{}, &Error{err}
	}
	if len(raw) == 0 {
		return luckyshare.Bytes32{}, nil
	}
	kind, content, _, err := rlp.Split(raw)
	if err != nil {
		return luckyshare.Bytes32{}, &Error{err}
	}
	if kind == rlp.List {
		// special case for rlp list, it should be customized storage value
		// return hash of raw data
		return luckyshare.Blake2b(raw), nil
	}
	return luckyshare.BytesToBytes32(content), nil
}

// SetStorage set storage value for the given address and key.
func (s *State) SetStorage(addr luckyshare.Address, key, value luckyshare.Bytes32) {
	if value.IsZero() {
		s.SetRawStorage(addr, key, nil)
		return
	}
	v, _ := rlp.EncodeToBytes(bytes.TrimLeft(value[:], "\x00"))
	s.SetRawStorage(addr, key, v)
}

// GetRawStorage returns storage value in rlp raw for given address and key.
func (s *State) GetRawStorage(addr luckyshare.Address, key luckyshare.Bytes32) (rlp.RawValue, error) {
	data, _, err := s.sm.Get(storageKey{addr, key})
	if err != nil {
		return nil, &Error{err}
	}
	return data.(rlp.RawValue), nil
}

// SetRawStorage set storage value in rlp raw.
func (s *State) SetRawStorage(addr luckyshare.Address, key luckyshare.Bytes32, raw rlp.RawValue) {
	s.sm.Put(storageKey{addr, key}, raw)
}

// EncodeStorage set storage value encoded by given enc method.
// Error returned by end will be absorbed by State instance.
func (s *State) EncodeStorage(addr luckyshare.Address, key luckyshare.Bytes32, enc func() ([]byte, error)) error {
	raw, err := enc()
	if err != nil {
		return &Error{err}
	}
	s.SetRawStorage(addr, key, raw)
	return nil
}

// DecodeStorage get and decode storage value.
// Error returned by dec will be absorbed by State instance.
func (s *State) DecodeStorage(addr luckyshare.Address, key luckyshare.Bytes32, dec func([]byte) error) error {
	raw, err := s.GetRawStorage(addr, key)
	if err != nil {
		return &Error{err}
	}
	if err := dec(raw); err != nil {
		return &Error{err}
	}
	return nil
}

// GetCode returns code for the given address.
func (s *State) GetCode(addr luckyshare.Address) ([]byte, error) {
	v, _, err := s.sm.Get(codeKey(addr))
	if err != nil {
		return nil, &Error{err}
	}
	return v.([]byte), nil
}

// GetCodeHash returns code hash for the given address.
func (s *State) GetCodeHash(addr luckyshare.Address) (luckyshare.Bytes32, error) {
	acc, err := s.getAccount(addr)
	if err != nil {
		return luckyshare.Bytes32{}, &Error{err}
	}
	return luckyshare.BytesToBytes32(acc.CodeHash), nil
}

// SetCode set code for the given address.
func (s *State) SetCode(addr luckyshare.Address, code []byte) error {
	var codeHash []byte
	if len(code) > 0 {
		s.sm.Put(codeKey(addr), code)
		codeHash = crypto.Keccak256(code)
		codeCache.Add(string(codeHash), code)
	} else {
		s.sm.Put(codeKey(addr), []byte(nil))
	}
	cpy, err := s.getAccountCopy(addr)
	if err != nil {
		return &Error{err}
	}
	cpy.CodeHash = codeHash
	s.updateAccount(addr, &cpy)
	return nil
}

// Exists returns whether an account exists at the given address.
// See Account.IsEmpty()
func (s *State) Exists(addr luckyshare.Address) (bool, error) {
	acc, err := s.getAccount(addr)
	if err != nil {
		return false, &Error{err}
	}
	return !acc.IsEmpty(), nil
}

// Delete delete an account at the given address.
// That's set balance, energy and code to zero value.
func (s *State) Delete(addr luckyshare.Address) {
	s.sm.Put(codeKey(addr), []byte(nil))
	s.updateAccount(addr, emptyAccount())
}

// NewCheckpoint makes a checkpoint of current state.
// It returns revision of the checkpoint.
func (s *State) NewCheckpoint() int {
	return s.sm.Push()
}

// RevertTo revert to checkpoint specified by revision.
func (s *State) RevertTo(revision int) {
	s.sm.PopTo(revision)
}

// BuildStorageTrie build up storage trie for given address with cumulative changes.
func (s *State) BuildStorageTrie(addr luckyshare.Address) (*muxdb.Trie, error) {
	acc, err := s.getAccount(addr)
	if err != nil {
		return nil, &Error{err}
	}

	root := luckyshare.BytesToBytes32(acc.StorageRoot)

	trie := s.db.NewSecureTrie(StorageTrieName(luckyshare.Blake2b(addr[:])), root)

	// traverse journal to filter out storage changes for addr
	s.sm.Journal(func(k, v interface{}) bool {
		switch key := k.(type) {
		case storageKey:
			if key.addr == addr {
				err = saveStorage(trie, key.key, v.(rlp.RawValue))
				if err != nil {
					return false
				}
			}
		}
		return true
	})
	if err != nil {
		return nil, &Error{err}
	}
	return trie, nil
}

// Stage makes a stage object to compute hash of trie or commit all changes.
func (s *State) Stage() (*Stage, error) {
	type changed struct {
		data    Account
		storage map[luckyshare.Bytes32]rlp.RawValue
	}

	var (
		changes = make(map[luckyshare.Address]*changed)
		codes   = make(map[luckyshare.Bytes32][]byte)
	)

	// get or create changed account
	getChanged := func(addr luckyshare.Address) (*changed, error) {
		if obj, ok := changes[addr]; ok {
			return obj, nil
		}
		co, err := s.getCachedObject(addr)
		if err != nil {
			return nil, &Error{err}
		}

		c := &changed{data: co.data}
		changes[addr] = c
		return c, nil
	}

	var jerr error
	// traverse journal to build changes
	s.sm.Journal(func(k, v interface{}) bool {
		var c *changed
		switch key := k.(type) {
		case luckyshare.Address:
			if c, jerr = getChanged(key); jerr != nil {
				return false
			}
			c.data = *(v.(*Account))
		case codeKey:
			code := v.([]byte)
			if len(code) > 0 {
				codes[luckyshare.Bytes32(crypto.Keccak256Hash(code))] = code
			}
		case storageKey:
			if c, jerr = getChanged(key.addr); jerr != nil {
				return false
			}
			if c.storage == nil {
				c.storage = make(map[luckyshare.Bytes32]rlp.RawValue)
			}
			c.storage[key.key] = v.(rlp.RawValue)
		}
		return true
	})
	if jerr != nil {
		return nil, &Error{jerr}
	}

	stage := &Stage{
		db:          s.db,
		accountTrie: s.db.NewSecureTrie(AccountTrieName, s.trie.Hash()),
		codes:       codes,
	}

	for addr, c := range changes {
		// skip storage changes if account is empty
		if !c.data.IsEmpty() {
			if len(c.storage) > 0 {
				storageTrie := s.db.NewSecureTrie(
					StorageTrieName(luckyshare.Blake2b(addr[:])),
					luckyshare.BytesToBytes32(c.data.StorageRoot))

				stage.storageTries = append(stage.storageTries, storageTrie)
				for k, v := range c.storage {
					if err := saveStorage(storageTrie, k, v); err != nil {
						return nil, &Error{err}
					}
				}
				c.data.StorageRoot = storageTrie.Hash().Bytes()
			}
		}
		if err := saveAccount(stage.accountTrie, addr, &c.data); err != nil {
			return nil, &Error{err}
		}
	}
	return stage, nil
}

type (
	storageKey struct {
		addr luckyshare.Address
		key  luckyshare.Bytes32
	}
	codeKey luckyshare.Address
)
