package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/ChrisTrenkamp/goxpath"
	"github.com/ChrisTrenkamp/goxpath/tree"
	"github.com/ChrisTrenkamp/goxpath/tree/xmltree"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

const strategyDisk = "disk"

type Iterator interface {
	Next() bool
	Error() error
}

type Store interface {
	Get(k string) (interface{}, error)
	Put(k string, v interface{}) error
	NewIterator(prefix string) Iterator
}

type MemoryStore struct {
	m map[string]interface{}
}

type LevelDBStore struct {
	db *leveldb.DB
}

func (*LevelDBStore) Put(k string, v interface{}) error {
	return nil
}

func (*LevelDBStore) Get(k string) (interface{}, error) {
	return nil, nil
}

func (*LevelDBStore) NewIterator(prefix string) Iterator {
	return nil
}

type State struct {
	Successful   bool
	Complete     bool
	TotalPages   int
	TotalSuccess int
	TotalError   int
}

type SpiderRequest struct {
	URL    *url.URL
	Config *ConfigSpider
}

type SpiderResult struct {
	URL      *url.URL
	Error    error
	Config   *ConfigSpider
	Response string
	Children []*url.URL
}

func getStore(config *ConfigStoreOptions) (Store, error) {
	switch config.Strategy {
	case strategyDisk:
		store := &LevelDBStore{}
		// XXX detect path
		db, err := leveldb.OpenFile("./db", nil)
		if err != nil {
			return nil, err
		}
		store.db = db
		return store, nil
	}

	return nil, errors.New("Unknown store strategy or strategy not found")
}

func RunSpiders(ctx context.Context, rugFile *RugFile) error {
	var conc = rugFile.Options.SpiderOptions.Concurrency
	var c = make(chan *SpiderResult, conc)
	store, err := getStore(rugFile.Options.StoreOptions)
	if err != nil {
		return err
	}
	var reqQueue []*SpiderRequest
	var inFlight int

	// XXX Check any unfinished in queue
	for _, configSpider := range rugFile.Spiders {
		for _, us := range configSpider.URLs {
			u, err := url.Parse(us)
			if err != nil {
				log.Errorf("Failed to parse URL %s", us)
				return err
			}
			req := &SpiderRequest{
				URL:    u,
				Config: configSpider,
			}
			reqQueue = append(reqQueue, req)
		}
	}

	for i := 0; i < len(reqQueue) && i < conc; i++ {
		inFlight++
		var r *SpiderRequest
		r, reqQueue = reqQueue[0], reqQueue[1:]
		go makeRequest(r, c)
	}

	for {
		select {
		case r := <-c:
			log.Println(r)
			store.Put(r.URL.String(), r)
			for _, u := range r.Children {
				req := &SpiderRequest{
					URL:    u,
					Config: r.Config,
				}
				reqQueue = append(reqQueue, req)
			}
			inFlight--
			for len(reqQueue) > 0 && inFlight < rugFile.Options.SpiderOptions.Concurrency {
				var r *SpiderRequest
				r = reqQueue[0]
				if len(reqQueue) > 1 {
					reqQueue = reqQueue[1:]
				} else {
					reqQueue = []*SpiderRequest{}
				}

				log.Info("GO AGAIN", r.URL)

				// Make sure we haven't visited this URL already
				value, err := store.Get(r.URL.String())
				if err != nil {
					return err
				}
				if value == nil {
					go makeRequest(r, c)
					inFlight++
				}
			}
			if inFlight == 0 && len(reqQueue) == 0 {
				log.Info("done")
				return nil
			}
		}
	}
}

func parseSettings(s *xmltree.ParseOptions) {
	s.Strict = false
}

func makeRequest(req *SpiderRequest, c chan *SpiderResult) {
	resp, err := http.Get(req.URL.String())
	var result = &SpiderResult{
		URL:      req.URL,
		Config:   req.Config,
		Children: []*url.URL{},
	}

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

	for _, l := range req.Config.LinksXPATH {
		xpExec, err := goxpath.Parse(l)
		if err != nil {
			log.Error(err)
			log.Errorf("%s is not a valid XPath expression", l)
		}
		xresult, err := xpExec.ExecNode(root)
		if err != nil {
			log.Error(err)
		}
		log.Infof("xresult %+v", xresult)
		for _, i := range xresult {
			if err != nil {
				log.Error(err)
			}
			log.Infof("string %s", i.ResValue())
			if xresult != nil {
				url, err := url.Parse(i.ResValue())
				if err != nil {
					log.Error(err)
				}
				if !url.IsAbs() {
					url = req.URL.ResolveReference(url)
					log.Infof("Is absolute? %s", url.String())
				}
				result.Children = append(result.Children, url)
			}
		}
	}

	c <- result
}
