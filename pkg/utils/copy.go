package utils

import (
	"fmt"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/zloylos/grsync"
	"net/http"
	"os"
	"strings"
	"time"
)

func DoCopy(config *v1.RunConfig) error {
	fmt.Printf("Copying cOS..\n")
	// Make sure the values have a / at the end.
	var source, target string
	if strings.HasSuffix(config.Source, "/") == false {
		source = fmt.Sprintf("%s/", config.Source)
	} else { source = config.Source }

	if strings.HasSuffix(config.Target, "/") == false {
		target = fmt.Sprintf("%s/", config.Target)
	} else { target = config.Target }

	// 1 - rsync all the system from source to target
	task := grsync.NewTask(
		source,
		target,
		grsync.RsyncOptions{
			Quiet: false,
			Archive: true,
			XAttrs: true,
			ACLs: true,
			Exclude: []string{"mnt", "proc", "sys", "dev", "tmp"},
		},
	)
	go func() {
		for {
			state := task.State()
			fmt.Printf(
				"progress: %.2f / rem. %d / tot. %d / sp. %s \n",
				state.Progress,
				state.Remain,
				state.Total,
				state.Speed,
			)
			<- time.After(time.Second)
		}
	}()
	if err := task.Run(); err != nil {
		return err
	}
	// 2 - if we got a cloud config file get it and store in the target
	if config.CloudInit != "" {
		client := &http.Client{}
		customConfig := fmt.Sprintf("%s/oem/99_custom.yaml", config.Target)

		if err :=
			GetUrl(client, config.CloudInit, customConfig); err != nil {
			return err
		}

		if err := os.Chmod(customConfig, 0600); err != nil {
			return err
		}
	}
	return nil
}
