package main

// FROM Slightly adapted from genuinetools/img worker

import (
	"context"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/namespaces"
	"github.com/genuinetools/img/types"
	"github.com/moby/buildkit/control"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/moby/buildkit/worker/base"
	"github.com/pkg/errors"
)

// Client holds the information for the client we will use for communicating
// with the buildkit controller.
type Client struct {
	backend   string
	localDirs map[string]string
	root      string

	sessionManager *session.Manager
	controller     *control.Controller
	opts           *base.WorkerOpt

	sess *session.Session
	ctx  context.Context
}

// New returns a new client for communicating with the buildkit controller.
func New(root string) (*Client, error) {
	// Native backend is fine, our images have just one layer. No need to depend on anything
	backend := types.NativeBackend

	// Create the root/
	root = filepath.Join(root, "runc", backend)
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	c := &Client{
		backend:   types.NativeBackend,
		root:      root,
		localDirs: nil,
	}

	if err := c.prepare(); err != nil {
		return nil, errors.Wrapf(err, "failed preparing client")
	}

	// Create the start of the client.
	return c, nil
}

func (c *Client) Close() {
	c.sess.Close()
}

func (c *Client) prepare() error {
	ctx := appcontext.Context()
	sess, sessDialer, err := c.Session(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed creating Session")
	}
	ctx = session.NewContext(ctx, sess.ID())
	ctx = namespaces.WithNamespace(ctx, "buildkit")

	c.ctx = ctx
	c.sess = sess

	go func() {
		sess.Run(ctx, sessDialer)
	}()
	return nil
}
