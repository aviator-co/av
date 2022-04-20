package cleanup

// Cleanup provides an easy way to clean up resources after an operation fails.
type Cleanup struct {
	fns []func()
}

func (c *Cleanup) Add(fn func()) {
	c.fns = append(c.fns, fn)
}

func (c *Cleanup) Cleanup() {
	for i := len(c.fns) - 1; i >= 0; i-- {
		c.fns[i]()
	}
}

func (c *Cleanup) Cancel() {
	c.fns = nil
}
