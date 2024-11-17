lookfat: lookfat.go
    go build -o bin/lookfat lookfat.go

mount:V:
    doas mount -o loop -t vfat wfat16.dat -o 'uid=1000,gid=1000' mnt

mountbsd:V:
    doas mount -o loop -t vfat fat16*openbsd*.dat -o 'uid=1000,gid=1000' mnt

mount32:V:
    doas mount -o loop -t vfat wfat32.dat -o 'uid=1000,gid=1000' mnt

drive:V:
    doas mount -o loop -t vfat pendrive32.dat -o 'uid=1000,gid=1000' mnt

reset:V:
    cp fat16.dat wfat16.dat; cp fat32.dat wfat32.dat

unmount:V:
    doas umount mnt
