/*
Package mutability provides types to make DSP components mutable.

Each DSP component is running in its own goroutine. The only way to avoid
data races and locking is to mutate components in that same goroutine. To
achieve that, two things must be done: the component should be mutable and
component should have defined mutations.

Mutable component

Mutability is a part of DSP component structures. Nil value of mutability
is immutable. To make it mutable, the one should do the following:

	Mutability: mutability.Mutable()

This will make component recognized as mutable during pipe start and enable
mutations execution. See examples for more details.
*/
package mutability
