package main

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/comparer"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/memdb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type Store interface {
	Init(config *ConfigStoreOptions) error
	Get(key []byte) ([]byte, error)
	Put(key []byte, value []byte) error
	Delete(key []byte) error
	Contains(key []byte) (bool, error)

	// Note: The returned iterator is not safe for concurrent use, but it is safe to use
	// multiple iterators concurrently. (via goleveldb godoc)
	NewIterator(slice *util.Range) iterator.Iterator
}

type MemoryStore struct {
	*memdb.DB
}

type DiskStore struct {
	*leveldb.DB
}

func (s *MemoryStore) Contains(key []byte) (bool, error) {
	exists := s.DB.Contains(key)
	return exists, nil
}

func (s *MemoryStore) Init(config *ConfigStoreOptions) error {
	db := memdb.New(comparer.DefaultComparer, 0)
	s.DB = db
	return nil
}

func (s *DiskStore) Get(key []byte) ([]byte, error) {
	return s.DB.Get(key, nil)
}

func (s *DiskStore) Contains(key []byte) (bool, error) {
	return s.DB.Has(key, nil)
}

func (s *DiskStore) Put(key []byte, value []byte) error {
	return s.DB.Put(key, value, nil)
}

func (s *DiskStore) Delete(key []byte) error {
	return s.DB.Delete(key, nil)
}

func (s *DiskStore) NewIterator(slice *util.Range) iterator.Iterator {
	return s.DB.NewIterator(slice, nil)
}

func (s *DiskStore) Init(config *ConfigStoreOptions) error {
	// XXX detect path
	db, err := leveldb.OpenFile("./db", nil)
	if err != nil {
		return err
	}
	s.DB = db
	return nil
}
