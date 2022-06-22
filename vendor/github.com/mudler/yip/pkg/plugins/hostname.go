package plugins

import (
	"bufio"
	"fmt"
	"math/rand"
	"strings"
	"syscall"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	uuid "github.com/satori/go.uuid"
	"github.com/twpayne/go-vfs"
)

const localHost = "127.0.0.1"

func Hostname(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	hostname := s.Hostname
	if hostname == "" {
		return nil
	}

	// Template the input string with random generated strings and UUID.
	// Those can be used to e.g. generate random node names based on patterns "foo-{{.UUID}}"
	rand.Seed(time.Now().UnixNano())

	id, _ := machineid.ID()
	myuuid := uuid.NewV4()
	tmpl, err := utils.TemplatedString(hostname,
		struct {
			UUID      string
			Random    string
			MachineID string
		}{
			UUID:      myuuid.String(),
			MachineID: id,
			Random:    utils.RandomString(32),
		},
	)
	if err != nil {
		return err
	}

	if err := syscall.Sethostname([]byte(tmpl)); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := SystemHostname(tmpl, fs); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := UpdateHostsFile(tmpl, fs); err != nil {
		errs = multierror.Append(errs, err)
	}
	return errs
}

func UpdateHostsFile(hostname string, fs vfs.FS) error {
	hosts, err := fs.Open("/etc/hosts")
	if err != nil {
		return err
	}
	defer hosts.Close()

	lines := bufio.NewScanner(hosts)
	content := ""
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == localHost {
			content += fmt.Sprintf("%s localhost %s\n", localHost, hostname)
			continue
		}
		content += line + "\n"
	}
	return fs.WriteFile("/etc/hosts", []byte(content), 0600)
}

func SystemHostname(hostname string, fs vfs.FS) error {
	return fs.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0644)
}
