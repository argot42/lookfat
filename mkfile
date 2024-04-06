lookfat: lookfat.go
    go build -o bin/lookfat lookfat.go

mount:V:
    doas mount -o loop -t vfat fat16.dat -o 'uid=1000,gid=1000' mnt

mountbsd:V:
    doas mount -o loop -t vfat fat16*openbsd*.dat -o 'uid=1000,gid=1000' mnt

mount32:V:
    doas mount -o loop -t vfat fat32.dat -o 'uid=1000,gid=1000' mnt
