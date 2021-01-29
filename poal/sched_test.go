// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package poa_test

import (
	"testing"

	"github.com/miniBamboo/luckyshare/luckyshare"
	"github.com/miniBamboo/luckyshare/poa"
	"github.com/stretchr/testify/assert"
)

var (
	p1 = luckyshare.BytesToAddress([]byte("p1"))
	p2 = luckyshare.BytesToAddress([]byte("p2"))
	p3 = luckyshare.BytesToAddress([]byte("p3"))
	p4 = luckyshare.BytesToAddress([]byte("p4"))
	p5 = luckyshare.BytesToAddress([]byte("p5"))

	proposers = []poa.Proposer{
		{p1, false},
		{p2, true},
		{p3, false},
		{p4, false},
		{p5, false},
	}

	parentTime = uint64(1001)
)

func TestSchedule(t *testing.T) {

	_, err := poa.NewScheduler(luckyshare.BytesToAddress([]byte("px")), proposers, 1, parentTime)
	assert.NotNil(t, err)

	sched, _ := poa.NewScheduler(p1, proposers, 1, parentTime)

	for i := uint64(0); i < 100; i++ {
		now := parentTime + i*luckyshare.BlockInterval/2
		nbt := sched.Schedule(now)
		assert.True(t, nbt >= now)
		assert.True(t, sched.IsTheTime(nbt))
	}
}

func TestIsTheTime(t *testing.T) {
	sched, _ := poa.NewScheduler(p2, proposers, 1, parentTime)

	tests := []struct {
		now  uint64
		want bool
	}{
		{parentTime - 1, false},
		{parentTime + luckyshare.BlockInterval/2, false},
		{parentTime + luckyshare.BlockInterval, true},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, sched.IsTheTime(tt.now))
	}
}

func TestUpdates(t *testing.T) {

	sched, _ := poa.NewScheduler(p1, proposers, 1, parentTime)

	tests := []struct {
		newBlockTime uint64
		want         uint64
	}{
		{parentTime + luckyshare.BlockInterval, 2},
		{parentTime + luckyshare.BlockInterval*30, 1},
	}

	for _, tt := range tests {
		_, score := sched.Updates(tt.newBlockTime)
		assert.Equal(t, tt.want, score)
	}
}
