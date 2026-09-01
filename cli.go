package lib

import (
	"encoding/json"
	"fmt"
	"os"
)

type Dependencies struct {
	ConfigFiles []string `json:"configFiles"`
}

func HandleCommonPluginArgs(args []string, version string, configFiles []string) {
	if len(os.Args) <= 1 {
		return
	}

	switch os.Args[1] {
	case "--version", "-v", "version", "v":
		fmt.Print(version)
		return
	case "--dependencies", "-d", "dependencies", "d":
		r := Dependencies{
			ConfigFiles: configFiles,
		}

		b, err := json.Marshal(r)
		if err != nil {
			fmt.Printf("Error encoding dependencies: %s", err.Error())
			return
		}

		fmt.Print(string(b))
		return
	default:
		return
	}
}
