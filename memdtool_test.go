package memdtool

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

type mockConn struct {
	io.Reader
}

func (*mockConn) Write(p []byte) (int, error) {
	return len(p), nil
}

func TestGetSlabStats(t *testing.T) {
	input := `STAT items:1:number 1
STAT items:1:age 102976348
STAT items:1:evicted 0
STAT items:1:evicted_nonzero 0
STAT items:1:evicted_time 0
STAT items:1:outofmemory 0
STAT items:1:tailrepairs 0
STAT items:1:reclaimed 171237
STAT items:2:number 795
STAT items:2:age 102375571
STAT items:2:evicted 0
STAT items:2:evicted_nonzero 0
STAT items:2:evicted_time 0
STAT items:2:outofmemory 0
STAT items:2:tailrepairs 0
STAT items:2:reclaimed 137342
STAT items:30:number 1
STAT items:30:age 52864995
STAT items:30:evicted 0
STAT items:30:evicted_nonzero 0
STAT items:30:evicted_time 0
STAT items:30:outofmemory 0
STAT items:30:tailrepairs 0
STAT items:30:reclaimed 0
END
STAT 1:chunk_size 96
STAT 1:chunks_per_page 10922
STAT 1:total_pages 1
STAT 1:total_chunks 10922
STAT 1:used_chunks 1
STAT 1:free_chunks 0
STAT 1:free_chunks_end 10921
STAT 1:mem_requested 88
STAT 1:get_hits 171239
STAT 1:cmd_set 171239
STAT 1:delete_hits 0
STAT 1:incr_hits 0
STAT 1:decr_hits 0
STAT 1:cas_hits 0
STAT 1:cas_badval 0
STAT 2:chunk_size 120
STAT 2:chunks_per_page 8738
STAT 2:total_pages 1
STAT 2:total_chunks 8738
STAT 2:used_chunks 795
STAT 2:free_chunks 5710
STAT 2:free_chunks_end 2233
STAT 2:mem_requested 84270
STAT 2:get_hits 18780
STAT 2:cmd_set 143922
STAT 2:delete_hits 0
STAT 2:incr_hits 0
STAT 2:decr_hits 0
STAT 2:cas_hits 0
STAT 2:cas_badval 0
STAT 30:chunk_size 66232
STAT 30:chunks_per_page 15
STAT 30:total_pages 1
STAT 30:total_chunks 15
STAT 30:used_chunks 1
STAT 30:free_chunks 0
STAT 30:free_chunks_end 14
STAT 30:mem_requested 57368
STAT 30:get_hits 0
STAT 30:cmd_set 1
STAT 30:delete_hits 0
STAT 30:incr_hits 0
STAT 30:decr_hits 0
STAT 30:cas_hits 0
STAT 30:cas_badval 0
STAT active_slabs 3
STAT total_malloced 3090552
END
`
	conn := &mockConn{strings.NewReader(input)}

	items, err := GetSlabStats(conn)
	if err != nil {
		t.Errorf("error should be nil but:%s", err.Error())
	}

	expeced := []*SlabStat{
		{
			ID:            1,
			Number:        1,
			Age:           102976348,
			Reclaimed:     171237,
			ChunkSize:     96,
			ChunksPerPage: 10922,
			TotalPages:    1,
			TotalChunks:   10922,
			UsedChunks:    1,
			FreeChunksEnd: 10921},
		{
			ID:            2,
			Number:        795,
			Age:           102375571,
			Reclaimed:     137342,
			ChunkSize:     120,
			ChunksPerPage: 8738,
			TotalPages:    1,
			TotalChunks:   8738,
			UsedChunks:    795,
			FreeChunks:    5710,
			FreeChunksEnd: 2233},
		{
			ID:            30,
			Number:        1,
			Age:           52864995,
			ChunkSize:     0x102b8,
			ChunksPerPage: 15,
			TotalPages:    1,
			TotalChunks:   15,
			UsedChunks:    1,
			FreeChunksEnd: 14},
	}
	if !reflect.DeepEqual(items, expeced) {
		t.Errorf("something went wrong")
	}
}
