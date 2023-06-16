#
# spec file for package elemental-toolkit
#
# Copyright (c) 2022 SUSE LLC
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (unless the
# license for the pristine package is not an Open Source License, in which
# case the license is the MIT License). An "Open Source License" is a
# license that conforms to the Open Source Definition (Version 1.9)
# published by the Open Source Initiative.

# Please submit bugfixes or comments via https://bugs.opensuse.org/
#
%define systemdir /system
%define oemdir %{systemdir}/oem

Name:           elemental-toolkit
Version:        0
Release:        0
Summary:        A set of Elemental tools and utilities
License:        Apache-2.0
Group:          System/Management
URL:            https://github.com/rancher/%{name}
Source:         %{name}-%{version}.tar.gz
Source1:        LICENSE
Source2:        README.md

Requires:       elemental-init-config = %{version}-%{release}
Requires:       elemental-immutable-rootfs = %{version}-%{release}
Requires:       elemental-grub-config = %{version}-%{release}
Requires:       elemental-dracut-config = %{version}-%{release}

BuildArch:      noarch
BuildRoot:      %{_tmppath}/%{name}-%{version}-build

%description
A runtime set of core Elemental tools and utilities

%package -n elemental-immutable-rootfs
Summary:        Dracut module for Elemental
Requires:       dracut
Requires:       rsync

%description -n elemental-immutable-rootfs
Dracut modules to provide Elemental rootfs immutability based on persistent and ephemeral overlays


%package -n elemental-init-setup
Summary:        Elemental initialization services
Requires:       dracut
Requires:       elemental-cli
%{?systemd_requires}

%description -n elemental-init-setup
Elemental initialization services run cloud-init or yip yaml files for predefined boot stages


%package -n elemental-init-config
Summary:        Elemental default initialization config files
Requires:       elemental-init-network = %{version}-%{release}
Requires:       elemental-init-recovery = %{version}-%{release}
Requires:       elemental-init-live = %{version}-%{release}
Requires:       elemental-init-boot-assessment = %{version}-%{release}
Requires:       elemental-init-services = %{version}-%{release}

%description -n elemental-init-config
Provides Elemental default initialization configuration files

%package -n elemental-init-rootfs
Summary:        Elemental init yaml files to set the ephemeral rootfs
Requires:       elemental-init-setup = %{version}-%{release}

%description -n elemental-init-rootfs
Provides basic Elemental init yaml files to set the ephemeral and persistent ares of the root tree

%package -n elemental-init-network
Summary:        Elemental init yaml files to set network
Requires:       elemental-init-setup = %{version}-%{release}

%description -n elemental-init-network
Provides basic Elemental init yaml files to set the network


%package -n elemental-init-recovery
Summary:        Elemental init yaml files to set a sentinel file for recovery mode
Requires:       elemental-init-setup = %{version}-%{release}

%description -n elemental-init-recovery
Provides basic Elemental init yaml files to set a sentinel file when booting from the recovery image


%package -n elemental-init-live
Summary:        Elemental init yaml files to set a sentinel file for live mode
Requires:       elemental-init-setup = %{version}-%{release}

%description -n elemental-init-live
Provides basic Elemental init yaml files to set a sentinel file when booting from a live ISO


%package -n elemental-init-boot-assessment
Summary:        Elemental init yaml files to set boot assessment
Requires:       elemental-init-setup = %{version}-%{release}

%description -n elemental-init-boot-assessment
Provides basic Elemental init yaml files to set the boot assessment functionality


%package -n elemental-init-services
Summary:        Elemental init yaml files to set services
Requires:       elemental-init-setup = %{version}-%{release}

%description -n elemental-init-services
Provides basic Elemental init yaml files to enable/disable additional systemd services

%package -n elemental-upgrade-hooks
Summary:        Elemental hook yaml files for extra steps on install or upgrade
Requires:       elemental-cli

%description -n elemental-upgrade-hooks
Provides Elemental hook yaml files to fine tune installation and/or upgrade procedures

%package -n elemental-grub-config
Summary:        Elemental grub configuration files
Requires:       grub2

%description -n elemental-grub-config
Provides the grub configuration files boot the Elemental Teal image


%package -n elemental-grub-bootargs
Summary:        Elemental kernel parameters for grub2
Requires:       elemental-grub-config = %{version}-%{release}

%description -n elemental-grub-bootargs
Provides the kernel parameters required to boot Elemental Teal with grub


%package -n elemental-dracut-config
Summary:        Elemental specific dracut configuration
Requires:       dracut

%description -n elemental-dracut-config
Provides dracut configuration files for Elemental


%prep
%setup -q -n %{name}-%{version}
cp %{S:1} .
cp %{S:2} .

%build

%install
# elemental-immutable-rootfs
%{__install} -d -m 755 %{buildroot}/usr/lib/dracut/modules.d/30elemental-immutable-rootfs
%{__install} -m 755 pkg/features/embedded/immutable-rootfs/usr/lib/dracut/modules.d/30elemental-immutable-rootfs/*.sh %{buildroot}/usr/lib/dracut/modules.d/30elemental-immutable-rootfs
%{__install} -m 644 pkg/features/embedded/immutable-rootfs/usr/lib/dracut/modules.d/30elemental-immutable-rootfs/elemental-immutable-rootfs.service %{buildroot}/usr/lib/dracut/modules.d/30elemental-immutable-rootfs

# elemental-init-setup
%{__install} -D -m 644 pkg/features/embedded/elemental-setup/usr/lib/systemd/system/elemental-setup-rootfs.service %{buildroot}%{_unitdir}/elemental-setup-rootfs.service
%{__install} -D -m 644 pkg/features/embedded/elemental-setup/usr/lib/systemd/system/elemental-setup-initramfs.service %{buildroot}%{_unitdir}/elemental-setup-initramfs.service
%{__install} -D -m 644 pkg/features/embedded/elemental-setup/usr/lib/systemd/system/elemental-setup-reconcile.timer %{buildroot}%{_unitdir}/elemental-setup-reconcile.timer
%{__install} -D -m 644 pkg/features/embedded/elemental-setup/usr/lib/systemd/system/elemental-setup-reconcile.service %{buildroot}%{_unitdir}/elemental-setup-reconcile.service
%{__install} -D -m 644 pkg/features/embedded/elemental-setup/usr/lib/systemd/system/elemental-setup-fs.service %{buildroot}%{_unitdir}/elemental-setup-fs.service
%{__install} -D -m 644 pkg/features/embedded/elemental-setup/usr/lib/systemd/system/elemental-setup-boot.service %{buildroot}%{_unitdir}/elemental-setup-boot.service
%{__install} -D -m 644 pkg/features/embedded/elemental-setup/usr/lib/systemd/system/elemental-setup-network.service %{buildroot}%{_unitdir}/elemental-setup-network.service
%{__install} -D -m 644 pkg/features/embedded/elemental-setup/etc/dracut.conf.d/02-elemental-setup-initramfs.conf %{buildroot}%{_sysconfdir}/dracut.conf.d/02-elemental-setup-initramfs.conf

# elemental-grub-config
%{__install} -D -m 644 pkg/features/embedded/grub-config/etc/cos/grub.cfg %{buildroot}%{_sysconfdir}/cos/grub.cfg

# elemental-grub-bootargs
%{__install} -D -m 644 pkg/features/embedded/grub-config/etc/cos/bootargs.cfg %{buildroot}%{_sysconfdir}/cos/bootargs.cfg

# elemental-dracut-config
%{__install} -D -m 644 pkg/features/embedded/dracut-config/etc/dracut.conf.d/50-elemental-initrd.conf %{buildroot}%{_sysconfdir}/dracut.conf.d/50-elemental-initrd.conf

# elemental-init-rootfs
%{__install} -D -m 644 pkg/features/embedded/cloud-config-essentials/system/oem/00_rootfs.yaml %{buildroot}%{oemdir}/00_rootfs.yaml

# elemental-init-network
%{__install} -D -m 644 pkg/features/embedded/cloud-config-essentials/system/oem/05_network.yaml %{buildroot}%{oemdir}/05_network.yaml

# elemental-init-recovery
%{__install} -D -m 644 pkg/features/embedded/cloud-config-essentials/system/oem/06_recovery.yaml %{buildroot}%{oemdir}/06_recovery.yaml

# elemental-init-live
%{__install} -D -m 644 pkg/features/embedded/cloud-config-essentials/system/oem/07_live.yaml %{buildroot}%{oemdir}/07_live.yaml

# elemental-init-boot-assessment
%{__install} -D -m 644 pkg/features/embedded/cloud-config-essentials/system/oem/08_boot_assessment.yaml %{buildroot}%{oemdir}/08_boot_assessment.yaml

# elemental-init-services
%{__install} -D -m 644 pkg/features/embedded/cloud-config-essentials/system/oem/09_services.yaml %{buildroot}%{oemdir}/09_services.yaml


%pre -n elemental-init-setup
%service_add_pre elemental-setup-rootfs.service
%service_add_pre elemental-setup-initramfs.service
%service_add_pre elemental-setup-reconcile.timer
%service_add_pre elemental-setup-reconcile.service
%service_add_pre elemental-setup-fs.service
%service_add_pre elemental-setup-boot.service
%service_add_pre elemental-setup-network.service

%post -n elemental-init-setup
%service_add_post elemental-setup-rootfs.service
%service_add_post elemental-setup-initramfs.service
%service_add_post elemental-setup-reconcile.timer
%service_add_post elemental-setup-reconcile.service
%service_add_post elemental-setup-fs.service
%service_add_post elemental-setup-boot.service
%service_add_post elemental-setup-network.service

%preun -n elemental-init-setup
%service_del_preun elemental-setup-rootfs.service
%service_del_preun elemental-setup-initramfs.service
%service_del_preun elemental-setup-reconcile.timer
%service_del_preun elemental-setup-reconcile.service
%service_del_preun elemental-setup-fs.service
%service_del_preun elemental-setup-boot.service
%service_del_preun elemental-setup-network.service

%postun -n elemental-init-setup
%service_del_postun elemental-setup-rootfs.service
%service_del_postun elemental-setup-initramfs.service
%service_del_postun elemental-setup-reconcile.timer
%service_del_postun elemental-setup-reconcile.service
%service_del_postun elemental-setup-fs.service
%service_del_postun elemental-setup-boot.service
%service_del_postun elemental-setup-network.service

%files
%defattr(-,root,root,-)
%doc README.md
%license LICENSE

%files -n elemental-immutable-rootfs
%defattr(-,root,root,-)
%license LICENSE
%dir /usr/lib/dracut
%dir /usr/lib/dracut/modules.d
%dir /usr/lib/dracut/modules.d/*
/usr/lib/dracut/modules.d/*/*
%dir %{_sysconfdir}/dracut.conf.d

%files -n elemental-init-setup
%defattr(-,root,root,-)
%license LICENSE
%dir %{_unitdir}
%{_unitdir}/elemental-setup-rootfs.service
%{_unitdir}/elemental-setup-initramfs.service
%{_unitdir}/elemental-setup-reconcile.timer
%{_unitdir}/elemental-setup-reconcile.service
%{_unitdir}/elemental-setup-fs.service
%{_unitdir}/elemental-setup-boot.service
%{_unitdir}/elemental-setup-network.service
%dir %{_sysconfdir}/dracut.conf.d
%config %{_sysconfdir}/dracut.conf.d/02-elemental-setup-initramfs.conf

%files -n elemental-grub-config
%defattr(-,root,root,-)
%license LICENSE
%dir %{_sysconfdir}/cos
%config %{_sysconfdir}/cos/grub.cfg

%files -n elemental-grub-bootargs
%defattr(-,root,root,-)
%license LICENSE
%dir %{_sysconfdir}/cos
%config %{_sysconfdir}/cos/bootargs.cfg

%files -n elemental-dracut-config
%defattr(-,root,root,-)
%license LICENSE
%dir %{_sysconfdir}/dracut.conf.d
%config %{_sysconfdir}/dracut.conf.d/50-elemental-initrd.conf

%files -n elemental-init-rootfs
%defattr(-,root,root,-)
%license LICENSE
%dir %{systemdir}
%dir %{oemdir}
%{oemdir}/00_rootfs.yaml

%files -n elemental-init-network
%defattr(-,root,root,-)
%license LICENSE
%dir %{systemdir}
%dir %{oemdir}
%{oemdir}/05_network.yaml

%files -n elemental-init-recovery
%defattr(-,root,root,-)
%license LICENSE
%dir %{systemdir}
%dir %{oemdir}
%{oemdir}/06_recovery.yaml

%files -n elemental-init-live
%defattr(-,root,root,-)
%license LICENSE
%dir %{systemdir}
%dir %{oemdir}
%{oemdir}/07_live.yaml

%files -n elemental-init-boot-assessment
%defattr(-,root,root,-)
%license LICENSE
%dir %{systemdir}
%dir %{oemdir}
%{oemdir}/08_boot_assessment.yaml

%files -n elemental-init-services
%defattr(-,root,root,-)
%license LICENSE
%dir %{systemdir}
%dir %{oemdir}
%{oemdir}/09_services.yaml

%files -n elemental-init-config
%defattr(-,root,root,-)
%license LICENSE

%changelog
