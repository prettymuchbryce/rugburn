bindata.go:
	go-bindata bindata

run:
	go run *.go

clean:
	rm -f -- ./bindata.go

test:
	go test ./... -v

install: clean bindata.go
	go install
