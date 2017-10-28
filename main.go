package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	curDir := filepath.Dir(os.Args[0])

	var flagVerbose bool
	var flagRunScrapers bool
	var flagRunSpider bool
	var flagRugPath string

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "config",
			Value:       "rug.json",
			Usage:       "The path to the rug.json",
			Destination: &flagRugPath,
		},
		cli.BoolFlag{
			Name:        "verbose",
			Usage:       "Enable verbose output",
			Destination: &flagVerbose,
		},
	}

	app.Name = "rugburn"
	app.Usage = "A configuration-based web scraper"

	app.Commands = []cli.Command{
		{
			Name:  "init",
			Usage: "Initialize a new rugburn project in this directory",
			Action: func(c *cli.Context) error {
				if _, err := os.Stat(fmt.Sprintf("%s/%s", curDir, flagRugPath)); !os.IsNotExist(err) {
					fmt.Println("A rugburn project already exists in this directory")
					os.Exit(1)
				}
				err := os.Mkdir("transforms", 0700)
				if err != nil {
					return err
				}

				// Create example rugfile
				f, err := os.Create("./rug.json")
				if err != nil {
					return err
				}
				d, err := Asset("bindata/rug.json")
				if err != nil {
					return err
				}
				_, err = f.Write(d)
				if err != nil {
					return err
				}
				err = f.Close()
				if err != nil {
					return err
				}

				// Create example transform
				f, err = os.Create("transforms/UppercaseTitle.lua")
				if err != nil {
					return err
				}
				d, err = Asset("bindata/UppercaseTitle.lua")
				if err != nil {
					return err
				}
				_, err = f.Write(d)
				if err != nil {
					return err
				}
				err = f.Close()
				if err != nil {
					return err
				}

				fmt.Println("...Done!")
				fmt.Println("Run \"rugburn run\" to get scraping!")
				return nil
			},
		},
		{
			Name:  "run",
			Usage: "Run the rugburn project in this directory",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "spider",
					Usage:       "Runs the spider only",
					Destination: &flagRunSpider,
				},
				cli.BoolFlag{
					Name:        "scrapers",
					Usage:       "Run the scrapers only",
					Destination: &flagRunScrapers,
				},
			},
			Action: func(c *cli.Context) error {
				if flagVerbose {
					log.SetLevel(log.DebugLevel)
					log.Debug("Verbose logging enabled.")
				}

				if !flagRunSpider && !flagRunScrapers {
					flagRunSpider = true
					flagRunScrapers = true
				}

				fileData, err := ioutil.ReadFile(flagRugPath)
				if err != nil {
					log.Errorf("Can't find rugburn config in %s", flagRugPath)
					return err
				}
				rugFile := &RugFile{}
				err = json.Unmarshal(fileData, rugFile)
				if err != nil {
					log.Errorf("Invalid JSON in %s", flagRugPath)
					return err
				}

				store, err := getDB(rugFile.Options.StoreOptions)
				if err != nil {
					return err
				}

				if flagRunSpider {
					log.Info("Starting spider..")
					err = RunSpider(store, rugFile)
					if err != nil {
						return err
					}
				}

				if flagRunScrapers {
					log.Info("Starting scrapers..")
					err = RunScraper(store, rugFile)
					if err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			Name:  "clean",
			Usage: "Cleans the cached data from the current rugburn project",
			Action: func(c *cli.Context) error {
				if _, err := os.Stat(fmt.Sprintf("%s/%s", curDir, flagRugPath)); os.IsNotExist(err) {
					fmt.Println("Can't find a rugburn project in this directory")
					os.Exit(1)
				}
				err := os.RemoveAll("./db")
				if err != nil {
					return err
				}
				fmt.Println("Cleared cache")
				return nil
			},
		},
	}

	app.Action = func(c *cli.Context) error {
		return nil
	}

	app.RunAndExitOnError()
}

type ConfigOptions struct {
	SpiderOptions *ConfigSpiderOptions `json:"spiders"`
	StoreOptions  *ConfigStoreOptions  `json:"store"`
}

type ConfigSpiderOptions struct {
	Concurrency int `json:"concurrency"`
	MaxResults  int `json:"max"`
}

type ConfigStoreOptions struct {
	Strategy string `json:"strategy"`
}

type ConfigScraper struct {
	Name       string                 `json:"name"`
	Output     string                 `json:"output"`
	Test       string                 `json:"test"`
	Context    string                 `json:"context"`
	Fields     map[string]interface{} `json:"fields"`
	Transforms []string               `json:"transforms"`
}

type ConfigSpider struct {
	URLs       []string `json:"urls"`
	TestXPATH  string   `json:"test"`
	LinksXPATH []string `json:"links"`
}

type RugFile struct {
	Name     string           `json:"name"`
	Options  *ConfigOptions   `json:"options"`
	Spider   *ConfigSpider    `json:"spider"`
	Scrapers []*ConfigScraper `json:"scrapers"`
}
