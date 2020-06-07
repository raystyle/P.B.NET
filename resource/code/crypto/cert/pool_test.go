package cert

import (
	"testing"
)

func TestGenerateTestPoolAddParallel(t *testing.T) {
	t.Run("PublicRootCACert", func(t *testing.T) {
		generateTestPoolAddCertParallel("PublicRootCA")
	})

	t.Run("PublicClientCACert", func(t *testing.T) {
		generateTestPoolAddCertParallel("PublicClientCA")
	})

	t.Run("PublicClientPair", func(t *testing.T) {
		generateTestPoolAddPairParallel("PublicClient")
	})

	t.Run("PrivateRootCAPair", func(t *testing.T) {
		generateTestPoolAddPairParallel("PrivateRootCA")
	})

	t.Run("PrivateRootCACert", func(t *testing.T) {
		generateTestPoolAddCertParallel("PrivateRootCA")
	})

	t.Run("PrivateClientCAPair", func(t *testing.T) {
		generateTestPoolAddPairParallel("PrivateClientCA")
	})

	t.Run("PrivateClientCACert", func(t *testing.T) {
		generateTestPoolAddCertParallel("PrivateClientCA")
	})

	t.Run("PrivateClientPair", func(t *testing.T) {
		generateTestPoolAddPairParallel("PrivateClient")
	})

	t.Run("all", func(t *testing.T) {
		generateTestPoolAddCertParallel("PublicRootCA")
		generateTestPoolAddCertParallel("PublicClientCA")
		generateTestPoolAddPairParallel("PublicClient")
		generateTestPoolAddPairParallel("PrivateRootCA")
		generateTestPoolAddCertParallel("PrivateRootCA")
		generateTestPoolAddPairParallel("PrivateClientCA")
		generateTestPoolAddCertParallel("PrivateClientCA")
		generateTestPoolAddPairParallel("PrivateClient")
	})
}
