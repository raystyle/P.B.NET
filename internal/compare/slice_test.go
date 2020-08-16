package compare

import (
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUniqueStrings(t *testing.T) {
	for _, item := range [...]*struct {
		new     []string
		old     []string
		added   []int
		deleted []int
	}{
		{
			new:     []string{"b", "c", "d"},
			old:     []string{"a", "b", "c"},
			added:   []int{2},
			deleted: []int{0},
		},
	} {
		added, deleted := UniqueStrings(item.new, item.old)
		require.Equal(t, item.added, added)
		require.Equal(t, item.deleted, deleted)
	}
}

func BenchmarkUniqueStrings(b *testing.B) {
	// see project/internal/module/netmon/netstat.go
	const (
		tcp4RowSize = net.IPv4len + 2 + net.IPv4len + 2
		tcp6RowSize = net.IPv6len + 4 + 2 + net.IPv6len + 4 + 2
	)

	b.Run("100 x tcp4RowSize", func(b *testing.B) {
		benchmarkUniqueStrings(b, 100, tcp4RowSize)
	})

	b.Run("1000 x tcp4RowSize", func(b *testing.B) {
		benchmarkUniqueStrings(b, 1000, tcp4RowSize)
	})

	b.Run("10000 x tcp4RowSize", func(b *testing.B) {
		benchmarkUniqueStrings(b, 10000, tcp4RowSize)
	})

	b.Run("100000 x tcp4RowSize", func(b *testing.B) {
		benchmarkUniqueStrings(b, 100000, tcp4RowSize)
	})

	b.Run("100 x tcp6RowSize", func(b *testing.B) {
		benchmarkUniqueStrings(b, 100, tcp6RowSize)
	})

	b.Run("1000 x tcp6RowSize", func(b *testing.B) {
		benchmarkUniqueStrings(b, 1000, tcp6RowSize)
	})

	b.Run("10000 x tcp6RowSize", func(b *testing.B) {
		benchmarkUniqueStrings(b, 10000, tcp6RowSize)
	})

	b.Run("100000 x tcp6RowSize", func(b *testing.B) {
		benchmarkUniqueStrings(b, 100000, tcp6RowSize)
	})
}

func benchmarkUniqueStrings(b *testing.B, size, factor int) {
	n := make([]string, size)
	for i := 0; i < size; i++ {
		n[i] = fmt.Sprintf("%0"+strconv.Itoa(factor)+"d", i+1)
	}
	o := make([]string, size)
	for i := 0; i < size; i++ {
		o[i] = fmt.Sprintf("%0"+strconv.Itoa(factor)+"d", i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		added, deleted := UniqueStrings(n, o)
		if len(added) != 1 {
			b.Fatal("invalid added number:", added)
		}
		if len(deleted) != 1 {
			b.Fatal("invalid deleted number:", deleted)
		}
		if added[0] != size-1 {
			b.Fatal("invalid added index:", added[0])
		}
		if deleted[0] != 0 {
			b.Fatal("invalid deleted index:", deleted[0])
		}
	}

	b.StopTimer()
}
