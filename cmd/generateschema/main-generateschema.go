// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/commandlinedev/starterm/pkg/sconfig"
	"github.com/commandlinedev/starterm/pkg/util/utilfn"
	"github.com/invopop/jsonschema"
)

const StarSchemaSettingsFileName = "schema/settings.json"
const StarSchemaConnectionsFileName = "schema/connections.json"
const StarSchemaAiPresetsFileName = "schema/aipresets.json"
const StarSchemaWidgetsFileName = "schema/widgets.json"

func generateSchema(template any, dir string) error {
	settingsSchema := jsonschema.Reflect(template)

	jsonSettingsSchema, err := json.MarshalIndent(settingsSchema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to parse local schema: %w", err)
	}
	written, err := utilfn.WriteFileIfDifferent(dir, jsonSettingsSchema)
	if !written {
		fmt.Fprintf(os.Stderr, "no changes to %s\n", dir)
	}
	if err != nil {
		return fmt.Errorf("failed to write local schema: %w", err)
	}
	return nil
}

func main() {
	err := generateSchema(&sconfig.SettingsType{}, StarSchemaSettingsFileName)
	if err != nil {
		log.Fatalf("settings schema error: %v", err)
	}

	connectionTemplate := make(map[string]sconfig.ConnKeywords)
	err = generateSchema(&connectionTemplate, StarSchemaConnectionsFileName)
	if err != nil {
		log.Fatalf("connections schema error: %v", err)
	}

	aiPresetsTemplate := make(map[string]sconfig.AiSettingsType)
	err = generateSchema(&aiPresetsTemplate, StarSchemaAiPresetsFileName)
	if err != nil {
		log.Fatalf("ai presets schema error: %v", err)
	}

	widgetsTemplate := make(map[string]sconfig.WidgetConfigType)
	err = generateSchema(&widgetsTemplate, StarSchemaWidgetsFileName)
	if err != nil {
		log.Fatalf("widgets schema error: %v", err)
	}
}
