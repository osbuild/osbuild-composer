package weldr

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/osbuild/osbuild-composer/internal/common"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type UploadResponse struct {
	Uuid         uuid.UUID              `json:"uuid"`
	Status       common.ImageBuildState `json:"status"`
	ProviderName string                 `json:"provider_name"`
	ImageName    string                 `json:"image_name"`
	CreationTime float64                `json:"creation_time"`
	Settings     target.TargetOptions   `json:"settings"`
}

type UploadRequest struct {
	Provider  string               `json:"provider"`
	ImageName string               `json:"image_name"`
	Settings  target.TargetOptions `json:"settings"`
}

type rawUploadRequest struct {
	Provider  string          `json:"provider"`
	ImageName string          `json:"image_name"`
	Settings  json.RawMessage `json:"settings"`
}

func (u *UploadRequest) UnmarshalJSON(data []byte) error {
	var rawUpload rawUploadRequest
	err := json.Unmarshal(data, &rawUpload)
	if err != nil {
		return err
	}

	// we need to convert provider name to target name to use the unmarshaller
	targetName := providerNameToTargetNameMap[rawUpload.Provider]
	options, err := target.UnmarshalTargetOptions(targetName, rawUpload.Settings)

	u.Provider = rawUpload.Provider
	u.ImageName = rawUpload.ImageName
	u.Settings = options

	return err
}

var targetNameToProviderNameMap = map[string]string{
	"org.osbuild.aws":   "aws",
	"org.osbuild.azure": "azure",
}

var providerNameToTargetNameMap = map[string]string{
	"aws":   "org.osbuild.aws",
	"azure": "org.osbuild.azure",
}

func TargetsToUploadResponses(targets []*target.Target) []UploadResponse {
	var uploads []UploadResponse
	for _, t := range targets {
		if t.Name == "org.osbuild.local" {
			continue
		}

		providerName, providerExist := targetNameToProviderNameMap[t.Name]
		if !providerExist {
			panic("target name " + t.Name + " is not defined in conversion map!")
		}
		upload := UploadResponse{
			Uuid:         t.Uuid,
			Status:       t.Status,
			ProviderName: providerName,
			ImageName:    t.ImageName,
			CreationTime: float64(t.Created.UnixNano()) / 1000000000,
		}

		switch options := t.Options.(type) {
		case *target.LocalTargetOptions:
			continue
		case *target.AWSTargetOptions:
			upload.Settings = &target.AWSTargetOptions{
				Region:          options.Region,
				AccessKeyID:     options.AccessKeyID,
				SecretAccessKey: options.SecretAccessKey,
				Bucket:          options.Bucket,
				Key:             options.Key,
			}
		case *target.AzureTargetOptions:
			upload.Settings = &target.AzureTargetOptions{
				Account:   options.Account,
				AccessKey: options.AccessKey,
				Container: options.Container,
			}
		}
		uploads = append(uploads, upload)
	}

	return uploads
}

func UploadRequestToTarget(u UploadRequest) (*target.Target, error) {
	var t target.Target
	targetName, targetExist := providerNameToTargetNameMap[u.Provider]

	if !targetExist {
		return nil, errors.New("Unknown provider name " + u.Provider)
	}

	t.Uuid = uuid.New()
	t.ImageName = u.ImageName
	t.Name = targetName
	t.Status = common.IBWaiting
	t.Created = time.Now()

	switch options := u.Settings.(type) {
	case *target.LocalTargetOptions:
		t.Options = &target.LocalTargetOptions{}
	case *target.AWSTargetOptions:
		t.Options = &target.AWSTargetOptions{
			Region:          options.Region,
			AccessKeyID:     options.AccessKeyID,
			SecretAccessKey: options.SecretAccessKey,
			Bucket:          options.Bucket,
			Key:             options.Key,
		}
	case *target.AzureTargetOptions:
		t.Options = &target.AzureTargetOptions{
			Account:   options.Account,
			AccessKey: options.AccessKey,
			Container: options.Container,
		}
	}

	return &t, nil
}
