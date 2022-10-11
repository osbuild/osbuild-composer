package weldr

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/sirupsen/logrus"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type uploadResponse struct {
	UUID         uuid.UUID              `json:"uuid"`
	Status       common.ImageBuildState `json:"status"`
	ProviderName string                 `json:"provider_name"`
	ImageName    string                 `json:"image_name"`
	CreationTime float64                `json:"creation_time"`
	Settings     uploadSettings         `json:"settings"`
}

type uploadSettings interface {
	isUploadSettings()
}

type awsUploadSettings struct {
	Region          string `json:"region"`
	AccessKeyID     string `json:"accessKeyID,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
	SessionToken    string `json:"sessionToken,omitempty"`
	Bucket          string `json:"bucket"`
	Key             string `json:"key"`
}

func (awsUploadSettings) isUploadSettings() {}

type awsS3UploadSettings struct {
	Region              string `json:"region"`
	AccessKeyID         string `json:"accessKeyID,omitempty"`
	SecretAccessKey     string `json:"secretAccessKey,omitempty"`
	SessionToken        string `json:"sessionToken,omitempty"`
	Bucket              string `json:"bucket"`
	Key                 string `json:"key"`
	Endpoint            string `json:"endpoint"`
	CABundle            string `json:"ca_bundle"`
	SkipSSLVerification bool   `json:"skip_ssl_verification"`
}

func (awsS3UploadSettings) isUploadSettings() {}

type azureUploadSettings struct {
	StorageAccount   string `json:"storageAccount,omitempty"`
	StorageAccessKey string `json:"storageAccessKey,omitempty"`
	Container        string `json:"container"`
}

func (azureUploadSettings) isUploadSettings() {}

type gcpUploadSettings struct {
	Region string `json:"region"`
	Bucket string `json:"bucket"`
	Object string `json:"object"`

	// base64 encoded GCP credentials JSON file
	Credentials string `json:"credentials,omitempty"`
}

func (gcpUploadSettings) isUploadSettings() {}

type vmwareUploadSettings struct {
	Host       string `json:"host"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Datacenter string `json:"datacenter"`
	Cluster    string `json:"cluster"`
	Datastore  string `json:"datastore"`
}

func (vmwareUploadSettings) isUploadSettings() {}

type ociUploadSettings struct {
	Tenancy     string `json:"tenancy"`
	Region      string `json:"region"`
	User        string `json:"user"`
	Bucket      string `json:"bucket"`
	Namespace   string `json:"namespace"`
	PrivateKey  string `json:"private_key"`
	Fingerprint string `json:"fingerprint"`
	Compartment string `json:"compartment"`
}

func (ociUploadSettings) isUploadSettings() {}

type containerUploadSettings struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	TlsVerify *bool `json:"tls_verify,omitempty"`
}

func (containerUploadSettings) isUploadSettings() {}

type uploadRequest struct {
	Provider  string         `json:"provider"`
	ImageName string         `json:"image_name"`
	Settings  uploadSettings `json:"settings"`
}

type rawUploadRequest struct {
	Provider  string          `json:"provider"`
	ImageName string          `json:"image_name"`
	Settings  json.RawMessage `json:"settings"`
}

func (u *uploadRequest) UnmarshalJSON(data []byte) error {
	var rawUploadRequest rawUploadRequest
	err := json.Unmarshal(data, &rawUploadRequest)
	if err != nil {
		return err
	}

	var settings uploadSettings
	switch rawUploadRequest.Provider {
	case "azure":
		settings = new(azureUploadSettings)
	case "aws":
		settings = new(awsUploadSettings)
	case "aws.s3":
		settings = new(awsS3UploadSettings)
	case "gcp":
		settings = new(gcpUploadSettings)
	case "vmware":
		settings = new(vmwareUploadSettings)
	case "oci":
		settings = new(ociUploadSettings)
	case "generic.s3":
		// While the API still accepts provider type "generic.s3", the request is handled
		// in the same way as for a request with provider type "aws.s3"
		settings = new(awsS3UploadSettings)
	case "container":
		settings = new(containerUploadSettings)
	default:
		return errors.New("unexpected provider name")
	}
	err = json.Unmarshal(rawUploadRequest.Settings, settings)
	if err != nil {
		return err
	}

	u.Provider = rawUploadRequest.Provider
	u.ImageName = rawUploadRequest.ImageName
	u.Settings = settings

	return err
}

// Converts a `Target` to a serializable `uploadResponse`.
//
// This ignore the status in `targets`, because that's never set correctly.
// Instead, it sets each target's status to the ImageBuildState equivalent of
// `state`.
//
// This also ignores any sensitive data passed into targets. Access keys may
// be passed as input to composer, but should not be possible to be queried.
func targetsToUploadResponses(targets []*target.Target, state ComposeState) []uploadResponse {
	var uploads []uploadResponse
	for _, t := range targets {
		upload := uploadResponse{
			UUID:         t.Uuid,
			ImageName:    t.ImageName,
			CreationTime: float64(t.Created.UnixNano()) / 1000000000,
		}

		switch state {
		case ComposeWaiting:
			upload.Status = common.IBWaiting
		case ComposeRunning:
			upload.Status = common.IBRunning
		case ComposeFinished:
			upload.Status = common.IBFinished
		case ComposeFailed:
			upload.Status = common.IBFailed
		}

		switch options := t.Options.(type) {
		case *target.AWSTargetOptions:
			upload.ProviderName = "aws"
			upload.Settings = &awsUploadSettings{
				Region: options.Region,
				Bucket: options.Bucket,
				Key:    options.Key,
				// AccessKeyID and SecretAccessKey are intentionally not included.
			}
			uploads = append(uploads, upload)
		case *target.AzureTargetOptions:
			upload.ProviderName = "azure"
			upload.Settings = &azureUploadSettings{
				Container: options.Container,
				// StorageAccount and StorageAccessKey are intentionally not included.
			}
			uploads = append(uploads, upload)
		case *target.GCPTargetOptions:
			upload.ProviderName = "gcp"
			upload.Settings = &gcpUploadSettings{
				Region: options.Region,
				Bucket: options.Bucket,
				Object: options.Object,
				// Credentials are intentionally not included.
			}
			uploads = append(uploads, upload)
		case *target.VMWareTargetOptions:
			upload.ProviderName = "vmware"
			upload.Settings = &vmwareUploadSettings{
				Host:       options.Host,
				Cluster:    options.Cluster,
				Datacenter: options.Datacenter,
				Datastore:  options.Datastore,
				// Username and Password are intentionally not included.
			}
			uploads = append(uploads, upload)
		case *target.AWSS3TargetOptions:
			upload.ProviderName = "aws.s3"
			upload.Settings = &awsS3UploadSettings{
				Region: options.Region,
				Bucket: options.Bucket,
				Key:    options.Key,
				// AccessKeyID and SecretAccessKey are intentionally not included.
			}
			uploads = append(uploads, upload)
		}
	}

	return uploads
}

func uploadRequestToTarget(u uploadRequest, imageType distro.ImageType) *target.Target {
	var t target.Target

	t.Uuid = uuid.New()
	t.ImageName = u.ImageName
	t.OsbuildArtifact.ExportFilename = imageType.Filename()
	t.OsbuildArtifact.ExportName = imageType.Exports()[0]
	t.Status = common.IBWaiting
	t.Created = time.Now()

	switch options := u.Settings.(type) {
	case *awsUploadSettings:
		key := options.Key
		if key == "" {
			key = fmt.Sprintf("composer-api-%s", uuid.New().String())
		}
		t.Name = target.TargetNameAWS
		t.Options = &target.AWSTargetOptions{
			Region:          options.Region,
			AccessKeyID:     options.AccessKeyID,
			SecretAccessKey: options.SecretAccessKey,
			SessionToken:    options.SessionToken,
			Bucket:          options.Bucket,
			Key:             key,
		}
	case *awsS3UploadSettings:
		key := options.Key
		if key == "" {
			key = fmt.Sprintf("composer-api-%s", uuid.New().String())
		}
		t.Name = target.TargetNameAWSS3
		t.Options = &target.AWSS3TargetOptions{
			Region:              options.Region,
			AccessKeyID:         options.AccessKeyID,
			SecretAccessKey:     options.SecretAccessKey,
			SessionToken:        options.SessionToken,
			Bucket:              options.Bucket,
			Key:                 key,
			Endpoint:            options.Endpoint,
			CABundle:            options.CABundle,
			SkipSSLVerification: options.SkipSSLVerification,
		}
	case *azureUploadSettings:
		t.Name = target.TargetNameAzure
		t.Options = &target.AzureTargetOptions{
			StorageAccount:   options.StorageAccount,
			StorageAccessKey: options.StorageAccessKey,
			Container:        options.Container,
		}
	case *gcpUploadSettings:
		t.Name = target.TargetNameGCP

		var gcpCredentials []byte
		var err error
		if options.Credentials != "" {
			gcpCredentials, err = base64.StdEncoding.DecodeString(options.Credentials)
			if err != nil {
				panic(err)
			}
		}

		// The uploaded image object name must have 'tar.gz' suffix to be imported
		objectName := options.Object
		if !strings.HasSuffix(objectName, ".tar.gz") {
			objectName = objectName + ".tar.gz"
			logrus.Infof("[GCP] object name must end with '.tar.gz', using %q as the object name", objectName)
		}

		t.Options = &target.GCPTargetOptions{
			Region:      options.Region,
			Os:          imageType.Arch().Distro().Name(),
			Bucket:      options.Bucket,
			Object:      objectName,
			Credentials: gcpCredentials,
		}
	case *vmwareUploadSettings:
		t.Name = target.TargetNameVMWare
		t.Options = &target.VMWareTargetOptions{
			Username:   options.Username,
			Password:   options.Password,
			Host:       options.Host,
			Cluster:    options.Cluster,
			Datacenter: options.Datacenter,
			Datastore:  options.Datastore,
		}
	case *ociUploadSettings:
		t.Name = target.TargetNameOCI
		t.Options = &target.OCITargetOptions{
			User:        options.User,
			Tenancy:     options.Tenancy,
			Region:      options.Region,
			PrivateKey:  options.PrivateKey,
			Fingerprint: options.Fingerprint,
			Bucket:      options.Bucket,
			Namespace:   options.Namespace,
			Compartment: options.Compartment,
		}
	case *containerUploadSettings:
		t.Name = target.TargetNameContainer
		t.Options = &target.ContainerTargetOptions{
			Username: options.Username,
			Password: options.Password,

			TlsVerify: options.TlsVerify,
		}
	}

	return &t
}
