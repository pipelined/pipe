module github.com/pipelined/pipe

require (
	github.com/pipelined/mock v0.2.1
	github.com/pipelined/signal v0.2.1
	github.com/stretchr/testify v1.4.0
	go.uber.org/goleak v0.10.0
)

replace github.com/pipelined/mock => ../mock

go 1.13
