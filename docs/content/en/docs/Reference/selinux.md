---
title: "SELinux"
linkTitle: "SELinux"
weight: 3
date: 2021-01-05
description: >
  Build SELinux policies with cOS
---

You can create a sample policy by using  `audit2allow` after running some
basic operations in permissive mode using system default policies. `allow2audit`
translates audit messages into allow/dontaudit SELinux policies which can be later
compiled as a SELinux module. This is the approach used in this illustration
example and mostly follows `audit2allow` [man pages](https://linux.die.net/man/1/audit2allow).

Once you have generated your policy (`mypolicy.te`) you can create a selinux package (create a new folder with `build.yaml`, `definition.yaml` and `mypolicy.te`) like so:

build.yaml:
```yaml
requires:
- name: "selinux-policies"
  category: "system"
  version: ">=0"
env:
- MODULE_NAME=mypolicy
steps:
- sed -i "s|^SELINUX=.*|SELINUX=permissive|g" /etc/selinux/config
- rm -rf /.autorelabel
- checkmodule -M -m -o ${MODULE_NAME}.mod ${MODULE_NAME}.te && semodule_package -o ${MODULE_NAME}.pp -m ${MODULE_NAME}.mod
- semodule -i ${MODULE_NAME}.pp
```

definition.yaml:
```yaml
name: "policy"
category: "selinux"
version: 0.0.1
```


Example mypolicy.te (generated with `audit2alllow`)
```
#==== cOS SELinux targeted policy module ========
#
# Disclaimer: This module is definition is for illustration use only. It
# has no guarantees of completeness, accuracy and usefulness. It should
# not be used "as is".
# 


module epinioOS 1.0;

require {
	type init_t;
	type audisp_t;
	type getty_t;
	type unconfined_t;
	type initrc_t;
	type bin_t;
	type tmpfs_t;
	type wicked_t;
	type systemd_logind_t;
	type sshd_t;
	type lib_t;
	type unlabeled_t;
	type chkpwd_t;
	type unconfined_service_t;
	type usr_t;
	type local_login_t;
	type cert_t;
	type system_dbusd_t;
	class lnk_file read;
	class file { execmod getattr open read };
	class dir { getattr read search watch };
}

#============= audisp_t ==============
allow audisp_t tmpfs_t:lnk_file read;

#============= chkpwd_t ==============
allow chkpwd_t tmpfs_t:file { getattr open read };

#============= getty_t ==============
allow getty_t tmpfs_t:file { getattr open read };

#============= init_t ==============
allow init_t cert_t:dir watch;
allow init_t usr_t:dir watch;

#============= initrc_t ==============

#!!!! This avc can be allowed using the boolean 'selinuxuser_execmod'
allow initrc_t bin_t:file execmod;

#============= local_login_t ==============
allow local_login_t tmpfs_t:file { getattr open read };
allow local_login_t tmpfs_t:lnk_file read;

#============= sshd_t ==============
allow sshd_t tmpfs_t:lnk_file read;

#============= system_dbusd_t ==============
allow system_dbusd_t lib_t:dir watch;
allow system_dbusd_t tmpfs_t:lnk_file read;

#============= systemd_logind_t ==============
allow systemd_logind_t unlabeled_t:dir { getattr search };

#============= unconfined_service_t ==============

#!!!! This avc can be allowed using the boolean 'selinuxuser_execmod'
allow unconfined_service_t bin_t:file execmod;

#============= unconfined_t ==============

#!!!! This avc can be allowed using the boolean 'selinuxuser_execmod'
allow unconfined_t bin_t:file execmod;

#============= wicked_t ==============
allow wicked_t tmpfs_t:dir read;
allow wicked_t tmpfs_t:file { getattr open read };
allow wicked_t tmpfs_t:lnk_file read;
```