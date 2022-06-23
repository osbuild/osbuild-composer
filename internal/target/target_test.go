package target

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test that `Filename` set in the `Target` options gets set also in the
// `Target.ExportFilename`.
// This covers the case when new worker receives a job from old composer.
// This covers the case when new worker receives a job from new composer.
func TestTargetOptionsFilenameCompatibilityUnmarshal(t *testing.T) {
	testCases := []struct {
		targetJSON     []byte
		expectedTarget *Target
	}{
		// Test that Filename set in the target options gets set also in the ExportFilename
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.aws","options":{"filename":"image.qcow2"}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name:    TargetNameAWS,
				Options: &AWSTargetOptions{},
			},
		},
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.aws.s3","options":{"region":"eu","accessKeyID":"id","secretAccessKey":"key","sessionToken":"token","bucket":"bkt","key":"key","endpoint":"endpoint","ca_bundle":"bundle","skip_ssl_verification":true,"filename":"image.qcow2"}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name: TargetNameAWSS3,
				Options: &AWSS3TargetOptions{
					Region:              "eu",
					AccessKeyID:         "id",
					SecretAccessKey:     "key",
					SessionToken:        "token",
					Bucket:              "bkt",
					Key:                 "key",
					Endpoint:            "endpoint",
					CABundle:            "bundle",
					SkipSSLVerification: true,
				},
			},
		},
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.azure","options":{"storageAccount":"account","storageAccessKey":"key","container":"container","filename":"image.qcow2"}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name: TargetNameAzure,
				Options: &AzureTargetOptions{
					StorageAccount:   "account",
					StorageAccessKey: "key",
					Container:        "container",
				},
			},
		},
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.azure.image","options":{"tenant_id":"tenant","location":"location","subscription_id":"id","resource_group":"group","filename":"image.qcow2"}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name: TargetNameAzureImage,
				Options: &AzureImageTargetOptions{
					TenantID:       "tenant",
					Location:       "location",
					SubscriptionID: "id",
					ResourceGroup:  "group",
				},
			},
		},
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.gcp","options":{"region":"eu","os":"rhel-8","bucket":"bkt","object":"obj","shareWithAccounts":["account@domain.org"],"credentials":"","filename":"image.qcow2"}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name: TargetNameGCP,
				Options: &GCPTargetOptions{
					Region:            "eu",
					Os:                "rhel-8",
					Bucket:            "bkt",
					Object:            "obj",
					ShareWithAccounts: []string{"account@domain.org"},
					Credentials:       []byte(""),
				},
			},
		},
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.koji","options":{"upload_directory":"koji-dir","server":"koji.example.org","filename":"image.qcow2"}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name: TargetNameKoji,
				Options: &KojiTargetOptions{
					UploadDirectory: "koji-dir",
					Server:          "koji.example.org",
				},
			},
		},
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.oci","options":{"user":"user","tenancy":"tenant","region":"eu","fingerprint":"finger","private_key":"key","bucket":"bkt","namespace":"space","compartment_id":"compartment","filename":"image.qcow2"}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name: TargetNameOCI,
				Options: &OCITargetOptions{
					User:        "user",
					Tenancy:     "tenant",
					Region:      "eu",
					Fingerprint: "finger",
					PrivateKey:  "key",
					Bucket:      "bkt",
					Namespace:   "space",
					Compartment: "compartment",
				},
			},
		},
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.vmware","options":{"host":"example.org","username":"user","password":"pass","datacenter":"center","cluster":"cluster","datastore":"store","filename":"image.qcow2"}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name: TargetNameVMWare,
				Options: &VMWareTargetOptions{
					Host:       "example.org",
					Username:   "user",
					Password:   "pass",
					Datacenter: "center",
					Cluster:    "cluster",
					Datastore:  "store",
				},
			},
		},
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.container","options":{"reference":"ref","username":"user","password":"pass","filename":"image.qcow2"}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name: TargetNameContainer,
				Options: &ContainerTargetOptions{
					Reference: "ref",
					Username:  "user",
					Password:  "pass",
				},
			},
		},
		// Test that the job as Marshalled by the current compatibility code is also acceptable.
		// Such job has Filename set in the Target options, as well in the ExportFilename.
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.aws","export_filename":"image.qcow2","options":{"filename":"image.qcow2"}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name:    TargetNameAWS,
				Options: &AWSTargetOptions{},
			},
		},
		// Test the case if the compatibility code for Filename in the target options was dropped.
		{
			targetJSON: []byte(`{"image_name":"my-image","name":"org.osbuild.aws","osbuild_artifact":{"export_filename":"image.qcow2"},"options":{}}`),
			expectedTarget: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name:    TargetNameAWS,
				Options: &AWSTargetOptions{},
			},
		},
	}

	for idx, testCase := range testCases {
		t.Run(fmt.Sprintf("Case #%d", idx), func(t *testing.T) {
			gotTarget := Target{}
			err := json.Unmarshal(testCase.targetJSON, &gotTarget)
			assert.NoError(t, err)
			assert.EqualValues(t, testCase.expectedTarget, &gotTarget)
		})
	}
}

// Test that that ExportFilename set in the Target get added to the options
// as Filename.
// This enables old worker to still pick and be able to handle jobs from new composer.
func TestTargetOptionsFilenameCompatibilityMarshal(t *testing.T) {
	testCases := []struct {
		targetJSON []byte
		target     *Target
	}{
		{
			targetJSON: []byte(`{"uuid":"00000000-0000-0000-0000-000000000000","image_name":"my-image","name":"org.osbuild.aws","created":"0001-01-01T00:00:00Z","status":"WAITING","options":{"region":"us","accessKeyID":"id","secretAccessKey":"key","sessionToken":"token","bucket":"bkt","key":"key","shareWithAccounts":["123456789"],"filename":"image.qcow2"},"osbuild_artifact":{"export_filename":"image.qcow2","export_name":""}}`),
			target: &Target{
				ImageName: "my-image",
				OsbuildArtifact: OsbuildArtifact{
					ExportFilename: "image.qcow2",
				},
				Name: TargetNameAWS,
				Options: &AWSTargetOptions{
					Region:            "us",
					AccessKeyID:       "id",
					SecretAccessKey:   "key",
					SessionToken:      "token",
					Bucket:            "bkt",
					Key:               "key",
					ShareWithAccounts: []string{"123456789"},
				},
			},
		},
	}

	for idx, testCase := range testCases {
		t.Run(fmt.Sprintf("Case #%d", idx), func(t *testing.T) {
			gotJSON, err := json.Marshal(testCase.target)
			assert.Nil(t, err)
			t.Logf("%s\n", gotJSON)
			assert.EqualValues(t, testCase.targetJSON, gotJSON)
		})
	}
}
