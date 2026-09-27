package lisan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// configName is the file every translation tree must carry at its root.
const configName = "config.json"

// envPrefix and envSuffix delimit an environment reference in a global value.
const (
	envPrefix = "${"
	envSuffix = "}"
)

// regionConfig declares one region: its display name and the locales it serves.
type regionConfig struct {
	Name    string   `json:"name"`
	Locales []string `json:"locales"`
}

// settingsConfig holds tree-wide settings.
type settingsConfig struct {
	BaseRegion string `json:"base_region"`
}

// configFile mirrors config.json.
type configFile struct {
	Globals  map[string]string       `json:"globals"`
	Regions  map[string]regionConfig `json:"regions"`
	Settings settingsConfig          `json:"settings"`
}

// loadConfig reads and validates config.json, resolving environment references
// in globals. Every defect found is reported, not just the first.
func loadConfig(fsys fs.FS) (configFile, []Problem) {
	data, err := fs.ReadFile(fsys, configName)
	if err != nil {
		return configFile{}, []Problem{{File: configName, Message: "is missing or unreadable: " + err.Error()}}
	}

	file := newSourceFile(configName, data)

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var parsed configFile
	if err := decoder.Decode(&parsed); err != nil {
		pos := file.position(offsetOf(err))

		return configFile{}, []Problem{{
			File:    configName,
			Line:    pos.line,
			Column:  pos.column,
			Message: "is not valid: " + err.Error(),
		}}
	}

	problems := validateConfig(parsed)

	globals, globalProblems := resolveGlobals(parsed.Globals)
	parsed.Globals = globals

	return parsed, append(problems, globalProblems...)
}

// validateConfig checks the structural requirements of config.json.
func validateConfig(parsed configFile) []Problem {
	var problems []Problem

	if len(parsed.Regions) == 0 {
		problems = append(problems, Problem{File: configName, Message: "declares no regions"})
	}

	problems = append(problems, validateBaseRegion(parsed)...)

	return append(problems, validateRegionConfigs(parsed.Regions)...)
}

// validateBaseRegion checks that settings.base_region names a declared region.
func validateBaseRegion(parsed configFile) []Problem {
	base := parsed.Settings.BaseRegion
	if base == "" {
		return []Problem{{File: configName, Message: "does not set settings.base_region"}}
	}

	if len(parsed.Regions) == 0 {
		return nil
	}

	if _, declared := parsed.Regions[base]; declared {
		return nil
	}

	return []Problem{{
		File:    configName,
		Message: fmt.Sprintf("sets settings.base_region to %q, which is not declared in regions", base),
	}}
}

// validateRegionConfigs checks each region declaration and rejects a locale
// claimed by two regions.
func validateRegionConfigs(regions map[string]regionConfig) []Problem {
	problems := make([]Problem, 0, len(regions))
	owner := make(map[string]string, len(regions))

	for _, code := range sortedKeys(regions) {
		region := regions[code]

		problems = append(problems, validateRegionFields(code, region)...)
		problems = append(problems, claimLocales(owner, code, region.Locales)...)
	}

	return problems
}

// validateRegionFields checks one region's required fields.
func validateRegionFields(code string, region regionConfig) []Problem {
	var problems []Problem

	if region.Name == "" {
		problems = append(problems, Problem{
			File:    configName,
			Message: fmt.Sprintf("region %q has no name", code),
		})
	}

	if len(region.Locales) == 0 {
		problems = append(problems, Problem{
			File:    configName,
			Message: fmt.Sprintf("region %q lists no locales", code),
		})
	}

	return problems
}

// claimLocales records which region serves each locale, rejecting any overlap.
func claimLocales(owner map[string]string, code string, locales []string) []Problem {
	var problems []Problem

	for _, locale := range locales {
		if previous, taken := owner[locale]; taken {
			problems = append(problems, Problem{
				File:    configName,
				Message: fmt.Sprintf("locale %q is claimed by both region %q and region %q", locale, previous, code),
			})

			continue
		}

		owner[locale] = code
	}

	return problems
}

// resolveGlobals substitutes ${VAR} references from the environment. An unset
// variable is an error rather than a silently empty string.
func resolveGlobals(globals map[string]string) (map[string]string, []Problem) {
	resolved := make(map[string]string, len(globals))

	var problems []Problem

	for _, key := range sortedKeys(globals) {
		value := globals[key]

		name, isRef := envReference(value)
		if !isRef {
			resolved[key] = value

			continue
		}

		actual, present := os.LookupEnv(name)
		if !present {
			problems = append(problems, Problem{
				File:    configName,
				Message: fmt.Sprintf("global %q references %s%s%s, which is not set", key, envPrefix, name, envSuffix),
			})

			continue
		}

		resolved[key] = actual
	}

	return resolved, problems
}

// envReference reports whether a global's value is a bare ${VAR} reference.
func envReference(value string) (string, bool) {
	if !strings.HasPrefix(value, envPrefix) || !strings.HasSuffix(value, envSuffix) {
		return "", false
	}

	name := value[len(envPrefix) : len(value)-len(envSuffix)]
	if !validPlaceholderName(name) {
		return "", false
	}

	return name, true
}
