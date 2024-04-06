package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"os"
)

const HexFMT = "0x%x"

type HexByte uint8

func (b HexByte) String() string {
	return fmt.Sprintf(HexFMT, uint8(b))
}

type Hex2Byte [2]uint8

func (b Hex2Byte) String() string {
	var str string
	for i, s := range b {
		if i != 0 {
			str += " "
		}
		str += fmt.Sprint(HexByte(s))
	}
	return fmt.Sprintf("|%s|", str)
}

type Hex3Byte [3]uint8

func (b Hex3Byte) String() string {
	var str string
	for i, s := range b {
		if i != 0 {
			str += " "
		}
		str += fmt.Sprint(HexByte(s))
	}
	return fmt.Sprintf("|%s|", str)
}

type Str8Byte [8]uint8

func (b Str8Byte) String() string {
	var buf []byte
	for _, s := range b {
		buf = append(buf, byte(s))
	}
	return fmt.Sprintf("\"%s\"", string(buf))
}

type Str10Byte [10]uint8

func (b Str10Byte) String() string {
	var buf []byte
	for _, s := range b {
		buf = append(buf, byte(s))
	}
	return fmt.Sprintf("\"%s\"", string(buf))
}

type Str11Byte [11]uint8

func (b Str11Byte) String() string {
	var buf []byte
	for _, s := range b {
		buf = append(buf, byte(s))
	}
	return fmt.Sprintf("\"%s\"", string(buf))
}

type BPB struct {
	JumpBoot            Hex3Byte
	OEMName             Str8Byte
	BytesPerSector      uint16
	SectorPerCluster    uint8
	ReservedSectorCount uint16
	NFATs               uint8
	RootEntryCount      uint16
	TotalSectors16      uint16
	Media               HexByte
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
	VolumenLabel  Str11Byte
	FSType        Str8Byte
	Empty         [448]uint8
	SignatureWord Hex2Byte
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
	VolumenLabel  Str11Byte
	FSType        Str8Byte
	Empty         [420]uint8
	SignatureWord Hex2Byte
}

type FATInfo struct {
	Type           uint8
	Warning        string
	FATNumber      uint32
	FATSectors     uint32
	FATOffset      uint32
	RootDirSectors uint32
	RootDirOffset  uint32
	DataSectors    uint32
	DataOffset     uint32
	TotalSectors   uint32
	ClusterCount   uint32
}

type DirEntry struct {
	Name    Str11Byte
	Attr    HexByte
	NTRes   uint8  // reserved must be 0?
	CTTenth uint8  // creation time. count tenths of a second 0 <= CCTenth <= 199
	CTime   uint16 // creation time. granularity is 2s
	CDate   uint16 // creation date
	// last accessed date.
	//This field must be updated on file modification (write operation) and the date value must be equal to WDate.
	LDate uint16
	// High word of first data cluster number for file/directory described by this entry.
	// Only valid for volumes formatted FAT32. Must be set to 0 on volumes formatted FAT12/FAT16.
	FirstClusterHI uint16
	WTime          uint16 // write time (must be equal to CTime at creation)
	WDate          uint16 // write date (must be equal to CDate at creation)
	FirstClusterLO uint16 // Low word of first data cluster number for file/dir described by this entry
	FileSize       uint32
}

type DirEntryLong struct {
	Ordinal HexByte // order of the long name entry. the contents of the fields must be masked with 0x40
	// for the last long directory name in the set
	Name1          Str10Byte // first 5 chars in name
	Attr           uint8     `print:"hex"`
	Type           uint8     // Reserved (set to 0)
	Checksum       uint8
	Name2          [12]uint8 `print:"str"` // 6 more chars in name
	FirstClusterLO uint16    // must be set to 0
	Name3          [4]uint8  `print:"str"` // last 2 chars in name
}

type EntryInfo struct {
	Name string
	Attr HexByte
	//Crt      time.Time
	//Mod      time.Time
	Location uint32
	Size     uint32
}

const (
	FAT12 = iota
	FAT16
	FAT32
)

const RootEntrySize = 32

func main() {
	printReserved := flag.Bool("r", false, "print reserved region")
	printRoot := flag.Bool("d", false, "print root directory region")
	printType := flag.Bool("t", false, "detect FAT size")
	printInfo := flag.Bool("i", false, "print fs info")

	flag.Parse()

	if !*printReserved && !*printType && !*printInfo && !*printRoot {
		flag.Usage()
		os.Exit(1)
	}

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(-1)
	}

	filepath := flag.Arg(0)

	file, err := os.Open(filepath)
	checkerr("", err)
	defer file.Close()

	bpb, ext16, ext32, info, err := readReservedSector(file)
	checkerr("", err)

	root, err := readRootDirSector(file, info)

	if *printReserved {
		pReserved(bpb, ext16, ext32, info)
	}
	if *printRoot {
		pRoot(info, root)
	}
	if *printType {
		pType(info)
	}
	if *printInfo {
		pInfo(info)
	}
}

func pReserved(bpb BPB, ext16 BPBExt16, ext32 BPBExt32, info FATInfo) {
	fmt.Printf("reserved: %+v\n", bpb)

	switch info.Type {
	case FAT12, FAT16:
		fmt.Printf("ext12/16: %+v\n", ext16)
	case FAT32:
		fmt.Printf("ext32: %+v\n", ext32)
	}
}

func pRoot(info FATInfo, root []EntryInfo) {
	fmt.Println("root directory: %+v\n", root)
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
FAT Region offset: %d
Root Region Sectors: %d
Root Region offset: %d
Data Region Sectors: %d
Data Region offset: %d
Total Sectors: %d
Cluster Count: %d
-----------------------------------
Warn: %s
`,
		info.FATSectors,
		info.FATOffset,
		info.RootDirSectors,
		info.RootDirOffset,
		info.DataSectors,
		info.DataOffset,
		info.TotalSectors,
		info.ClusterCount,
		info.Warning,
	)
}

func readReservedSector(file *os.File) (bpb BPB, ext16 BPBExt16, ext32 BPBExt32, info FATInfo, err error) {
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

	// set number of FAT entries
	info.FATNumber = uint32(bpb.NFATs)

	// calculate total number of sectors for volume
	if bpb.TotalSectors16 != 0 {
		info.TotalSectors = uint32(bpb.TotalSectors16)
	} else {
		info.TotalSectors = bpb.TotalSectors32
	}

	// calculate number of sectors per FAT entry
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

	// calculate offsets
	switch info.Type {
	case FAT12, FAT16:
		info.RootDirOffset = (uint32(bpb.ReservedSectorCount) + info.FATNumber*info.FATSectors) * uint32(bpb.BytesPerSector)
	}

	info.FATOffset = uint32(bpb.ReservedSectorCount) * uint32(bpb.BytesPerSector)

	return
}

func readRootDirSector(file *os.File, info FATInfo) (root []EntryInfo, err error) {
	if _, err = file.Seek(int64(info.RootDirOffset), 0); err != nil {
		return
	}

OUT:
	for {
		if _, err = file.Seek(11, 1); err != nil {
			return
		}

		var attr uint8
		if err = binary.Read(file, binary.LittleEndian, &attr); err != nil {
			return
		}

		if _, err = file.Seek(-12, 1); err != nil {
			return
		}

		var entryInfo EntryInfo

		switch attr {
		case 0x0: // end of entries
			break OUT
		case 0x28, 0x8: // volume id
			var entry DirEntry
			if err = binary.Read(file, binary.LittleEndian, &entry); err != nil {
				return
			}

			var name []byte
			for _, v := range entry.Name {
				name = append(name, v)
			}

			entryInfo = EntryInfo{
				Name: string(name),
				Attr: entry.Attr,
			}

		case 0xf: // long filename
			var entry DirEntryLong
			var parts [][]byte

			for {
				if err = binary.Read(file, binary.LittleEndian, &entry); err != nil {
					return
				}

				var p []byte

				for _, v := range entry.Name1 {
					if v == 0xff {
						break
					}
					p = append(p, v)
				}

				for _, v := range entry.Name2 {
					if v == 0xff {
						break
					}
					p = append(p, v)
				}

				for _, v := range entry.Name3 {
					if v == 0xff {
						break
					}
					p = append(p, v)
				}

				parts = append(parts, p)

				if entry.Ordinal&0x3f == 1 {
					break
				}
			}

			var name []byte
			for i := len(parts) - 1; i >= 0; i-- {
				for _, p := range parts[i] {
					name = append(name, p)
				}
			}

			var short DirEntry
			if err = binary.Read(file, binary.LittleEndian, &short); err != nil {
				return
			}

			entryInfo = EntryInfo{
				Name:     string(name),
				Attr:     short.Attr,
				Location: uint32(short.FirstClusterLO),
				Size:     short.FileSize,
			}

		default: // short filename
			var short DirEntry
			if err = binary.Read(file, binary.LittleEndian, &short); err != nil {
				return
			}

			var name []byte
			for _, v := range short.Name {
				name = append(name, v)
			}

			entryInfo = EntryInfo{
				Name:     string(name),
				Attr:     short.Attr,
				Location: uint32(short.FirstClusterLO),
				Size:     short.FileSize,
			}
		}

		root = append(root, entryInfo)
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
