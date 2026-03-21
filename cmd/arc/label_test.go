package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabelCmdExists(t *testing.T) {
	// labelCmd should be registered as a subcommand of rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "label" {
			found = true
			break
		}
	}
	assert.True(t, found, "labelCmd should be registered on rootCmd")
}

func TestLabelCmdSubcommands(t *testing.T) {
	var names []string
	for _, cmd := range labelCmd.Commands() {
		names = append(names, cmd.Name())
	}

	assert.Contains(t, names, "list")
	assert.Contains(t, names, "create")
	assert.Contains(t, names, "update")
	assert.Contains(t, names, "delete")
}

func TestLabelCreateCmdFlags(t *testing.T) {
	// Find labelCreateCmd
	var createSub *struct{}
	for _, cmd := range labelCmd.Commands() {
		if cmd.Name() == "create" {
			colorFlag := cmd.Flags().Lookup("color")
			assert.NotNil(t, colorFlag, "--color flag should exist")

			descFlag := cmd.Flags().Lookup("description")
			assert.NotNil(t, descFlag, "--description flag should exist")

			createSub = &struct{}{}
			break
		}
	}
	require.NotNil(t, createSub, "create subcommand should exist")
}

func TestLabelUpdateCmdFlags(t *testing.T) {
	for _, cmd := range labelCmd.Commands() {
		if cmd.Name() == "update" {
			colorFlag := cmd.Flags().Lookup("color")
			assert.NotNil(t, colorFlag, "--color flag should exist")

			descFlag := cmd.Flags().Lookup("description")
			assert.NotNil(t, descFlag, "--description flag should exist")
			return
		}
	}
	t.Fatal("update subcommand should exist")
}

func TestCreateCmdHasLabelFlag(t *testing.T) {
	flag := createCmd.Flags().Lookup("label")
	assert.NotNil(t, flag, "--label flag should be registered on createCmd")
}

func TestUpdateCmdHasLabelAddFlag(t *testing.T) {
	flag := updateCmd.Flags().Lookup("label-add")
	assert.NotNil(t, flag, "--label-add flag should be registered on updateCmd")
}

func TestUpdateCmdHasLabelRemoveFlag(t *testing.T) {
	flag := updateCmd.Flags().Lookup("label-remove")
	assert.NotNil(t, flag, "--label-remove flag should be registered on updateCmd")
}

func TestPrimeOutputContainsLabelCommands(t *testing.T) {
	var buf bytes.Buffer
	err := outputCLIContext(&buf, "")
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "arc label list")
	assert.Contains(t, output, "arc label create")
	assert.Contains(t, output, "arc label update")
	assert.Contains(t, output, "arc label delete")
	assert.Contains(t, output, "--label=bug")
	assert.Contains(t, output, "--label-add=critical")
	assert.Contains(t, output, "--label-remove=stale")
}

func TestLabelUpdateRequiresFlag(t *testing.T) {
	// Find the update subcommand and verify its RunE checks for at least one flag
	for _, cmd := range labelCmd.Commands() {
		if cmd.Name() == "update" {
			// Calling RunE with no flags set should return an error
			err := cmd.RunE(cmd, []string{"test-label"})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "at least one of --color or --description is required")
			return
		}
	}
	t.Fatal("update subcommand should exist")
}
