package cmd

import "fmt"

var (

	GitTag    string // semver(branch)
	GitBranch string // branch
	GitCommit string // patch
	BuildDate string // time

	// name        = "chat"
	version     = "0.0.0"
	description = "Webitel Micro Chat Service(s)"
)

func Version() string {

	fullVersion := version

	if GitTag != "" {
		fullVersion += "@"+ GitTag
	}

	if BuildDate != "" {
		fullVersion += fmt.Sprintf("-%s", BuildDate)
	}

	if GitCommit != "" {
		fullVersion += fmt.Sprintf("-%s", GitCommit)
	}

	return fullVersion
}