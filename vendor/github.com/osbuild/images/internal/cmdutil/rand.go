package cmdutil

import (
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"math"
	"math/big"
	"os"
	"strconv"

	"github.com/osbuild/images/internal/buildconfig"
)

const RNG_SEED_ENV_KEY = "OSBUILD_TESTING_RNG_SEED"

// newRNGSeed generates a random seed value unless the env var
// OSBUILD_TESTING_RNG_SEED is set.
func newRNGSeed() (int64, error) {
	if envSeedStr := os.Getenv(RNG_SEED_ENV_KEY); envSeedStr != "" {
		envSeedInt, err := strconv.ParseInt(envSeedStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse %s: %s", RNG_SEED_ENV_KEY, err)
		}
		fmt.Fprintf(os.Stderr, "TEST MODE: using rng seed %d\n", envSeedInt)
		return envSeedInt, nil
	}
	randSeed, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random seed: %s", err)
	}
	return randSeed.Int64(), nil
}

func SeedArgFor(bc *buildconfig.BuildConfig, imgTypeName, distributionName, archName string) (int64, error) {
	rngSeed, err := newRNGSeed()
	if err != nil {
		return 0, err
	}

	h := fnv.New64()
	h.Write([]byte(distributionName))
	h.Write([]byte(archName))
	h.Write([]byte(imgTypeName))
	if bc != nil {
		h.Write([]byte(bc.Name))
	}

	// nolint:gosec
	return rngSeed + int64(h.Sum64()), nil
}
