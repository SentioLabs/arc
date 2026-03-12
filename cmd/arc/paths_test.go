package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathsCommandExists(t *testing.T) {
	// pathsCmd should be registered as a subcommand of rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "paths" {
			found = true
			break
		}
	}
	assert.True(t, found, "paths command should be registered with rootCmd")
}

func TestPathsSubcommands(t *testing.T) {
	// Find the paths command
	var pathsCmdFound *struct{}
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "paths" {
			// Check subcommands: add, remove, list
			subNames := make(map[string]bool)
			for _, sub := range cmd.Commands() {
				subNames[sub.Name()] = true
			}
			assert.True(t, subNames["add"], "paths should have 'add' subcommand")
			assert.True(t, subNames["remove"], "paths should have 'remove' subcommand")
			assert.True(t, subNames["list"], "paths should have 'list' subcommand")
			pathsCmdFound = &struct{}{}
			break
		}
	}
	require.NotNil(t, pathsCmdFound, "paths command must exist")
}

func TestPathsAddFlags(t *testing.T) {
	// The add subcommand should have --label and --hostname flags
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "paths" {
			for _, sub := range cmd.Commands() {
				if sub.Name() == "add" {
					labelFlag := sub.Flags().Lookup("label")
					assert.NotNil(t, labelFlag, "add subcommand should have --label flag")
					hostnameFlag := sub.Flags().Lookup("hostname")
					assert.NotNil(t, hostnameFlag, "add subcommand should have --hostname flag")
					return
				}
			}
			t.Fatal("add subcommand not found")
		}
	}
	t.Fatal("paths command not found")
}

func TestPathsListAllFlag(t *testing.T) {
	// The list subcommand should have --all flag
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "paths" {
			for _, sub := range cmd.Commands() {
				if sub.Name() == "list" {
					allFlag := sub.Flags().Lookup("all")
					assert.NotNil(t, allFlag, "list subcommand should have --all flag")
					return
				}
			}
			t.Fatal("list subcommand not found")
		}
	}
	t.Fatal("paths command not found")
}
