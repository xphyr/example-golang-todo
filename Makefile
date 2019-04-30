all: build

build:
	packr clean
	packr
	env GO111MODULE=on go test
	env GO111MODULE=on go build 

