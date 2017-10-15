package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"net/url"
	"strconv"

	"github.com/syndtr/goleveldb/leveldb"
	lerrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func init() {
	gob.Register(&SpiderRequest{})
	gob.Register(&SpiderResult{})
}

const strategyDisk = "disk"
const strategyMem = "memory"

type SpiderRequest struct {
	URL *url.URL
}

type SpiderResult struct {
	URL      *url.URL
	Error    string
	Response string
	Children []*url.URL
}

func getDB(config *ConfigStoreOptions) (*leveldb.DB, error) {
	switch config.Strategy {
	case strategyDisk:
		return leveldb.OpenFile("./db", nil)
	case strategyMem:
		return leveldb.Open(storage.NewMemStorage(), nil)
	default:
		return nil, errors.New("Unknown store strategy or strategy not found")

	}
}

func getStoredResultCount(db *leveldb.DB) (int, error) {
	var resCountInt = 0

	resCount, err := db.Get([]byte("count-res"), nil)
	if err != nil && err != lerrors.ErrNotFound {
		return 0, nil
	}

	if resCount != nil {
		resCountInt, err = strconv.Atoi(string(resCount))
		if err != nil {
			return 0, err
		}
	}

	return resCountInt, nil
}

func getStoredResult(db *leveldb.DB, url string) (*SpiderResult, error) {
	v, err := db.Get([]byte("res-"+url), nil)
	if err != nil {
		return nil, err
	}

	var buffer = bytes.NewBuffer(v)
	var r = &SpiderResult{}
	d := gob.NewDecoder(buffer)
	err = d.Decode(r)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func getResultIterator(db *leveldb.DB) iterator.Iterator {
	return db.NewIterator(util.BytesPrefix([]byte("res-")), nil)
}

func storeResult(db *leveldb.DB, r *SpiderResult) error {
	var key = "res-" + r.URL.String()
	var buffer = bytes.NewBuffer([]byte{})
	e := gob.NewEncoder(buffer)
	err := e.Encode(r)
	if err != nil {
		return err
	}

	resCountInt, err := getStoredResultCount(db)
	if err != nil {
		return err
	}

	transaction, err := db.OpenTransaction()
	if err != nil {
		return err
	}

	err = transaction.Put([]byte(key), buffer.Bytes(), nil)
	if err != nil {
		return err
	}

	resCountInt++

	resCountString := strconv.Itoa(resCountInt)
	err = transaction.Put([]byte("count-res"), []byte(resCountString), nil)
	if err != nil {
		return err
	}

	return transaction.Commit()
}

func storeRequest(db *leveldb.DB, r *SpiderRequest) error {
	var key = "req-" + r.URL.String()
	var buffer = bytes.NewBuffer([]byte{})
	e := gob.NewEncoder(buffer)
	err := e.Encode(r)
	if err != nil {
		return err
	}
	return db.Put([]byte(key), buffer.Bytes(), nil)
}

func hasResult(db *leveldb.DB, url *url.URL) (bool, error) {
	return db.Has([]byte("res-"+url.String()), nil)
}
