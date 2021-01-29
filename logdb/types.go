// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package logdb

import (
	"fmt"
	"math/big"

	"github.com/miniBamboo/luckyshare/luckyshare"
)

//Event represents tx.Event that can be stored in db.
type Event struct {
	BlockNumber uint32
	Index       uint32
	BlockID     luckyshare.Bytes32
	BlockTime   uint64
	TxID        luckyshare.Bytes32
	TxOrigin    luckyshare.Address //contract caller
	ClauseIndex uint32
	Address     luckyshare.Address // always a contract address
	Topics      [5]*luckyshare.Bytes32
	Data        []byte
}

//Transfer represents tx.Transfer that can be stored in db.
type Transfer struct {
	BlockNumber uint32
	Index       uint32
	BlockID     luckyshare.Bytes32
	BlockTime   uint64
	TxID        luckyshare.Bytes32
	TxOrigin    luckyshare.Address
	ClauseIndex uint32
	Sender      luckyshare.Address
	Recipient   luckyshare.Address
	Amount      *big.Int
}

type Order string

const (
	ASC  Order = "asc"
	DESC Order = "desc"
)

type Range struct {
	From uint32
	To   uint32
}

type Options struct {
	Offset uint64
	Limit  uint64
}

type EventCriteria struct {
	Address *luckyshare.Address // always a contract address
	Topics  [5]*luckyshare.Bytes32
}

func (c *EventCriteria) toWhereCondition() (cond string, args []interface{}) {
	cond = "1"
	if c.Address != nil {
		cond += " AND address = " + refIDQuery
		args = append(args, c.Address.Bytes())
	}
	for i, topic := range c.Topics {
		if topic != nil {
			cond += fmt.Sprintf(" AND topic%v = ", i) + refIDQuery
			args = append(args, topic.Bytes())
		}
	}
	return
}

//EventFilter filter
type EventFilter struct {
	CriteriaSet []*EventCriteria
	Range       *Range
	Options     *Options
	Order       Order //default asc
}

type TransferCriteria struct {
	TxOrigin  *luckyshare.Address //who send transaction
	Sender    *luckyshare.Address //who transferred tokens
	Recipient *luckyshare.Address //who recieved tokens
}

func (c *TransferCriteria) toWhereCondition() (cond string, args []interface{}) {
	cond = "1"
	if c.TxOrigin != nil {
		cond += " AND txOrigin = " + refIDQuery
		args = append(args, c.TxOrigin.Bytes())
	}
	if c.Sender != nil {
		cond += " AND sender = " + refIDQuery
		args = append(args, c.Sender.Bytes())
	}
	if c.Recipient != nil {
		cond += " AND recipient = " + refIDQuery
		args = append(args, c.Recipient.Bytes())
	}
	return
}

type TransferFilter struct {
	CriteriaSet []*TransferCriteria
	Range       *Range
	Options     *Options
	Order       Order //default asc
}
