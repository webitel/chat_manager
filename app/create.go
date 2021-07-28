package app

// CreateOptions Context
type CreateOptions struct {
	// Operation Context
	Context
	// Optional. Partial fields set to be fetched into given object to create.
	// Otherwise, it means to update nothing in given object created.
	// Mostly that is references, to be able to return valid display names
	Fields []string
}

// CreateOperation general pattern design
type CreateOperation func(ctx *CreateOptions, add interface{}) error