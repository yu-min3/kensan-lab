package agentclient

import (
	"os/exec"
	"strings"
	"testing"
)

func TestKonroDoesNotImportRuntimeImplementation(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./...")
	cmd.Dir = "../.."
	output, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("dependency graph: %s %v", output, e)
	}
	for _, line := range strings.Fields(string(output)) {
		if strings.Contains(line, "github.com/yu-min3/koubou") || strings.Contains(line, "agent-runtime") {
			t.Fatalf("Konro must use the HTTP contract: %s", line)
		}
	}
}
