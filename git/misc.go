package git

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func Must[K any](k K, err error) K {
	if err != nil {
		log.Panic(err)
	}
	return k
}

func FileExists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

func GitExec(format string, args ...any) *exec.Cmd {
	return exec.Command("bash", "-c", "git "+fmt.Sprintf(format, args...))
}
