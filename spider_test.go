package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

func TestRunSpiderSunnyCase(t *testing.T) {
	testDB, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		panic(err)
	}

	var urls []string
	var url string
	var i = 0

	handler := func(w http.ResponseWriter, r *http.Request) {
		queryIndex := strconv.Itoa(i)
		w.WriteHeader(200)
		turl := url + "?=" + queryIndex
		urls = append(urls, turl)
		w.Write([]byte(fmt.Sprintf(`<div><a href="%s">test</a></div>`, turl)))
		i++
	}

	ts := httptest.NewServer(http.HandlerFunc(handler))
	url = ts.URL
	urls = append(urls, url)
	defer ts.Close()

	max := 3
	rugFile := &RugFile{
		Name: "Test",
		Options: &ConfigOptions{
			SpiderOptions: &ConfigSpiderOptions{
				Concurrency: 1,
				MaxResults:  max,
			},
			StoreOptions: &ConfigStoreOptions{
				Strategy: "memory",
			},
		},
		Spider: &ConfigSpider{
			URLs:       []string{url},
			LinksXPATH: []string{"//a/@href"},
		},
	}

	err = RunSpider(testDB, rugFile)
	assert.NoError(t, err)

	for i, v := range urls {
		if i >= max {
			break
		}
		v, err := testDB.Get([]byte("res-"+v), nil)
		assert.NoError(t, err)
		assert.NotEqual(t, "", v)
	}
}

func TestRunSpiderServerError(t *testing.T) {
	testDB, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		panic(err)
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(""))
	}

	ts := httptest.NewServer(http.HandlerFunc(handler))
	url := ts.URL
	defer ts.Close()

	rugFile := &RugFile{
		Name: "Test",
		Options: &ConfigOptions{
			SpiderOptions: &ConfigSpiderOptions{
				Concurrency: 1,
				MaxResults:  1,
			},
			StoreOptions: &ConfigStoreOptions{
				Strategy: "memory",
			},
		},
		Spider: &ConfigSpider{
			URLs:       []string{url},
			LinksXPATH: []string{"//a/@href"},
		},
	}

	err = RunSpider(testDB, rugFile)
	assert.NoError(t, err)

	r, err := getStoredResult(testDB, url)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusText(500), r.Error)
}

func TestRunSpiderMalformed(t *testing.T) {
	testDB, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		panic(err)
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("<span>hello</span></span>"))
	}

	ts := httptest.NewServer(http.HandlerFunc(handler))
	url := ts.URL
	defer ts.Close()

	rugFile := &RugFile{
		Name: "Test",
		Options: &ConfigOptions{
			SpiderOptions: &ConfigSpiderOptions{
				Concurrency: 1,
				MaxResults:  1,
			},
			StoreOptions: &ConfigStoreOptions{
				Strategy: "memory",
			},
		},
		Spider: &ConfigSpider{
			URLs:       []string{url},
			LinksXPATH: []string{"//span/text()"},
		},
	}

	err = RunSpider(testDB, rugFile)
	assert.NoError(t, err)

	r, err := getStoredResult(testDB, url)
	assert.NoError(t, err)
	assert.Equal(t, r.Response, "<html><head></head><body><span>hello</span></body></html>")
}
