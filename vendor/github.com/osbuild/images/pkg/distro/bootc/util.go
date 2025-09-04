package bootc

import (
	cryptorand "crypto/rand"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
)

func createRand() *rand.Rand {
	seed, err := cryptorand.Int(cryptorand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		panic("Cannot generate an RNG seed.")
	}

	// math/rand is good enough in this case
	/* #nosec G404 */
	return rand.New(rand.NewSource(seed.Int64()))
}

// getContainerSize returns the size of an already pulled container image in bytes
func getContainerSize(imgref string) (uint64, error) {
	output, err := exec.Command("podman", "image", "inspect", imgref, "--format", "{{.Size}}").Output()
	if err != nil {
		return 0, fmt.Errorf("failed inspect image: %w, output\n%s", err, output)
	}
	size, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse image size: %w", err)
	}

	return size, nil
}
