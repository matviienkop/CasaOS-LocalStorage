package command

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	exec2 "github.com/IceWhaleTech/CasaOS-Common/utils/exec"
)

// exec smart
func ExecSmartCTLByPath(path, deviceType string) ([]byte, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	args := []string{"-a", "-n", "standby,3,5", "-j"}
	if deviceType != "" {
		args = append(args, "-d", deviceType)
	}
	args = append(args, path)
	output, err := exec2.CommandContext(ctx, "smartctl", args...).Output()
	if err == nil {
		return output, 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return output, exitError.ExitCode()
	}
	return output, -1
}

func ExecEnabledSMART(path string) ([]byte, error) {
	return exec2.Command("smartctl", "-s", "on", path).CombinedOutput()
}

// 执行 lsblk 命令
func ExecLSBLKByPath(path string) []byte {
	output, err := exec2.Command("lsblk", path, "-O", "-J", "-b").Output()
	if err != nil {
		fmt.Println("lsblk", err)
		return nil
	}
	return output
}

// 执行 lsblk 命令
func ExecLSBLK() []byte {
	output, err := exec2.Command("lsblk", "-O", "-J", "-b").Output()
	if err != nil {
		fmt.Println("lsblk", err)
		return nil
	}
	return output
}

func ExecuteCommand(name string, arg ...string) ([]byte, error) {
	cmd := exec2.Command(name, arg...)
	println(cmd.String())

	out, err := cmd.Output()
	println(string(out))
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			message := string(exitError.Stderr)
			println(message)
			return nil, errors.New(message)
		}
		return nil, err
	}

	return out, nil
}
