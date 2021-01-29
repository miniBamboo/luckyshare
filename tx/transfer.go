// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package tx

import (
	"math/big"

	"github.com/miniBamboo/luckyshare/luckyshare"
)

// Transfer token transfer log.
type Transfer struct {
	Sender    luckyshare.Address
	Recipient luckyshare.Address
	Amount    *big.Int
}

// Transfers slisce of transfer logs.
type Transfers []*Transfer
