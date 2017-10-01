BIN ?= bin/${NAME}

all: ${BIN}

${BIN}:
	go build -o $@

run:
	go run *.go

clean:
	rm -f ${BIN}

test:
	go test ./... -v

