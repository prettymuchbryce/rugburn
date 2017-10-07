package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/prettymuchbryce/goxpath"
	"github.com/prettymuchbryce/goxpath/tree"
	"github.com/prettymuchbryce/goxpath/tree/xmltree"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func init() {
	gob.Register(&SpiderRequest{})
	gob.Register(&SpiderResult{})
}

const strategyDisk = "disk"
const strategyMem = "memory"

func getStoredResultCount(db *leveldb.DB) (int, error) {
	var resCountInt = 0

	resCount, err := db.Get([]byte("count-res"), nil)
	if err != nil {
		return 0, err
	}

	if resCount != nil {
		resCountInt, err = strconv.Atoi(string(resCount))
		if err != nil {
			return 0, err
		}
	}

	return resCountInt, nil
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

	return db.Put([]byte(key), buffer.Bytes(), nil)
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

func getNextRequest(db *leveldb.DB) (bool, *SpiderRequest, error) {
	iter := db.NewIterator(util.BytesPrefix([]byte("req-")), nil)
	if !iter.Next() {
		return false, nil, nil
	}

	v := iter.Value()

	iter.Release()
	err := iter.Error()

	if err != nil {
		return false, nil, err
	}

	var buffer = bytes.NewBuffer(v)
	var r = &SpiderRequest{}
	d := gob.NewDecoder(buffer)
	err = d.Decode(r)
	if err != nil {
		return false, nil, err
	}

	return true, r, nil
}

func hasResult(db *leveldb.DB, url *url.URL) (bool, error) {
	return db.Has([]byte("res-"+url.String()), nil)
}

type spiderManager struct {
	config   *ConfigSpider
	inFlight int
	conc     int
	c        chan *SpiderResult
}

type SpiderRequest struct {
	URL *url.URL
}

type SpiderResult struct {
	URL      *url.URL
	Error    error
	Response string
	Children []*url.URL
}

func getDB(config *ConfigStoreOptions) (*leveldb.DB, error) {
	switch config.Strategy {
	case strategyDisk:
		return leveldb.OpenFile("./", nil)
	case strategyMem:
		return leveldb.Open(storage.NewMemStorage(), nil)
	default:
		return nil, errors.New("Unknown store strategy or strategy not found")

	}
}

func RunSpider(db *leveldb.DB, rugFile *RugFile) error {
	m := &spiderManager{
		inFlight: 0,
		config:   rugFile.Spider,
		conc:     rugFile.Options.SpiderOptions.Concurrency,
		c:        make(chan *SpiderResult, rugFile.Options.SpiderOptions.Concurrency),
	}

	for _, us := range m.config.URLs {
		u, err := url.Parse(us)
		if err != nil {
			log.Errorf("Failed to parse URL %s", us)
			return err
		}
		req := &SpiderRequest{
			URL: u,
		}
		err = storeRequest(db, req)
		if err != nil {
			return err
		}
	}

	var done bool
	done, err := makeRequests(db, m)
	if err != nil {
		return err
	}
	if done {
		log.Info("..Done!")
		return nil
	}

	for {
		select {
		case r := <-m.c:
			log.Info(r)
			err = storeResult(db, r)
			if err != nil {
				return err
			}
			err = db.Delete([]byte("req-"+r.URL.String()), nil)
			if err != nil {
				return err
			}
			for _, u := range r.Children {
				req := &SpiderRequest{
					URL: u,
				}
				err := storeRequest(db, req)
				if err != nil {
					return err
				}
			}
			m.inFlight--

			c, err := getStoredResultCount(db)
			if err != nil {
				return err
			}

			if c >= rugFile.Options.SpiderOptions.MaxResults {
				log.Info("...Done!")
				return nil
			}

			var done bool
			done, err = makeRequests(db, m)
			if err != nil {
				return err
			}
			if done {
				log.Info("..Done!")
				return nil
			}
		}
	}
}

func makeRequests(db *leveldb.DB, m *spiderManager) (done bool, err error) {
	iter := db.NewIterator(util.BytesPrefix([]byte("req-")), nil)
	defer iter.Release()
	for m.inFlight < m.conc {
		if !iter.Next() {
			break
		}

		v := iter.Value()
		err := iter.Error()
		if err != nil {
			return false, err
		}

		var buffer = bytes.NewBuffer(v)
		var r = &SpiderRequest{}
		d := gob.NewDecoder(buffer)
		err = d.Decode(r)
		if err != nil {
			return false, err
		}

		visited, err := hasResult(db, r.URL)
		if err != nil {
			return false, err
		}
		if visited {
			continue
		}
		go makeRequest(m, r, m.c)
		m.inFlight++
	}

	if m.inFlight == 0 {
		return true, nil
	}

	return false, nil
}

func parseSettings(s *xmltree.ParseOptions) {
	s.Strict = false
}

func makeRequest(m *spiderManager, req *SpiderRequest, c chan *SpiderResult) {
	resp, err := http.Get(req.URL.String())
	var result = &SpiderResult{
		URL:      req.URL,
		Children: []*url.URL{},
	}

	log.Infof("Making request to %s", req.URL.String())

	var root tree.Node
	if err == nil {
		root, err = xmltree.ParseXML(resp.Body, parseSettings)
	}

	if err != nil {
		result.Error = err
		c <- result
		return
	}

	resp.Body.Close()

	for _, l := range m.config.LinksXPATH {
		xpExec, err := goxpath.Parse(l)
		if err != nil {
			log.Error(err)
			log.Errorf("%s is not a valid XPath expression", l)
		}
		xresult, err := xpExec.ExecNode(root)
		if err != nil {
			log.Error(err)
		}
		for _, i := range xresult {
			if err != nil {
				log.Error(err)
			}
			if xresult != nil {
				url, err := url.Parse(i.ResValue())
				if err != nil {
					log.Error(err)
				}
				if !url.IsAbs() {
					url = req.URL.ResolveReference(url)
				}
				result.Children = append(result.Children, url)
			}
		}
	}

	c <- result
}
