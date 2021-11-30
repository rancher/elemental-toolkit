This package ships the `immutable-rootfs` dracut module responsible of mounting the root tree during 
boot time with the immutable specific setup. The immutability concept refers
to read only root (`/`) system. To ensure the linux OS is still functional
certain paths or areas are required to be writable, in those cases an
ephemeral overaly tmpfs is set in place. Additionaly, the immutable rootfs
module can also mount a custom list of device blocks with read write
permissions, those are mostly devoted to store persistent data.

The dracut module is mostly configured via kernel command line parameters or
via the `/run/cos/cos-layout.env` environment file.

These are the read write paths the module mounts as part of the overlay
ephemeral tmpfs: `/etc`, `/root`, `/home`, `/opt`, `/srv`, `/usr/local`
and `/var`.

These paths will be all ephemeral unless there is a block device configured
to be mounted in the same path.

It is important to remark all the immutable root configuration is applied
in initrd before switching root and after `rootfs` cloud-init stage but
before `initramfs` stage. So immutable rootfs configuration via cloud-init
using the `/run/cos/cos-layout.env` file is only effective if called in any
of the `rootfs.before`, `rootfs` or `rootfs.after` cloud-init stages.

## Kernel configuraton paramters

The immutable rootfs can be configured witht he following kernel parameters:

* `cos-img/filename=<imgfile>`: This is one of the main parameters, it defines
  the location of the image file to boot from.

* `rd.cos.overlay=tmpfs:<size>`: This defines the size of the tmpfs used for
  the ephemeral overlayfs. It can be expressed in MiB or as a % of the available
  memory. Defaults to `rd.cos.overlay=tmpfs:20%` if not present.

* `rd.cos.overlay=LABEL=<vol_label>`: Optionally and mostly for debugging
  purposes the overlayfs can be mounted on top of a persistent block device.
  Block devices can be expressed by LABEL (`LABEL=<blk_label>`) or by UUID
  (`UUID=<blk_uuid>`)

* `rd.cos.mount=LABEL:<blk_label>:<mountpoint>`: This option defines a
  persistent block device and its mountpoint. Block devices can also be
  defined by UUID (`UUID=<blk_uuid>:<mountpoint>`). This option can be passed
  multiple times.

* `rd.cos.oemtimeout=<seconds>`: cOS by default assumes the existence of a
  persistent block device labelled `COS_OEM` which is used to keep some
  configuration data (mostly cloud-init files). The immutable rootfs tries
  to mount this device at very early stages of the boot even before applying
  the immutable rootfs configs. It done this way to enable to configure the
  immutable rootfs module within the cloud-init files. As the `COS_OEM` device
  might not be always present the boot process just continues without failing
  after a certain timeout. This option configures such a timeout. Defaults to
  10s.

* `rd.cos.debugrw`: This is a boolean option, true if present, false if not.
  This option sets the root image to be mounted as a writable device. Note this
  completely breaks the concept of an immutable root. This is helpful for
  debugging or testing purposes, so changes persist across reboots.

* `rd.cos.disable`: This is a boolean option, true if present, false if not.
  It disables the execution of any immutable rootfs module logic at boot.

### Configuration with an environment file

The immutable rootfs can be configured with the `/run/cos/cos-layout.env`
environment file. It is important to note that all the immutable root
configuration is applied in initrd before switching root and after
`rootfs` cloud-init stage but before `initramfs` stage. So immutable rootfs
configuration via cloud-init using the `/run/cos/cos-layout.env` file is
only effective if called in any of the `rootfs.before`, `rootfs` or
`rootfs.after` cloud-init stages.


In the environment file few options are available:


* `VOLUMES=LABEL=<blk_label>:<mountpoint>`: This variable expects a block device
  and it mountpoint pair space separated list. The default cOS configuration is:

  `VOLUMES="LABEL=COS_OEM:/oem LABEL=COS_PERSISTENT:/usr/local"`
  
* `OVERLAY`: It defines the underlaying device for the overlayfs as in
  `rd.cos.overlay=` kernel parameter.

* `DEBUGRW=true`: Sets the root (`/`) to be mounted with read/write permissions.

* `MERGE=true`: Sets makes the `VOLUMES` values to be merged with any other
  volume that might have been defined in the kernel command line. The merging
  criteria is simple: any overlapping volume is overwritten all others are
  appended to whatever was already defined as a kernel parameter. If not
  defined defaults to `true`.

* `RW_PATHS`: This is a space separated list of paths. These are the paths
  that will be used for the ephemeral overlayfs. These are the paths that
  will be mounted as overlay on top of the `OVERLAY` (or `rd.cos.overlay`)
  device. Default value is:

  `RW_PATHS="/etc /root /home /opt /srv /usr/local /var"`
  **Note**: as those paths are overlayed with an ephemeral mount (`tmpfs`), 
            additional data wrote on those location won't be available on subsequent boots.

* `PERSISTENT_STATE_TARGET`: This is the folder where the persistent state data
  will be stored, if any. Default value is `/usr/local/.state`.

* `PERSISTENT_STATE_PATHS`: This is a space separated list of paths. These are
  the paths that will become writable and store its data inside
  `PERSISTENT_STATE_TARGET`. By default this variable is empty, which means
  no persistent state area is created or used.

  **Note**: The specified paths needs either to exist or be located in an area 
            which is writeable ( for example, inside locations specified with `RW_PATHS`).
            The dracut module will attempt to create non-existant directories, 
            but might fail if the mountpoint where are located is read-only.

* `PERSISTENT_STATE_BIND="true|false"`: When this variable is set to true
  the persistent state paths are bind mounted (instead of using overlayfs)
  after being mirrored with the original content. By default this variable is
  set to `false`.

Note that persistent state are is setup once the ephemeral paths and persistent
volumes are mounted. Persistent state paths can't be an already existing mount
point. If the persistent state requires any of the paths that are part of the
ephemeral area by default, then `RW_PATHS` needs to be defined to avoid
overlapping paths.

For exmaple a common cOS configuration can be expressed as part of the
cloud-init configuration as follows:

```yaml
name: example
stage:
  rootfs:
    - name: "Layout configuration"
      environment_file: /run/cos/cos-layout.env
      environment:
        VOLUMES: "LABEL=COS_OEM:/oem LABEL=COS_PERSISTENT:/usr/local"
        OVERLAY: "tmpfs:25%"
```
