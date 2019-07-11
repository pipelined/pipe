module github.com/pipelined/pipe

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pipelined/mock v0.0.0-20190618070412-67f8ec06cab6
	github.com/pipelined/signal v0.0.0-20190411172221-40f38ff7f90f
	github.com/rs/xid v1.2.1
	github.com/stretchr/testify v1.3.0
	go.uber.org/goleak v0.10.0
)

replace github.com/pipelined/mock => ../mock
