package plugins

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/user"
	"path"
	"strings"
	"sync"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/pkg/errors"
	prv "github.com/rancher-sandbox/linuxkit/providers"
	"github.com/twpayne/go-vfs"
)

func unique(stringSlice []string) []string {
	keys := make(map[string]bool)
	var list []string

	// If the key(values of the slice) is not equal
	// to the already present value in new slice (list)
	// then we append it. else we jump on another element.
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func DataSources(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var AvailableProviders = []prv.Provider{}
	var CdromProviders = []prv.Provider{}

	if s.DataSources.Providers == nil || len(s.DataSources.Providers) == 0 {
		return nil
	}
	// Avoid duplication
	uniqueProviders := unique(s.DataSources.Providers)

	for _, dSProviders := range uniqueProviders {
		switch {
		case dSProviders == "aws":
			AvailableProviders = append(AvailableProviders, prv.NewAWS())
		case dSProviders == "azure":
			AvailableProviders = append(AvailableProviders, prv.NewAzure())
		case dSProviders == "gcp":
			AvailableProviders = append(AvailableProviders, prv.NewGCP())
		case dSProviders == "hetzner":
			AvailableProviders = append(AvailableProviders, prv.NewHetzner())
		case dSProviders == "openstack":
			AvailableProviders = append(AvailableProviders, prv.NewOpenstack())
		case dSProviders == "packet":
			AvailableProviders = append(AvailableProviders, prv.NewPacket())
		case dSProviders == "scaleway":
			AvailableProviders = append(AvailableProviders, prv.NewScaleway())
		case dSProviders == "vultr":
			AvailableProviders = append(AvailableProviders, prv.NewVultr())
		case dSProviders == "digitalocean":
			AvailableProviders = append(AvailableProviders, prv.NewDigitalOcean())
		case dSProviders == "metaldata":
			AvailableProviders = append(AvailableProviders, prv.NewMetalData())
		case dSProviders == "vmware":
			AvailableProviders = append(AvailableProviders, prv.NewVMware())
		case dSProviders == "cdrom":
			CdromProviders = append(CdromProviders, prv.ListCDROMs()...)
		case dSProviders == "config-drive":
			CdromProviders = append(CdromProviders, prv.ListConfigDrives()...)
		case dSProviders == "file" && s.DataSources.Path != "":
			AvailableProviders = append(AvailableProviders, prv.FileProvider(s.DataSources.Path))
		}
	}

	if err := EnsureDirectories(l, schema.Stage{
		Directories: []schema.Directory{
			{
				Path:        prv.ConfigPath,
				Permissions: 0755,
				Owner:       os.Getuid(),
				Group:       os.Getgid(),
			},
		},
	}, fs, console); err != nil {
		return err
	}

	var p prv.Provider
	var userdata []byte
	var err error

	if len(AvailableProviders) > 0 {
		l.Debugf("Full provider list: %s", AvailableProviders)
	}

	// Run first cdrom providers
	for _, p = range CdromProviders {
		l.Debugf("Starting provider %s", p.String())
		if p.Probe() {
			userdata, err = p.Extract()
			if err != nil {
				l.Warnf("Failed extracting data from %s provider: %s", p.String(), err.Error())
			}
			l.Debugf("Found userdata from %s", p.String())
			break
		}
		l.Debugf("Didnt found userdata from %s", p.String())
	}

	// If we haven't found the userdata on cdroms, continue with the other datasources
	if userdata == nil {
		userdataDone := make(chan []byte, len(uniqueProviders))
		var wg sync.WaitGroup
		for _, p = range AvailableProviders {
			l.Debugf("Starting provider %s", p.String())
			prov := p
			wg.Add(1)
			go func(l logger.Interface, p prv.Provider) {
				defer wg.Done()
				if p.Probe() {
					userdata, err := p.Extract()
					if err != nil {
						l.Warnf("Failed extracting data from %s provider: %s", p.String(), err.Error())
						return
					}
					userdataDone <- userdata
					l.Debugf("Found userdata from %s", p.String())
					return
				}
				l.Debugf("Didnt found userdata from %s", p.String())
				return
			}(l, prov)
		}

		// wait until all have finished
		wg.Wait()

		// Try to get the userdata from the channel
		select {
		case v, ok := <-userdataDone:
			if ok { // check if it was ok, otherwise the channel can be closed and dragons happen
				userdata = v
			}
		default: // no userdata :(
		}
	}

	if userdata == nil {
		return fmt.Errorf("no metadata/userdata found")
	}

	basePath := prv.ConfigPath
	if s.DataSources.Path != "" && s.DataSources.Path != p.String() {
		basePath = s.DataSources.Path
	}

	if userdata != nil {
		if err := processUserData(l, basePath, userdata, fs, console); err != nil {
			return err
		}
	}

	//Apply the hostname if the provider extracted a hostname file
	if _, err := fs.Stat(path.Join(prv.ConfigPath, prv.Hostname)); err == nil {
		if err := processHostnameFile(l, fs, console); err != nil {
			return err
		}
	}

	//Apply the authorized_keys if the provider extracted a ssh/authorized_keys file
	if _, err := fs.Stat(path.Join(prv.ConfigPath, prv.SSH, authorizedFile)); err == nil {
		if err := processSSHFile(l, fs, console); err != nil {
			return err
		}
	}
	return nil
}

func processHostnameFile(l logger.Interface, fs vfs.FS, console Console) error {
	hostname, err := fs.ReadFile(path.Join(prv.ConfigPath, prv.Hostname))
	if err != nil {
		return err
	}

	return Hostname(l, schema.Stage{Hostname: string(hostname)}, fs, console)
}

func processSSHFile(l logger.Interface, fs vfs.FS, console Console) error {
	auth_keys, err := fs.ReadFile(path.Join(prv.ConfigPath, prv.SSH, authorizedFile))
	if err != nil {
		return err
	}
	var keys []string
	var line string
	usr, err := user.Current()
	if err != nil {
		return errors.Wrap(err, "could not get current user info")
	}

	scanner := bufio.NewScanner(strings.NewReader(string(auth_keys)))
	for scanner.Scan() {
		line = strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			keys = append(keys, line)
		}
	}
	return SSH(l, schema.Stage{SSHKeys: map[string][]string{usr.Username: keys}}, fs, console)
}

// DecodeMultipartVmware will try to decode the user-data from VMWARE provider as it returns a
// multipart/mixed data instead of the simple cloud-config
func DecodeMultipartVmware(data []byte) []byte {
	reader := bytes.NewReader(data)
	r := multipart.NewReader(reader, "MIMEBOUNDARY")
	for {
		part, err := r.NextPart()
		if err != nil {
			break
		}
		// Only parse cloud-config and the first instance
		if part.Header.Get("Content-Type") == "text/cloud-config" {
			d, err := io.ReadAll(part)
			if err != nil {
				break
			}
			return d
		}
	}
	return data
}

// If userdata can be parsed as a yipConfig file will create a <basePath>/userdata.yaml file
func processUserData(l logger.Interface, basePath string, data []byte, fs vfs.FS, console Console) error {
	// VMWARE provider returns a multipart/mixed data, so try first to parse that
	// If we fail to parse it it will return the original data unchanged
	data = DecodeMultipartVmware(data)
	dataS := string(data)

	// always save unprocessed data to "userdata"
	if err := writeToFile(l, path.Join(basePath, "userdata"), dataS, 0644, fs, console); err != nil {
		return err
	}

	if _, err := schema.Load(dataS, fs, nil, nil); err == nil {
		return writeToFile(l, path.Join(basePath, "userdata.yaml"), dataS, 0644, fs, console)
	}

	scanner := bufio.NewScanner(strings.NewReader(dataS))
	scanner.Scan()
	if strings.HasPrefix(scanner.Text(), "#!") {
		l.Infof("Found shebang '%s' excuting user-data as a script\n", scanner.Text())
		script := path.Join(basePath, "userdata")
		err := writeToFile(l, script, dataS, 0744, fs, console)
		if err != nil {
			return err
		}
		l.Infof("Running %s\n", script)
		out, err := console.Run(script)
		if err != nil {
			return err
		}
		l.Info(out)
		return nil
	}

	l.Info("Could not unmarshall userdata and no shebang detected")
	return nil
}

func writeToFile(l logger.Interface, filename string, content string, perm uint32, fs vfs.FS, console Console) error {
	err := EnsureFiles(l, schema.Stage{
		Files: []schema.File{
			{
				Path:        filename,
				Content:     content,
				Permissions: perm,
				Owner:       os.Getuid(),
				Group:       os.Getgid(),
			},
		},
	}, fs, console)
	if err != nil {
		return errors.Wrap(err, "could not write file")
	}
	return nil
}
