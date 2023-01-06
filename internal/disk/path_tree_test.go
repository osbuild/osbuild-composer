package disk

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/osbuild-composer/internal/common"
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
				"/": common.ToPtr(1),
			},
			trie: &PathTrie{
				Name:    []string{},
				Payload: common.ToPtr(1),
			},
		},
		{
			entries: map[string]interface{}{
				"/":                            common.ToPtr(1),
				"/var":                         common.ToPtr(2),
				"/var/lib/chrony":              common.ToPtr(3),
				"/var/lib/chrony/logs":         common.ToPtr(4),
				"/var/lib/osbuild":             common.ToPtr(5),
				"/var/lib/osbuild/store/cache": common.ToPtr(6),
				"/boot":                        common.ToPtr(7),
				"/boot/efi":                    common.ToPtr(8),
			},
			trie: &PathTrie{
				Name:    []string{},
				Payload: common.ToPtr(1),
				Paths: []*PathTrie{
					{
						Name:    []string{"boot"},
						Payload: common.ToPtr(7),
						Paths: []*PathTrie{
							{
								Name:    []string{"efi"},
								Payload: common.ToPtr(8),
							},
						},
					},
					{
						Name:    []string{"var"},
						Payload: common.ToPtr(2),
						Paths: []*PathTrie{
							{
								Name:    []string{"lib", "chrony"},
								Payload: common.ToPtr(3),
								Paths: []*PathTrie{
									{
										Name:    []string{"logs"},
										Payload: common.ToPtr(4),
									},
								},
							},
							{
								Name:    []string{"lib", "osbuild"},
								Payload: common.ToPtr(5),
								Paths: []*PathTrie{
									{
										Name:    []string{"store", "cache"},
										Payload: common.ToPtr(6),
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
