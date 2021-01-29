// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package node

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/miniBamboo/luckyshare/luckyshare"
)

type Master struct {
	PrivateKey  *ecdsa.PrivateKey
	Beneficiary *luckyshare.Address
}

func (m *Master) Address() luckyshare.Address {
	return luckyshare.Address(crypto.PubkeyToAddress(m.PrivateKey.PublicKey))
}
