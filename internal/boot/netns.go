//go:build integration

package boot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"
)

const netnsDir = "/var/run/netns"

// Network namespace abstraction
type NetNS string

// newNetworkNamespace returns a new network namespace with a random
// name. The calling goroutine remains in the same namespace
// as before the call.
func newNetworkNamespace() (NetNS, error) {
	// This method needs to unshare the current thread. Go runtime can switch
	// the goroutine to run on a different thread at any point, so we need
	// to ensure that this method runs in the same thread for its whole
	// lifetime.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	_, err := os.Stat(netnsDir)

	if err != nil {
		if os.IsNotExist(err) {
			err := os.Mkdir(netnsDir, 0755)
			if err != nil {
				return "", fmt.Errorf("cannot create %s: %v", netnsDir, err)
			}
		} else {
			return "", fmt.Errorf("cannot stat %s: %v", netnsDir, err)
		}
	}

	f, err := os.CreateTemp(netnsDir, "osbuild-composer-namespace")
	if err != nil {
		return "", fmt.Errorf("cannot create a tempfile: %v", err)
	}

	// We want to remove the temporary file if the namespace initialization fails.
	// The best method I could thought of is to have the following variable
	// denoting if the initialization was successful. It is set to true right
	// before the end of this function.
	initOK := false
	defer func() {
		if !initOK {
			err := os.Remove(f.Name())
			if err != nil {
				log.Printf("cannot remove the temporary namespace: %v", err)
			}
		}
	}()

	oldNS, err := os.Open("/proc/self/ns/net")
	if err != nil {
		return "", fmt.Errorf("cannot open the current namespace: %v", err)
	}

	err = syscall.Unshare(syscall.CLONE_NEWNET)
	if err != nil {
		return "", fmt.Errorf("cannot unshare the network namespace")
	}
	defer func() {
		// The Fd() is actually an int cast into a uintptr, so casting back to an int is fine
		/* #nosec G115 */
		err = unix.Setns(int(oldNS.Fd()), syscall.CLONE_NEWNET)
		if err != nil {
			// We cannot return to the original namespace.
			// As we don't know nothing about affected threads, let's just
			// quit immediately.
			log.Fatalf("returning to the original namespace failed, quitting: %v", err)
		}
	}()

	cmd := exec.Command("ip", "link", "set", "up", "dev", "lo")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("cannot set up a loopback device in the new namespace: %v", err)
	}

	// There's no potential command injection vector here
	/* #nosec G204 */
	cmd = exec.Command("mount", "-o", "bind", "/proc/self/ns/net", f.Name())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("cannot bind mount the new namespace: %v", err)
	}

	ns := NetNS(path.Base(f.Name()))

	// Initialization OK, do not delete the namespace file.
	initOK = true
	return ns, nil
}

// NamespaceCommand returns an *exec.Cmd struct with the difference
// that it's prepended by "ip netns exec NAMESPACE_NAME" command, which
// runs the command in a namespaced environment.
func (n NetNS) NamespacedCommand(name string, arg ...string) *exec.Cmd {
	args := []string{"netns", "exec", string(n), name}
	args = append(args, arg...)
	return exec.Command("ip", args...)
}

// NamespaceCommand returns an *exec.Cmd struct with the difference
// that it's prepended by "ip netns exec NAMESPACE_NAME" command, which
// runs the command in a namespaced environment.
func (n NetNS) NamespacedCommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	args := []string{"netns", "exec", string(n), name}
	args = append(args, arg...)
	return exec.CommandContext(ctx, "ip", args...)
}

// Path returns the path to the namespace file
func (n NetNS) Path() string {
	return path.Join(netnsDir, string(n))
}

// Delete deletes the namespaces
func (n NetNS) Delete() error {
	// There's no potential command injection vector here
	/* #nosec G204 */
	cmd := exec.Command("umount", n.Path())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("cannot unmount the network namespace: %v", err)
	}

	err = os.Remove(n.Path())
	if err != nil {
		return fmt.Errorf("cannot delete the network namespace file: %v", err)
	}

	return nil
}
