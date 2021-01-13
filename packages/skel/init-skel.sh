#!/bin/sh

if [ ! -e "/etc/fstab" ]; then
	touch /etc/fstab
fi

if [ ! -e "/etc/passwd" ]; then
	cp -rfv /etc/skel.defaults/etc/passwd /etc/passwd
fi

if [ ! -e "/etc/shadow" ]; then
	cp -rfv /etc/skel.defaults/etc/shadow /etc/shadow
fi

if [ -e "/etc/gshadow" ]; then
	chmod 644 /etc/gshadow
fi

if [ ! -e "/etc/hosts" ]; then
	cp -rfv /etc/skel.defaults/etc/hosts /etc/hosts
fi

if [ ! -e "/etc/group" ]; then
	cp -rfv /etc/skel.defaults/etc/group /etc/group
fi

if [ ! -e "/etc/hostname" ]; then
	cp -rfv /etc/skel.defaults/etc/hostname /etc/hostname
fi

if [ ! -e "/etc/profile" ]; then
	cp -rfv /etc/skel.defaults/etc/profile /etc/profile
fi

if [ ! -e "/etc/issue" ]; then
cat > /etc/issue << 'COS'

.-----.
| .-. |
| |.| |
| `-' |
`-----'

COS

cat >> /etc/issue << EOF

Welcome to cOS (login with user: root, password: cos)

EOF

fi

if [ ! -d "/root" ]; then
	mkdir /root
fi

if [ ! -d "/run/vsysctl.d" ]; then
	mkdir -p /run/vsysctl.d
fi

# Required on boot
if [ ! -e "/etc/sysctl.conf" ]; then
	touch /etc/sysctl.conf
fi

if [ ! -d "/var/tmp" ]; then
	mkdir -p /var/tmp
fi

