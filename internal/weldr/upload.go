package weldr

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type UploadResponse struct {
	Uuid         uuid.UUID            `json:"uuid"`
	Status       string               `json:"status"`
	ProviderName string               `json:"provider_name"`
	ImageName    string               `json:"image_name"`
	CreationTime float64              `json:"creation_time"`
	Settings     target.TargetOptions `json:"settings"`
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

func targetToUploadResponse(t *target.Target) UploadResponse {
	var u UploadResponse

	providerName, providerExist := targetNameToProviderNameMap[t.Name]
	if !providerExist {
		panic("target name " + t.Name + " is not defined in conversion map!")
	}

	u.CreationTime = float64(t.Created.UnixNano()) / 1000000000
	u.ImageName = t.ImageName
	u.ProviderName = providerName
	u.Status = t.Status
	u.Uuid = t.Uuid
	u.Settings = t.Options

	return u
}

func TargetsToUploadResponses(targets []*target.Target) []UploadResponse {
	var uploads []UploadResponse
	for _, t := range targets {
		if t.Name == "org.osbuild.local" {
			continue
		}

		upload := targetToUploadResponse(t)

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
	t.Options = u.Settings
	t.Name = targetName
	t.Status = "WAITING"
	t.Created = time.Now()

	return &t, nil
}
