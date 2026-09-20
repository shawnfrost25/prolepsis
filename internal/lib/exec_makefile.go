package lib

import (
	"os"
	"os/exec"
)

func RunMake(PathToMakefile string, target ...string) error {
	cmd := exec.Command("make", target...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = PathToMakefile

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
