bindata.go:
	go-bindata bindata

run:
	go run *.go

clean:
	rm -f -- ./bindata.go

test:
	go test ./... -v

deps:
	go get -ugo get -u github.com/kardianos/govendor
	go get -u github.com/jteeuwen/go-bindata/...
	govendor sync
	govendor install

install: clean bindata.go
	go install
