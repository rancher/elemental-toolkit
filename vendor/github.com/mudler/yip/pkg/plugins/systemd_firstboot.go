package plugins

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
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
