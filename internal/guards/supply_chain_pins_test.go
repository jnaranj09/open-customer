package guards

// supply-chain-pins (docs/designs/devsecops.md section 3; docs/threat-model.md T76 and T91): a tag
// can be moved upstream, so every GitHub Action is pinned by its full commit SHA (with the release
// in a `# vX.Y.Z` comment, which Dependabot keeps in step), and every image a build, a scan or the
// stack pulls is pinned by digest (`<tag>@sha256:<64 hex>`). The one exception is this repo's own
// app and backup images in deploy/compose.yaml: their tag is the release a deployer picks
// (APP_VERSION). The tool images live in security/tools.Dockerfile, where Dependabot reads them;
// each Makefile `*_IMAGE` must read its stage there (`$(call tool_image,<stage>)`) or be a pin
// itself. Every Dockerfile in the repo is found by name (`Dockerfile`, `*.Dockerfile`), and every
// workflow (`*.yml`, `*.yaml`). make check-pins keeps the copies of each pin equal; this guard keeps
// each one a pin.
//
// Two gaps a digest does not close (security audit A9) are checked too: an `apk add` installs only
// the packages in acceptedApk (a new tool comes from its own image by digest instead), and every Dependabot ecosystem that proposes versions waits
// 14 days (cooldown default-days), the "at least two weeks old" rule.

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
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
	// image: <ref> in a workflow (a service or a job container), or the short `container: <ref>`.
	workflowImage = regexp.MustCompile(`^[ \t]*(?:image|container):[ \t]*([^ \t#]+)`)
	fromLine      = regexp.MustCompile(`(?i)^[ \t]*FROM[ \t]+(?:--platform=\S+[ \t]+)?(\S+)(?:[ \t]+AS[ \t]+(\S+))?`)
	makeImageVar  = regexp.MustCompile(`^([A-Z0-9_]+_IMAGE)[ \t]*:?=[ \t]*(.*)$`)
	makeToolImage = regexp.MustCompile(`^\$\(call tool_image,([a-z0-9-]+)\)$`)
	shellImageVar = regexp.MustCompile(`^[ \t]*(?:readonly[ \t]+|export[ \t]+)?([A-Z0-9_]+_IMAGE)=(\S*)`)
)

// acceptedApk are the Alpine packages a Dockerfile may `apk add` unpinned. The base image pins the
// Alpine release by digest. bash and jq are accepted (threat model T91). restic is here only until
// the owner decides (TASKS.md "Found, not fixed"): its official images fail make image-scan.
var acceptedApk = []string{"bash", "jq", "restic"}

// apkAdd matches an `apk add` and captures its arguments, up to the end of the shell command.
var apkAdd = regexp.MustCompile(`apk[ \t]+add[ \t]+([^&;|\n]*)`)

// checkApkAdds returns the packages a Dockerfile's `apk add` installs that acceptedApk does not hold.
func checkApkAdds(name, src string) []string {
	var problems []string
	for i, line := range strings.Split(src, "\n") {
		for _, m := range apkAdd.FindAllStringSubmatch(line, -1) {
			for _, arg := range strings.Fields(m[1]) {
				if !strings.HasPrefix(arg, "-") && !slices.Contains(acceptedApk, arg) {
					problems = append(problems, fmt.Sprintf("%s:%d: apk add %s is not an accepted package (copy it from a digest-pinned image)", name, i+1, arg))
				}
			}
		}
	}
	return problems
}

// cooldownEcosystems are the Dependabot ecosystems that must wait 14 days before a version.
var cooldownEcosystems = []string{"gomod", "npm", "docker", "docker-compose"}

// checkDependabotCooldown returns the dependabot.yml entries of cooldownEcosystems whose
// cooldown default-days is under 14, and an ecosystem of that list with no entry at all.
func checkDependabotCooldown(name string, src []byte) []string {
	var doc struct {
		Updates []struct {
			Ecosystem string `yaml:"package-ecosystem"`
			Directory string `yaml:"directory"`
			Cooldown  struct {
				DefaultDays int `yaml:"default-days"`
			} `yaml:"cooldown"`
		} `yaml:"updates"`
	}
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return []string{fmt.Sprintf("%s: not valid YAML: %v", name, err)}
	}
	var problems []string
	seen := map[string]bool{}
	for _, u := range doc.Updates {
		if !slices.Contains(cooldownEcosystems, u.Ecosystem) {
			continue
		}
		seen[u.Ecosystem] = true
		if u.Cooldown.DefaultDays < 14 {
			problems = append(problems, fmt.Sprintf("%s: %s %s: cooldown default-days is %d, want at least 14", name, u.Ecosystem, u.Directory, u.Cooldown.DefaultDays))
		}
	}
	for _, e := range cooldownEcosystems {
		if !seen[e] {
			problems = append(problems, fmt.Sprintf("%s: no %s entry", name, e))
		}
	}
	return problems
}

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

// toolStages returns the stage name → image of the `FROM <image> AS <name>` lines of a Dockerfile,
// and the names given more than once.
func toolStages(src string) (map[string]string, []string) {
	stages := map[string]string{}
	var dups []string
	for _, line := range strings.Split(src, "\n") {
		if m := fromLine.FindStringSubmatch(line); m != nil && m[2] != "" {
			if _, dup := stages[m[2]]; dup {
				dups = append(dups, m[2])
			}
			stages[m[2]] = m[1]
		}
	}
	return stages, dups
}

// checkMakefilePins returns the Makefile `*_IMAGE` assignments that are neither a pin nor a read of
// one stage of the tools Dockerfile.
func checkMakefilePins(name, src string, stages map[string]string) []string {
	var problems []string
	for i, line := range strings.Split(src, "\n") {
		m := makeImageVar.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		value := strings.TrimSpace(m[2])
		if t := makeToolImage.FindStringSubmatch(value); t != nil {
			if _, ok := stages[t[1]]; !ok {
				problems = append(problems, fmt.Sprintf("%s:%d: %s reads stage %q, which %s does not have", name, i+1, m[1], t[1], toolsDockerfile))
			}
			continue
		}
		if !pinnedImage.MatchString(value) {
			problems = append(problems, fmt.Sprintf("%s:%d: %s %q has no digest", name, i+1, m[1], value))
		}
	}
	return problems
}

// repoDockerfiles returns every file named Dockerfile or *.Dockerfile in the repo (skipDirs not
// walked), repo-relative.
func repoDockerfiles(t *testing.T) []string {
	t.Helper()
	var out []string
	err := fs.WalkDir(os.DirFS(root(t)), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() == "Dockerfile" || strings.HasSuffix(d.Name(), ".Dockerfile") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

const toolsDockerfile = "security/tools.Dockerfile"

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
	goodMakefile := "PG_IMAGE := $(call tool_image,postgres)\nZAP_IMAGE := ghcr.io/zaproxy/zap-stable:2.17.0" + digest + "\nOTHER := x\n"
	stages := map[string]string{"postgres": "postgres:18.6-alpine" + digest}
	goodShell := "readonly COSIGN_IMAGE=ghcr.io/sigstore/cosign/cosign:v3.1.4" + digest + "\nOTHER=x\n"
	goodCompose := "services:\n  db:\n    image: postgres:18.6-alpine" + digest + "\n  app:\n    image: ghcr.io/jnaranj09/open-customer:${APP_VERSION:?x}\n  backup:\n    image: ghcr.io/jnaranj09/open-customer-backup:${APP_VERSION:?x}\n"
	goodApk := "RUN apk add --no-cache bash jq && rm -rf /x\nRUN apk add jq; echo restic\n"
	goodDependabot := "updates:\n  - package-ecosystem: gomod\n    cooldown:\n      default-days: 14\n  - package-ecosystem: npm\n    cooldown:\n      default-days: 21\n" +
		"  - package-ecosystem: docker\n    cooldown:\n      default-days: 14\n  - package-ecosystem: docker-compose\n    cooldown:\n      default-days: 14\n" +
		"  - package-ecosystem: github-actions\n"
	if got := slices.Concat(checkWorkflowPins("w", goodWorkflow), checkDockerfilePins("d", goodDockerfile), checkMakefilePins("m", goodMakefile, stages),
		checkVarPins("s", goodShell, shellImageVar), checkComposePins("c", []byte(goodCompose)), checkApkAdds("d", goodApk),
		checkDependabotCooldown("b", []byte(goodDependabot))); len(got) != 0 {
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
		{"Makefile image without a digest", checkMakefilePins("m", "TRIVY_IMAGE := aquasec/trivy:0.76.1\n", stages)},
		{"Makefile image reading a stage the tools file lacks", checkMakefilePins("m", "TRIVY_IMAGE := $(call tool_image,trivy)\n", stages)},
		{"Makefile image reading a stage with another call", checkMakefilePins("m", "TRIVY_IMAGE := $(shell cat x)\n", stages)},
		{"workflow job container short form without a digest", checkWorkflowPins("w", "    container: node:26-alpine\n")},
		{"workflow job container short form with a comment", checkWorkflowPins("w", "    container: node:26-alpine # the build image\n")},
		{"shell image without a digest", checkVarPins("s", "COSIGN_IMAGE=ghcr.io/sigstore/cosign/cosign:v3.1.4\n", shellImageVar)},
		{"compose image without a digest", checkComposePins("c", []byte("services:\n  cf:\n    image: cloudflare/cloudflared:2026.10.7\n"))},
		{"compose own image with a fixed tag", checkComposePins("c", []byte("services:\n  app:\n    image: ghcr.io/jnaranj09/open-customer:1.0.0\n"))},
		{"compose other image with the APP_VERSION tag", checkComposePins("c", []byte("services:\n  x:\n    image: ghcr.io/someone/else:${APP_VERSION:?x}\n"))},
		{"compose not YAML", checkComposePins("c", []byte("services: ["))},
		{"apk add of a package not accepted", checkApkAdds("d", "RUN apk add --no-cache bash curl jq\n")},
		{"apk add of a package after a flag", checkApkAdds("d", "RUN apk add --virtual .x curl\n")},
		{"gomod without a cooldown", checkDependabotCooldown("b", []byte("updates:\n  - package-ecosystem: gomod\n"))},
		{"npm with a short cooldown", checkDependabotCooldown("b", []byte("updates:\n  - package-ecosystem: npm\n    cooldown:\n      default-days: 7\n"))},
		{"dependabot not YAML", checkDependabotCooldown("b", []byte("updates: ["))},
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
	yml, err := filepath.Glob(filepath.Join(base, ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	yamlFiles, err := filepath.Glob(filepath.Join(base, ".github", "workflows", "*.yaml"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	workflows := slices.Concat(yml, yamlFiles)
	if len(workflows) == 0 {
		t.Fatal("no workflows found")
	}
	var problems []string
	for _, w := range workflows {
		rel := ".github/workflows/" + filepath.Base(w)
		problems = append(problems, checkWorkflowPins(rel, readText(t, rel))...)
	}
	dockerfiles := repoDockerfiles(t)
	for _, want := range []string{"Dockerfile", "deploy/backup/Dockerfile", toolsDockerfile} {
		if !slices.Contains(dockerfiles, want) {
			t.Fatalf("the Dockerfile walk found %q, not %s", dockerfiles, want)
		}
	}
	for _, rel := range dockerfiles {
		problems = append(problems, checkDockerfilePins(rel, readText(t, rel))...)
		problems = append(problems, checkApkAdds(rel, readText(t, rel))...)
	}
	problems = append(problems, checkDependabotCooldown(".github/dependabot.yml", []byte(readText(t, ".github/dependabot.yml")))...)
	stages, dups := toolStages(readText(t, toolsDockerfile))
	for _, d := range dups {
		problems = append(problems, fmt.Sprintf("%s: stage %s is named twice", toolsDockerfile, d))
	}
	problems = append(problems, checkMakefilePins("Makefile", readText(t, "Makefile"), stages)...)
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
