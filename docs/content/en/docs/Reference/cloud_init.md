
---
title: "Cloud-init support"
linkTitle: "Cloud-init support"
weight: 3
date: 2021-01-05
description: >
  Features inherited by cOS derivatives that are also available in the cOS vanilla images
---


Below is a reference of all keys available in the cloud-init style files.

```yaml
stages:
   # "network" is the stage where network is expected to be up
   # It is called internally when network is available from 
   # the cos-setup-network unit.
   network:
     # Here there are a list of 
     # steps to be run in the network stage
     - name: "Some setup happening"
       files:
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
       datasource:
         providers:
         - "aws"
         - "digitalocean"
         path: "/etc/cloud-data"
```

The default cloud-config format is split into *stages* (*initramfs*, *boot*, *network*, *initramfs*, *reconcile*, called generically **STAGE_ID** below) [see also stages](../customizing/stages) that are emitted internally during the various phases by calling `cos-setup STAGE_ID`. 
*steps* (**STEP_NAME** below) defined for each stage are executed in order.

Each cloud-config file is loaded and executed only at the apprioriate stage, this allows further components to emit their own stages at the desired time.

{{% pageinfo %}}
The [cloud-init tool](https://github.com/mudler/yip#readme) can be also run standalone, this helps debugging locally and also during development, you can find separate [releases here](https://github.com/mudler/yip/releases/tag/0.9.6), or just run it with docker:

```bash
cat <<EOF | docker run -i --rm quay.io/costoolkit/releases-green:cos-recovery-0.6.0 yip -s test -
stages:
 test:
 - commands:
   - echo "test"
EOF
```
{{% /pageinfo %}}

_Note_: Each cloud-init option can be either run in *dot notation* ( e.g. `stages.network[0].authorized_keys.user=github:user` ) in the boot args or either can supply a cloud-init URL at boot with the `cos.setup=$URL` parameter.

### Using templates

With Cloud Init support, templates can be used to allow dynamic configuration. More information about templates can be found [here](https://github.com/mudler/yip#node-data-interpolation) and also [here for sprig](http://masterminds.github.io/sprig/) functions.

### Compatibility with Cloud Init format

A subset of the official [cloud-config spec](http://cloudinit.readthedocs.org/en/latest/topics/format.html#cloud-config-data) is implemented. 

If a yaml file starts with `#cloud-config` it is parsed as a standard cloud-init and automatically associated it to the `boot` stage. For example:

```yaml
#cloud-config
growpart:
  mode: auto
  devices: ['/']

users:
- name: "bar"
  passwd: "foo"
  lock_passwd: true
  uid: "1002"
  groups: "users"
  ssh_authorized_keys:
  - faaapploo
ssh_authorized_keys:
  - asdd
runcmd:
- foo
hostname: "bar"
write_files:
- encoding: b64
  content: CiMgVGhpcyBmaWxlIGNvbnRyb2xzIHRoZSBzdGF0ZSBvZiBTRUxpbnV4
  path: /foo/bar
  permissions: "0644"
  owner: "bar"
```

Is executed at boot, by using the standard `cloud-config` format.

### `stages.STAGE_ID.STEP_NAME.name`

A description of the stage step. Used only when printing output to console.

### `stages.STAGE_ID.STEP_NAME.files`

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

### `stages.STAGE_ID.STEP_NAME.directories`

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

### `stages.STAGE_ID.STEP_NAME.dns`

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
### `stages.STAGE_ID.STEP_NAME.hostname`

A string representing the machine hostname. It sets it in the running system, updates `/etc/hostname` and adds the new hostname to `/etc/hosts`.
Templates can be used to allow dynamic configuration. For example in mass-install scenario it could be needed (and easier) to specify hostnames for multiple machines from a single cloud-init config file.

```yaml
stages:
   default:
     - name: "Setup hostname"
       hostname: "node-{{ trunc 4 .MachineID }}"
```
### `stages.STAGE_ID.STEP_NAME.sysctl`

Kernel configuration. It sets `/proc/sys/<key>` accordingly, similarly to `sysctl`.

```yaml
stages:
   default:
     - name: "Setup exception trace"
       systctl:
         debug.exception-trace: "0"
```

### `stages.STAGE_ID.STEP_NAME.authorized_keys`

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

### `stages.STAGE_ID.STEP_NAME.node`

If defined, the node hostname where this stage has to run, otherwise it skips the execution. The node can be also a regexp in the Golang format.

```yaml
stages:
   default:
     - name: "Setup logging"
       node: "bastion"
```

### `stages.STAGE_ID.STEP_NAME.users`

A map of users and user info to set. Passwords can be also encrypted.

The `users` parameter adds or modifies the specified list of users. Each user is an object which consists of the following fields. Each field is optional and of type string unless otherwise noted.
In case the user is already existing, the entry is ignored.

- **name**: Required. Login name of user
- **gecos**: GECOS comment of user
- **passwd**: Hash of the password to use for this user. Unencrypted strings are supported too.
- **homedir**: User's home directory. Defaults to /home/*name*
- **no-create-home**: Boolean. Skip home directory creation.
- **primary-group**: Default group for the user. Defaults to a new group created named after the user.
- **groups**: Add user to these additional groups
- **no-user-group**: Boolean. Skip default group creation.
- **ssh-authorized-keys**: List of public SSH keys to authorize for this user
- **system**: Create the user as a system user. No home directory will be created.
- **no-log-init**: Boolean. Skip initialization of lastlog and faillog databases.
- **shell**: User's login shell.

```yaml
stages:
   default:
     - name: "Setup users"
       users: 
          bastion: 
            passwd: "strongpassword"
            homedir: "/home/foo
```

### `stages.STAGE_ID.STEP_NAME.ensure_entities`

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
### `stages.STAGE_ID.STEP_NAME.delete_entities`

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
### `stages.STAGE_ID.STEP_NAME.modules`

A list of kernel modules to load.

```yaml
stages:
   default:
     - name: "Setup users"
       modules:
       - nvidia
```
### `stages.STAGE_ID.STEP_NAME.systemctl`

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
### `stages.STAGE_ID.STEP_NAME.environment`

A map of variables to write in `/etc/environment`, or otherwise specified in `environment_file`

```yaml
stages:
   default:
     - name: "Setup users"
       environment:
         FOO: "bar"
```
### `stages.STAGE_ID.STEP_NAME.environment_file`

A string to specify where to set the environment file

```yaml
stages:
   default:
     - name: "Setup users"
       environment_file: "/home/user/.envrc"
       environment:
         FOO: "bar"
```
### `stages.STAGE_ID.STEP_NAME.timesyncd`

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

### `stages.STAGE_ID.STEP_NAME.commands`

A list of arbitrary commands to run after file writes and directory creation.

```yaml
stages:
   default:
     - name: "Setup something"
       commands:
         - echo 1 > /bar
```

### `stages.STAGE_ID.STEP_NAME.datasource`

Sets to fetch user data from the specified cloud providers. It populates
provider specific data into `/run/config` folder and the custom user data
is stored into the provided path.


```yaml
stages:
   default:
     - name: "Fetch cloud provider's user data"
       datasource:
         providers:
         - "aws"
         - "digitalocean"
         path: "/etc/cloud-data"
```

### `stages.STAGE_ID.STEP_NAME.layout`


Sets additional partitions on disk free space, if any, and/or expands the last
partition. All sizes are expressed in MiB only and default value of `size: 0`
means all available free space in disk. This plugin is useful to be used in
oem images where the default partitions might not suit the actual disk geometry.


```yaml
stages:
   default:
     - name: "Repart disk"
       layout:
         device:
           # It will partition a device including the given filesystem label
           # or partition label (filesystem label matches first) or the device
           # provided in 'path'. The label check has precedence over path when
           # both are provided.
           label: "COS_RECOVERY"
           path: "/dev/sda"
         # Only last partition can be expanded and it happens before any other
         # partition is added. size: 0 means all available free space
         expand_partition:
           size: 4096
         add_partitions:
           - fsLabel: "COS_STATE"
             size: 8192
             # No partition label is applied if omitted
             pLabel: "state"
           - fsLabel: "COS_PERSISTENT"
             # default filesystem is ext2 if omitted
             filesystem: "ext4"
```
