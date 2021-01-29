// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/miniBamboo/luckyshare/luckyshare"
	"github.com/miniBamboo/luckyshare/muxdb"
	"github.com/stretchr/testify/assert"
)

func TestCachedObject(t *testing.T) {
	db := muxdb.NewMem()
	addr := luckyshare.Address{}

	stgTrie := db.NewSecureTrie(StorageTrieName(luckyshare.Blake2b(addr[:])), luckyshare.Bytes32{})
	storages := []struct {
		k luckyshare.Bytes32
		v rlp.RawValue
	}{
		{luckyshare.BytesToBytes32([]byte("key1")), []byte("value1")},
		{luckyshare.BytesToBytes32([]byte("key2")), []byte("value2")},
		{luckyshare.BytesToBytes32([]byte("key3")), []byte("value3")},
		{luckyshare.BytesToBytes32([]byte("key4")), []byte("value4")},
	}

	for _, s := range storages {
		saveStorage(stgTrie, s.k, s.v)
	}

	storageRoot, _ := stgTrie.Commit()

	code := make([]byte, 100)
	rand.Read(code)

	codeHash := crypto.Keccak256(code)
	db.NewStore(codeStoreName).Put(codeHash, code)

	account := Account{
		Balance:     &big.Int{},
		CodeHash:    codeHash,
		StorageRoot: storageRoot[:],
	}

	obj := newCachedObject(db, addr, &account)

	assert.Equal(t,
		M(code, nil),
		M(obj.GetCode()))

	for _, s := range storages {
		assert.Equal(t,
			M(s.v, nil),
			M(obj.GetStorage(s.k)))

	}
}
