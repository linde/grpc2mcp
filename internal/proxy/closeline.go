package proxy

// Closer is a function that can be closed for example with http servers or
// network connections.

// CloseLine is a line of closers that can be closed sequentially.
type CloseLine struct {
	closers []func()
}

// Add adds a closer to the close line.
func (c *CloseLine) Add(closer func()) {
	c.closers = append(c.closers, closer)
}

func (c *CloseLine) AddE(closeWithError func() error) {

	errorIgnorer := func() {
		closeWithError()
	}
	c.closers = append(c.closers, errorIgnorer)
}

// Close closes all the closers and removes them
func (c *CloseLine) Close() {

	for _, f := range c.closers {
		if f != nil {
			f()
		}
		c.closers = c.closers[1:]
	}

}

// Generated with gemini -- this was my prompt:

// hi, i'd like to move the logic around accumulatedCancelFuncs in
// test_utils.go into its own go file close.go and have a new struct called a
// CloseLine which has a Add() that takes a Closer() which is just a func()
// when its close is called, it closes the line of added closers
// sequentially. got it?  can you please plan it out with tests in
// close_test.go and let me know what you have in mind

// then a follow-ip refinement (just one!)

// the closeline should take a Closer interface. that interface is just
// func() which is the same thing. i rather closer.go just reason with
// Closer interface instances
