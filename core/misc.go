package core

import (
	"log"
	"os"
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
