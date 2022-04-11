package cmd

import (
	"fmt"
	"os"

	"github.com/micro/cli/v2"
	"github.com/micro/go-micro/v2/config/cmd"
)

var (

	GitTag    string // semver(branch)
	GitBranch string // branch
	GitCommit string // patch
	BuildDate string // time

	// name        = "chat"
	version     = "0.0.1"
	description = "Webitel Micro Chat Service(s)"
)

// Build Version string
func Version() string {

	fullVersion := version

	if GitTag != "" {
		fullVersion += "@"+ GitTag
	}

	if GitCommit != "" {
		fullVersion += fmt.Sprintf("-%s", GitCommit)
	}

	if BuildDate != "" {
		fullVersion += fmt.Sprintf("-%s", BuildDate)
	}

	return fullVersion
}

// ShowVersion prints buld version string to output and exit
func ShowVersion(ctx *cli.Context) error {
	fmt.Println(Version())
	os.Exit(0) // OK
	return nil
}

// Register Version Command
func init() {
	
	cmdVer := cli.Command{
		Name:   "version",
		Action: ShowVersion,
	}
	
	
	cmd.App().Commands = append(cmd.App().Commands, &cmdVer)
}