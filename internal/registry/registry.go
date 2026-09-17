package registry

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

//go:embed frameworks.json
var embeddedData []byte

type Framework struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Ecosystem        string   `json:"ecosystem"`
	Type             string   `json:"type,omitempty"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	NpmPackage       string   `json:"npm_package,omitempty"`
	PackagistPackage string   `json:"packagist_package,omitempty"`
	GithubRepo       string   `json:"github_repo,omitempty"`
	Interactive      bool     `json:"interactive,omitempty"`
	// Detect lists the dependencies that identify an existing project using
	// this framework; all must be present, "a|b" accepts either.
	Detect []string `json:"detect,omitempty"`
}

type registryPayload struct {
	Frameworks []Framework `json:"frameworks"`
}

func Load() ([]Framework, error) {
	var payload registryPayload
	if err := json.Unmarshal(embeddedData, &payload); err != nil {
		return nil, err
	}
	return payload.Frameworks, nil
}

// DetectProjectFramework returns the framework of the project in dir for the
// given ecosystem ("php" or "js"), based on its composer.json or package.json
// dependencies. Full frameworks win over the libraries they are built on
// (a Remix app also depends on react and vite), then the most specific match.
func DetectProjectFramework(dir, ecosystem string) (Framework, bool) {
	deps := projectDependencies(dir, ecosystem)
	if len(deps) == 0 {
		return Framework{}, false
	}
	frameworks, err := Load()
	if err != nil {
		return Framework{}, false
	}

	var best Framework
	found := false
	for _, fw := range frameworks {
		if fw.Ecosystem != ecosystem || !matchesAll(fw.Detect, deps) {
			continue
		}
		if !found || betterMatch(fw, best) {
			best, found = fw, true
		}
	}
	return best, found
}

func betterMatch(candidate, current Framework) bool {
	candidateLib, currentLib := candidate.Type == "library", current.Type == "library"
	if candidateLib != currentLib {
		return !candidateLib
	}
	return len(candidate.Detect) > len(current.Detect)
}

func matchesAll(detect []string, deps map[string]struct{}) bool {
	if len(detect) == 0 {
		return false
	}
	for _, rule := range detect {
		matched := false
		for _, name := range strings.Split(rule, "|") {
			if _, ok := deps[name]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func projectDependencies(dir, ecosystem string) map[string]struct{} {
	file, sections := "package.json", []string{"dependencies", "devDependencies"}
	if ecosystem == "php" {
		file, sections = "composer.json", []string{"require", "require-dev"}
	}
	data, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		return nil
	}
	var manifest map[string]json.RawMessage
	if json.Unmarshal(data, &manifest) != nil {
		return nil
	}
	deps := make(map[string]struct{})
	for _, section := range sections {
		var names map[string]json.RawMessage
		if json.Unmarshal(manifest[section], &names) != nil {
			continue
		}
		for name := range names {
			deps[name] = struct{}{}
		}
	}
	return deps
}
