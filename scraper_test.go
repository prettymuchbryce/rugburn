package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/prettymuchbryce/goxpath/tree"
	"github.com/prettymuchbryce/goxpath/tree/xmltree"
	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

func TestScraper(t *testing.T) {
	testDB, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		panic(err)
	}

	var page = `
	<html>
		<body>
			<div class="container">
				<span class="title">title1</span>
			</div>
			<div class="container">
				<span class="title">title2</span>
			</div>
		</body>
	</html>
	`

	u, _ := url.Parse("foo.com")

	res := &SpiderResult{
		URL:      u,
		Response: page,
	}

	storeResult(testDB, res)

	rugFile := &RugFile{
		Name: "Test",
		Options: &ConfigOptions{
			StoreOptions: &ConfigStoreOptions{
				Strategy: "memory",
			},
		},
		Scrapers: []*ConfigScraper{
			&ConfigScraper{
				Name:    "Test",
				Output:  "test.jsonl",
				Context: "//div",
				Fields: map[string]interface{}{
					"title": "//span/text()",
				},
			},
		},
	}

	err = RunScraper(testDB, rugFile)
	assert.NoError(t, err)

	b, _ := ioutil.ReadFile("test.jsonl")
	assert.Equal(t, string(b), "{\"title\":\"title1\"}\n{\"title\":\"title2\"}\n")

	err = os.Remove("test.jsonl")
	if err != nil {
		panic(err)
	}

}

func TestLuaJSON(t *testing.T) {
	var value = make(map[string]interface{})
	value["foo"] = 10
	result, err := ApplyTransform(value, `
		function transform(state)
			state["foo"] = 20
			return state
		end
	`)
	assert.NoError(t, err)
	assert.Equal(t, result["foo"], float64(20))
}

func TestParseFields(t *testing.T) {
	var page = `
	<html>
		<body>
			<div class="container">
				<span class="title">title1</span>
			</div>
			<div class="container">
				<span class="title">title2</span>
			</div>
		</body>
	</html>
	`

	var configFields = `{
		"containers": {
			"context": "//div[@class=\"container\"]",
			"fields": {
				"title": "//span[@class=\"title\"]/text()"
			}
		}
	}`

	configFields = strings.Replace(configFields, "\n", "", -1)
	configFields = strings.Replace(configFields, "\t", "", -1)

	var m = make(map[string]interface{})
	err := json.Unmarshal([]byte(configFields), &m)
	assert.NoError(t, err)
	var root tree.Node
	var buffer = bytes.NewBuffer([]byte(page))
	root, _ = xmltree.ParseXML(buffer, parseSettings)

	result, err := parseFields(m, root)
	assert.NoError(t, err)

	containers, _ := result["containers"].([]map[string]interface{})
	assert.Equal(t, 2, len(containers))
	title1, _ := containers[0]["title"].(string)
	title2, _ := containers[1]["title"].(string)
	assert.Equal(t, "title1", title1)
	assert.Equal(t, "title2", title2)
}
