package client_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListLabels(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	// Initially empty
	labels, err := c.ListLabels()
	require.NoError(t, err)
	assert.Empty(t, labels)
}

func TestCreateLabel(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	label, err := c.CreateLabel("bug", "#ff0000", "Something is broken")
	require.NoError(t, err)
	assert.Equal(t, "bug", label.Name)
	assert.Equal(t, "#ff0000", label.Color)
	assert.Equal(t, "Something is broken", label.Description)
}

func TestCreateAndListLabels(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.CreateLabel("bug", "#ff0000", "Something is broken")
	require.NoError(t, err)

	_, err = c.CreateLabel("feature", "#00ff00", "New feature")
	require.NoError(t, err)

	labels, err := c.ListLabels()
	require.NoError(t, err)
	assert.Len(t, labels, 2)
}

func TestUpdateLabel(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.CreateLabel("bug", "#ff0000", "Something is broken")
	require.NoError(t, err)

	updated, err := c.UpdateLabel("bug", map[string]string{
		"color":       "#cc0000",
		"description": "A bug report",
	})
	require.NoError(t, err)
	assert.Equal(t, "bug", updated.Name)
	assert.Equal(t, "#cc0000", updated.Color)
	assert.Equal(t, "A bug report", updated.Description)
}

func TestUpdateLabelClearDescription(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.CreateLabel("bug", "#ff0000", "Something is broken")
	require.NoError(t, err)

	// Passing empty string for description should clear it
	updated, err := c.UpdateLabel("bug", map[string]string{
		"description": "",
	})
	require.NoError(t, err)
	assert.Equal(t, "bug", updated.Name)
	assert.Equal(t, "#ff0000", updated.Color) // color unchanged
	assert.Equal(t, "", updated.Description)  // description cleared
}

func TestUpdateLabelPartialUpdate(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.CreateLabel("bug", "#ff0000", "Something is broken")
	require.NoError(t, err)

	// Only update color, leave description unchanged
	updated, err := c.UpdateLabel("bug", map[string]string{
		"color": "#cc0000",
	})
	require.NoError(t, err)
	assert.Equal(t, "#cc0000", updated.Color)
	assert.Equal(t, "Something is broken", updated.Description) // unchanged
}

func TestDeleteLabel(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.CreateLabel("bug", "#ff0000", "Something is broken")
	require.NoError(t, err)

	err = c.DeleteLabel("bug")
	require.NoError(t, err)

	labels, err := c.ListLabels()
	require.NoError(t, err)
	assert.Empty(t, labels)
}

func TestAddLabelToIssue(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Test issue")

	_, err := c.CreateLabel("bug", "#ff0000", "")
	require.NoError(t, err)

	err = c.AddLabelToIssue(proj.ID, issue.ID, "bug")
	require.NoError(t, err)
}

func TestGetIssueDetailsIncludesLabels(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Test issue")

	_, err := c.CreateLabel("bug", "#ff0000", "")
	require.NoError(t, err)

	err = c.AddLabelToIssue(proj.ID, issue.ID, "bug")
	require.NoError(t, err)

	// GetIssueDetails should include the label
	details, err := c.GetIssueDetails(proj.ID, issue.ID)
	require.NoError(t, err)
	assert.Contains(t, details.Labels, "bug", "issue details should include applied labels")
}

func TestRemoveLabelFromIssue(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Test issue")

	_, err := c.CreateLabel("bug", "#ff0000", "")
	require.NoError(t, err)

	err = c.AddLabelToIssue(proj.ID, issue.ID, "bug")
	require.NoError(t, err)

	err = c.RemoveLabelFromIssue(proj.ID, issue.ID, "bug")
	require.NoError(t, err)
}
