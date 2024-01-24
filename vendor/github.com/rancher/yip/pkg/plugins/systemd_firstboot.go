package plugins

import (
	"fmt"
	"sort"
	"strings"

	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/yip/pkg/logger"
	"github.com/rancher/yip/pkg/schema"
)

func SystemdFirstboot(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var err error
	var args []string
	var out string

	for k, v := range s.SystemdFirstBoot {
		if v == "true" {
			args = append(args, fmt.Sprintf("--%s", strings.ToLower(k)))
		} else {
			args = append(args, fmt.Sprintf("--%s=%s", strings.ToLower(k), v))
		}
	}

	if len(args) > 0 {
		sort.Strings(args)
		arguments := strings.Join(args, " ")
		l.Debugf("running 'systemd-firstboot' with arguments: %s", arguments)
		out, err = console.Run(fmt.Sprintf("systemd-firstboot %s", arguments))
		l.Debugf("systemd-fristboot output: %s", out)
	}

	return err
}
