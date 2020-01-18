// +build macos

package crypt

func crypt(pass, salt string) (string, error) {
	panic("You must not run osbuild-composer on macOS!")
	return "", nil
}
