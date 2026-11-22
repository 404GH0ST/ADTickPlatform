package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cliRule answers invocations whose joined arguments match the glob (only `*`
// is special). The nth matching call gets outs[n] and codes[n]; the last
// entry repeats once a rule runs out.
type cliRule struct {
	match string
	outs  []string
	codes []int
}

func reply(out string, code int) cliRule { return cliRule{outs: []string{out}, codes: []int{code}} }

func on(match string, rule cliRule) cliRule {
	rule.match = match
	return rule
}

func seq(match string, steps ...cliRule) cliRule {
	rule := cliRule{match: match}
	for _, step := range steps {
		rule.outs = append(rule.outs, step.outs...)
		rule.codes = append(rule.codes, step.codes...)
	}
	return rule
}

func shellGlob(pattern string) string {
	parts := strings.Split(pattern, "*")
	for i, part := range parts {
		parts[i] = "'" + strings.ReplaceAll(part, "'", `'\''`) + "'"
	}
	return strings.Join(parts, "*")
}

// fakeCLI writes an executable that logs every invocation and answers by the
// first matching rule; unmatched calls succeed silently. It returns the binary
// path and a function reading the invocation log.
func fakeCLI(t *testing.T, rules ...cliRule) (string, func() []string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	var script strings.Builder
	script.WriteString("#!/bin/sh\nargs=\"$*\"\nprintf '%s\\n' \"$args\" >> '" + logPath + "'\ncase \"$args\" in\n")
	for i, rule := range rules {
		for k := range rule.outs {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("out.%d.%d", i, k+1)), []byte(rule.outs[k]), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("code.%d.%d", i, k+1)), []byte(fmt.Sprint(rule.codes[k])), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		fmt.Fprintf(&script, "  %s) i=%d; len=%d ;;\n", shellGlob(rule.match), i, len(rule.outs))
	}
	// Most runtime fixtures use the same instance. Return a matching inspect
	// record unless a test supplies an explicit ownership rule above.
	script.WriteString("  'inspect --format {{.Id}} {{json .Config.Labels}} svc-101-1') printf '%s\\n' 'svc-101-1 {\"adplatform.team_id\":\"101\",\"adplatform.challenge_id\":\"1\",\"adplatform.state_volume\":\"vol-101-1\"}'; exit 0 ;;\n")
	script.WriteString("  *) exit 0 ;;\nesac\n")
	fmt.Fprintf(&script, "d='%s'\nn=$(cat \"$d/cnt.$i\" 2>/dev/null || echo 0); n=$((n+1)); echo $n > \"$d/cnt.$i\"\n", dir)
	script.WriteString("k=$n; [ $k -gt $len ] && k=$len\ncat \"$d/out.$i.$k\"\nexit $(cat \"$d/code.$i.$k\")\n")
	bin := filepath.Join(dir, "cli")
	if err := os.WriteFile(bin, []byte(script.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, func() []string {
		data, _ := os.ReadFile(logPath)
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}
}

func callsMatching(calls []string, prefix string) int {
	count := 0
	for _, call := range calls {
		if strings.HasPrefix(call, prefix) {
			count++
		}
	}
	return count
}

func TestFakeCLIDispatchesAndSequences(t *testing.T) {
	bin, calls := fakeCLI(t,
		seq("probe *", reply("not yet", 1), reply("ready", 0)),
		on("say it's *", reply("quoted", 0)),
	)
	run := func(args ...string) (string, int) {
		out, err := runForTest(bin, args...)
		if err != nil {
			return out, 1
		}
		return out, 0
	}
	if out, code := run("probe", "x"); out != "not yet" || code != 1 {
		t.Fatalf("first probe: %q %d", out, code)
	}
	if out, code := run("probe", "x"); out != "ready" || code != 0 {
		t.Fatalf("second probe: %q %d", out, code)
	}
	if out, _ := run("probe", "x"); out != "ready" {
		t.Fatalf("last reply must repeat: %q", out)
	}
	if out, _ := run("say", "it's", "fine"); out != "quoted" {
		t.Fatalf("quotes in patterns: %q", out)
	}
	if out, code := run("unmatched"); out != "" || code != 0 {
		t.Fatalf("unmatched calls succeed silently: %q %d", out, code)
	}
	if len(calls()) != 5 {
		t.Fatalf("calls: %v", calls())
	}
}

func runForTest(bin string, args ...string) (string, error) {
	output, err := exec.Command(bin, args...).Output()
	return string(output), err
}
