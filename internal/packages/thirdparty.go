package packages

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

const (
	drupalOrgProjectURL = "https://www.drupal.org/jsonapi/node/project_module"
	wordpressOrgAPIURL  = "https://api.wordpress.org/plugins/info/1.2/"
)

// fetchDrupalOrgSummary fills a package from drupal.org's own JSON:API,
// keyed by the module's machine name (the part of "drupal/<name>" after the
// slash). The sparse fieldset keeps the response to a few kilobytes instead
// of the full node payload.
func fetchDrupalOrgSummary(ctx context.Context, client *http.Client, pkg *Package) {
	machineName := strings.TrimPrefix(pkg.Name, "drupal/")

	q := url.Values{}
	q.Set("filter[field_project_machine_name]", machineName)
	q.Set("fields[node--project_module]", "body,field_active_installs_total")
	endpoint := drupalOrgProjectURL + "?" + q.Encode()

	var resp struct {
		Data []struct {
			Attributes struct {
				Body struct {
					Summary string `json:"summary"`
				} `json:"body"`
				ActiveInstallsTotal int64 `json:"field_active_installs_total"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := getJSON(ctx, client, endpoint, "application/vnd.api+json", &resp); err != nil || len(resp.Data) == 0 {
		return
	}

	attrs := resp.Data[0].Attributes
	pkg.Description = attrs.Body.Summary
	pkg.Weekly = attrs.ActiveInstallsTotal
}

// fetchWordPressOrgSummary fills a package from the WordPress.org plugin
// directory API, keyed by the plugin slug (the part of
// "wpackagist-plugin/<slug>" after the slash — the same slug wpackagist
// mirrors into the composer package). Every field but the two needed is
// explicitly turned off: by default this endpoint also returns the full
// description, changelog, screenshots and contributor list.
func fetchWordPressOrgSummary(ctx context.Context, client *http.Client, pkg *Package) {
	slug := strings.TrimPrefix(pkg.Name, "wpackagist-plugin/")

	q := url.Values{}
	q.Set("action", "plugin_information")
	q.Set("request[slug]", slug)
	for _, field := range []string{
		"description", "sections", "tested", "requires", "rating", "ratings",
		"downloadlink", "last_updated", "homepage", "tags", "compatibility",
		"versions", "donate_link", "reviews", "banners", "icons",
		"contributors", "screenshots", "author", "author_profile", "support_url",
	} {
		q.Set("request[fields]["+field+"]", "0")
	}
	q.Set("request[fields][short_description]", "1")
	q.Set("request[fields][active_installs]", "1")
	endpoint := wordpressOrgAPIURL + "?" + q.Encode()

	var resp struct {
		Version          string `json:"version"`
		ShortDescription string `json:"short_description"`
		ActiveInstalls   int64  `json:"active_installs"`
	}
	if err := getJSON(ctx, client, endpoint, "application/json", &resp); err != nil {
		return
	}

	pkg.Description = resp.ShortDescription
	pkg.LatestVersion = resp.Version
	pkg.Weekly = resp.ActiveInstalls
}
