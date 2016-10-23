package memdtool

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
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

// Run the memdtool
func (cli *CLI) Run(argv []string) int {
	log.SetOutput(cli.ErrStream)
	log.SetFlags(0)

	addr := "localhost:11211"
	if len(argv) > 0 {
		addr = argv[0]
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

func GetSlabStats(conn io.ReadWriteCloser) (map[uint64]*SlabStat, error) {
	defer conn.Close()
	ret := make(map[uint64]*SlabStat)
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
		ss, ok := ret[slabNum]
		if !ok {
			ss = &SlabStat{ID: slabNum}
			ret[slabNum] = ss
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
		ss, ok := ret[slabNum]
		if !ok {
			ss = &SlabStat{}
			ret[slabNum] = ss
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
	return ret, nil
}
