package app

type DeleteOptions struct {

	Context
	// Unique IDentifiers
	ID []int64
	Permanent bool
}