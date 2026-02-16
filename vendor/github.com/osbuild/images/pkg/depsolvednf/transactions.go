package depsolvednf

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/osbuild/images/pkg/rpmmd"
)

// TransactionList represents an ordered list of package transactions.
// Each transaction contains packages that should be installed together
// in a single RPM stage. Transactions must be installed in order.
type TransactionList []rpmmd.PackageList

// AllPackages returns a flat list of all packages across all transactions,
// sorted by full NEVRA for deterministic ordering.
func (t TransactionList) AllPackages() rpmmd.PackageList {
	var all rpmmd.PackageList
	for _, pkgs := range t {
		all = append(all, pkgs...)
	}
	slices.SortFunc(all, func(a, b rpmmd.Package) int {
		return cmp.Compare(a.FullNEVRA(), b.FullNEVRA())
	})
	return all
}

// FindPackage searches for a package by name across all transactions.
func (t TransactionList) FindPackage(name string) (*rpmmd.Package, error) {
	for i := range t {
		for j := range t[i] {
			if t[i][j].Name == name {
				return &t[i][j], nil
			}
		}
	}
	return nil, fmt.Errorf("package %q not found in transactions", name)
}

// TransactionFileInfo contains information about a file provided by a package
// within a transaction list.
type TransactionFileInfo struct {
	// Path is the file path that was looked up
	Path string
	// Package that provides the file
	Package *rpmmd.Package
	// TxIndex is the transaction index where the file becomes available
	TxIndex int
}

// GetFilesTransactionInfo searches for packages that provide any of the given
// file paths. Returns a map from file path to TransactionFileInfo, containing
// only the files that were found. Files not provided by any package are not
// included in the result. If a file is provided by multiple packages,
// potentially from different transactions, the first occurrence is returned.
func (t TransactionList) GetFilesTransactionInfo(paths []string) map[string]TransactionFileInfo {
	if len(paths) == 0 {
		return nil
	}

	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	result := make(map[string]TransactionFileInfo)
	for txIdx := range t {
		for pkgIdx := range t[txIdx] {
			for _, f := range t[txIdx][pkgIdx].Files {
				if pathSet[f] {
					result[f] = TransactionFileInfo{
						Path:    f,
						Package: &t[txIdx][pkgIdx],
						TxIndex: txIdx,
					}
					delete(pathSet, f)
					if len(pathSet) == 0 {
						return result
					}
				}
			}
		}
	}
	return result
}
