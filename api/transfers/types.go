// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package transfers

import (
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/miniBamboo/luckyshare/api/events"
	"github.com/miniBamboo/luckyshare/logdb"
	"github.com/miniBamboo/luckyshare/luckyshare"
)

type LogMeta struct {
	BlockID        luckyshare.Bytes32 `json:"blockID"`
	BlockNumber    uint32             `json:"blockNumber"`
	BlockTimestamp uint64             `json:"blockTimestamp"`
	TxID           luckyshare.Bytes32 `json:"txID"`
	TxOrigin       luckyshare.Address `json:"txOrigin"`
	ClauseIndex    uint32             `json:"clauseIndex"`
}

type FilteredTransfer struct {
	Sender    luckyshare.Address    `json:"sender"`
	Recipient luckyshare.Address    `json:"recipient"`
	Amount    *math.HexOrDecimal256 `json:"amount"`
	Meta      LogMeta               `json:"meta"`
}

func convertTransfer(transfer *logdb.Transfer) *FilteredTransfer {
	v := math.HexOrDecimal256(*transfer.Amount)
	return &FilteredTransfer{
		Sender:    transfer.Sender,
		Recipient: transfer.Recipient,
		Amount:    &v,
		Meta: LogMeta{
			BlockID:        transfer.BlockID,
			BlockNumber:    transfer.BlockNumber,
			BlockTimestamp: transfer.BlockTime,
			TxID:           transfer.TxID,
			TxOrigin:       transfer.TxOrigin,
			ClauseIndex:    transfer.ClauseIndex,
		},
	}
}

type TransferFilter struct {
	CriteriaSet []*logdb.TransferCriteria
	Range       *events.Range
	Options     *logdb.Options
	Order       logdb.Order //default asc
}
