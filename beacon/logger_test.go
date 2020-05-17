package beacon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func TestLogger(t *testing.T) {
	const (
		prefixF  = "test format %s %s"
		prefix   = "test print"
		prefixLn = "test println"
		src      = "test src"
		log1     = "test"
		log2     = "log"
	)

	cfg := testGenerateConfig(t)
	beacon, err := New(cfg)
	require.NoError(t, err)

	lg := beacon.logger

	lg.Printf(logger.Debug, src, prefixF, log1, log2)
	lg.Print(logger.Debug, src, prefix, log1, log2)
	lg.Println(logger.Debug, src, prefixLn, log1, log2)

	lg.Printf(logger.Warning, src, prefixF, log1, log2)
	lg.Print(logger.Warning, src, prefix, log1, log2)
	lg.Println(logger.Warning, src, prefixLn, log1, log2)

	lg.Printf(logger.Exploit, src, prefixF, log1, log2)
	lg.Print(logger.Exploit, src, prefix, log1, log2)
	lg.Println(logger.Exploit, src, prefixLn, log1, log2)

	lg.Printf(logger.Fatal, src, prefixF, log1, log2)
	lg.Print(logger.Fatal, src, prefix, log1, log2)
	lg.Println(logger.Fatal, src, prefixLn, log1, log2)
}
