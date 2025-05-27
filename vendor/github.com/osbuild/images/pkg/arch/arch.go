package arch

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/osbuild/images/internal/common"
)

type Arch uint64

const ( // architecture enum
	ARCH_UNSET Arch = iota
	ARCH_AARCH64
	ARCH_PPC64LE
	ARCH_S390X
	ARCH_X86_64
	ARCH_RISCV64
)

func (a Arch) String() string {
	switch a {
	case ARCH_UNSET:
		return "unset"
	case ARCH_AARCH64:
		return "aarch64"
	case ARCH_PPC64LE:
		return "ppc64le"
	case ARCH_S390X:
		return "s390x"
	case ARCH_X86_64:
		return "x86_64"
	case ARCH_RISCV64:
		return "riscv64"
	default:
		panic("invalid architecture")
	}
}

func (a *Arch) UnmarshalJSON(data []byte) (err error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*a, err = FromString(s)
	return err
}

func (a *Arch) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(a, unmarshal)
}

func FromString(a string) (Arch, error) {
	switch a {
	case "amd64", "x86_64":
		return ARCH_X86_64, nil
	case "arm64", "aarch64":
		return ARCH_AARCH64, nil
	case "s390x":
		return ARCH_S390X, nil
	case "ppc64le":
		return ARCH_PPC64LE, nil
	case "riscv64":
		return ARCH_RISCV64, nil
	default:
		return ARCH_UNSET, fmt.Errorf("unsupported architecture %q", a)
	}
}

var runtimeGOARCH = runtime.GOARCH

func Current() Arch {
	return common.Must(FromString(runtimeGOARCH))
}

func IsX86_64() bool {
	return Current() == ARCH_X86_64
}

func IsAarch64() bool {
	return Current() == ARCH_AARCH64
}

func IsPPC() bool {
	return Current() == ARCH_PPC64LE
}

func IsS390x() bool {
	return Current() == ARCH_S390X
}

func IsRISCV64() bool {
	return Current() == ARCH_RISCV64
}
