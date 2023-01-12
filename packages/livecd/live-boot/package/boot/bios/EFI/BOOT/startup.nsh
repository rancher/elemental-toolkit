echo -off
echo Elemental Linux Live is starting.
\boot\kernel.xz initrd=\boot\rootfs.xz root=live:CDLABEL=COS_LIVE rd.live.dir=/ rd.live.squashimg=rootfs.squashfs rd.cos.disable
