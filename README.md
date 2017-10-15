# rugburn
[![GoDoc](https://godoc.org/github.com/prettymuchbryce/rugburn?status.svg)](https://godoc.org/github.com/prettymuchbryce/rugburn)
[![Go Report Card](https://goreportcard.com/badge/github.com/prettymuchbryce/rugburn)](https://goreportcard.com/report/github.com/prettymuchbryce/rugburn)
[![Build Status](https://travis-ci.org/prettymuchbryce/rugburn.svg?branch=master)](https://travis-ci.org/prettymuchbryce/rugburn)
[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/prettymuchbryce/rugburn/master/LICENSE)

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

Caching also means you can check your configuration alongside your database into version control.
This means your configuration and transforms will remain deterministic and the history of them
will be retained.

Caching is optional in the case where it is not desired, or not infeasible due to a larger
dataset.

## Installation

Install locally via `go install`

## Commands

`rugburn init`

`rugburn run`

`rugburn clean`

`rugburn help`

## Configuration options

TODO 
