package plugins

import (
	"github.com/moby/moby/libnetwork/resolvconf"
	"github.com/twpayne/go-vfs"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
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
	_, err := resolvconf.Build(path, s.Dns.Nameservers, s.Dns.DnsSearch, s.Dns.DnsOptions)
	return err
}
