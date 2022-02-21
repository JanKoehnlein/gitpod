// Copyright (c) 2021 Gitpod GmbH. All rights reserved.
// Licensed under the Gitpod Enterprise Source Code License,
// See License.enterprise.txt in the project root folder.

package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/gitpod-io/gitpod/agent-smith/pkg/config"

	"github.com/alecthomas/jsonschema"
	"github.com/gitpod-io/gitpod/common-go/log"
	"github.com/spf13/cobra"
)

// configSchemaCmd represents the configSchema command
var configSchemaCmd = &cobra.Command{
	Use:   "config-schema",
	Short: "Generates the JSON schema validating the configuration",
	Run: func(cmd *cobra.Command, args []string) {
		schema := jsonschema.Reflect(&config.ServiceConfig{})
		schema.Title = "agent-smith config schema - generated using agent-smith config-schema"
		out, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			log.WithError(err).Fatal()
			return
		}
		fmt.Print(string(out))
	},
}

func init() {
	rootCmd.AddCommand(configSchemaCmd)
}
