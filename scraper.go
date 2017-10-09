package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/prettymuchbryce/goxpath"
	"github.com/prettymuchbryce/goxpath/tree"
	"github.com/prettymuchbryce/goxpath/tree/xmltree"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-json"
)

type ScrapeJob struct {
	config     *ConfigScraper
	transforms []string
	output     *os.File
}

func RunScraper(db *leveldb.DB, rugFile *RugFile) error {
	iter := db.NewIterator(util.BytesPrefix([]byte("res-")), nil)

	var jobs = []*ScrapeJob{}
	for _, sc := range rugFile.Scrapers {
		f, err := os.OpenFile(sc.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}

		var transforms = []string{}
		for _, t := range sc.Transforms {
			// Open transforms
			tByte, err := ioutil.ReadFile(t)
			if err != nil {
				return err
			}
			transforms = append(transforms, string(tByte))
		}

		defer f.Close()

		job := &ScrapeJob{
			config:     sc,
			output:     f,
			transforms: transforms,
		}

		jobs = append(jobs, job)
	}

	for iter.Next() {
		for _, job := range jobs {
			rv := iter.Value()
			var buffer = bytes.NewBuffer(rv)
			var r = &SpiderResult{}
			d := gob.NewDecoder(buffer)
			err := d.Decode(r)
			if err != nil {
				return err
			}

			var root tree.Node
			buffer = bytes.NewBuffer([]byte(r.Response))
			root, err = xmltree.ParseXML(buffer, parseSettings)
			if err != nil {
				return err
			}

			if job.config.Test != "" {
				xpTest, err := goxpath.Parse(job.config.Test)
				if err != nil {
					return err
				}

				xresults, err := xpTest.ExecNode(root)
				if err != nil {
					return err
				}

				if len(xresults) == 0 {
					return nil
				}
			}

			var results []map[string]interface{} = []map[string]interface{}{}

			if job.config.Context != "" {
				xpContext, err := goxpath.Parse(job.config.Context)
				if err != nil {
					return err
				}

				cresults, err := xpContext.ExecNode(root)
				if err != nil {
					return err
				}

				for _, r := range cresults {
					result, err := parseFields(job.config.Fields, r)
					if err != nil {
						return err
					}
					results = append(results, result)
				}
			} else {
				result, err := parseFields(job.config.Fields, root)
				if err != nil {
					return err
				}
				results = append(results, result)

			}

			for _, t := range job.transforms {
				for i, r := range results {
					result, err := ApplyTransform(r, t)
					if err != nil {
						return err
					}
					results[i] = result
				}
			}

			for _, r := range results {
				if len(r) == 0 {
					continue
				}
				j, err := json.Marshal(r)
				if err != nil {
					return err
				}
				if _, err = job.output.WriteString(string(j) + "\n"); err != nil {
					return err
				}
			}

		}
	}
	iter.Release()
	err := iter.Error()
	if err != nil {
		return err
	}
	log.Info("..Done!")
	return nil
}

func ApplyTransform(result map[string]interface{}, transform string) (map[string]interface{}, error) {
	s, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	l := lua.NewState()
	defer l.Close()
	luajson.Preload(l)
	l.SetGlobal("result", lua.LString(s))
	err = l.DoString(`
		json = require("json")
		oresult = json.decode(result)

		function do_transform()
			value = transform(oresult)
			return json.encode(value)
		end	
	`)
	if err != nil {
		return nil, err
	}
	l.DoString(transform)
	if err := l.CallByParam(lua.P{
		Fn:      l.GetGlobal("do_transform"),
		NRet:    1,
		Protect: true,
	}, l.GetGlobal("oresult")); err != nil {
		return nil, err
	}
	ret := l.Get(-1)
	l.Pop(1)

	vmResultString := ret.String()
	var vmResult = make(map[string]interface{})
	err = json.Unmarshal([]byte(vmResultString), &vmResult)
	if err != nil {
		return nil, err
	}

	return vmResult, nil
}

func runParser(test string, config map[string]interface{}, document string) (map[string]interface{}, error) {
	var root tree.Node
	var buffer = bytes.NewBuffer([]byte(document))
	root, err := xmltree.ParseXML(buffer, parseSettings)
	if err != nil {
		return nil, err
	}

	xpHtml, err := goxpath.Parse("//html")
	if err != nil {
		return nil, err
	}

	html, err := xpHtml.ExecNode(root)
	if err != nil {
		return nil, err
	}

	if len(html) == 0 {
		return nil, errors.New("Document not found") // TODO this error is bad
	}

	if test != "" {
		xpTest, err := goxpath.Parse(test)
		if err != nil {
			return nil, err
		}

		xresults, err := xpTest.ExecNode(html[0])
		if err != nil {
			return nil, err
		}

		if len(xresults) == 0 {
			return nil, nil
		}
	}

	return parseFields(config, html[0])

}

func parseFields(config map[string]interface{}, node tree.Node) (map[string]interface{}, error) {
	var result = make(map[string]interface{})
	for k, v := range config {
		switch f := v.(type) {
		default:
			return nil, fmt.Errorf("Unexpected type for value \"%s\"", k)
		case map[string]interface{}:
			if _, ok := f["fields"]; ok {
				fields, ok := f["fields"].(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("Unexpected type for value \"fields\". Should be an object.")
				}

				nextNodes := tree.NodeSet{node}
				if _, ok = f["context"]; ok {
					contextString, ok := f["context"].(string)
					if !ok {
						return nil, fmt.Errorf("Unexpected type for value \"context\". Should be string.")
					}
					xpContainer, err := goxpath.Parse(contextString)
					if err != nil {
						return nil, err
					}

					xresult, err := xpContainer.ExecNode(node)
					if err != nil {
						return nil, err
					}

					nextNodes = xresult
				}
				value := []map[string]interface{}{}
				for _, n := range nextNodes {
					parsed, err := parseFields(fields, n)
					if err != nil {
						return nil, err
					}
					value = append(value, parsed)
				}
				result[k] = value
			}
		case string:
			xpField, err := goxpath.Parse(f)
			if err != nil {
				return nil, err
			}

			xresult, err := xpField.ExecNode(node)
			if err != nil {
				return nil, err
			}

			var sresult string
			if len(xresult) == 1 {
				sresult = xresult[0].ResValue()
				result[k] = sresult
				continue
			}

			var ssresult []string = []string{}
			for _, v := range xresult {
				ssresult = append(ssresult, v.ResValue())
			}
			result[k] = ssresult
		}
	}
	return result, nil
}
