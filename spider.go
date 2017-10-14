package main

import (
	"bytes"
	"encoding/gob"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/prettymuchbryce/goxpath"
	"github.com/prettymuchbryce/goxpath/tree"
	"github.com/prettymuchbryce/goxpath/tree/xmltree"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type spiderManager struct {
	config   *ConfigSpider
	inFlight int
	conc     int
	c        chan *SpiderResult
}

func RunSpider(db *leveldb.DB, rugFile *RugFile) error {
	m := &spiderManager{
		inFlight: 0,
		config:   rugFile.Spider,
		conc:     rugFile.Options.SpiderOptions.Concurrency,
		c:        make(chan *SpiderResult, rugFile.Options.SpiderOptions.Concurrency),
	}

	count, err := getStoredResultCount(db)
	if err != nil {
		return err
	}

	if count >= rugFile.Options.SpiderOptions.MaxResults {
		log.Info("..Done!")
		return nil
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
	done, err = makeRequests(db, m)
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
			log.Info("Found cached page.. skipping")
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

func makeRequest(m *spiderManager, req *SpiderRequest, c chan *SpiderResult) {
	resp, err := http.Get(req.URL.String())
	var result = &SpiderResult{
		URL:      req.URL,
		Children: []*url.URL{},
	}

	log.Debugf("Making request to %s", req.URL.String())

	if err != nil {
		result.Error = err.Error()
		log.Errorf("%s %s", req.URL, err)
		c <- result
		return
	}

	if resp.StatusCode >= 400 {
		result.Error = http.StatusText(resp.StatusCode)
		log.Errorf("%s %s", req.URL, result.Error)
		c <- result
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("%s %s", req.URL, err)
		return
	}

	resp.Body.Close()

	var root tree.Node
	if err == nil {
		buf := bytes.NewBuffer(body)
		root, err = xmltree.ParseXML(buf, parseSettings)
		if err != nil {
			log.Errorf("%s %s", req.URL, err)
			result.Error = err.Error()
			c <- result
			return
		}
	}

	result.Response = string(body)

	for _, l := range m.config.LinksXPATH {
		xpExec, err := goxpath.Parse(l)
		if err != nil {
			log.Errorf("%s %s %s", req.URL, err, l)
			log.Errorf("%s is not a valid XPath expression", l)
			continue
		}
		xresult, err := xpExec.ExecNode(root)
		if err != nil {
			log.Errorf("%s %s %s", req.URL, err, l)
			continue
		}
		for _, i := range xresult {
			if err != nil {
				log.Errorf("%s %s", req.URL, err)
			}
			if xresult != nil {
				url, err := url.Parse(i.ResValue())
				if err != nil {
					log.Errorf("%s %s", req.URL, err)
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
