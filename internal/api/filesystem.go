package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/labstack/echo/v4"
)

type browseEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"is_dir"`
	IsGitRepo bool   `json:"is_git_repo"`
}

func (s *Server) browseFilesystem(c echo.Context) error {
	dir := c.QueryParam("dir")
	if dir == "" {
		return errorJSON(c, http.StatusBadRequest, "dir query parameter is required")
	}
	if !filepath.IsAbs(dir) {
		return errorJSON(c, http.StatusBadRequest, "dir must be an absolute path")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, "cannot read directory: "+err.Error())
	}

	var result []browseEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden directories (starting with .)
		if len(name) > 0 && name[0] == '.' {
			continue
		}
		fullPath := filepath.Join(dir, name)
		isGitRepo := isGitRepository(fullPath)
		result = append(result, browseEntry{
			Name:      name,
			Path:      fullPath,
			IsDir:     true,
			IsGitRepo: isGitRepo,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	if result == nil {
		result = []browseEntry{}
	}

	return c.JSON(http.StatusOK, result)
}

func isGitRepository(path string) bool {
	gitPath := filepath.Join(path, ".git")
	_, err := os.Stat(gitPath)
	return err == nil
}
