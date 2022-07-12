package osbuild

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewCloudInitStage(t *testing.T) {
	expectedStage := &Stage{
		Type: "org.osbuild.cloud-init",
		Options: &CloudInitStageOptions{
			Filename: "aaa",
			Config: CloudInitConfigFile{
				SystemInfo: &CloudInitConfigSystemInfo{
					DefaultUser: &CloudInitConfigDefaultUser{
						Name: "foo",
					},
				},
			},
		},
	}
	actualStage := NewCloudInitStage(&CloudInitStageOptions{
		Filename: "aaa",
		Config: CloudInitConfigFile{
			SystemInfo: &CloudInitConfigSystemInfo{
				DefaultUser: &CloudInitConfigDefaultUser{
					Name: "foo",
				},
			},
		},
	})
	assert.Equal(t, expectedStage, actualStage)
}

func TestCloudInitStage_NewStage_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options CloudInitStageOptions
	}{
		{
			name:    "empty-options",
			options: CloudInitStageOptions{},
		},
		{
			name: "no-config-file-section",
			options: CloudInitStageOptions{
				Filename: "00-default_user.cfg",
				Config:   CloudInitConfigFile{},
			},
		},
		{
			name: "no-system-info-section-option",
			options: CloudInitStageOptions{
				Filename: "00-default_user.cfg",
				Config: CloudInitConfigFile{
					SystemInfo: &CloudInitConfigSystemInfo{},
				},
			},
		},
		{
			name: "no-default-user-section-option",
			options: CloudInitStageOptions{
				Filename: "00-default_user.cfg",
				Config: CloudInitConfigFile{
					SystemInfo: &CloudInitConfigSystemInfo{
						DefaultUser: &CloudInitConfigDefaultUser{},
					},
				},
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() { NewCloudInitStage(&tt.options) }, "NewCloudInitStage didn't panic, but it should [idx: %d]", idx)
		})
	}
}

func TestCloudInitStage_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		options CloudInitStageOptions
		json    string
	}{
		{
			name: "simple-cloud-init-config-with-system-info",
			options: CloudInitStageOptions{
				Config: CloudInitConfigFile{
					SystemInfo: &CloudInitConfigSystemInfo{
						DefaultUser: &CloudInitConfigDefaultUser{
							Name: "foo",
						},
					},
				},
			},
			json: `{"filename":"","config":{"system_info":{"default_user":{"name":"foo"}}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := json.Marshal(tt.options)
			assert.NoError(t, err)
			assert.Equal(t, tt.json, string(gotBytes))
		})
	}
}

func TestCloudInitStage_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		options CloudInitStageOptions
		json    string
	}{
		{
			name: "simple-cloud-init-config-with-system-info",
			options: CloudInitStageOptions{
				Config: CloudInitConfigFile{
					SystemInfo: &CloudInitConfigSystemInfo{
						DefaultUser: &CloudInitConfigDefaultUser{
							Name: "foo",
						},
					},
				},
			},
			json: `{"filename":"","config":{"system_info":{"default_user":{"name":"foo"}}}}`,
		},
		{
			name: "osbuild-test-suite-1",
			options: CloudInitStageOptions{
				Filename: "10-azure-kfp.cfg",
				Config: CloudInitConfigFile{
					Reporting: &CloudInitConfigReporting{
						Logging: &CloudInitConfigReportingHandlers{
							Type: "log",
						},
						Telemetry: &CloudInitConfigReportingHandlers{
							Type: "hyperv",
						},
					},
				},
			},
			json: `{
				"filename": "10-azure-kfp.cfg",
				"config": {
				  "reporting": {
					"logging": {
					  "type": "log"
					},
					"telemetry": {
					  "type": "hyperv"
					}
				  }
				}
			  }`,
		},
		{
			name: "osbuild-test-suite-2",
			options: CloudInitStageOptions{
				Filename: "91-azure_datasource.cfg",
				Config: CloudInitConfigFile{
					DatasourceList: []string{"Azure"},
					Datasource: &CloudInitConfigDatasource{
						Azure: &CloudInitConfigDatasourceAzure{
							ApplyNetworkConfig: false,
						},
					},
				},
			},
			json: `{
				"filename": "91-azure_datasource.cfg",
				"config": {
				  "datasource_list": [
					"Azure"
				  ],
				  "datasource": {
					"Azure": {
					  "apply_network_config": false
					}
				  }
				}
			  }`,
		},
		{
			name: "osbuild-test-suite-3",
			options: CloudInitStageOptions{
				Filename: "06_logging_override.cfg",
				Config: CloudInitConfigFile{
					Output: &CloudInitConfigOutput{
						All: common.StringToPtr(">> /var/log/cloud-init-all.log"),
					},
				},
			},
			json: `{
				"filename": "06_logging_override.cfg",
				"config": {
				  "output": {
					"all": ">> /var/log/cloud-init-all.log"
				  }
				}
			  }`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotOptions CloudInitStageOptions
			err := json.Unmarshal([]byte(tt.json), &gotOptions)
			assert.NoError(t, err)
			assert.True(t, reflect.DeepEqual(tt.options, gotOptions))
		})
	}
}
