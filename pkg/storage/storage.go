package storage

import (
	"fmt"
	"os"
	"path"

	"elasticgpu.io/elastic-gpu-agent/pkg/types"
	"github.com/boltdb/bolt"
	_ "github.com/boltdb/bolt"
)

const RootBucket = "root"

type Storage interface {
	Save(info *types.PodInfo) error
	Load(namespace, name string) (*types.PodInfo, error)
	LoadOrCreate(namespace, name string) *types.PodInfo
	Delete(namespace, name string) error
	ForEach(func(info *types.PodInfo) error) error
	Close() error
}

type BoltStorage struct {
	db *bolt.DB
}

func NewStorage(file string) (Storage, error) {
	if err := os.MkdirAll(path.Dir(file), 0755); err != nil {
		return nil, err
	}
	db, err := bolt.Open(file, 0755, bolt.DefaultOptions)
	if err != nil {
		return nil, err
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(RootBucket))
		return err
	}); err != nil {
		return nil, err
	}

	return &BoltStorage{db: db}, nil
}

func (b *BoltStorage) Delete(namespace, name string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(RootBucket)).Delete([]byte(fmt.Sprintf("%s/%s", namespace, name)))
	})
}

func (b *BoltStorage) Save(info *types.PodInfo) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(RootBucket)).Put(info.Key(), info.Val())
	})
}

func (b *BoltStorage) Load(namespace, name string) (*types.PodInfo, error) {
	pod := types.NewPI(namespace, name)
	return pod, b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(RootBucket))
		val := bucket.Get(pod.Key())
		if val == nil {
			return fmt.Errorf("no such key: %s", string(pod.Key()))
		}
		return pod.SetVal(val)
	})
}

func (b *BoltStorage) LoadOrCreate(namespace, name string) *types.PodInfo {
	pod, err := b.Load(namespace, name)
	if err == nil {
		return pod
	}
	return types.NewPI(namespace, name)
}

func (b *BoltStorage) ForEach(f func(info *types.PodInfo) error) error {
	return b.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(RootBucket)).ForEach(func(k, v []byte) error {
			pod, err := types.NewPIFromRaw(k, v)
			if err != nil {
				return err
			}
			return f(pod)
		})
	})
}

func (b *BoltStorage) Close() error {
	return b.db.Close()
}
