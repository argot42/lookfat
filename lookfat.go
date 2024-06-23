package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

// printables
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

type Str4Byte [4]uint8

func (b Str4Byte) String() string {
	var buf []byte
	for _, s := range b {
		buf = append(buf, byte(s))
	}
	return fmt.Sprintf("\"%s\"", string(buf))
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

type Str12Byte [12]uint8

func (b Str12Byte) String() string {
	var buf []byte
	for _, s := range b {
		buf = append(buf, byte(s))
	}
	return fmt.Sprintf("\"%s\"", string(buf))
}

// FAT header
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
	FATsz32  uint32 // number of sectors per FAT (FAT32 only)
	ExtFlags [2]uint8
	FSVer    [2]uint8
	// RootCluster is the cluster number where the root directory begins inside the data region
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

// FATInfo is a helper struct that saves useful information calculated with headers' info
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
	SectorSize     uint32
	ClusterCount   uint32
	ClusterSize    uint32
}

// dir/file entries
type DirEntry struct {
	Name    Str11Byte
	Attr    HexByte
	NTRes   uint8  // reserved must be 0?
	CTTenth uint8  // creation time. count tenths of a second 0 <= CCTenth <= 199
	CTime   uint16 // creation time. granularity is 2s
	CDate   uint16 // creation date
	// last accessed date.
	// This field must be updated on file modification (write operation)
	// and the date value must be equal to WDate.
	LDate uint16
	// High word of first data cluster number for file/directory described by this entry.
	// Only valid for volumes formatted FAT32. Must be set to 0 on volumes formatted FAT12/FAT16.
	FirstClusterHI uint16
	WTime          uint16 // write time (must be equal to CTime at creation)
	WDate          uint16 // write date (must be equal to CDate at creation)
	// Low word of first data cluster number for file/dir described by this entry
	FirstClusterLO uint16
	FileSize       uint32
}

type DirEntryLong struct {
	// order of the long name entry. the contents of the fields must be masked with 0x40
	Ordinal HexByte
	// for the last long directory name in the set
	Name1          Str10Byte // first 5 chars in name
	Attr           HexByte
	Type           uint8 // Reserved (set to 0)
	Checksum       uint8
	Name2          Str12Byte // 6 more chars in name
	FirstClusterLO uint16    // must be set to 0
	Name3          Str4Byte  // last 2 chars in name
}

type EntryInfo struct {
	ShortName string
	LongName  string
	Attr      HexByte
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

type Flags struct {
	printReserved bool
	printRoot     bool
	printType     bool
	printInfo     bool
	printFAT      bool
	filename      string
}

func main() {
	printHelp := flag.Bool("h", false, "print usage")
	printReserved := flag.Bool("r", false, "print reserved region")
	printRoot := flag.Bool("d", false, "print root directory region")
	printType := flag.Bool("t", false, "detect FAT type")
	printInfo := flag.Bool("i", false, "print fs info")
	printFAT := flag.Bool("a", false, "print all FAT entries")
	filename := flag.String("f", "", "get content from file")

	flag.Parse()

	flags := Flags{
		printReserved: *printReserved,
		printRoot:     *printRoot,
		printType:     *printType,
		printInfo:     *printInfo,
		printFAT:      *printFAT,
		filename:      *filename,
	}

	if flags == (Flags{}) {
		flag.Usage()
		os.Exit(1)
	}

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(-1)
	}

	if *printHelp {
		flag.Usage()
		os.Exit(1)
	}

	filepath := flag.Arg(0)

	file, err := os.Open(filepath)
	checkerr("", err)
	defer file.Close()

	bpb, ext16, ext32, info, err := readReservedSector(file)
	checkerr("", err)

	root, err := readDir(file, int64(info.RootDirOffset))
	checkerr("", err)

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
	if *printFAT {
		pFAT(file, info)
	}
	if *filename != "" {
		err = pFile(file, *filename, bpb, info, root)
		checkerr("", err)
	}
}

func pFAT(file *os.File, info FATInfo) (err error) {
	_, entry := mkentry(info.Type)

	if _, err = file.Seek(int64(info.FATOffset), io.SeekStart); err != nil {
		return
	}

	pos := info.FATOffset
	max := info.FATOffset + info.FATSectors*info.FATNumber*info.SectorSize

	var i int
	for pos < max {
		err := binary.Read(file, binary.LittleEndian, entry)
		if err != nil {
			return err
		}

		fmt.Printf("(%d: 0x%x) %v -> %v\n", i, pos, entry, locFromEntry(info.Type, entry))

		pos += uint32(len(entry))
		i++
	}

	return
}

func pFile(file *os.File, path string, bpb BPB, info FATInfo, root []EntryInfo) (err error) {
	splited := splitPath(path)
	filename := splited[len(splited)-1]

	for _, p := range splited {
		root, err = walk(file, bpb, info, root, p)
		if err != nil {
			return
		}
	}

	ok, fileInfo := findFile(filename, root)
	if !ok {
		return errors.New("entry not found")
	}

	var fatEntry []byte
	var eof, location, entrySize uint32

	eof, fatEntry = mkentry(info.Type)

	location = fileInfo.Location
	entrySize = uint32(len(fatEntry))

	var b []byte

	for {
		// here we calculate the file offset inside the file region
		// the first two clusters numbers are reserved
		// so we substract them from the `Location` number
		fileOffset := info.DataOffset +
			(location-2)*uint32(bpb.SectorPerCluster)*uint32(bpb.BytesPerSector)

		// buffer to store parts of the file stored inside the fs cluster
		chunk := make([]byte, info.ClusterSize)

		// read the cluster chunk
		if _, err = file.Seek(int64(fileOffset), io.SeekStart); err != nil {
			return
		}
		if err = binary.Read(file, binary.LittleEndian, chunk); err != nil {
			return
		}

		b = append(b, chunk...)

		// calculate fat entry offset (to look for next file part)
		fatEntryOffset := info.FATOffset + location*entrySize

		// read fat entry
		if _, err = file.Seek(int64(fatEntryOffset), io.SeekStart); err != nil {
			return
		}
		if err = binary.Read(file, binary.LittleEndian, fatEntry); err != nil {
			return
		}

		location = locFromEntry(info.Type, fatEntry)

		// if the new location is EOF stop reading
		if location == eof {
			break
		}
	}

	err = binary.Write(os.Stdout, binary.LittleEndian, b[:fileInfo.Size])

	return
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
	fmt.Println("files in root dir:")
	for _, v := range root {
		fmt.Printf("%+v\n", v)
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
	fmt.Printf(`FAT Quantity: %d
FAT Region Sectors: %d
FAT Region offset: 0x%x
Root Region Sectors: %d
Root Region offset: 0x%x
Data Region Sectors: %d
Data Region offset: 0x%x
Total Sectors: %d
Cluster Count: %d
Cluster Size: %d
Sector Size: %d
`,
		info.FATNumber,
		info.FATSectors,
		info.FATOffset,
		info.RootDirSectors,
		info.RootDirOffset,
		info.DataSectors,
		info.DataOffset,
		info.TotalSectors,
		info.ClusterCount,
		info.ClusterSize,
		info.SectorSize,
	)

	if info.Warning != "" {
		fmt.Printf("-----------------------------------\nWarn: %s\n", info.Warning)
	}
}

func readReservedSector(file *os.File) (
	bpb BPB,
	ext16 BPBExt16,
	ext32 BPBExt32,
	info FATInfo,
	err error,
) {
	if err = binary.Read(file, binary.LittleEndian, &bpb); err != nil {
		return
	}

	if !doILookFAT(bpb) {
		err = errors.New("not a msdos FAT FS")
		return
	}

	// TODO: check this calculation RootEntrySize is set to 32 bytes but that would be only in FAT32
	// also RootEntryCount is zero in FAT32 DOUBLE CHECK!
	info.RootDirSectors = (uint32(bpb.RootEntryCount)*RootEntrySize +
		uint32(bpb.BytesPerSector) - 1) / uint32(bpb.BytesPerSector)

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

	// save sector size
	info.SectorSize = uint32(bpb.BytesPerSector)

	// save number of FAT entries
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
	// for some reason you can create different FAT types disregarding
	// cluster count when using mkfs.fat on linux
	// that's why I tried another method to figure out FAT type checking root dir sector count
	// I'm not sure if it is correct
	info.DataSectors = info.TotalSectors - (uint32(bpb.ReservedSectorCount) +
		uint32(bpb.NFATs)*info.FATSectors +
		uint32(info.RootDirSectors))

	info.ClusterCount = info.DataSectors / uint32(bpb.SectorPerCluster)

	// calculate cluster size in bytes
	info.ClusterSize = uint32(bpb.SectorPerCluster) * uint32(bpb.BytesPerSector)

	// set FAT type by cluster count or set a warning if the type mismatch
	switch ccnt := info.ClusterCount; {
	case ccnt < 4085:
		if info.Type == 0 {
			info.Type = FAT12
		} else {
			info.Warning = fmt.Sprintf(
				"according to my calculations FAT type is %d but the cluster count point to FAT12",
				info.Type,
			)
		}
	case ccnt < 65525:
		if info.Type == 0 {
			info.Type = FAT16
		} else {
			info.Warning = fmt.Sprintf(
				"according to my calculations FAT type is %d but the cluster count point to FAT16",
				info.Type,
			)
		}
	default:
		if info.Type == 0 || info.Type == FAT32 {
			info.Type = FAT32
		} else {
			info.Warning = fmt.Sprintf(
				"according to my calculations FAT type is %d but the cluster count point to FAT32",
				info.Type,
			)
		}
	}

	// calculate offsets
	switch info.Type {
	case FAT12, FAT16:
		info.RootDirOffset = (uint32(bpb.ReservedSectorCount) + info.FATNumber*info.FATSectors) *
			uint32(bpb.BytesPerSector)
	case FAT32:
		info.RootDirOffset = (uint32(bpb.ReservedSectorCount) +
			info.FATNumber*info.FATSectors +
			(ext32.RootCluster-2)*uint32(bpb.SectorPerCluster)) * uint32(bpb.BytesPerSector)
	}

	info.FATOffset = uint32(bpb.ReservedSectorCount) * uint32(bpb.BytesPerSector)
	info.DataOffset = (uint32(bpb.ReservedSectorCount) +
		info.FATNumber*info.FATSectors + info.RootDirSectors) *
		uint32(bpb.BytesPerSector)

	return
}

func findFile(name string, entries []EntryInfo) (ok bool, file EntryInfo) {
	for _, v := range entries {
		if name == v.LongName || name == v.ShortName {
			file = v
			break
		}
	}
	return file != (EntryInfo{}), file
}

func walk(
	file *os.File,
	bpb BPB,
	info FATInfo,
	src []EntryInfo,
	dst string,
) (content []EntryInfo, err error) {
	ok, entry := findFile(dst, src)
	if !ok {
		return nil, errors.New("entry not found")
	}

	if entry.Attr&0x10 == 0 {
		return []EntryInfo{entry}, nil
	}

	offset := info.DataOffset +
		(entry.Location-2)*uint32(bpb.SectorPerCluster)*uint32(bpb.BytesPerSector)

	return readDir(file, int64(offset))
}

func readDir(file *os.File, offset int64) (entries []EntryInfo, err error) {
	if _, err = file.Seek(offset, io.SeekStart); err != nil {
		return
	}

	var lastLongFilename uint8

	longFilenames := make(map[uint8][][]byte)

OUT:
	for {
		// seek forward to only read attribute
		// maybe this can be done in a nicer way
		if _, err = file.Seek(11, io.SeekCurrent); err != nil {
			return
		}

		var attr uint8
		if err = binary.Read(file, binary.LittleEndian, &attr); err != nil {
			return
		}

		// move cursor back and read the whole entry
		if _, err = file.Seek(-12, io.SeekCurrent); err != nil {
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
				ShortName: string(name),
				Attr:      entry.Attr,
			}

		case 0xf: // long filename
			var entry DirEntryLong

			if err = binary.Read(file, binary.LittleEndian, &entry); err != nil {
				return
			}

			lf, ok := longFilenames[entry.Checksum]

			var part []byte

			for _, v := range entry.Name1 {
				if v == 0xff {
					break
				}
				part = append(part, v)
			}

			for _, v := range entry.Name2 {
				if v == 0xff {
					break
				}
				part = append(part, v)
			}

			for _, v := range entry.Name3 {
				if v == 0xff {
					break
				}
				part = append(part, v)
			}

			if !ok {
				longFilenames[entry.Checksum] = [][]byte{part}
			} else {
				lf = append(lf, part)
				longFilenames[entry.Checksum] = lf
			}

			if entry.Ordinal&0x3f == 1 {
				lastLongFilename = entry.Checksum
			}

			continue

		default: // short filename
			var short DirEntry
			if err = binary.Read(file, binary.LittleEndian, &short); err != nil {
				return
			}

			var shortName []byte
			for _, v := range short.Name {
				shortName = append(shortName, v)
			}

			entryInfo = EntryInfo{
				ShortName: string(shortName),
				Attr:      short.Attr,
				Location:  uint32(short.FirstClusterHI)<<16 + uint32(short.FirstClusterLO),
				Size:      short.FileSize,
			}

			// if there's a checksum saved add long filename to the entry
			if lastLongFilename != 0 {
				longName := buildLongFilename(longFilenames[lastLongFilename])
				entryInfo.LongName = string(longName)
				lastLongFilename = 0
			}
		}

		entries = append(entries, entryInfo)
	}

	return
}

// splitPath split the path and returns a slice with all the names
func splitPath(path string) (elements []string) {
	var dir, file string
	dir = path

	for {
		dir = filepath.Clean(dir)
		dir, file = filepath.Split(dir)

		if file == "" {
			break
		}

		elements = append(elements, file)

		if dir == "" {
			break
		}
	}
	slices.Reverse(elements)
	return
}

// buildLongFilename process parts of a long filename and converts it into a byte slice
func buildLongFilename(src [][]byte) (lf []byte) {
	for i := len(src) - 1; i >= 0; i-- {
		part := src[i]
		limit := len(part)

		if i == 0 {
			limit -= 2
		}

		for j := 0; j < limit; j += 2 {
			lf = append(lf, part[j])
		}
	}

	return
}

// doILookFAT checks if it's an actual FAT filesystem
func doILookFAT(bpb BPB) bool {
	switch bpb.JumpBoot[0] {
	case 0xEB, 0xE9:
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

func mkentry(t uint8) (eof uint32, fatEntry []byte) {
	switch t {
	case FAT12:
		fatEntry = make([]byte, 2)
		eof = 0xfff
	case FAT16:
		fatEntry = make([]byte, 2)
		eof = 0xffff
	case FAT32:
		fatEntry = make([]byte, 4)
		eof = 0xfffffff
	}
	return
}

func locFromEntry(t uint8, fatEntry []byte) uint32 {
	switch t {
	case FAT12, FAT16:
		return uint32(binary.LittleEndian.Uint16(fatEntry))
	case FAT32:
		return binary.LittleEndian.Uint32(fatEntry)
	}

	panic("not a correct fat type")
}
