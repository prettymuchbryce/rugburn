# rugburn

> A performant configuration-based caching web scraper

## What is rugburn?

Rugburn is a web scraping framework. Unlike other web scraping frameworks which require writing
software, rugburn attempts to be a completely configuration-based web scraper.

`rugburn` is installed as a CLI tool and allows you to create and modify a "sraping" environment
using only configuration. To see an example of a scraper for the website `Hacker News`, simply run
`rugburn init` in a new directory. To see all available options run `rugburn help`.

For cases where additional custom behavior is required, rugburn supports transformations.
Transformations are scripts written in LUA to allow for you to "transform" your data into a more
desirable format.

Rugburn supports caching of requests and responses into a local on-disk database. This is
recommended in order to avoid IP bans, improve performance, and in order to preserve the backwards
compatability of selectors.

Caching also means you can check your scrapers and your database into github like any other
software project. It means your configuration and transforms will remain deterministic and the
history of them will be retained.

Caching is optional in the case where it is not desired, or not infeasible due to a larger
dataset.

## Installation

Install locally via `go install`

## Commands

TODO

## Configuration options

TODO 
