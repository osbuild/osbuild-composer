package target

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKojiTargetOptionsUnmarshalJSON tests that the resulting struct
// has appropriate fields set when legacy JSON is used.
func TestKojiTargetResultOptionsUnmarshalJSON(t *testing.T) {
	type testCase struct {
		name           string
		JSON           []byte
		expectedResult *KojiTargetResultOptions
		err            bool
	}

	testCases := []testCase{
		{
			name: "new format",
			JSON: []byte(`{"image":{"checksum_type":"md5","checksum":"hash","filename":"image.raw","size":123456}}`),
			expectedResult: &KojiTargetResultOptions{
				Image: &KojiOutputInfo{
					Filename:     "image.raw",
					ChecksumType: "md5",
					Checksum:     "hash",
					Size:         123456,
				},
			},
		},
		{
			name: "old format",
			JSON: []byte(`{"image_md5":"hash","image_size":123456}`),
			expectedResult: &KojiTargetResultOptions{
				Image: &KojiOutputInfo{
					ChecksumType: "md5",
					Checksum:     "hash",
					Size:         123456,
				},
			},
		},
		{
			name: "full format",
			JSON: []byte(`{"image":{"checksum_type":"md5","checksum":"hash","filename":"image.raw","size":123456},"log":{"checksum_type":"md5","checksum":"hash","filename":"log.txt","size":123456},"osbuild_manifest":{"checksum_type":"md5","checksum":"hash","filename":"manifest.json","size":123456}}`),
			expectedResult: &KojiTargetResultOptions{
				Image: &KojiOutputInfo{
					Filename:     "image.raw",
					ChecksumType: "md5",
					Checksum:     "hash",
					Size:         123456,
				},
				Log: &KojiOutputInfo{
					Filename:     "log.txt",
					ChecksumType: "md5",
					Checksum:     "hash",
					Size:         123456,
				},
				OSBuildManifest: &KojiOutputInfo{
					Filename:     "manifest.json",
					ChecksumType: "md5",
					Checksum:     "hash",
					Size:         123456,
				},
			},
		},
		{
			name: "invalid JSON",
			JSON: []byte(`{"image_md5":"hash","image_size":123456`),
			err:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result KojiTargetResultOptions
			err := json.Unmarshal(tc.JSON, &result)
			if tc.err {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedResult, &result)
		})
	}
}

// TestKojiTargetResultOptionsMarshalJSON tests that the resulting JSON
// has the legacy fields set for backwards compatibility.
func TestKojiTargetResultOptionsMarshalJSON(t *testing.T) {
	type testCase struct {
		name         string
		results      *KojiTargetResultOptions
		expectedJSON []byte
		err          bool
	}

	testCases := []testCase{
		{
			name: "backwards compatibility",
			results: &KojiTargetResultOptions{
				Image: &KojiOutputInfo{
					Filename:     "image.raw",
					ChecksumType: ChecksumTypeMD5,
					Checksum:     "hash",
					Size:         123456,
				},
			},
			expectedJSON: []byte(`{"image":{"filename":"image.raw","checksum_type":"md5","checksum":"hash","size":123456},"image_md5":"hash","image_size":123456}`),
		},
		{
			name: "full format",
			results: &KojiTargetResultOptions{
				Image: &KojiOutputInfo{
					Filename:     "image.raw",
					ChecksumType: ChecksumTypeMD5,
					Checksum:     "hash",
					Size:         123456,
				},
				Log: &KojiOutputInfo{
					Filename:     "log.txt",
					ChecksumType: ChecksumTypeMD5,
					Checksum:     "hash",
					Size:         654321,
				},
				OSBuildManifest: &KojiOutputInfo{
					Filename:     "manifest.json",
					ChecksumType: ChecksumTypeMD5,
					Checksum:     "hash",
					Size:         123321,
				},
			},
			expectedJSON: []byte(`{"image":{"filename":"image.raw","checksum_type":"md5","checksum":"hash","size":123456},"log":{"filename":"log.txt","checksum_type":"md5","checksum":"hash","size":654321},"osbuild_manifest":{"filename":"manifest.json","checksum_type":"md5","checksum":"hash","size":123321},"image_md5":"hash","image_size":123456}`),
		},
		{
			name: "invalid checksum type",
			results: &KojiTargetResultOptions{
				Image: &KojiOutputInfo{
					Filename:     "image.raw",
					ChecksumType: "sha256",
					Checksum:     "hash",
					Size:         123456,
				},
			},
			err: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := json.Marshal(tc.results)
			if tc.err {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedJSON, result)
		})
	}
}
