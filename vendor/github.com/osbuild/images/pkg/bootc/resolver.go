package bootc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/depsolvednf"
	"golang.org/x/exp/slices"
)

// envPath is written by podman
const envPath = "/run/.containerenv"

// rootlessKey is set when we are rootless
const rootlessKey = "rootless=1"

// isPodmanRootless detects if we are running rootless in podman;
// other situations (e.g. docker) will successfuly return false.
func isPodmanRootless() (bool, error) {
	buf, err := os.ReadFile(envPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		if scanner.Text() == rootlessKey {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("parsing %s: %w", envPath, err)
	}
	return false, nil
}

// Container is a simpler wrapper around a running podman container.
// This type isn't meant as a general-purpose container management tool, but
// as an opinonated library for bootc-image-builder.
type Container struct {
	ref       string
	id        string
	root      string
	arch      string
	extraOpts []string
}

// New creates a new running container from the given image reference.
//
// NB:
// - --net host is used to make networking work in a nested container
// - /run/secrets is mounted from the host to make sure RHSM credentials are available
func NewContainer(ref string) (*Container, error) {
	extraOpts := []string{}
	if isRootless, _ := isPodmanRootless(); isRootless {
		// When running bc-i-b In a rootless container, its typically the case that /var/lib/containers/storage
		// is a bind-mount of ~/.local/share/containers/storage, and we can't use this directly with podman
		// because it will complain:
		//     database static dir "~/.local/share/containers/storage/libpod" does not match our
		//     static dir "/var/lib/containers/storage/libpod": database configuration mismatch
		// To avoid this we use an empty graphroot, and point --imagestore at /var/lib/containers/storage.
		// This means the database is in the right place, and we only look at the image layers in the real store.
		extraOpts = append(extraOpts,
			"--root=/run/osbuild/containers/store",
			"--imagestore=/var/lib/containers/storage")
	}

	const secretDir = "/run/secrets"
	secretVolume := fmt.Sprintf("%s:%s", secretDir, secretDir)

	args := []string{
		"run",
		"--rm",
		"--init", // If sleep infinity is run as PID 1, it doesn't get signals, thus we cannot easily stop the container
		"--detach",
		"--net", "host", // Networking in a nested container doesn't work without re-using this container's network
		"--entrypoint", "sleep", // The entrypoint might be arbitrary, so let's just override it with sleep, we don't want to run anything
	}

	// Re-mount the secret directory if it exists
	if _, err := os.Stat(secretDir); err == nil {
		args = append(args, "--volume", secretVolume)
	}

	args = append(args, extraOpts...)

	args = append(args, ref, "infinity")

	output, err := exec.Command("podman", args...).Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("running %s container failed: %w\nstderr:\n%s", ref, e, e.Stderr)
		}
		return nil, fmt.Errorf("running %s container failed with generic error: %w", ref, err)
	}

	c := &Container{
		ref:       ref,
		extraOpts: extraOpts,
	}
	c.id = strings.TrimSpace(string(output))
	// Ensure that the container is stopped when this function errors
	defer func() {
		if err != nil {
			if stopErr := c.Stop(); stopErr != nil {
				err = fmt.Errorf("%w\nstopping the container failed too: %s", err, stopErr)
			}
			c = nil
		}
	}()
	// not all containers set {{.Architecture}} so fallback
	c.arch, err = findContainerArchInspect(c.id, ref, extraOpts)
	if err != nil {
		return nil, err
	}

	args = []string{"mount"}
	args = append(args, extraOpts...)
	args = append(args, c.id)

	/* #nosec G204 */
	output, err = exec.Command("podman", args...).Output()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("mounting %s container failed: %w\nstderr:\n%s", ref, err, err.Stderr)
		}
		return nil, fmt.Errorf("mounting %s container failed with generic error: %w", ref, err)
	}
	c.root = strings.TrimSpace(string(output))

	return c, err
}

// Stop stops the container. Since New() creates a container with --rm, this
// removes the container as well.
func (c *Container) Stop() error {

	args := []string{"stop"}
	args = append(args, c.extraOpts...)
	args = append(args, c.id)

	/* #nosec G204 */
	if output, err := exec.Command("podman", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("stopping %s container failed: %w\noutput:\n%s", c.id, err, output)
	}

	args = []string{"rm"}
	args = append(args, c.extraOpts...)
	args = append(args, "--ignore", c.id)

	// when the container is stopped by podman it may not honor the "--rm"
	// that was passed in `New()` so manually remove the container here if it is still available
	/* #nosec G204 */
	if output, err := exec.Command("podman", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("removing %s container failed: %w\noutput:\n%s", c.id, err, output)
	}

	return nil
}

// ResolveInfo loads all information from the running container.
func (c *Container) ResolveInfo() (*Info, error) {
	bootcInfo := &Info{
		Imgref:  c.ref,
		ImageID: c.id,
		Arch:    c.Arch(),
	}

	os, err := osinfo.Load(c.Root())
	if err != nil {
		return nil, err
	}
	if os.KernelInfo != nil {
		modules, err := c.InitrdModules(os.KernelInfo.Version)
		if err != nil {
			return nil, err
		}
		os.InitrdModules = modules
	}
	bootcInfo.OSInfo = os

	defaultFs, err := c.DefaultRootfsType()
	if err != nil {
		return nil, err
	}
	bootcInfo.DefaultRootFs = defaultFs

	size, err := getContainerSize(c.ref, c.extraOpts)
	if err != nil {
		return nil, err
	}
	bootcInfo.Size = size

	return bootcInfo, nil
}

// Root returns the root directory of the container as available on the host.
func (c *Container) Root() string {
	return c.root
}

// Arch returns the architecture of the container
func (c *Container) Arch() string {
	return c.arch
}

// Reads a file from the container
func (c *Container) ReadFile(path string) ([]byte, error) {
	args := []string{"exec"}
	args = append(args, c.extraOpts...)
	args = append(args, c.id, "cat", path)

	/* #nosec G204 */
	output, err := exec.Command("podman", args...).Output()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("reading %s from %s container failed: %w\nstderr:\n%s", path, c.id, err, err.Stderr)
		}
		return nil, fmt.Errorf("reading %s from %s container failed with generic error: %w", path, c.id, err)
	}

	return output, nil
}

// CopyInto copies a file into the container.
func (c *Container) CopyInto(src, dest string) error {
	args := []string{"cp"}
	args = append(args, c.extraOpts...)
	args = append(args, src, c.id+":"+dest)

	/* #nosec G204 */
	if output, err := exec.Command("podman", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("copying %s into %s container failed: %w\noutput:\n%s", src, c.id, err, output)
	}

	return nil
}

func (c *Container) ExecArgv() []string {
	args := []string{"podman", "exec"}
	args = append(args, c.extraOpts...)
	args = append(args, "-i", c.id)
	return args
}

// DefaultRootfsType returns the default rootfs type (e.g. "ext4") as
// specified by the bootc container install configuration. An empty
// string is valid and means the container sets no default.
func (c *Container) DefaultRootfsType() (string, error) {
	args := []string{"exec"}
	args = append(args, c.extraOpts...)
	args = append(args, c.id, "bootc", "install", "print-configuration")

	/* #nosec G204 */
	output, err := exec.Command("podman", args...).Output()
	if err != nil {
		return "", fmt.Errorf("failed to run bootc install print-configuration: %w, output:\n%s", err, output)
	}

	var bootcConfig struct {
		Filesystem struct {
			Root struct {
				Type string `json:"type"`
			} `json:"root"`
		} `json:"filesystem"`
	}

	if err := json.Unmarshal(output, &bootcConfig); err != nil {
		return "", fmt.Errorf("failed to unmarshal bootc configuration: %w", err)
	}

	// filesystem.root.type is the preferred way instead of the old root-fs-type top-level key.
	// See https://github.com/containers/bootc/commit/558cd4b1d242467e0ffec77fb02b35166469dcc7
	fsType := bootcConfig.Filesystem.Root.Type
	// Note that these are the only filesystems that the "images" library
	// knows how to handle, i.e. how to construct the required osbuild
	// stages for.
	// TODO: move this into a helper in "images" so that there is only
	// a single place that needs updating when we add e.g. btrfs or
	// bcachefs
	supportedFS := []string{"ext4", "xfs", "btrfs"}

	if fsType == "" {
		return "", nil
	}
	if !slices.Contains(supportedFS, fsType) {
		return "", fmt.Errorf("unsupported root filesystem type: %s, supported: %s", fsType, strings.Join(supportedFS, ", "))
	}

	return fsType, nil
}

// InitrdModules gets the list of modules from the container's initrd
func (c *Container) InitrdModules(kver string) ([]string, error) {
	args := []string{"exec"}
	args = append(args, c.extraOpts...)
	args = append(args, c.id, "lsinitrd", "--mod", "--kver", kver)

	/* #nosec G204 */
	output, err := exec.Command("podman", args...).Output()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to run lsinitrd --mod --kver %s: %w, output:\n%s\nstderr:\n%s", kver, err, output, err.Stderr)
		}
		return nil, fmt.Errorf("failed to run lsinitrd --mod --kver %s: %w, output:\n%s", kver, err, output)
	}

	return strings.Split(strings.TrimRight(string(output), "\n"), "\n"), nil
}

func findImageIdFor(cntId, ref string, extraOpts []string) (string, error) {
	args := []string{"inspect"}
	args = append(args, extraOpts...)
	args = append(args, "-f", "{{.Image}}", cntId)

	/* #nosec G204 */
	output, err := exec.Command("podman", args...).Output()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("inspecting container %q failed: %w\nstderr:\n%s", ref, err, err.Stderr)
		}
		return "", fmt.Errorf("inspecting %s container failed with generic error: %w", ref, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func findContainerArchInspect(cntId, ref string, extraOpts []string) (string, error) {
	// get image id first, then get the arch from the image,
	// it seems this is the most reliable way to get the
	// architecture
	imageId, err := findImageIdFor(cntId, ref, extraOpts)
	if err != nil {
		return "", err
	}

	args := []string{"inspect"}
	args = append(args, extraOpts...)
	args = append(args, "-f", "{{.Architecture}}", imageId)

	/* #nosec G204 */
	output, err := exec.Command("podman", args...).Output()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("inspecting container %q failed: %w\nstderr:\n%s", ref, err, err.Stderr)
		}
		return "", fmt.Errorf("inspecting %s container failed with generic error: %w", ref, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// getContainerSize returns the size of an already pulled container image in bytes
func getContainerSize(imgref string, extraOpts []string) (uint64, error) {
	args := []string{"image", "inspect"}
	args = append(args, extraOpts...)
	args = append(args, imgref, "--format", "{{.Size}}")

	output, err := exec.Command("podman", args...).Output()
	if err != nil {
		return 0, fmt.Errorf("failed inspect image: %w, output\n%s", err, output)
	}
	size, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse image size: %w", err)

	}

	return size, nil
}

// InitDNF initializes dnf in the container. This is necessary when
// the caller wants to read the image's dnf repositories, but they are
// not static, but rather configured by dnf dynamically. The primaru
// use-case for this is RHEL and subscription-manager.
//
// The implementation is simple: We just run plain `dnf` in the
// container so that the subscription-manager gets initialized. For
// compatibility with both dnf and dnf5 we cannot just run "dnf" as
// dnf5 will error and do nothing in this case. So we use "dnf check
// --duplicates" as this is fast on both dnf4/dnf5 (just doing "dnf5
// check" without arguments takes around 25s so that is not a great
// option).
func (c *Container) InitDNF() error {
	/* #nosec G204 */
	if err := exec.Command("podman", "exec", c.id, "sh", "-c", `command -v dnf`).Run(); err != nil {
		return ErrNoDnf
	}

	/* #nosec G204 */
	if output, err := exec.Command("podman", "exec", c.id, "dnf", "check", "--duplicates").CombinedOutput(); err != nil {
		return fmt.Errorf("initializing dnf in %s container failed: %w\noutput:\n%s", c.id, err, string(output))
	}

	return nil
}

func (cnt *Container) hasRunSecrets() bool {
	_, err := os.Stat(filepath.Join(cnt.root, "/run/secrets/redhat.repo"))
	return err == nil
}

// setupRunSecretsBindMount will synthesise a /run/secrets dir
// in the container root
func (cnt *Container) setupRunSecrets() error {
	if cnt.hasRunSecrets() {
		return nil
	}
	dst := filepath.Join(cnt.root, "/run/secrets")
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// We cannot just bind mount here because
	// /usr/share/rhel/secrets contains a bunch of relative symlinks
	// that will point to the container root not the host when resolved
	// from the outside (via the host container mount).
	//
	// So instead of bind mounting we create a copy of the
	// /run/secrets/ - they are static so that should be fine.
	//
	// We want to support /usr/share/rhel/secrets too to be able
	// to run "bootc-image-builder manifest" directly on the host
	// (which is useful for e.g. composer).
	for _, src := range []string{"/run/secrets", "/usr/share/rhel/secrets"} {
		if st, err := os.Stat(src); err != nil || !st.IsDir() {
			continue
		}

		dents, err := filepath.Glob(src + "/*")
		if err != nil {
			return err
		}
		for _, ent := range dents {
			// Check if the target file actually exists (i.e. for
			// symlinks that they are valid) and only copy if so.
			// This covers unsubscribed machines.
			if _, err := os.Stat(ent); err != nil {
				continue
			}

			// Note the use of "-L" here to dereference/copy links
			if output, err := exec.Command("cp", "-rvL", ent, dst).CombinedOutput(); err != nil {
				return fmt.Errorf("failed to setup /run/secrets: %w, output:\n%s", err, string(output))
			}
		}
	}

	// workaround broken containers (like f41) that use absolute symlinks
	// to point to the entitlements-host and rhsm-host, they need to be
	// relative so that the "SetRootdir()" from the resolver works, i.e.
	// they need to point into the mounted container.
	symlink := filepath.Join(cnt.root, "/etc/pki/entitlement-host")
	target := "../../run/secrets/etc-pki-entitlement"
	if err := forceSymlink(symlink, target); err != nil {
		return err
	}
	symlink = filepath.Join(cnt.root, "/etc/rhsm-host")
	target = "../run/secrets/rhsm"
	if err := forceSymlink(symlink, target); err != nil {
		return err
	}
	return nil
}

func (cnt *Container) NewContainerSolver(cacheRoot string, architecture arch.Arch, sourceInfo *osinfo.Info) (*depsolvednf.Solver, error) {
	solver := depsolvednf.NewSolver(
		sourceInfo.OSRelease.PlatformID,
		sourceInfo.OSRelease.VersionID,
		architecture.String(),
		fmt.Sprintf("%s-%s", sourceInfo.OSRelease.ID, sourceInfo.OSRelease.VersionID),
		cacheRoot)

	// we copy the data directly into the cnt.root, no need to
	// cleanup here because podman stop will remove the dir
	if err := cnt.setupRunSecrets(); err != nil {
		return nil, err
	}
	solver.SetRootDir(cnt.root)
	return solver, nil
}

var ErrNoDnf = errors.New("no dnf in container")

func forceSymlink(symlinkPath, target string) error {
	if output, err := exec.Command("ln", "-sf", target, symlinkPath).CombinedOutput(); err != nil {
		return fmt.Errorf("cannot run ln: %w, output:\n%s", err, output)
	}
	return nil
}

// ResolveBootcInfo resolves the bootc container reference and returns the info structure
func ResolveBootcInfo(ref string) (*Info, error) {
	c, err := NewContainer(ref)
	if err != nil {
		return nil, err
	}
	info, err := c.ResolveInfo()
	if stopErr := c.Stop(); stopErr != nil {
		if err != nil {
			err = fmt.Errorf("%w\nstopping the container failed too: %s", err, stopErr)
		} else {
			err = fmt.Errorf("stopping the container failed: %s", stopErr)
		}
	}
	return info, err
}
