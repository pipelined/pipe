package mutable_test

import (
	"fmt"

	"pipelined.dev/pipe/mutable"
)

type mutableType struct {
	mutable.Context
	parameter int
}

func (v *mutableType) setParameter(value int) mutable.Mutation {
	return v.Context.Mutate(func() {
		v.parameter = value
	})
}

func Example_mutation() {
	// create new mutable component
	component := &mutableType{
		Context: mutable.Mutable(),
	}
	fmt.Println(component.parameter)

	// create new mutation
	mutation := component.setParameter(10)
	fmt.Println(component.parameter)

	// apply mutation
	mutation.Apply()
	fmt.Println(component.parameter)

	// Output:
	// 0
	// 0
	// 10
}
