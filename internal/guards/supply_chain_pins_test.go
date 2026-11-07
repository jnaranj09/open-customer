package guards

// supply-chain-pins (docs/designs/devsecops.md section 3; docs/threat-model.md T76 and T91): a tag
// can be moved upstream, so every GitHub Action is pinned by its full commit SHA (with the release
// in a `# vX.Y.Z` comment, which Dependabot keeps in step), and every image a build, a scan or the
// stack pulls is pinned by digest (`<tag>@sha256:<64 hex>`). The one exception is this repo's own
// app and backup images in deploy/compose.yaml: their tag is the release a deployer picks
// (APP_VERSION). make check-pins keeps the copies of each pin equal; this guard keeps each one
// a pin.

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var (
	// owner/repo[/path]@<40 hex> # vX.Y.Z (the comment is required).
	pinnedUses = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}[ \t]+# v[0-9]+\.[0-9]+\.[0-9]+$`)
	usesLine   = regexp.MustCompile(`^[ \t]*(?:-[ \t]+)?uses:[ \t]*(.*)$`)
	// An image with a tag and the digest of its index: name:tag@sha256:<64 hex>.
	pinnedImage = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*(?::[0-9]+)?(?:/[a-z0-9._-]+)*:[A-Za-z0-9._-]+@sha256:[0-9a-f]{64}$`)
	// image: <ref> in a workflow (a service or a job container).
	workflowImage = regexp.MustCompile(`^[ \t]*image:[ \t]*(\S+)`)
	fromLine      = regexp.MustCompile(`(?i)^[ \t]*FROM[ \t]+(?:--platform=\S+[ \t]+)?(\S+)(?:[ \t]+AS[ \t]+(\S+))?`)
	makeImageVar  = regexp.MustCompile(`^([A-Z0-9_]+_IMAGE)[ \t]*:?=[ \t]*(.*)$`)
	shellImageVar = regexp.MustCompile(`^[ \t]*(?:readonly[ \t]+|export[ \t]+)?([A-Z0-9_]+_IMAGE)=(\S*)`)
)

// ownImages are the compose images pinned by the release tag, not a digest (see the header).
var ownImages = []string{"ghcr.io/jnaranj09/open-customer", "ghcr.io/jnaranj09/open-customer-backup"}

// checkWorkflowPins returns the unpinned `uses:` and `image:` lines of a workflow.
func checkWorkflowPins(name, src string) []string {
	var problems []string
	for i, line := range strings.Split(src, "\n") {
		if m := usesLine.FindStringSubmatch(line); m != nil && !pinnedUses.MatchString(strings.TrimSpace(m[1])) {
			problems = append(problems, fmt.Sprintf("%s:%d: uses %q is not <action>@<40-hex commit SHA> # vX.Y.Z", name, i+1, strings.TrimSpace(m[1])))
		}
		if m := workflowImage.FindStringSubmatch(line); m != nil && !pinnedImage.MatchString(m[1]) {
			problems = append(problems, fmt.Sprintf("%s:%d: image %q has no digest", name, i+1, m[1]))
		}
	}
	return problems
}

// checkDockerfilePins returns the FROM lines of a Dockerfile whose image has no digest (a FROM of
// an earlier stage or of scratch is not an image pull).
func checkDockerfilePins(name, src string) []string {
	var problems []string
	stages := map[string]bool{"scratch": true}
	for i, line := range strings.Split(src, "\n") {
		m := fromLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if !stages[strings.ToLower(m[1])] && !pinnedImage.MatchString(m[1]) {
			problems = append(problems, fmt.Sprintf("%s:%d: FROM %q has no digest", name, i+1, m[1]))
		}
		if m[2] != "" {
			stages[strings.ToLower(m[2])] = true
		}
	}
	return problems
}

// checkVarPins returns the <NAME>_IMAGE assignments (Makefile or shell, by re) with no digest.
func checkVarPins(name, src string, re *regexp.Regexp) []string {
	var problems []string
	for i, line := range strings.Split(src, "\n") {
		if m := re.FindStringSubmatch(line); m != nil && !pinnedImage.MatchString(strings.TrimSpace(m[2])) {
			problems = append(problems, fmt.Sprintf("%s:%d: %s %q has no digest", name, i+1, m[1], strings.TrimSpace(m[2])))
		}
	}
	return problems
}

// checkComposePins returns the compose services whose image has no digest, other than ownImages.
func checkComposePins(name string, src []byte) []string {
	var doc struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return []string{fmt.Sprintf("%s: not valid YAML: %v", name, err)}
	}
	var problems []string
	for _, svc := range slices.Sorted(maps.Keys(doc.Services)) {
		image := doc.Services[svc].Image
		repo, _, _ := strings.Cut(image, ":${APP_VERSION")
		if slices.Contains(ownImages, repo) && repo != image {
			continue
		}
		if !pinnedImage.MatchString(image) {
			problems = append(problems, fmt.Sprintf("%s: service %s: image %q has no digest", name, svc, image))
		}
	}
	return problems
}

func TestSupplyChainPinsProbes(t *testing.T) {
	t.Parallel()
	sha := strings.Repeat("a", 40)
	digest := "@sha256:" + strings.Repeat("b", 64)
	goodWorkflow := "jobs:\n  a:\n    services:\n      pg:\n        image: postgres:18.6-alpine" + digest + "\n    steps:\n" +
		"      - uses: actions/checkout@" + sha + " # v7.0.1\n" +
		"        uses: github/codeql-action/upload-sarif@" + sha + " # v4.39.1\n"
	goodDockerfile := "FROM node:26-alpine" + digest + " AS web\nFROM --platform=linux/amd64 golang:1.27-alpine" + digest + " AS go\nCOPY --from=web /a /b\nFROM web\nFROM scratch\n"
	goodMakefile := "PG_IMAGE := postgres:18.6-alpine" + digest + "\nZAP_IMAGE := ghcr.io/zaproxy/zap-stable:2.17.0" + digest + "\nOTHER := x\n"
	goodShell := "readonly COSIGN_IMAGE=ghcr.io/sigstore/cosign/cosign:v3.1.4" + digest + "\nOTHER=x\n"
	goodCompose := "services:\n  db:\n    image: postgres:18.6-alpine" + digest + "\n  app:\n    image: ghcr.io/jnaranj09/open-customer:${APP_VERSION:?x}\n  backup:\n    image: ghcr.io/jnaranj09/open-customer-backup:${APP_VERSION:?x}\n"
	if got := slices.Concat(checkWorkflowPins("w", goodWorkflow), checkDockerfilePins("d", goodDockerfile), checkVarPins("m", goodMakefile, makeImageVar),
		checkVarPins("s", goodShell, shellImageVar), checkComposePins("c", []byte(goodCompose))); len(got) != 0 {
		t.Fatalf("pinned files are refused: %q", got)
	}
	bad := []struct {
		name     string
		problems []string
	}{
		{"uses a moving tag", checkWorkflowPins("w", "      - uses: actions/checkout@v7\n")},
		{"uses a full version tag", checkWorkflowPins("w", "      - uses: actions/checkout@v7.0.1\n")},
		{"uses a short SHA", checkWorkflowPins("w", "      - uses: actions/checkout@3d3c42e # v7.0.1\n")},
		{"uses a SHA without the version comment", checkWorkflowPins("w", "      - uses: actions/checkout@"+sha+"\n")},
		{"uses a SHA with a moving version comment", checkWorkflowPins("w", "      - uses: actions/checkout@"+sha+" # v7\n")},
		{"uses an uppercase SHA", checkWorkflowPins("w", "      - uses: actions/checkout@"+strings.Repeat("A", 40)+" # v7.0.1\n")},
		{"uses a docker image", checkWorkflowPins("w", "      - uses: docker://alpine:3\n")},
		{"workflow service image without a digest", checkWorkflowPins("w", "        image: postgres:18.6-alpine\n")},
		{"FROM without a digest", checkDockerfilePins("d", "FROM postgres:18.6-alpine\n")},
		{"FROM with a short digest", checkDockerfilePins("d", "FROM postgres:18.6-alpine@sha256:abc\n")},
		{"FROM by digest without a tag", checkDockerfilePins("d", "FROM postgres"+digest+"\n")},
		{"FROM a later stage name before it exists", checkDockerfilePins("d", "FROM web\nFROM node:26"+digest+" AS web\n")},
		{"Makefile image without a digest", checkVarPins("m", "TRIVY_IMAGE := aquasec/trivy:0.76.1\n", makeImageVar)},
		{"shell image without a digest", checkVarPins("s", "COSIGN_IMAGE=ghcr.io/sigstore/cosign/cosign:v3.1.4\n", shellImageVar)},
		{"compose image without a digest", checkComposePins("c", []byte("services:\n  cf:\n    image: cloudflare/cloudflared:2026.10.7\n"))},
		{"compose own image with a fixed tag", checkComposePins("c", []byte("services:\n  app:\n    image: ghcr.io/jnaranj09/open-customer:1.0.0\n"))},
		{"compose other image with the APP_VERSION tag", checkComposePins("c", []byte("services:\n  x:\n    image: ghcr.io/someone/else:${APP_VERSION:?x}\n"))},
		{"compose not YAML", checkComposePins("c", []byte("services: ["))},
	}
	for _, c := range bad {
		if len(c.problems) == 0 {
			t.Errorf("%s: no problem reported", c.name)
		}
	}
}

func TestSupplyChainPins(t *testing.T) {
	t.Parallel()
	base := root(t)
	workflows, err := filepath.Glob(filepath.Join(base, ".github", "workflows", "*.yml"))
	if err != nil || len(workflows) == 0 {
		t.Fatalf("no workflows found (err %v)", err)
	}
	var problems []string
	for _, w := range workflows {
		rel := ".github/workflows/" + filepath.Base(w)
		problems = append(problems, checkWorkflowPins(rel, readText(t, rel))...)
	}
	for _, rel := range []string{"Dockerfile", "deploy/backup/Dockerfile"} {
		problems = append(problems, checkDockerfilePins(rel, readText(t, rel))...)
	}
	problems = append(problems, checkVarPins("Makefile", readText(t, "Makefile"), makeImageVar)...)
	scripts, err := filepath.Glob(filepath.Join(base, "deploy", "scripts", "*.sh"))
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no deploy scripts found (err %v)", err)
	}
	for _, s := range scripts {
		rel := "deploy/scripts/" + filepath.Base(s)
		problems = append(problems, checkVarPins(rel, readText(t, rel), shellImageVar)...)
	}
	problems = append(problems, checkComposePins(composePath, []byte(readText(t, composePath)))...)
	for _, p := range problems {
		t.Error(p)
	}
}
