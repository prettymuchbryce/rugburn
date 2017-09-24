package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"net/http"
	"net/url"

	"github.com/ChrisTrenkamp/goxpath"
	"github.com/ChrisTrenkamp/goxpath/tree"
	"github.com/ChrisTrenkamp/goxpath/tree/xmltree"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func init() {
	gob.Register(&SpiderRequest{})
	gob.Register(&SpiderResult{})
}

const strategyDisk = "disk"
const strategyMem = "memory"

func storeResult(s Store, r *SpiderResult) error {
	var key = "res-" + r.URL.String()
	var buffer = bytes.NewBuffer([]byte{})
	e := gob.NewEncoder(buffer)
	err := e.Encode(r)
	if err != nil {
		return err
	}
	return s.Put([]byte(key), buffer.Bytes())
}

func storeRequest(s Store, r *SpiderRequest) error {
	var key = "req-" + r.URL.String()
	var buffer = bytes.NewBuffer([]byte{})
	e := gob.NewEncoder(buffer)
	err := e.Encode(r)
	if err != nil {
		return err
	}
	return s.Put([]byte(key), buffer.Bytes())
}

func getNextRequest(s Store) (bool, *SpiderRequest, error) {
	iter := s.NewIterator(util.BytesPrefix([]byte("req-")))
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

func hasResult(s Store, url *url.URL) (bool, error) {
	return s.Contains([]byte("res-" + url.String()))
}

type spiderManager struct {
	inFlight int
	conc     int
	c        chan *SpiderResult
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
	var store Store
	switch config.Strategy {
	case strategyDisk:
		store = &DiskStore{}
	case strategyMem:
		store = &MemoryStore{}
	default:
		return nil, errors.New("Unknown store strategy or strategy not found")

	}

	store.Init(config)
	return store, nil
}

func RunSpiders(ctx context.Context, rugFile *RugFile) error {
	store, err := getStore(rugFile.Options.StoreOptions)
	if err != nil {
		return err
	}

	m := &spiderManager{
		inFlight: 0,
		conc:     rugFile.Options.SpiderOptions.Concurrency,
		c:        make(chan *SpiderResult, rugFile.Options.SpiderOptions.Concurrency),
	}

	log.Info("Hey")

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
			err = storeRequest(store, req)
			if err != nil {
				return err
			}
		}
	}

	var done bool
	done, err = makeRequests(store, m)
	if err != nil {
		return err
	}
	if done {
		log.Info("..Done!")
		return nil
	}

	log.Info("Prob waiting for stuff")

	for {
		select {
		case r := <-m.c:
			log.Info(r)
			err = storeResult(store, r)
			if err != nil {
				return err
			}
			err = store.Delete([]byte("req-" + r.URL.String()))
			if err != nil {
				return err
			}
			for _, u := range r.Children {
				req := &SpiderRequest{
					URL:    u,
					Config: r.Config,
				}
				err := storeRequest(store, req)
				if err != nil {
					return err
				}
			}
			m.inFlight--

			var done bool
			done, err = makeRequests(store, m)
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

func makeRequests(store Store, m *spiderManager) (done bool, err error) {
	for m.inFlight < m.conc {
		exists, r, err := getNextRequest(store)
		if err != nil {
			return false, err
		}
		if !exists {
			break
		}
		visited, err := hasResult(store, r.URL)
		if err != nil {
			return false, err
		}
		if visited {
			continue
		}
		go makeRequest(r, m.c)
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
