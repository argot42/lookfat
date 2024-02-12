package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"
)

type BPB struct {
	JumpBoot            [3]uint8 `print:"hex"`
	OEMName             [8]uint8 `print:"str"`
	BytesPerSector      uint16
	SectorPerCluster    uint8
	ReservedSectorCount uint16
	NFATs               uint8
	RootEntryCount      uint16
	TotalSectors16      uint16
	Media               uint8  `print:"hex"`
	FATsz16             uint16 // number of sectors per FAT
	SectorPerTrack      uint16
	NumberHeads         uint16
	HiddenSectors       uint32
	TotalSectors32      uint32
}

type BPBExt16 struct {
	DriveNumber   uint8
	Reserved      uint8
	BootSignature uint8
	VolumenID     uint32
	VolumenLabel  [11]uint8 `print:"str"`
	FSType        [8]uint8  `print:"str"`
	Empty         [448]uint8
	SignatureWord [2]uint8 `print:"hex"`
}

type BPBExt32 struct {
	FATsz32       uint32 // number of sectors per FAT (FAT32 only)
	ExtFlags      [2]uint8
	FSVer         [2]uint8
	RootCluster   uint32
	FSInfo        uint16
	BkBootSec     uint16
	Reserved      [12]uint8
	DriveNum      uint8
	Reserved1     uint8
	BootSignature uint8
	VolumenID     uint32
	VolumenLabel  [11]uint8 `print:"str"`
	FSType        [8]uint8  `print:"str"`
	Empty         [420]uint8
	SignatureWord [2]uint8 `print:"hex"`
}

type FATInfo struct {
	Type           uint8
	Warning        string
	FATSectors     uint32
	RootDirSectors uint32
	DataSectors    uint32
	TotalSectors   uint32
	ClusterCount   uint32
}

const (
	FAT12 = iota
	FAT16
	FAT32
)

const RootEntrySize = 32

func main() {
	printReserved := flag.Bool("r", false, "print reserved region")
	printType := flag.Bool("t", false, "detect FAT size")
	printInfo := flag.Bool("i", false, "print fs info")

	flag.Parse()

	if !*printReserved && !*printType && !*printInfo {
		flag.Usage()
		os.Exit(1)
	}

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(-1)
	}

	filepath := flag.Arg(0)

	bpb, ext16, ext32, info, err := readReservedSector(filepath)
	checkerr("", err)

	if *printReserved {
		pReserved(bpb, ext16, ext32, info)
	}
	if *printType {
		pType(info)
	}
	if *printInfo {
		pInfo(info)
	}
}

func pReserved(bpb BPB, ext16 BPBExt16, ext32 BPBExt32, info FATInfo) {
	superprint(bpb)

	switch info.Type {
	case FAT12, FAT16:
		superprint(ext16)
	case FAT32:
		superprint(ext32)
	}
}

func pType(info FATInfo) {
	switch info.Type {
	case FAT12:
		fmt.Println("FAT12")
	case FAT16:
		fmt.Println("FAT16")
	case FAT32:
		fmt.Println("FAT32")
	}
}

func pInfo(info FATInfo) {
	fmt.Printf(`FAT Region Sectors: %d
Root Region Sectors: %d
Data Region Sectors: %d
Total Sectors: %d
Cluster Count: %d
-----------------------------------
Warn: %s
`, info.FATSectors, info.RootDirSectors, info.DataSectors, info.TotalSectors, info.ClusterCount, info.Warning)
}

func readReservedSector(filepath string) (bpb BPB, ext16 BPBExt16, ext32 BPBExt32, info FATInfo, err error) {
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	if err = binary.Read(file, binary.LittleEndian, &bpb); err != nil {
		return
	}

	if !doILookFAT(bpb) {
		err = errors.New("not a msdos FAT FS")
		return
	}

	info.RootDirSectors = (uint32(bpb.RootEntryCount)*RootEntrySize + uint32(bpb.BytesPerSector) - 1) / uint32(bpb.BytesPerSector)

	// root entry count greater than 0 usually means FAT12/16
	if bpb.RootEntryCount != 0 {
		if err = binary.Read(file, binary.LittleEndian, &ext16); err != nil {
			return
		}
	} else { // if root entry count is 0 the type is FAT32
		if err = binary.Read(file, binary.LittleEndian, &ext32); err != nil {
			return
		}
		// set fat type
		info.Type = FAT32
	}

	if bpb.TotalSectors16 != 0 {
		info.TotalSectors = uint32(bpb.TotalSectors16)
	} else {
		info.TotalSectors = bpb.TotalSectors32
	}

	if bpb.FATsz16 != 0 {
		info.FATSectors = uint32(bpb.FATsz16)
	} else {
		info.FATSectors = ext32.FATsz32
	}

	// this formula is used to get the total count of clusters in the partition
	// then use it to determinate the FAT type as follows
	// if clusterCount < 4085       = FAT12
	// else if clusterCount < 65525 = FAT16
	// else                         = FAT32
	// for some reason you can create different FAT types disregarding cluster count when using mkfs.fat on linux
	// that's why I tried another method to figure out FAT type checking root dir sector count
	// I'm not sure if it is correct
	info.DataSectors = info.TotalSectors - (uint32(bpb.ReservedSectorCount) + uint32(bpb.NFATs)*info.FATSectors + uint32(info.RootDirSectors))

	info.ClusterCount = info.DataSectors / uint32(bpb.SectorPerCluster)

	// set FAT type by cluster count or set a warning if the type mismatch
	switch ccnt := info.ClusterCount; {
	case ccnt < 4085:
		if info.Type == 0 {
			info.Type = FAT12
		} else {
			info.Warning = fmt.Sprintf("according to my calculations FAT type is %d but the cluster count point to FAT12", info.Type)
		}
	case ccnt < 65525:
		if info.Type == 0 {
			info.Type = FAT16
		} else {
			info.Warning = fmt.Sprintf("according to my calculations FAT type is %d but the cluster count point to FAT16", info.Type)
		}
	default:
		if info.Type == 0 {
			info.Type = FAT32
		} else {
			info.Warning = fmt.Sprintf("according to my calculations FAT type is %d but the cluster count point to FAT32", info.Type)
		}
	}

	return
}

func doILookFAT(bpb BPB) bool {
	// checks if it's an actual FAT filesystem
	switch bpb.JumpBoot[0] {
	case 0xEB:
		fallthrough
	case 0xE9:
		return true
	}

	return false
}

func checkerr(msg string, err error) {
	if err != nil {
		if msg == "" {
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "%s: %s\n", msg, err.Error())
		}

		os.Exit(-1)
	}
}

func superprint(v interface{}) {
	t := reflect.TypeOf(v)
	val := reflect.ValueOf(v)

	fmt.Println(t.Name() + "{")

	for i, field := range reflect.VisibleFields(t) {
		fmt.Printf("\t%s:", field.Name)

		current := val.Field(i)

		switch field.Tag.Get("print") {
		case "hex":
			switch field.Type.Kind() {
			case reflect.Slice, reflect.Array:
				fmt.Print("[")
				for j := 0; j < current.Len(); j++ {
					if j == 0 {
						fmt.Printf("0x%x", current.Index(j).Interface())
					} else {
						fmt.Printf(" 0x%x", current.Index(j).Interface())
					}
				}
				fmt.Println("]")
			default:
				fmt.Printf("0x%x\n", current)
			}
		case "str":
			switch field.Type.Kind() {
			case reflect.Slice, reflect.Array:
				var buf []byte
				for j := 0; j < current.Len(); j++ {
					buf = append(buf, current.Index(j).Interface().(byte))
				}
				fmt.Printf("\"%s\"\n", string(buf))
			}
		default:
			fmt.Println(current)
		}
	}

	fmt.Println("}")
}
