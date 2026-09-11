package command

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSMARTCommandPreservesStandbyAndFailureStatus(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "smartctl")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SMART_ARGS\"\nprintf '%s' '{\"smartctl\":{\"exit_status\":3}}'\nexit \"$SMART_EXIT\"\n"
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	argsPath := filepath.Join(dir, "args")
	t.Setenv("SMART_ARGS", argsPath)
	for _, code := range []int{0, 2, 3, 5, 8} {
		t.Setenv("SMART_EXIT", strconv.Itoa(code))
		output, status := ExecSmartCTLByPath("/dev/sda", "ata")
		if status != code || len(output) == 0 {
			t.Fatalf("status %d, expected %d", status, code)
		}
		args, err := os.ReadFile(argsPath)
		if err != nil {
			t.Fatal(err)
		}
		expected := []string{"-a", "-n", "standby,3,5", "-j", "-d", "ata", "/dev/sda"}
		if string(args) != strings.Join(expected, "\n")+"\n" {
			t.Fatalf("unsafe arguments: %s", args)
		}
	}
}
