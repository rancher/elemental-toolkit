package plugins

import (
	"bytes"
	"os"
	"strings"

	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/yip/pkg/logger"
	"github.com/rancher/yip/pkg/schema"
)

func DNS(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.Dns.Nameservers) != 0 {
		return applyDNS(s)
	}
	return nil
}

func applyDNS(s schema.Stage) error {
	path := s.Dns.Path
	if path == "" {
		path = "/etc/resolv.conf"
	}
	return writeResolvConf(path, s.Dns.Nameservers, s.Dns.DnsSearch, s.Dns.DnsOptions)
}

// Build generates and writes a configuration file to path containing a nameserver
// entry for every element in nameservers, a "search" entry for every element in
// dnsSearch, and an "options" entry for every element in dnsOptions. It returns
// a File containing the generated content and its (sha256) hash.
//
// Note that the resolv.conf file is written, but the hash file is not.
// Duplicated from https://github.com/moby/moby/blob/9d07820b221db010bf1bdc26ca904468804ca712/libnetwork/resolvconf/resolvconf.go#L131
// Because of use of internal module.
func writeResolvConf(path string, nameservers, dnsSearch, dnsOptions []string) error {
	content := bytes.NewBuffer(nil)
	if len(dnsSearch) > 0 {
		if searchString := strings.Join(dnsSearch, " "); strings.Trim(searchString, " ") != "." {
			if _, err := content.WriteString("search " + searchString + "\n"); err != nil {
				return err
			}
		}
	}
	for _, dns := range nameservers {
		if _, err := content.WriteString("nameserver " + dns + "\n"); err != nil {
			return err
		}
	}
	if len(dnsOptions) > 0 {
		if optsString := strings.Join(dnsOptions, " "); strings.Trim(optsString, " ") != "" {
			if _, err := content.WriteString("options " + optsString + "\n"); err != nil {
				return err
			}
		}
	}

	return os.WriteFile(path, content.Bytes(), 0o644)
}
