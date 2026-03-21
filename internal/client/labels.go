// Label client methods for global label CRUD and issue-label associations.
// Labels are global resources (not project-scoped); issue-label endpoints
// require a project ID for workspace validation.
package client

import (
	"encoding/json"
	"fmt"

	"github.com/sentiolabs/arc/internal/types"
)

// ListLabels returns all global labels.
func (c *Client) ListLabels() ([]*types.Label, error) {
	resp, err := c.get("/api/v1/labels")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var labels []*types.Label
	if err := json.NewDecoder(resp.Body).Decode(&labels); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return labels, nil
}

// CreateLabel creates a new global label.
func (c *Client) CreateLabel(name, color, description string) (*types.Label, error) {
	body := map[string]string{
		"name":        name,
		"color":       color,
		"description": description,
	}

	resp, err := c.post("/api/v1/labels", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var label types.Label
	if err := json.NewDecoder(resp.Body).Decode(&label); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &label, nil
}

// UpdateLabel updates a label's color or description.
// The fields map should contain only the fields that were explicitly set by the caller.
func (c *Client) UpdateLabel(name string, fields map[string]string) (*types.Label, error) {
	resp, err := c.put("/api/v1/labels/"+name, fields)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var label types.Label
	if err := json.NewDecoder(resp.Body).Decode(&label); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &label, nil
}

// DeleteLabel deletes a global label.
func (c *Client) DeleteLabel(name string) error {
	resp, err := c.delete("/api/v1/labels/" + name)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// AddLabelToIssue associates a label with an issue.
func (c *Client) AddLabelToIssue(projectID, issueID, label string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s/labels", projectID, issueID)

	body := map[string]string{
		"label": label,
	}

	resp, err := c.post(path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// RemoveLabelFromIssue removes a label association from an issue.
func (c *Client) RemoveLabelFromIssue(projectID, issueID, label string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s/labels/%s", projectID, issueID, label)

	resp, err := c.delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
