package guards

// app-image (docs/threat-model.md T72): the app image runs as a non-root user and has no shell an
// attacker could use. Trivy config (make config-scan, DS-0002) sees only "not root"; this guard
// checks the rest on the root Dockerfile's final stage: its base is distroless static with the
// nonroot tag (no shell, no package manager), pinned by digest; it has no RUN (a RUN needs a shell,
// so one would mean a base that has one); and its last USER is 65532:65532.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// appBase is the final stage's base: distroless static, nonroot tag, by digest.
var appBase = regexp.MustCompile(`^gcr\.io/distroless/static-debian[0-9]+:nonroot@sha256:[0-9a-f]{64}$`)

// checkAppImage returns what is wrong with the final stage of a Dockerfile.
func checkAppImage(name, src string) []string {
	var final []string
	base := ""
	for _, line := range strings.Split(src, "\n") {
		if m := fromLine.FindStringSubmatch(line); m != nil {
			base, final = m[1], nil
			continue
		}
		final = append(final, strings.TrimSpace(line))
	}
	if base == "" {
		return []string{name + ": no FROM line"}
	}
	var problems []string
	if !appBase.MatchString(base) {
		problems = append(problems, fmt.Sprintf("%s: the final stage is FROM %q, not gcr.io/distroless/static-debianNN:nonroot by digest (no shell)", name, base))
	}
	user := ""
	for _, line := range final {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "RUN":
			problems = append(problems, fmt.Sprintf("%s: the final stage has a RUN (%q), which needs a shell", name, line))
		case "USER":
			user = strings.Join(fields[1:], " ")
		}
	}
	if user != "65532:65532" {
		problems = append(problems, fmt.Sprintf("%s: the final stage's last USER is %q, want 65532:65532", name, user))
	}
	return problems
}

func TestAppImageProbes(t *testing.T) {
	t.Parallel()
	digest := "@sha256:" + strings.Repeat("b", 64)
	builder := "FROM golang:1.27.3-alpine" + digest + " AS go\nRUN go build\n"
	good := builder + "FROM gcr.io/distroless/static-debian12:nonroot" + digest + "\nCOPY --from=go /a /b\nUSER 65532:65532\nENTRYPOINT [\"/b\"]\n"
	if got := checkAppImage("d", good); len(got) != 0 {
		t.Fatalf("a distroless nonroot final stage is refused: %q", got)
	}
	bad := []struct{ name, src string }{
		{"no FROM", "COPY a b\n"},
		{"an alpine final stage", builder + "FROM alpine:3.22" + digest + "\nUSER 65532:65532\n"},
		{"distroless with the root tag", builder + "FROM gcr.io/distroless/static-debian12:latest" + digest + "\nUSER 65532:65532\n"},
		{"distroless with a shell (debug tag)", builder + "FROM gcr.io/distroless/static-debian12:debug-nonroot" + digest + "\nUSER 65532:65532\n"},
		{"distroless without a digest", builder + "FROM gcr.io/distroless/static-debian12:nonroot\nUSER 65532:65532\n"},
		{"a RUN in the final stage", strings.Replace(good, "USER 65532:65532", "RUN chmod 700 /b\nUSER 65532:65532", 1)},
		{"no USER", strings.Replace(good, "USER 65532:65532\n", "", 1)},
		{"USER root last", strings.Replace(good, "USER 65532:65532", "USER 65532:65532\nUSER root", 1)},
		{"USER without the group", strings.Replace(good, "USER 65532:65532", "USER 65532", 1)},
		{"the good stage is not the last one", good + "FROM alpine:3.22" + digest + "\n"},
	}
	for _, c := range bad {
		if len(checkAppImage("d", c.src)) == 0 {
			t.Errorf("%s: no problem reported", c.name)
		}
	}
}

func TestAppImage(t *testing.T) {
	t.Parallel()
	for _, p := range checkAppImage("Dockerfile", readText(t, "Dockerfile")) {
		t.Error(p)
	}
}
