// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block_test

import (
	"math"
	"testing"

	"github.com/miniBamboo/luckyshare/block"
	"github.com/miniBamboo/luckyshare/luckyshare"
	"github.com/stretchr/testify/assert"
)

func TestGasLimit_IsValid(t *testing.T) {

	tests := []struct {
		gl       uint64
		parentGL uint64
		want     bool
	}{
		{luckyshare.MinGasLimit, luckyshare.MinGasLimit, true},
		{luckyshare.MinGasLimit - 1, luckyshare.MinGasLimit, false},
		{luckyshare.MinGasLimit, luckyshare.MinGasLimit * 2, false},
		{luckyshare.MinGasLimit * 2, luckyshare.MinGasLimit, false},
		{luckyshare.MinGasLimit + luckyshare.MinGasLimit/luckyshare.GasLimitBoundDivisor, luckyshare.MinGasLimit, true},
		{luckyshare.MinGasLimit*2 + luckyshare.MinGasLimit/luckyshare.GasLimitBoundDivisor, luckyshare.MinGasLimit * 2, true},
		{luckyshare.MinGasLimit*2 - luckyshare.MinGasLimit/luckyshare.GasLimitBoundDivisor, luckyshare.MinGasLimit * 2, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, block.GasLimit(tt.gl).IsValid(tt.parentGL))
	}
}

func TestGasLimit_Adjust(t *testing.T) {

	tests := []struct {
		gl    uint64
		delta int64
		want  uint64
	}{
		{luckyshare.MinGasLimit, 1, luckyshare.MinGasLimit + 1},
		{luckyshare.MinGasLimit, -1, luckyshare.MinGasLimit},
		{math.MaxUint64, 1, math.MaxUint64},
		{luckyshare.MinGasLimit, int64(luckyshare.MinGasLimit), luckyshare.MinGasLimit + luckyshare.MinGasLimit/luckyshare.GasLimitBoundDivisor},
		{luckyshare.MinGasLimit * 2, -int64(luckyshare.MinGasLimit), luckyshare.MinGasLimit*2 - (luckyshare.MinGasLimit*2)/luckyshare.GasLimitBoundDivisor},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, block.GasLimit(tt.gl).Adjust(tt.delta))
	}
}

func TestGasLimit_Qualify(t *testing.T) {
	tests := []struct {
		gl       uint64
		parentGL uint64
		want     uint64
	}{
		{luckyshare.MinGasLimit, luckyshare.MinGasLimit, luckyshare.MinGasLimit},
		{luckyshare.MinGasLimit - 1, luckyshare.MinGasLimit, luckyshare.MinGasLimit},
		{luckyshare.MinGasLimit, luckyshare.MinGasLimit * 2, luckyshare.MinGasLimit*2 - (luckyshare.MinGasLimit*2)/luckyshare.GasLimitBoundDivisor},
		{luckyshare.MinGasLimit * 2, luckyshare.MinGasLimit, luckyshare.MinGasLimit + luckyshare.MinGasLimit/luckyshare.GasLimitBoundDivisor},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, block.GasLimit(tt.gl).Qualify(tt.parentGL))
	}
}
