package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/prettymuchbryce/goxpath"
	"github.com/prettymuchbryce/goxpath/tree"
	"github.com/prettymuchbryce/goxpath/tree/xmltree"
	"github.com/syndtr/goleveldb/leveldb/util"
	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-json"
)

func RunScraper(store Store, rugFile *RugFile) error {
	iter := store.NewIterator(util.BytesPrefix([]byte("res-")))
	for iter.Next() {
		for _, sc := range rugFile.Scrapers {
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

			if sc.Test != "" {
				xpTest, err := goxpath.Parse(sc.Test)
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

			parseFields(sc.Fields, root)
		}
	}
	iter.Release()
	err := iter.Error()
	if err != nil {
		return err
	}
	// Iterate over all results in the store
	// Check the test
	// Run the scrapers over them which pass the test
	// Run the transforms through the result
	// Store in the output file
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
			if len(xresult) >= 1 {
				sresult = xresult[0].ResValue()
			}
			// XXX validation on string or strings
			result[k] = sresult
			continue
		}
	}
	return result, nil
}
