// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/commandlinedev/starterm/cmd/wsh/cmd"
	"github.com/commandlinedev/starterm/pkg/starbase"
)

// set by main-server.go
var StarVersion = "0.0.0"
var BuildTime = "0"

func main() {
	starbase.StarVersion = StarVersion
	starbase.BuildTime = BuildTime
	cmd.Execute()
}
