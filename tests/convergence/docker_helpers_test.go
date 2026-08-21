package convergence

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func composeFilePath(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine convergence test path")
	}

	return filepath.Clean(
		filepath.Join(
			filepath.Dir(currentFile),
			"..",
			"..",
			"docker-compose.yml",
		),
	)
}

func runDockerCompose(
	t *testing.T,
	arguments ...string,
) {
	t.Helper()

	commandArguments := []string{
		"compose",
		"-f",
		composeFilePath(t),
	}

	commandArguments = append(
		commandArguments,
		arguments...,
	)

	command := exec.Command(
		"docker",
		commandArguments...,
	)

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"docker compose %v failed: %v\n%s",
			arguments,
			err,
			string(output),
		)
	}
}
