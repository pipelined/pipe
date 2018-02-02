package main

import (
	"fmt"

	"github.com/dudk/phono/pool"
)

var vstPaths []string

func main() {
	fmt.Printf("%v\n", vstPaths)
	pool.New(vstPaths)

	fmt.Println(pool.Get())
}
