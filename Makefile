.PHONY: all build run gotool clean

BINARY="ceddit"

all: gotool build

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ${BINARY}

run:
	@go run ./main.go 

gotool:
	go fmt ./...
	go vet ./...

clean:
	@if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi
