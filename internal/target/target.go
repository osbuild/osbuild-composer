package target

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/common"
)

type TargetName string

// OsbuildArtifact represents a configuration to produce osbuild artifact
// specific to a target.
type OsbuildArtifact struct {
	// Filename of the image as produced by osbuild for a given export
	ExportFilename string `json:"export_filename"`
	// Name of the osbuild pipeline, which should be exported for this target
	ExportName string `json:"export_name"`
}

type Target struct {
	Uuid uuid.UUID `json:"uuid"`
	// Desired name of the image in the target environment
	ImageName string `json:"image_name"`
	// Name of the specific target type
	Name    TargetName             `json:"name"`
	Created time.Time              `json:"created"`
	Status  common.ImageBuildState `json:"status"`
	// Target type specific options
	Options TargetOptions `json:"options"`
	// Configuration to produce osbuild artifact specific to this target
	OsbuildArtifact OsbuildArtifact `json:"osbuild_artifact"`
}

func newTarget(name TargetName, options TargetOptions) *Target {
	return &Target{
		Uuid:    uuid.New(),
		Name:    name,
		Created: time.Now(),
		Status:  common.IBWaiting,
		Options: options,
	}
}

type TargetOptions interface {
	isTargetOptions()
}

type rawTarget struct {
	Uuid      uuid.UUID              `json:"uuid"`
	ImageName string                 `json:"image_name"`
	Name      TargetName             `json:"name"`
	Created   time.Time              `json:"created"`
	Status    common.ImageBuildState `json:"status"`
	Options   json.RawMessage        `json:"options"`
	// Configuration to produce osbuild artifact specific to this target
	OsbuildArtifact OsbuildArtifact `json:"osbuild_artifact"`
}

func (target *Target) UnmarshalJSON(data []byte) error {
	var rawTarget rawTarget
	err := json.Unmarshal(data, &rawTarget)
	if err != nil {
		return err
	}

	var options TargetOptions
	switch rawTarget.Name {
	case TargetNameAzure:
		options = new(AzureTargetOptions)
	case TargetNameAWS:
		options = new(AWSTargetOptions)
	case TargetNameAWSS3:
		options = new(AWSS3TargetOptions)
	case TargetNameGCP:
		options = new(GCPTargetOptions)
	case TargetNameAzureImage:
		options = new(AzureImageTargetOptions)
	// Kept for backward compatibility
	case TargetNameLocal:
		options = new(LocalTargetOptions)
	case TargetNameKoji:
		options = new(KojiTargetOptions)
	case TargetNameVMWare:
		options = new(VMWareTargetOptions)
	case TargetNameOCI:
		options = new(OCITargetOptions)
	case TargetNameOCIObjectStorage:
		options = new(OCIObjectStorageTargetOptions)
	case TargetNameContainer:
		options = new(ContainerTargetOptions)
	case TargetNameWorkerServer:
		options = new(WorkerServerTargetOptions)
	default:
		return fmt.Errorf("unexpected target name: %s", rawTarget.Name)
	}

	err = json.Unmarshal(rawTarget.Options, options)
	if err != nil {
		return err
	}

	target.Uuid = rawTarget.Uuid
	target.ImageName = rawTarget.ImageName
	target.OsbuildArtifact = rawTarget.OsbuildArtifact
	target.Name = rawTarget.Name
	target.Created = rawTarget.Created
	target.Status = rawTarget.Status
	target.Options = options

	type compatOptionsType struct {
		// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
		Filename string `json:"filename"`
	}

	var compat compatOptionsType
	err = json.Unmarshal(rawTarget.Options, &compat)
	if err != nil {
		return err
	}

	// Kept for backward compatibility
	// If the `ExportTarget` is not set in the `Target`, the request is most probably
	// coming from an old composer. Copy the value from the target options.
	if target.OsbuildArtifact.ExportFilename == "" {
		target.OsbuildArtifact.ExportFilename = compat.Filename
	}

	return nil
}

func (target Target) MarshalJSON() ([]byte, error) {
	// We can't use composition of the `TargetOptions` interface into a compatibility
	// structure, because the value assigned to the embedded interface type member
	// would get marshaled under the name of the type.
	var rawOptions []byte
	var err error
	if target.Options != nil {
		switch t := target.Options.(type) {
		case *AWSTargetOptions:
			type compatOptionsType struct {
				*AWSTargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				AWSTargetOptions: t,
				Filename:         target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		case *AWSS3TargetOptions:
			type compatOptionsType struct {
				*AWSS3TargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				AWSS3TargetOptions: t,
				Filename:           target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		case *AzureTargetOptions:
			type compatOptionsType struct {
				*AzureTargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				AzureTargetOptions: t,
				Filename:           target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		case *GCPTargetOptions:
			type compatOptionsType struct {
				*GCPTargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				GCPTargetOptions: t,
				Filename:         target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		case *AzureImageTargetOptions:
			type compatOptionsType struct {
				*AzureImageTargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				AzureImageTargetOptions: t,
				Filename:                target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		// Kept for backward compatibility
		case *LocalTargetOptions:
			type compatOptionsType struct {
				*LocalTargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				LocalTargetOptions: t,
				Filename:           target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		case *KojiTargetOptions:
			type compatOptionsType struct {
				*KojiTargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				KojiTargetOptions: t,
				Filename:          target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		case *VMWareTargetOptions:
			type compatOptionsType struct {
				*VMWareTargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				VMWareTargetOptions: t,
				Filename:            target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		case *OCITargetOptions:
			type compatOptionsType struct {
				*OCITargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				OCITargetOptions: t,
				Filename:         target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		case *OCIObjectStorageTargetOptions:
			type compatOptionsType struct {
				*OCIObjectStorageTargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				OCIObjectStorageTargetOptions: t,
				Filename:                      target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		case *ContainerTargetOptions:
			type compatOptionsType struct {
				*ContainerTargetOptions
				// Deprecated: `Filename` is now set in the target itself as `ExportFilename`, not in its options.
				Filename string `json:"filename"`
			}
			compat := compatOptionsType{
				ContainerTargetOptions: t,
				Filename:               target.OsbuildArtifact.ExportFilename,
			}
			rawOptions, err = json.Marshal(compat)

		case *WorkerServerTargetOptions:
			// WorkerServer target does not handle the backward compatibility
			// for the Filename in target options, because it was added after
			// the incompatible change.
			rawOptions, err = json.Marshal(target.Options)

		default:
			return nil, fmt.Errorf("unexpected target options type: %t", t)
		}

		// check error from marshaling
		if err != nil {
			return nil, err
		}
	}

	alias := rawTarget{
		Uuid:            target.Uuid,
		ImageName:       target.ImageName,
		OsbuildArtifact: target.OsbuildArtifact,
		Name:            target.Name,
		Created:         target.Created,
		Status:          target.Status,
		Options:         json.RawMessage(rawOptions),
	}

	return json.Marshal(alias)
}
