// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package node

import (
	"github.com/miniBamboo/luckyshare/commu"
	"github.com/miniBamboo/luckyshare/luckyshare"
)

type Network interface {
	PeersStats() []*commu.PeerStats
}

type PeerStats struct {
	Name        string             `json:"name"`
	BestBlockID luckyshare.Bytes32 `json:"bestBlockID"`
	TotalScore  uint64             `json:"totalScore"`
	PeerID      string             `json:"peerID"`
	NetAddr     string             `json:"netAddr"`
	Inbound     bool               `json:"inbound"`
	Duration    uint64             `json:"duration"`
}

func ConvertPeersStats(ss []*commu.PeerStats) []*PeerStats {
	if len(ss) == 0 {
		return nil
	}
	peersStats := make([]*PeerStats, len(ss))
	for i, peerStats := range ss {
		peersStats[i] = &PeerStats{
			Name:        peerStats.Name,
			BestBlockID: peerStats.BestBlockID,
			TotalScore:  peerStats.TotalScore,
			PeerID:      peerStats.PeerID,
			NetAddr:     peerStats.NetAddr,
			Inbound:     peerStats.Inbound,
			Duration:    peerStats.Duration,
		}
	}
	return peersStats
}
