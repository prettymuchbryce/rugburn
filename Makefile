BIN ?= bin/${NAME}

all: ${BIN}

${BIN}:
	go build -o $@

bindata.go:
	go-bindata bindata

run:
	go run *.go

clean:
	rm -f ${BIN}

test:
	go test ./... -v

