package disk

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewPathTrieFromMap(t *testing.T) {
	assert := assert.New(t)

	type testCase struct {
		entries map[string]interface{}
		trie    *PathTrie
	}

	tests := []testCase{
		{
			entries: map[string]interface{}{},
			trie: &PathTrie{
				Name: []string{},
			},
		},
		{
			entries: map[string]interface{}{
				"/": common.IntToPtr(1),
			},
			trie: &PathTrie{
				Name:    []string{},
				Payload: common.IntToPtr(1),
			},
		},
		{
			entries: map[string]interface{}{
				"/":                            common.IntToPtr(1),
				"/var":                         common.IntToPtr(2),
				"/var/lib/chrony":              common.IntToPtr(3),
				"/var/lib/chrony/logs":         common.IntToPtr(4),
				"/var/lib/osbuild":             common.IntToPtr(5),
				"/var/lib/osbuild/store/cache": common.IntToPtr(6),
				"/boot":                        common.IntToPtr(7),
				"/boot/efi":                    common.IntToPtr(8),
			},
			trie: &PathTrie{
				Name:    []string{},
				Payload: common.IntToPtr(1),
				Paths: []*PathTrie{
					{
						Name:    []string{"boot"},
						Payload: common.IntToPtr(7),
						Paths: []*PathTrie{
							{
								Name:    []string{"efi"},
								Payload: common.IntToPtr(8),
							},
						},
					},
					{
						Name:    []string{"var"},
						Payload: common.IntToPtr(2),
						Paths: []*PathTrie{
							{
								Name:    []string{"lib", "chrony"},
								Payload: common.IntToPtr(3),
								Paths: []*PathTrie{
									{
										Name:    []string{"logs"},
										Payload: common.IntToPtr(4),
									},
								},
							},
							{
								Name:    []string{"lib", "osbuild"},
								Payload: common.IntToPtr(5),
								Paths: []*PathTrie{
									{
										Name:    []string{"store", "cache"},
										Payload: common.IntToPtr(6),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		have := NewPathTrieFromMap(tc.entries)
		assert.NotNil(have)
		assert.Equal(tc.trie, have)
	}
}

func TestPathTrieLookup(t *testing.T) {
	assert := assert.New(t)

	entries := map[string]interface{}{
		"/":                            "/",
		"/boot":                        "/boot",
		"/boot/efi":                    "/boot/efi",
		"/var":                         "/var",
		"/var/lib/osbuild":             "/var/lib/osbuild",
		"/var/lib/osbuild/store/cache": "/var/lib/osbuild/store/cache",
		"/var/lib/chrony":              "/var/lib/chrony",
		"/var/lib/chrony/logs":         "/var/lib/chrony/logs",
	}

	trie := NewPathTrieFromMap(entries)

	testCases := map[string]string{
		"/":                         "/",
		"/srv":                      "/",
		"/srv/data":                 "/",
		"/boot":                     "/boot",
		"/boot/efi":                 "/boot/efi",
		"/boot/grub2":               "/boot",
		"/boot/efi/fedora":          "/boot/efi",
		"/var/lib/osbuild":          "/var/lib/osbuild",
		"/var/lib/osbuild/test":     "/var/lib/osbuild",
		"/var/lib/chrony":           "/var/lib/chrony",
		"/var/lib/chrony/test":      "/var/lib/chrony",
		"/var/lib/chrony/logs":      "/var/lib/chrony/logs",
		"/var/lib/chrony/logs/data": "/var/lib/chrony/logs",
	}

	for k, v := range testCases {
		node, _ := trie.Lookup(k)
		assert.NotNil(node)
		assert.Equal(v, node.Payload, "Lookup path: '%s' (%+v)", k, node.Name)
	}
}
