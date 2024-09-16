#
# spec file for package elemental-toolkit
#
# Copyright (c) 2022 - 2023 SUSE LINUX GmbH, Nuernberg, Germany.
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

# These variables are coupled to builder scripts
%define commit _replaceme_
%define c_date _replaceme_

Name:           elemental-toolkit
Version:        0
Release:        0
Summary:        The command line client for Elemental
License:        Apache-2.0
Group:          System/Management
Url:            https://github.com/rancher/elemental-toolkit
Source:         %{name}.tar.xz

Requires:       dosfstools
Requires:       e2fsprogs
# for blkdeactivate
Requires: lvm2
Requires:       parted
Requires:       rsync
Requires:       udev
Requires:       xfsprogs
Requires:       btrfsprogs
Requires:       snapper
Requires:       xorriso >= 1.5
Requires:       mtools
Requires:       util-linux
Requires:       gptfdisk

%if 0%{?suse_version}
BuildRequires:  golang(API) >= 1.22
BuildRequires:  golang-packaging
%{go_provides}
%else
%global goipath    google.golang.org/api
%global forgeurl   https://github.com/rancher/elemental-toolkit
%global commit     d1ae3f9a425de2618f9058f3b37583ef3ce52c7d
%gometa
%if (0%{?centos_version} == 800) || (0%{?rhel_version} == 800)
BuildRequires:  go1.22
%else
BuildRequires:  compiler(go-compiler)
%endif
%endif
BuildRequires:  xz

BuildRoot:      %{_tmppath}/%{name}-%{version}-build

%description
This package provides a universal command line client to access
Elemental functionality

%prep
%setup -q -n %{name}

%build

if [ "%{commit}" = "_replaceme_" ]; then
  echo "No commit hash provided"
  exit 1
fi

if [ "%{c_date}" = "_replaceme_" ]; then
  echo "No commit date provided"
  exit 1
fi

export GIT_TAG=$(echo "%{version}" | cut -d "+" -f 1)
GIT_COMMIT=$(echo "%{commit}")
export GIT_COMMIT=${GIT_COMMIT:0:8}
export COMMITDATE="%{c_date}"

make build-cli


%install
mkdir -p %{buildroot}%{_bindir}
install -m755 build/elemental %{buildroot}%{_bindir}

%files
%defattr(-,root,root,-)
%license LICENSE
%{_bindir}/*

%changelog
