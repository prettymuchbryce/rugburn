package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	var rugPath string

	app := cli.NewApp()

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "config",
			Value:       "./rug.json",
			Usage:       "The path to the rug.json",
			Destination: &rugPath,
		},
	}

	app.Name = "rugburn"
	app.Usage = "rugburn"

	app.Action = func(c *cli.Context) error {
		err := run(rugPath)
		return err
	}

	app.Run(os.Args)
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
	Fields     map[string]interface{} `json:"fields"`
	Transforms []string               `json:"tranforms"`
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

func run(rugPath string) error {
	fileData, err := ioutil.ReadFile(rugPath)
	if err != nil {
		log.Errorf("Can't find rug.json in %s", rugPath)
		return err
	}
	rugFile := &RugFile{}
	err = json.Unmarshal(fileData, rugFile)
	if err != nil {
		log.Errorf("Invalid JSON in %s", rugPath)
		return err
	}

	store, err := getDB(rugFile.Options.StoreOptions)
	if err != nil {
		return err
	}

	log.Info("Starting spider..")
	err = RunSpider(store, rugFile)
	if err != nil {
		return err
	}

	log.Info("Starting scraper..")
	err = RunScraper(store, rugFile)
	if err != nil {
		return err
	}

	return nil
}
