package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/miniBamboo/luckyshare/luckyshare"
	"github.com/miniBamboo/luckyshare/muxdb"
	"github.com/miniBamboo/luckyshare/muxdb/kv"
	"github.com/miniBamboo/luckyshare/trie"
)

const (
	rootKey       = "root"
	totalKeyCount = 5000000
	readKeyCount  = 5000
	iterateCount  = 10000
)

type bench struct {
	path      string
	optimized bool
}

func (b *bench) openDB() (*muxdb.MuxDB, error) {
	return muxdb.Open(b.path, &muxdb.Options{
		EncodedTrieNodeCacheSizeMB:   0,
		DecodedTrieNodeCacheCapacity: 0,
		DisablePageCache:             true,
		OpenFilesCacheCapacity:       500,
		ReadCacheMB:                  256,
		WriteBufferMB:                128,
	})
}

func (b *bench) Write(f func(put kv.PutFunc) error) error {
	db, err := b.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	root, err := loadRoot(db)
	if err != nil {
		return err
	}

	if !root.IsZero() {
		return nil
	}

	if b.optimized {
		tr := db.NewTrie("", luckyshare.Bytes32{})
		count := 0

		if err := f(func(key, val []byte) error {
			if err := tr.Update(key, val); err != nil {
				return err
			}
			if count > 0 && count%10000 == 0 {
				if _, err := tr.CommitPermanently(); err != nil {
					return err
				}
			}
			count++
			return nil
		}); err != nil {
			return err
		}
		if root, err = tr.CommitPermanently(); err != nil {
			return err
		}
	} else {
		tr, err := trie.New(luckyshare.Bytes32{}, db.LowStore())
		if err != nil {
			return err
		}
		count := 0

		if err := f(func(key, val []byte) error {
			if err := tr.TryUpdate(key, val); err != nil {
				return err
			}
			if count > 0 && count%10000 == 0 {
				if _, err := tr.Commit(); err != nil {
					return err
				}
			}
			count++
			return nil
		}); err != nil {
			return err
		}
		if root, err = tr.Commit(); err != nil {
			return err
		}
	}
	return saveRoot(db, root)
}

func (b *bench) Read(f func(get kv.GetFunc) error) error {
	db, err := b.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	root, err := loadRoot(db)
	if err != nil {
		return err
	}

	if b.optimized {
		return f(func(key []byte) ([]byte, error) {
			return db.NewTrie("", root).Get(key)
		})
	}

	return f(func(key []byte) ([]byte, error) {
		tr, err := trie.New(root, db.LowStore())
		if err != nil {
			return nil, err
		}
		return tr.TryGet(key)
	})
}

func (b *bench) Iterate(n int) error {
	db, err := b.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	root, err := loadRoot(db)
	if err != nil {
		return err
	}

	var iter trie.NodeIterator
	if b.optimized {
		iter = db.NewTrie("", root).NodeIterator(nil)
	} else {
		tr, err := trie.New(root, db.LowStore())
		if err != nil {
			return err
		}
		iter = tr.NodeIterator(nil)
	}

	for i := 0; i < n && iter.Next(true); i++ {
	}
	return iter.Error()
}

func (b *bench) Run() error {
	fmt.Println("fill", totalKeyCount, "keys ...")
	t := time.Now().UnixNano()
	if err := b.Write(func(put kv.PutFunc) error {
		for i := 0; i < totalKeyCount; i++ {
			var b4 [4]byte
			binary.BigEndian.PutUint32(b4[:], uint32(i))
			key := luckyshare.Blake2b(b4[:])
			value := luckyshare.Blake2b(key[:])
			if err := put(key[:], value[:]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	fmt.Println("elapse:", time.Duration(time.Now().UnixNano()-t))

	fmt.Println("read", readKeyCount, "keys ...")
	t = time.Now().UnixNano()
	if err := b.Read(func(get kv.GetFunc) error {
		for i := 0; i < readKeyCount; i++ {
			var b4 [4]byte
			binary.BigEndian.PutUint32(b4[:], uint32(i))
			key := luckyshare.Blake2b(b4[:])
			if _, err := get(key[:]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	fmt.Println("elapse:", time.Duration(time.Now().UnixNano()-t))

	fmt.Println("iterate", iterateCount, "nodes ...")
	t = time.Now().UnixNano()
	if err := b.Iterate(iterateCount); err != nil {
		return err
	}
	fmt.Println("elapse:", time.Duration(time.Now().UnixNano()-t))
	return nil
}

func loadRoot(db *muxdb.MuxDB) (luckyshare.Bytes32, error) {
	val, err := db.NewStore("c").Get([]byte(rootKey))
	if err != nil {
		if db.IsNotFound(err) {
			return luckyshare.Bytes32{}, nil
		}
		return luckyshare.Bytes32{}, err
	}
	return luckyshare.BytesToBytes32(val), nil
}

func saveRoot(db *muxdb.MuxDB, root luckyshare.Bytes32) error {
	return db.NewStore("c").Put([]byte(rootKey), root[:])
}
