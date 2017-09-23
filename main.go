package main

import (
	"context"
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
}

type ConfigStoreOptions struct {
	Strategy string `json:"strategy"`
}

type ConfigScraper struct {
}

type ConfigSpider struct {
	URLs       []string `json:"urls"`
	TestXPATH  string   `json:"test"`
	LinksXPATH []string `json:"links"`
}

type RugFile struct {
	Name     string           `json:"name"`
	Options  *ConfigOptions   `json:"options"`
	Spiders  []*ConfigSpider  `json:"spiders"`
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

	ctx := context.Background()
	RunSpiders(ctx, rugFile)

	log.Info("Starting scraper..")
	return nil
}
