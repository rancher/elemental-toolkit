# cOS

**cOS** is an Immutable Adaptable Linux Distribution designed to reduce the maintenance surface exposed to the user. It is cloud-init driven and also designed to be adaptive-first, allowing easily to build changes on top.

## In a nutshell

cOS is built from Docker containers, and completely hosted on Docker registries. The build process results in a single Docker image used to deliver regular upgrades in OTA approach.

cOS supports different release channels, all the final images used are tagged and pushed regularly [to DockerHub](https://hub.docker.com/r/raccos/releases-amd64/) and can be pulled for inspection from the registry as well. 
Those are exactly the same images used during upgrades.

For example, if you want to see locally what's in cOS 0.4.16, you can:

```bash
$ docker run -ti raccos/releases-amd64:cos-system-0.4.16
```

## Design goals:

- Immutable distribution
- Cloud-init driven
- Based on systemd
- Built and upgraded from containers - It is a [single image OS](https://hub.docker.com/r/raccos/releases-amd64/)!
- OTA updates
- Easy to customize

## Quick start

Download the ISO from the latest [release]() and run it in your virtualization hypervisor of choice (baremetal, too). You can login with the user `root` and `cos`. That's a live ISO and no changes will be persisted.

Run `cos-installer <device>` to start the installation process. Remove the ISO and reboot.

_Note_: `cos-installer` supports other options as well. Run `cos-installer --help` to see a complete help.

## Upgrades:

To upgrade the system, just run `cos-upgrade` and reboot.

cOS during installation sets two `.img` images files in the `COS_STATE` partition:
- `/cOS/active.img` labeled `COS_ACTIVE`: Where `cOS` typically boots from
- `/cOS/passive.img` labeled `COS_PASSIVE`: Where `cOS` boots for fallback

Those are used by the upgrade mechanism to prepare and install a pristine `cOS` each time an upgrade is attempted.

## Reset state

### Recovery partition

cOS can be recovered anytime from the `cOS recovery` partition by running `cos-reset`. This will regenerate the bootloader and the images in `COS_STATE` by using the recovery image created during installation.

The recovery partition can also be upgraded by running `UPGRADE_RECOVERY=true cos-upgrade` in the standard partitions used for boot.

### From ISO
The ISO can be also used as a recovery medium: type `cos-upgrade` from a LiveCD. It will then try to upgrade the image of the active partition installed in the system.

## File system layout

As cOS is an immutable distribution, the file system layout is a core aspect. A running `cOS` will look as follows:

```
/usr/local - persistent (COS_PERSISTENT)
/oem - persistent (COS_OEM)
/etc - ephemeral
/usr - read only
/ immutable
```

Any changes that are not specified by cloud-init are not persisting across reboots. 

## Persistent changes

By default cOS reads and executes cloud-init files in (lexicopgrahic) sequence present in `/usr/local/cloud-config` and `/oem` during boot. It is also possible to run cloud-init file in a different location from boot cmdline by using  the `cos.setup=..` option. 

For example, if you want to change `/etc/issue` of the system persistently, you can create `/usr/local/cloud-config/90_after_install.yaml` with the following content:

```yaml
# The following is executed before fs is setted up:
stages:
    fs:
        - name: "After install"
          files:
          - path: /etc/issue
            content: |
                    Welcome, have fun!
            permissions: 0644
            owner: 0
            group: 0
          systemctl:
            disable:
            - wicked
        - name: "After install (second step)"
          files:
          - path: /etc/motd
            content: |
                    Welcome, have more fun!
            permissions: 0644
            owner: 0
            group: 0
```

For more examples, `/oem` contains files used to configure on boot a pristine `cOS`. Mind to not edit those directly, but copy them or apply local changes to `/usr/local/cloud-config`. See the OEM section below.

### Available stages

Cloud-init files are applied in 5 different phases: `boot`, `network`, `fs`, `initramfs` and `reconcile`. All the available cloud-init keywords can be used in each stage. Additionally, it's possible also to hook before or after a stage has run, each one has a specific stage which is possible to run steps: `boot.after`, `network.before`, `fs.after` etc.

#### initramfs

This is the earliest stage, running before switching root. Here you can apply radical changes to the booting setup of `cOS`.

#### boot

This stage is executed after initramfs has switched root, during the `systemd` bootup process.

#### fs

This stage is executed when fs is mounted and is guaranteed to have access to `COS_STATE` and `COS_PERSISTENT`.

#### network

This stage is executed when network is available

#### reconcile

This stage is executed `5m` after boot and periodically each `60m`.

## OEM customizations

It is possible to install a custom cloud-init file during install with `--config` to `cos-installer` or, it's possible to add more files manually to the `/oem` folder after installation.

Inside the `/oem` folders there are also files being shipped by `cOS` during upgrades. If you wish to add persistent changes and write them to the OEM folder, be sure to not clash with `cOS` ones, by prefixing your files with numbers starting from `90` e.g. `90_custom.yaml`, `91_custom_after.yam` ...

## Configuration reference

Below is a reference of all keys available in the cloud-init style files.

```yaml
stages:
   # "network" is the stage
   network:
     - files:
        - path: /tmp/foo
          content: |
                    test
          permissions: 0777
          owner: 1000
          group: 100
       commands:
        - echo "test"
       modules:
       - nvidia
       environment:
         FOO: "bar"
       systctl:
         debug.exception-trace: "0"
       hostname: "foo"
       systemctl:
         enable:
         - foo
         disable:
         - bar
         start:
         - baz
         mask:
         - foobar
       authorized_keys:
          user:
          - "github:mudler"
          - "ssh-rsa ...."
       dns:
         path: /etc/resolv.conf
         nameservers:
         - 8.8.8.8
       ensure_entities:
       -  path: /etc/passwd
          entity: |
                  kind: "user"
                  username: "foo"
                  password: "pass"
                  uid: 0
                  gid: 0
                  info: "Foo!"
                  homedir: "/home/foo"
                  shell: "/bin/bash"
       delete_entities:
       -  path: /etc/passwd
          entity: |
                  kind: "user"
                  username: "foo"
                  password: "pass"
                  uid: 0
                  gid: 0
                  info: "Foo!"
                  homedir: "/home/foo"
                  shell: "/bin/bash"
```


### `stages.<stageID>.[<stepN>].name`

A description of the stage step. Used only when printing output to console.

### `stages.<stageID>.[<stepN>].files`

A list of files to write to disk.

```yaml
stages:
   default:
     - files:
        - path: /tmp/bar
          content: |
                    #!/bin/sh
                    echo "test"
          permissions: 0777
          owner: 1000
          group: 100
```

### `stages.<stageID>.[<stepN>].directories`

A list of directories to be created on disk. Runs before `files`.

```yaml
stages:
   default:
     - name: "Setup folders"
       directories: 
       - path: "/etc/foo"
         permissions: 0600
         owner: 0
         group: 0
```

### `stages.<stageID>.[<stepN>].dns`

A way to configure the `/etc/resolv.conf` file.

```yaml
stages:
   default:
     - name: "Setup dns"
       dns: 
         nameservers:
         - 8.8.8.8
         - 1.1.1.1
         search:
         - foo.bar
         options:
         - ..
         path: "/etc/resolv.conf.bak"
```
### `stages.<stageID>.[<stepN>].hostname`

A string representing the machine hostname. It sets it in the running system, updates `/etc/hostname` and adds the new hostname to `/etc/hosts`.

```yaml
stages:
   default:
     - name: "Setup hostname"
       hostname: "foo"
```
### `stages.<stageID>.[<stepN>].sysctl`

Kernel configuration. It sets `/proc/sys/<key>` accordingly, similarly to `sysctl`.

```yaml
stages:
   default:
     - name: "Setup exception trace"
       systctl:
         debug.exception-trace: "0"
```

### `stages.<stageID>.[<stepN>].authorized_keys`

A list of SSH authorized keys that should be added for each user. 
SSH keys can be obtained from GitHub user accounts by using the format github:${USERNAME},  similarly for Gitlab with gitlab:${USERNAME}.

```yaml
stages:
   default:
     - name: "Setup exception trace"
       authorized_keys:
         mudler:
         - github:mudler
         - ssh-rsa: ...
```

### `stages.<stageID>.[<stepN>].node`

If defined, the node hostname where this stage has to run, otherwise it skips the execution. The node can be also a regexp in the Golang format.

```yaml
stages:
   default:
     - name: "Setup logging"
       node: "bastion"
```

### `stages.<stageID>.[<stepN>].users`

A map of users and password to set. Passwords can be also encrypted.

```yaml
stages:
   default:
     - name: "Setup users"
       users: 
          bastion: "strongpassword"
```

### `stages.<stageID>.[<stepN>].ensure_entities`

A `user` or a `group` in the [entity](https://github.com/mudler/entities) format to be configured in the system

```yaml
stages:
   default:
     - name: "Setup users"
       ensure_entities:
       -  path: /etc/passwd
          entity: |
                  kind: "user"
                  username: "foo"
                  password: "x"
                  uid: 0
                  gid: 0
                  info: "Foo!"
                  homedir: "/home/foo"
                  shell: "/bin/bash"
```
### `stages.<stageID>.[<stepN>].delete_entities`

A `user` or a `group` in the [entity](https://github.com/mudler/entities) format to be pruned from the system

```yaml
stages:
   default:
     - name: "Setup users"
       delete_entities:
       -  path: /etc/passwd
          entity: |
                  kind: "user"
                  username: "foo"
                  password: "x"
                  uid: 0
                  gid: 0
                  info: "Foo!"
                  homedir: "/home/foo"
                  shell: "/bin/bash"
```
### `stages.<stageID>.[<stepN>].modules`

A list of kernel modules to load.

```yaml
stages:
   default:
     - name: "Setup users"
       modules:
       - nvidia
```
### `stages.<stageID>.[<stepN>].systemctl`

A list of systemd services to `enable`, `disable`, `mask` or `start`.

```yaml
stages:
   default:
     - name: "Setup users"
       systemctl:
         enable:
          - systemd-timesyncd
          - cronie
         mask:
          - purge-kernels
         disable:
          - crond
         start:
          - cronie
```
### `stages.<stageID>.[<stepN>].environment`

A map of variables to write in `/etc/environment`, or otherwise specified in `environment_file`

```yaml
stages:
   default:
     - name: "Setup users"
       environment:
         FOO: "bar"
```
### `stages.<stageID>.[<stepN>].environment_file`

A string to specify where to set the environment file

```yaml
stages:
   default:
     - name: "Setup users"
       environment_file: "/home/user/.envrc"
       environment:
         FOO: "bar"
```
### `stages.<stageID>.[<stepN>].timesyncd`

Sets the `systemd-timesyncd` daemon file (`/etc/system/timesyncd.conf`) file accordingly. The documentation for `timesyncd` and all the options can be found [here](https://www.freedesktop.org/software/systemd/man/timesyncd.conf.html).

```yaml
stages:
   default:
     - name: "Setup NTP"
       systemctl:
         enable:
         - systemd-timesyncd
       timesyncd: 
          NTP: "0.pool.org foo.pool.org"
          FallbackNTP: ""
          ...
```

### `stages.<stageID>.[<stepN>].commands`

A list of arbitrary commands to run after file writes and directory creation.

```yaml
stages:
   default:
     - name: "Setup something"
       commands:
         - echo 1 > /bar
```


## Issues

See the [Releases](https://github.com/mudler/cOS/projects/1) GitHub project for a short-term Roadmap

## Links

- [Development notes](/docs/dev.md)

