package main

import (
	"fmt"

	"github.com/dudk/phono"
)

var vstPaths []string

var vst2 phono.Vst2

func main() {
	fmt.Printf("%v\n", vstPaths)
	vst2 = phono.NewVst2(vstPaths)
	//pool.New(vstPaths)

	fmt.Println(vst2)
}
