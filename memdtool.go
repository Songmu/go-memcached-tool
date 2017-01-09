package memdtool

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	exitCodeOK = iota
	exitCodeParseFlagErr
	exitCodeErr
)

// CLI is struct for command line tool
type CLI struct {
	OutStream, ErrStream io.Writer
}

var helpReg = regexp.MustCompile(`^--?h(?:elp)?$`)

// Run the memdtool
func (cli *CLI) Run(argv []string) int {
	log.SetOutput(cli.ErrStream)
	log.SetFlags(0)

	mode := "display"
	addr := "127.0.0.1:11211"
	if len(argv) > 0 {
		modeCandidate := argv[len(argv)-1]
		if modeCandidate == "display" || modeCandidate == "dump" {
			mode = modeCandidate
			argv = argv[:len(argv)-1]
		}
		if len(argv) > 0 {
			addr = argv[0]
			if helpReg.MatchString(addr) {
				printHelp(cli.ErrStream)
				return exitCodeOK
			}
		}
	}

	var proto = "tcp"
	if strings.Contains(addr, "/") {
		proto = "unix"
	}
	conn, err := net.Dial(proto, addr)
	if err != nil {
		log.Println(err.Error())
		return exitCodeErr
	}
	defer conn.Close()

	switch mode {
	case "display":
		return cli.display(conn)
	case "dump":
		return cli.dump(conn)
	}
	return exitCodeErr
}

func (cli *CLI) display(conn io.ReadWriter) int {
	items, err := GetSlabStats(conn)
	if err != nil {
		log.Println(err.Error())
		return exitCodeErr
	}

	fmt.Fprint(cli.OutStream, "  #  Item_Size  Max_age   Pages   Count   Full?  Evicted Evict_Time OOM\n")
	for _, ss := range items {
		if ss.TotalPages == 0 {
			continue
		}
		size := fmt.Sprintf("%dB", ss.ChunkSize)
		if ss.ChunkSize > 1024 {
			size = fmt.Sprintf("%.1fK", float64(ss.ChunkSize)/1024.0)
		}
		full := "no"
		if ss.FreeChunksEnd == 0 {
			full = "yes"
		}
		fmt.Fprintf(cli.OutStream,
			"%3d %8s %9ds %7d %7d %7s %8d %8d %4d\n",
			ss.ID,
			size,
			ss.Age,
			ss.TotalPages,
			ss.Number,
			full,
			ss.Evicted,
			ss.EvictedTime,
			ss.Outofmemory,
		)
	}
	return exitCodeOK
}

func (cli *CLI) dump(conn io.ReadWriter) int {
	fmt.Fprint(conn, "stats items\r\n")
	slabItems := make(map[string]uint64)
	rdr := bufio.NewReader(conn)
	for {
		lineBytes, _, err := rdr.ReadLine()
		if err != nil {
			log.Println(err.Error())
			return exitCodeErr
		}
		line := string(lineBytes)
		if line == "END" {
			break
		}
		// ex. STAT items:1:number 1
		if !strings.Contains(line, ":number ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			log.Printf("result of `stats items` is strange: %s\n", line)
			return exitCodeErr
		}
		fields2 := strings.Split(fields[1], ":")
		if len(fields2) != 3 {
			log.Printf("result of `stats items` is strange: %s\n", line)
			return exitCodeErr
		}
		value, _ := strconv.ParseUint(fields[2], 10, 64)
		slabItems[fields2[1]] = value
	}

	var totalItems uint64
	for _, v := range slabItems {
		totalItems += v
	}
	fmt.Fprintf(cli.ErrStream, "Dumping memcache contents\n")
	fmt.Fprintf(cli.ErrStream, "  Number of buckets: %d\n", len(slabItems))
	fmt.Fprintf(cli.ErrStream, "  Number of items  : %d\n", totalItems)

	for k, v := range slabItems {
		fmt.Fprintf(cli.ErrStream, "Dumping bucket %s - %d total items\n", k, v)

		keyexp := make(map[string]string, int(v))
		fmt.Fprintf(conn, "stats cachedump %s %d\r\n", k, v)
		for {
			lineBytes, _, err := rdr.ReadLine()
			if err != nil {
				log.Println(err.Error())
				return exitCodeErr
			}
			line := string(lineBytes)
			if line == "END" {
				break
			}
			// return format like this
			// ITEM piyo [1 b; 1483953061 s]
			fields := strings.Fields(line)
			if len(fields) == 6 && fields[0] == "ITEM" {
				keyexp[fields[1]] = fields[4]
			}
		}

		for cachekey, exp := range keyexp {
			fmt.Fprintf(conn, "get %s\r\n", cachekey)
			for {
				lineBytes, _, err := rdr.ReadLine()
				if err != nil {
					log.Println(err.Error())
					return exitCodeErr
				}
				line := string(lineBytes)
				if line == "END" {
					break
				}
				// VALUE hoge 0 6
				// hogege
				fields := strings.Fields(line)
				if len(fields) != 4 || fields[0] != "VALUE" {
					continue
				}
				flags := fields[2]
				sizeStr := fields[3]
				size, err := strconv.Atoi(sizeStr)
				if err != nil {
					log.Println(err.Error())
					return exitCodeErr
				}
				buf := make([]byte, size)
				_, err = rdr.Read(buf)
				if err != nil {
					log.Println(err.Error())
					return exitCodeErr
				}
				fmt.Fprintf(cli.OutStream, "add %s %s %s %s\r\n%s\r\n", cachekey, flags, exp, sizeStr, string(buf))
				rdr.ReadLine()
			}
		}
	}
	return exitCodeOK
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `Usage: memcached-tool <host[:port] | /path/to/socket>

       memcached-tool 127.0.0.1:11211    # shows slabs
`)
}

type SlabStat struct {
	ID             uint64
	Number         uint64 // Count?
	Age            uint64
	Evicted        uint64
	EvictedNonzero uint64
	EvictedTime    uint64
	Outofmemory    uint64
	Reclaimed      uint64
	ChunkSize      uint64
	ChunksPerPage  uint64
	TotalPages     uint64
	TotalChunks    uint64
	UsedChunks     uint64
	FreeChunks     uint64
	FreeChunksEnd  uint64
}

func GetSlabStats(conn io.ReadWriter) ([]*SlabStat, error) {
	retMap := make(map[int]*SlabStat)
	fmt.Fprint(conn, "stats items\r\n")
	scr := bufio.NewScanner(bufio.NewReader(conn))
	for scr.Scan() {
		// ex. STAT items:1:number 1
		line := scr.Text()
		if line == "END" {
			break
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("result of `stats items` is strange: %s", line)
		}
		fields2 := strings.Split(fields[1], ":")
		if len(fields2) != 3 {
			return nil, fmt.Errorf("result of `stats items` is strange: %s", line)
		}
		key := fields2[2]
		slabNum, _ := strconv.ParseUint(fields2[1], 10, 64)
		value, _ := strconv.ParseUint(fields[2], 10, 64)
		ss, ok := retMap[int(slabNum)]
		if !ok {
			ss = &SlabStat{ID: slabNum}
			retMap[int(slabNum)] = ss
		}
		switch key {
		case "number":
			ss.Number = value
		case "age":
			ss.Age = value
		case "evicted":
			ss.Evicted = value
		case "evicted_nonzero":
			ss.EvictedNonzero = value
		case "evicted_time":
			ss.EvictedNonzero = value
		case "outofmemory":
			ss.Outofmemory = value
		case "reclaimed":
			ss.Reclaimed = value
		}
	}
	if err := scr.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to GetSlabStats while scaning stats items")
	}

	fmt.Fprint(conn, "stats slabs\r\n")
	for scr.Scan() {
		// ex. STAT 1:chunk_size 96
		line := scr.Text()
		if line == "END" {
			break
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("result of `stats slabs` is strange: %s", line)
		}
		fields2 := strings.Split(fields[1], ":")
		if len(fields2) != 2 {
			continue
		}
		key := fields2[1]
		slabNum, _ := strconv.ParseUint(fields2[0], 10, 64)
		value, _ := strconv.ParseUint(fields[2], 10, 64)
		ss, ok := retMap[int(slabNum)]
		if !ok {
			ss = &SlabStat{}
			retMap[int(slabNum)] = ss
		}

		switch key {
		case "chunk_size":
			ss.ChunkSize = value
		case "chunks_per_page":
			ss.ChunksPerPage = value
		case "total_pages":
			ss.TotalPages = value
		case "total_chunks":
			ss.TotalChunks = value
		case "used_chunks":
			ss.UsedChunks = value
		case "free_chunks":
			ss.FreeChunks = value
		case "free_chunks_end":
			ss.FreeChunksEnd = value
		}
	}
	if err := scr.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to GetSlabStats while scaning stats slabs")
	}

	keys := make([]int, 0, len(retMap))
	for i := range retMap {
		keys = append(keys, i)
	}
	sort.Ints(keys)
	ret := make([]*SlabStat, len(keys))
	for i, v := range keys {
		ret[i] = retMap[v]
	}
	return ret, nil
}
