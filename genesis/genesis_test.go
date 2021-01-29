// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package genesis_test

import (
	"testing"

	"github.com/miniBamboo/luckyshare/genesis"
	"github.com/miniBamboo/luckyshare/luckyshare"
	"github.com/miniBamboo/luckyshare/muxdb"
	"github.com/miniBamboo/luckyshare/state"
	"github.com/stretchr/testify/assert"
)

func TestTestnetGenesis(t *testing.T) {
	db := muxdb.NewMem()
	gene := genesis.NewTestnet()

	b0, _, _, err := gene.Build(state.NewStater(db))
	assert.Nil(t, err)

	st := state.New(db, b0.Header().StateRoot())

	v, err := st.Exists(luckyshare.MustParseAddress("0xe59D475Abe695c7f67a8a2321f33A856B0B4c71d"))
	assert.Nil(t, err)
	assert.True(t, v)
}
