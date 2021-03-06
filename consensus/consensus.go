// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"fmt"

	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/miniBamboo/luckyshare/block"
	"github.com/miniBamboo/luckyshare/chain"
	sharer "github.com/miniBamboo/luckyshare/sharer"

	"github.com/miniBamboo/luckyshare/luckyshare"
	"github.com/miniBamboo/luckyshare/runtime"
	"github.com/miniBamboo/luckyshare/state"
	"github.com/miniBamboo/luckyshare/tx"
	"github.com/miniBamboo/luckyshare/xenv"
)

// Consensus check whether the block is verified,
// and predicate which trunk it belong to.
type Consensus struct {
	repo                 *chain.Repository
	stater               *state.Stater
	forkConfig           luckyshare.ForkConfig
	correctReceiptsRoots map[string]string
	candidatesCache      *simplelru.LRU
}

// New create a Consensus instance.
func New(repo *chain.Repository, stater *state.Stater, forkConfig luckyshare.ForkConfig) *Consensus {
	candidatesCache, _ := simplelru.NewLRU(16, nil)
	return &Consensus{
		repo:                 repo,
		stater:               stater,
		forkConfig:           forkConfig,
		correctReceiptsRoots: luckyshare.LoadCorrectReceiptsRoots(),
		candidatesCache:      candidatesCache,
	}
}

// Process process a block.
func (c *Consensus) Process(blk *block.Block, nowTimestamp uint64) (*state.Stage, tx.Receipts, error) {
	header := blk.Header()

	if _, err := c.repo.GetBlockSummary(header.ID()); err != nil {
		if !c.repo.IsNotFound(err) {
			return nil, nil, err
		}
	} else {
		return nil, nil, errKnownBlock
	}

	parentSummary, err := c.repo.GetBlockSummary(header.ParentID())
	if err != nil {
		if !c.repo.IsNotFound(err) {
			return nil, nil, err
		}
		return nil, nil, errParentMissing
	}

	state := c.stater.NewState(parentSummary.Header.StateRoot())

	vip191 := c.forkConfig.VIP191
	if vip191 == 0 {
		vip191 = 1
	}
	// Before process hook of VIP-191, update sharer extension contract's code to V2
	if header.Number() == vip191 {
		if err := state.SetCode(sharer.Extension.Address, sharer.Extension.V2.RuntimeBytecodes()); err != nil {
			return nil, nil, err
		}
	}

	var features tx.Features
	if header.Number() >= vip191 {
		features |= tx.DelegationFeature
	}

	if header.TxsFeatures() != features {
		return nil, nil, consensusError(fmt.Sprintf("block txs features invalid: want %v, have %v", features, header.TxsFeatures()))
	}

	stage, receipts, err := c.validate(state, blk, parentSummary.Header, nowTimestamp)
	if err != nil {
		return nil, nil, err
	}

	return stage, receipts, nil
}

func (c *Consensus) NewRuntimeForReplay(header *block.Header, skipPoA bool) (*runtime.Runtime, error) {
	signer, err := header.Signer()
	if err != nil {
		return nil, err
	}
	parentSummary, err := c.repo.GetBlockSummary(header.ParentID())
	if err != nil {
		if !c.repo.IsNotFound(err) {
			return nil, err
		}
		return nil, errParentMissing
	}
	state := c.stater.NewState(parentSummary.Header.StateRoot())
	if !skipPoA {
		if _, err := c.validateProposer(header, parentSummary.Header, state); err != nil {
			return nil, err
		}
	}

	return runtime.New(
		c.repo.NewChain(header.ParentID()),
		state,
		&xenv.BlockContext{
			Beneficiary: header.Beneficiary(),
			Signer:      signer,
			Number:      header.Number(),
			Time:        header.Timestamp(),
			GasLimit:    header.GasLimit(),
			TotalScore:  header.TotalScore(),
		},
		c.forkConfig), nil
}
