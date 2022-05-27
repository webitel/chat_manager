package cmd

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

var (
	GitTag    string // semver
	GitCommit string // patch
	BuildDate string // time

	// name        = "chat"
	version     = "v3"
	description = "Micro Chat Service(s)"
)

// Build Version string
func Version() string {

	goVer := version

	if GitTag != "" {
		goVer = GitTag
	}

	if BuildDate != "" {
		goVer += "-" + BuildDate
	}

	if GitCommit != "" {
		goVer += "-" + GitCommit
	}

	return goVer
}

// ShowVersion prints buld version string to output and exit
func ShowVersion(ctx *cli.Context) error {
	fmt.Println(Version())
	os.Exit(0) // OK
	return nil
}

// // Register Version Command
// func init() {

// 	cmdVer := cli.Command{
// 		Name:   "version",
// 		Usage:  "Print the version and exit",
// 		Action: ShowVersion,
// 	}

// 	cmd.Register(&cmdVer)
// }
