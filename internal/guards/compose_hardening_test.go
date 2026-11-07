package guards

// compose-hardening (docs/designs/devsecops.md section 3, decision D1): no config scanner reads
// Compose files, so this guard reads deploy/compose.yaml (YAML merge keys resolved) and fails when
// a service can gain privileges, keeps a Linux capability, has no memory limit, has a writable
// root file system, publishes a port, or reaches the host (a host namespace, another container's
// network, an unconfined seccomp or AppArmor profile, a host device, the Docker socket, or a bind
// mount of /, /proc, /sys or /etc), or sets a pull_policy that pulls by itself (anything but
// missing, if_not_present or never: upgrade.sh pulls and checks each pulled image against the
// digest cosign verified, so `up -d` must run that local image and never pull a tag again;
// docs/threat-model.md T68). Each exemption below has its one-line
// reason, and an exemption the file no longer needs fails too, so the lists only shrink.

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const composePath = "deploy/compose.yaml"

// composeWritableRoot lists the services allowed a writable root file system. Empty: each service
// writes only to its volumes and to the tmpfs mounts compose.yaml gives it.
var composeWritableRoot = map[string]string{}

// composeCapAdd lists, per service, the capabilities it may add back after `cap_drop: [ALL]`, each
// with its reason. Each one was seen failing without it (docs/threat-model.md T73).
var composeCapAdd = map[string]map[string]string{
	"backup": {
		"DAC_OVERRIDE": "root without it cannot open the lock file in /backup-status (owned by 65532) or write a BACKUP_TARGET owned by another user",
	},
	"restore": {
		"DAC_OVERRIDE": "as backup: it creates /backup-status/restore-first and clears /backup-status/work",
		"CHOWN":        "restic restores each file's owner (lchown), and backup.sh then hands the media files to 65532",
		"FOWNER":       "restic then restores the times of files it no longer owns",
	},
}

type composeService struct {
	SecurityOpt []string `yaml:"security_opt"`
	CapDrop     []string `yaml:"cap_drop"`
	CapAdd      []string `yaml:"cap_add"`
	MemLimit    string   `yaml:"mem_limit"`
	ReadOnly    bool     `yaml:"read_only"`
	Ports       []any    `yaml:"ports"`
	Privileged  bool     `yaml:"privileged"`
	NetworkMode string   `yaml:"network_mode"`
	Pid         string   `yaml:"pid"`
	Ipc         string   `yaml:"ipc"`
	UsernsMode  string   `yaml:"userns_mode"`
	Cgroup      string   `yaml:"cgroup"`
	Devices     []any    `yaml:"devices"`
	Volumes     []any    `yaml:"volumes"`
	PullPolicy  string   `yaml:"pull_policy"`
}

// composeLocalPullPolicies are the pull_policy values under which compose never pulls an image
// that is already local ("" is the default, missing).
var composeLocalPullPolicies = []string{"", "missing", "if_not_present", "never"}

// volumeSource is the host side of a volume entry: the part before the first ":" of the short
// syntax, or the long syntax's source.
func volumeSource(v any) string {
	switch v := v.(type) {
	case string:
		src, _, _ := strings.Cut(v, ":")
		return src
	case map[string]any:
		src, _ := v["source"].(string)
		return src
	}
	return ""
}

// hostSystemPath reports whether a bind source is the host's root or under /proc, /sys or /etc.
func hostSystemPath(src string) bool {
	if !strings.HasPrefix(src, "/") {
		return false
	}
	clean := path.Clean(src)
	if clean == "/" {
		return true
	}
	for _, dir := range []string{"/proc", "/sys", "/etc"} {
		if clean == dir || strings.HasPrefix(clean, dir+"/") {
			return true
		}
	}
	return false
}

// checkComposeHardening returns every hardening problem in a Compose file.
func checkComposeHardening(src []byte, writableRoot map[string]string, capAdd map[string]map[string]string) []string {
	var doc struct {
		Services map[string]composeService `yaml:"services"`
	}
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return []string{fmt.Sprintf("not valid YAML: %v", err)}
	}
	if len(doc.Services) == 0 {
		return []string{"no services"}
	}
	var problems []string
	bad := func(name, format string, args ...any) {
		problems = append(problems, name+": "+fmt.Sprintf(format, args...))
	}
	for _, name := range slices.Sorted(maps.Keys(doc.Services)) {
		s := doc.Services[name]
		if !slices.Contains(s.SecurityOpt, "no-new-privileges:true") {
			bad(name, "security_opt has no no-new-privileges:true")
		}
		for _, opt := range s.SecurityOpt {
			if o := strings.ReplaceAll(opt, "=", ":"); o == "seccomp:unconfined" || o == "apparmor:unconfined" {
				bad(name, "security_opt %s", opt)
			}
		}
		if !slices.Equal(s.CapDrop, []string{"ALL"}) {
			bad(name, "cap_drop is %q, want [ALL]", s.CapDrop)
		}
		for _, c := range s.CapAdd {
			if capAdd[name][c] == "" {
				bad(name, "cap_add %s has no exemption with a reason in this guard", c)
			}
		}
		for _, c := range slices.Sorted(maps.Keys(capAdd[name])) {
			if !slices.Contains(s.CapAdd, c) {
				bad(name, "the exemption for cap_add %s is no longer needed: remove it", c)
			}
		}
		if strings.TrimSpace(s.MemLimit) == "" {
			bad(name, "no mem_limit")
		}
		_, writable := writableRoot[name]
		if !s.ReadOnly && !writable {
			bad(name, "read_only is not true and the service has no exemption with a reason")
		}
		if s.ReadOnly && writable {
			bad(name, "the read_only exemption is no longer needed: remove it")
		}
		if len(s.Ports) > 0 {
			bad(name, "publishes ports")
		}
		if s.Privileged {
			bad(name, "privileged")
		}
		for key, mode := range map[string]string{"network_mode": s.NetworkMode, "pid": s.Pid, "ipc": s.Ipc, "userns_mode": s.UsernsMode, "cgroup": s.Cgroup} {
			if mode == "host" {
				bad(name, "%s: host", key)
			}
		}
		if strings.HasPrefix(s.NetworkMode, "container:") || strings.HasPrefix(s.NetworkMode, "service:") {
			bad(name, "network_mode %s shares another container's network", s.NetworkMode)
		}
		if len(s.Devices) > 0 {
			bad(name, "maps host devices")
		}
		if !slices.Contains(composeLocalPullPolicies, s.PullPolicy) {
			bad(name, "pull_policy %s pulls by itself; upgrade.sh's verified-digest check would not cover what up -d runs", s.PullPolicy)
		}
		for _, v := range s.Volumes {
			if strings.Contains(fmt.Sprint(v), "docker.sock") {
				bad(name, "mounts the Docker socket")
			}
			if src := volumeSource(v); hostSystemPath(src) {
				bad(name, "bind-mounts the host's %s", src)
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(capAdd)) {
		if _, ok := doc.Services[name]; !ok {
			bad(name, "has a cap_add exemption but is not a service: remove it")
		}
	}
	for _, name := range slices.Sorted(maps.Keys(writableRoot)) {
		if _, ok := doc.Services[name]; !ok {
			bad(name, "has a read_only exemption but is not a service: remove it")
		}
	}
	return problems
}

func TestComposeHardeningProbes(t *testing.T) {
	t.Parallel()
	good := `
x-hardening: &hardening
  security_opt: ["no-new-privileges:true"]
  cap_drop: [ALL]
  read_only: true
services:
  web:
    <<: *hardening
    mem_limit: 256m
  job:
    <<: *hardening
    cap_add: [CHOWN]
    mem_limit: 1g
`
	capAdd := map[string]map[string]string{"job": {"CHOWN": "a reason"}}
	if got := checkComposeHardening([]byte(good), map[string]string{}, capAdd); len(got) != 0 {
		t.Fatalf("a hardened file is refused: %q", got)
	}
	// with returns the good file with the web service's extra lines (indented four spaces).
	with := func(lines ...string) string {
		return strings.Replace(good, "    mem_limit: 256m\n", "    mem_limit: 256m\n    "+strings.Join(lines, "\n    ")+"\n", 1)
	}
	bad := []struct {
		name         string
		src          string
		writableRoot map[string]string
		capAdd       map[string]map[string]string
	}{
		{"not YAML", "services: [", nil, capAdd},
		{"no services", "name: x\n", nil, capAdd},
		{"no no-new-privileges", with(`security_opt: ["label:disable"]`), nil, capAdd},
		{"no-new-privileges false", with(`security_opt: ["no-new-privileges:false"]`), nil, capAdd},
		{"cap_drop not ALL", with("cap_drop: [NET_RAW]"), nil, capAdd},
		{"cap_drop empty", with("cap_drop: []"), nil, capAdd},
		{"cap_add without an exemption", with("cap_add: [NET_ADMIN]"), nil, capAdd},
		{"cap_add exemption for another service", good, nil, map[string]map[string]string{"web": {"CHOWN": "r"}, "job": {"CHOWN": "r"}}},
		{"cap_add exemption no longer needed", strings.Replace(good, "    cap_add: [CHOWN]\n", "", 1), nil, capAdd},
		{"cap_add exemption for a service that is gone", good, nil, map[string]map[string]string{"job": {"CHOWN": "r"}, "gone": {"CHOWN": "r"}}},
		{"no mem_limit", strings.Replace(good, "    mem_limit: 256m\n", "", 1), nil, capAdd},
		{"read_only false", with("read_only: false"), nil, capAdd},
		{"read_only exemption no longer needed", good, map[string]string{"web": "r"}, capAdd},
		{"read_only exemption for a service that is gone", good, map[string]string{"gone": "r"}, capAdd},
		{"ports", with(`ports: ["8080:8080"]`), nil, capAdd},
		{"privileged", with("privileged: true"), nil, capAdd},
		{"network_mode host", with("network_mode: host"), nil, capAdd},
		{"pid host", with("pid: host"), nil, capAdd},
		{"ipc host", with("ipc: host"), nil, capAdd},
		{"devices", with(`devices: ["/dev/sda:/dev/sda"]`), nil, capAdd},
		{"docker socket", with(`volumes: ["/var/run/docker.sock:/var/run/docker.sock"]`), nil, capAdd},
		{"seccomp unconfined", with(`security_opt: ["no-new-privileges:true", "seccomp:unconfined"]`), nil, capAdd},
		{"seccomp=unconfined", with(`security_opt: ["no-new-privileges:true", "seccomp=unconfined"]`), nil, capAdd},
		{"apparmor unconfined", with(`security_opt: ["no-new-privileges:true", "apparmor:unconfined"]`), nil, capAdd},
		{"userns_mode host", with("userns_mode: host"), nil, capAdd},
		{"cgroup host", with("cgroup: host"), nil, capAdd},
		{"network_mode container", with("network_mode: container:other"), nil, capAdd},
		{"network_mode service", with("network_mode: service:job"), nil, capAdd},
		{"bind of the host root", with(`volumes: ["/:/host:ro"]`), nil, capAdd},
		{"bind of /proc", with(`volumes: ["/proc:/host/proc:ro"]`), nil, capAdd},
		{"bind of /sys", with(`volumes: ["/sys/fs/cgroup:/sys/fs/cgroup"]`), nil, capAdd},
		{"bind of /etc", with(`volumes: ["/etc/:/host/etc:ro"]`), nil, capAdd},
		{"long-syntax bind of /etc", with("volumes:\n      - type: bind\n        source: /etc/passwd\n        target: /x"), nil, capAdd},
		{"pull_policy always", with("pull_policy: always"), nil, capAdd},
		{"pull_policy build", with("pull_policy: build"), nil, capAdd},
		{"pull_policy daily", with("pull_policy: daily"), nil, capAdd},
	}
	for _, c := range bad {
		if got := checkComposeHardening([]byte(c.src), c.writableRoot, c.capAdd); len(got) == 0 {
			t.Errorf("%s: no problem reported", c.name)
		}
	}
}

func TestComposeHardening(t *testing.T) {
	t.Parallel()
	for _, p := range checkComposeHardening([]byte(readText(t, composePath)), composeWritableRoot, composeCapAdd) {
		t.Errorf("%s: %s", composePath, p)
	}
}
