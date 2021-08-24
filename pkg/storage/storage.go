package storage

import (
	"fmt"

	"github.com/nano-gpu/nano-gpu-agent/pkg/types"
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
	key := []byte(fmt.Sprintf("%s:%s", namespace, name))
	return b.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(RootBucket)).Delete(key)
	})
}

func (b *BoltStorage) Save(info *types.PodInfo) error {
	key := []byte(fmt.Sprintf("%s:%s", info.Namespace, info.Name))
	val := info.ToBytes()
	return b.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(RootBucket)).Put(key, val)
	})
}

func (b *BoltStorage) Load(namespace, name string) (*types.PodInfo, error) {
	info := &types.PodInfo{
		Namespace:        namespace,
		Name:             name,
		ContainerDevices: map[string]types.Device{},
	}
	key := []byte(fmt.Sprintf("%s:%s", namespace, name))
	return info, b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(RootBucket))
		val := bucket.Get(key)
		if val == nil {
			return fmt.Errorf("no such key: %s", string(key))
		}
		info.FromBytes(tx.Bucket([]byte(RootBucket)).Get(key))
		return nil
	})
}

func (b *BoltStorage) LoadOrCreate(namespace, name string) *types.PodInfo {
	info, err := b.Load(namespace, name)
	if err == nil {
		return info
	}
	return types.NewPodInfo(namespace, name)
}

func (b *BoltStorage) ForEach(f func(info *types.PodInfo) error) error {
	return b.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(RootBucket)).ForEach(func(k, v []byte) error {
			info := &types.PodInfo{}
			info.FromBytes(v)
			return f(info)
		})
	})
}

func (b *BoltStorage) Close() error {
	return b.db.Close()
}
