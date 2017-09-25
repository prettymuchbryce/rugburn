package main

import (
	"bytes"
	"encoding/gob"

	"github.com/ChrisTrenkamp/goxpath"
	"github.com/ChrisTrenkamp/goxpath/tree"
	"github.com/syndtr/goleveldb/leveldb/util"
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
			xpTest, err := goxpath.Parse(sc.Test)
			if err != nil {
				return err
			}

			xresult, err := xpTest.ExecNode(root)
			if err != nil {
				return err
			}

			if len(xresult) == 0 {
				continue
			}

			for k, v := range sc.Fields {
				switch f := v.(type) {
				case string:

				case map[string]interface{}:

				}
				xpField, err := goxpath.Parse(v)
				if err != nil {
					return err
				}

				xresult, err = xpField.ExecNode(root)
				if err != nil {
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
	// Iterate over all results in the store
	// Check the test
	// Run the scrapers over them which pass the test
	// Run the transforms through the result
	// Store in the output file
	return nil
}
