package auth

// AccessMode
type AccessMode = uint8

// Access Mode(s) Flags
const (

	DELETE AccessMode = 1 << iota // 0000 0001
	WRITE                         // 0000 0010
	READ                          // 0000 0100
	ADD                           // 0000 1000

	NONE AccessMode = 0           // 0000 0000
	FULL = ADD|READ|WRITE|DELETE  // 0000 1111
)