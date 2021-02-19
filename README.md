cOS is an Adaptable Linux Distribution based on openSUSE Tumbleweed.

cOS is:
- Immutable distribution
- Cloud-init driven
- Based on systemd
- Built and upgraded from containers - (single image OS)(https://hub.docker.com/r/raccos/releases-amd64/)
- OTA updates

# Description

cOS is an Immutable Linux Distro built from Docker containers, and completely hosted on Docker registries. The build process results in a single Docker image used to deliver regular upgrades in OTA approach.

cOS supports different release channels, all the final images used are tagged and pushed regularly [to DockerHub](https://hub.docker.com/r/raccos/releases-amd64/) and can be pulled for inspection from the Hub as well as are exactly the same images used during upgrades.

## Installation

Once booted, run `cos-installer <device>` to start the installation process. Run `cos-installer` to see the options.


## Upgrades / reset:

cOS supports A/B seamless upgrades. To upgrade the system, just run `cos-upgrade` and reboot.

cOS during installation sets two partitions:
- `COS_ACTIVE`: Where `cOS` typically boots from
- `COS_PASSIVE`: Where `cOS` boots from recovery

Those are used by the upgrade mechanism to prepare and install a pristine `cOS` each time an upgrade is attempted.


## Recovery

The ISO can be also used as a recovery medium: type `cos-upgrade` from a LiveCD. It will attempt to reset the state of the active partition.

## File system layout

- persistent `/usr/local`
- ephemeral `/etc`
- persistent oem partition `/oem`
- immutable `/` - read-only `/usr`

# Cloud-init configuration

By default cOS reads and executes cloud-init files present in `/usr/local/cloud-config` and `/oem`. It is also possible to run cloud-init file from boot cmdline by using  the `cos.setup=..` option. 

This is the prefered way to make persistent changes into `cOS`.

## Persistent changes

cOS is immutable, and by default creates a `COS_PERSISTENT` partition to keep the changes between upgrades. When booting into `cOS`, `/usr/local` is mounted to that partition, and cloud-init style files are read and executed during boot stages.

For example, if you want to change `/etc/issue` of the system persistently, you can create `/usr/local/cloud-config/90_after_install.yaml` with the following content:

```yaml
# Execute this cloud-init before switch root:
stages:
    initramfs.after:
        - name: "After install"
        files:
        - path: /etc/issue
            content: |
                    Welcome, have fun!
            permissions: 0644
            owner: 0
            group: 0
EOF
```

## OEM

To make it part of the oem setup, you can install the custom cloud-init file during install with `--config` to `cos-installer` or add it manually to the `/oem` folder.

### Available stages

## Issues

See the [Releases](https://github.com/mudler/cOS/projects/1) GitHub project for a short-term Roadmap

## Links

- [Development notes](/docs/dev.md)