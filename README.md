# rugburn
[![Go Report Card](https://goreportcard.com/badge/github.com/prettymuchbryce/rugburn)](https://goreportcard.com/report/github.com/prettymuchbryce/rugburn)
[![Build Status](https://travis-ci.org/prettymuchbryce/rugburn.svg?branch=master)](https://travis-ci.org/prettymuchbryce/rugburn)
[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/prettymuchbryce/rugburn/master/LICENSE)

> A performant configuration-based caching web scraper

## What is rugburn?

Rugburn is a web scraping framework. Unlike other web scraping frameworks which require writing
code, rugburn attempts to be a completely configuration-based web scraper.

`rugburn` is installed as a CLI tool and allows you to create and modify a "scraping" environment
using only configuration. To see an example of a scraper for the website `Hacker News`, simply run
`rugburn init` in a new directory. To see all available options run `rugburn help`.

Rugburn configuration files specify which pages to download (spider) and which elements to extract
via XPath (scrapers). For cases where additional custom behavior is required, rugburn supports
transformations. Transformations are scripts written in LUA which allow "transformation" of your 
data into a more desirable format. You can re-use transformations between scrapers.

Rugburn supports caching of requests and responses into a local on-disk database. This is
recommended in order to avoid IP bans, improve performance, and in order to preserve the backwards
compatability of selectors.

Caching also means you can check your configuration alongside your database into version control.
This means your configuration and transforms will remain deterministic and the history of them
will be retained.

Caching is optional in the case where it is not desired, or infeasible due to a larger
dataset.

## Status

Rugburn is still experimental and likely to contain breaking changes going forward.

## Installation

Mac OS X release binaries are available on the releases page. Other platforms coming soon.

Windows and Linux can still manually build and install:

```
go get github.com/prettymuchbryce/rugburn
cd $GOPATH/src/github.com/prettymuchbryce/rugburn
make deps
make install

# Now make sure it installed successfully
rugburn help
```

## CLI Commands

`rugburn init` - Initialize a new rugburn project in the current directory.

`rugburn run` - Run the rugburn project in this directory.

`rugburn clean` - Clean the rugburn cache of the project in this directory.

`rugburn help` - Print some help information.

## Configuration options

Configuration Example:
```json
{
	"name": "Hacker News Scraper",
	"options": {
		"store": {
			"strategy": "disk"
		},
		"spiders": {
			"concurrency": 3,
			"max": 5
		}
	},
	"spider": {
		"urls": [
			"https://news.ycombinator.com/news"
		],
		"links": [
			"//a[@class=\"morelink\"]/@href"
		]
	},
	"scrapers": [
		{
			"name": "Links",
			"output": "links.jsonl",
			"context": "//a[@class=\"storylink\"]",
			"fields": {
				"title": "/text()"
			},
			"transforms": [
			  "./transforms/UppercaseTitle.lua"
			]
		}
	]
}
```

## Transform Example

```lua
function transform (state)
	-- Uppercase the title field
	if state["title"] ~= nil then
		state["title"] = string.upper(state["title"])
	end
	return state
end
```
