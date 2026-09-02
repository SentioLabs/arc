package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCreateCmdPriorityFlagDefaultsToTwo pins that omitting --priority on
// `arc create` still defaults to priority 2. The flag's default must stay in
// sync with defaultPriority so the request built from it does the same.
func TestCreateCmdPriorityFlagDefaultsToTwo(t *testing.T) {
	flag := createCmd.Flags().Lookup("priority")
	assert.NotNil(t, flag, "createCmd should have --priority flag")
	assert.Equal(t, "2", flag.DefValue, "priority flag should default to 2")
}
