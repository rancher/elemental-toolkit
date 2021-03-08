package main

// FROM Slightly adapted from genuinetools/img worker

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/diff/apply"
	"github.com/containerd/containerd/diff/walking"
	ctdmetadata "github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/remotes/docker"
	ctdsnapshot "github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/native"
	"github.com/moby/buildkit/cache/metadata"
	containerdsnapshot "github.com/moby/buildkit/snapshot/containerd"
	"github.com/moby/buildkit/util/binfmt_misc"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/worker/base"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	bolt "go.etcd.io/bbolt"
)

// createWorkerOpt creates a base.WorkerOpt to be used for a new worker.
func (c *Client) createWorkerOpt() (opt base.WorkerOpt, err error) {
	if c.opts != nil {
		return *c.opts, nil
	}

	// Create the metadata store.
	md, err := metadata.NewStore(filepath.Join(c.root, "metadata.db"))
	if err != nil {
		return opt, err
	}

	snapshotRoot := filepath.Join(c.root, "snapshots")

	s, err := native.NewSnapshotter(snapshotRoot)
	if err != nil {
		return opt, fmt.Errorf("creating %s snapshotter failed: %v", c.backend, err)
	}

	// Create the content store locally.
	contentStore, err := local.NewStore(filepath.Join(c.root, "content"))
	if err != nil {
		return opt, err
	}

	// Open the bolt database for metadata.
	db, err := bolt.Open(filepath.Join(c.root, "containerdmeta.db"), 0644, nil)
	if err != nil {
		return opt, err
	}

	// Create the new database for metadata.
	mdb := ctdmetadata.NewDB(db, contentStore, map[string]ctdsnapshot.Snapshotter{
		c.backend: s,
	})
	if err := mdb.Init(context.TODO()); err != nil {
		return opt, err
	}

	// Create the image store.
	imageStore := ctdmetadata.NewImageStore(mdb)

	contentStore = containerdsnapshot.NewContentStore(mdb.ContentStore(), "buildkit")

	id, err := base.ID(c.root)
	if err != nil {
		return opt, err
	}

	xlabels := base.Labels("oci", c.backend)

	var supportedPlatforms []specs.Platform
	for _, p := range binfmt_misc.SupportedPlatforms(false) {
		parsed, err := platforms.Parse(p)
		if err != nil {
			return opt, err
		}
		supportedPlatforms = append(supportedPlatforms, platforms.Normalize(parsed))
	}

	opt = base.WorkerOpt{
		ID:             id,
		Labels:         xlabels,
		MetadataStore:  md,
		Snapshotter:    containerdsnapshot.NewSnapshotter(c.backend, mdb.Snapshotter(c.backend), "buildkit", nil),
		ContentStore:   contentStore,
		Applier:        apply.NewFileSystemApplier(contentStore),
		Differ:         walking.NewWalkingDiff(contentStore),
		ImageStore:     imageStore,
		Platforms:      supportedPlatforms,
		RegistryHosts:  docker.ConfigureDefaultRegistries(),
		LeaseManager:   leaseutil.WithNamespace(ctdmetadata.NewLeaseManager(mdb), "buildkit"),
		GarbageCollect: mdb.GarbageCollect,
	}

	c.opts = &opt

	return opt, err
}
