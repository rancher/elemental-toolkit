// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package plugins

import (
	"path/filepath"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gith "github.com/go-git/go-git/v5/plumbing/transport/http"
	ssh2 "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	"github.com/pkg/errors"
	"github.com/twpayne/go-vfs"
	"golang.org/x/crypto/ssh"
)

func Git(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if s.Git.URL == "" {
		return nil
	}

	branch := "master"
	if s.Git.Branch != "" {
		branch = s.Git.Branch
	}

	gitconfig := s.Git
	path, err := fs.RawPath(s.Git.Path)
	if err != nil {
		return err
	}
	l.Infof("Cloning git repository '%s'", s.Git.URL)

	if utils.Exists(filepath.Join(path, ".git")) {
		l.Info("Repository already exists, updating it")
		// is a git repo, update it
		// We instantiate a new repository targeting the given path (the .git folder)
		r, err := git.PlainOpen(path)
		if err != nil {
			return err
		}

		w, err := r.Worktree()
		if err != nil {
			return err
		}

		err = w.Pull(&git.PullOptions{
			Auth:            authMethod(s),
			SingleBranch:    s.Git.BranchOnly,
			Force:           true,
			InsecureSkipTLS: s.Git.Auth.Insecure,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return err
		}

		err = w.Reset(&git.ResetOptions{
			Commit: plumbing.NewHash(branch),
			Mode:   git.HardReset,
		})

		if err != nil {
			return err
		}
		return nil

	}

	opts := &git.CloneOptions{
		URL:          gitconfig.URL,
		SingleBranch: s.Git.BranchOnly,
	}

	applyOptions(s, opts)

	_, err = git.PlainClone(path, false, opts)
	if err != nil {
		return errors.Wrap(err, "failed cloning repo")
	}
	return nil
}

func authMethod(s schema.Stage) transport.AuthMethod {
	var t transport.AuthMethod

	if s.Git.Auth.Username != "" {
		t = &gith.BasicAuth{Username: s.Git.Auth.Username, Password: s.Git.Auth.Password}
	}

	if s.Git.Auth.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(s.Git.Auth.PrivateKey))
		if err != nil {
			return t
		}

		userName := "git"
		if s.Git.Auth.Username != "" {
			userName = s.Git.Auth.Username
		}
		sshAuth := &ssh2.PublicKeys{
			User:   userName,
			Signer: signer,
		}
		if s.Git.Auth.Insecure {
			sshAuth.HostKeyCallbackHelper = ssh2.HostKeyCallbackHelper{
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}
		}
		if s.Git.Auth.PublicKey != "" {
			key, err := ssh.ParsePublicKey([]byte(s.Git.Auth.PublicKey))
			if err != nil {
				return t
			}
			sshAuth.HostKeyCallbackHelper = ssh2.HostKeyCallbackHelper{
				HostKeyCallback: ssh.FixedHostKey(key),
			}
		}

		t = sshAuth
	}
	return t
}

func applyOptions(s schema.Stage, g *git.CloneOptions) {

	g.Auth = authMethod(s)

	if s.Git.Branch != "" {
		g.ReferenceName = plumbing.NewBranchReferenceName(s.Git.Branch)
	}
	if s.Git.BranchOnly {
		g.SingleBranch = true
	}
}
