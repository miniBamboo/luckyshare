// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import (
	"github.com/miniBamboo/luckyshare/kv"
	"github.com/miniBamboo/luckyshare/luckyshare"
	"github.com/miniBamboo/luckyshare/muxdb"
)

// Stage abstracts changes on the main accounts trie.
type Stage struct {
	db           *muxdb.MuxDB
	accountTrie  *muxdb.Trie
	storageTries []*muxdb.Trie
	codes        map[luckyshare.Bytes32][]byte
}

// Hash computes hash of the main accounts trie.
func (s *Stage) Hash() luckyshare.Bytes32 {
	return s.accountTrie.Hash()
}

// Commit commits all changes into main accounts trie and storage tries.
func (s *Stage) Commit() (luckyshare.Bytes32, error) {
	codeStore := s.db.NewStore(codeStoreName)

	// write codes
	if err := codeStore.Batch(func(w kv.PutFlusher) error {
		for hash, code := range s.codes {
			if err := w.Put(hash[:], code); err != nil {
				return &Error{err}
			}
		}
		return nil
	}); err != nil {
		return luckyshare.Bytes32{}, &Error{err}
	}

	// commit storage tries
	for _, t := range s.storageTries {
		if _, err := t.Commit(); err != nil {
			return luckyshare.Bytes32{}, &Error{err}
		}
	}

	// commit accounts trie
	return s.accountTrie.Commit()
}
