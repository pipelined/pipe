package mutability_test

import (
	"fmt"

	"pipelined.dev/pipe/mutability"
)

type mutable struct {
	mutability.Mutability
	parameter int
}

func (m *mutable) setParameter(value int) mutability.Mutation {
	return m.Mutability.Mutate(func() error {
		m.parameter = value
		return nil
	})
}

func Example_mutation() {
	// create new mutable component
	component := &mutable{
		Mutability: mutability.Mutable(),
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
