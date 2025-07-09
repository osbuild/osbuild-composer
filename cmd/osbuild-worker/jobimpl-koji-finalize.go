package main

import (
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type KojiFinalizeJobImpl struct {
	KojiServers map[string]kojiServer
}

func (impl *KojiFinalizeJobImpl) kojiImport(
	server string,
	build koji.Build,
	buildRoots []koji.BuildRoot,
	outputs []koji.BuildOutput,
	directory, token string) error {

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	kojiServer, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	transport := koji.CreateKojiTransport(kojiServer.relaxTimeoutFactor, NewRHLeveledLogger(nil))
	k, err := koji.NewFromGSSAPI(server, &kojiServer.creds, transport, NewRHLeveledLogger(nil))
	if err != nil {
		return err
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			logrus.Warnf("koji logout failed: %v", err)
		}
	}()

	_, err = k.CGImport(build, buildRoots, outputs, directory, token)
	if err != nil {
		return fmt.Errorf("Could not import build into koji: %v", err)
	}

	return nil
}

func (impl *KojiFinalizeJobImpl) kojiFail(server string, buildID int, token string) error {

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	kojiServer, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	transport := koji.CreateKojiTransport(kojiServer.relaxTimeoutFactor, NewRHLeveledLogger(nil))
	k, err := koji.NewFromGSSAPI(server, &kojiServer.creds, transport, NewRHLeveledLogger(nil))
	if err != nil {
		return err
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			logrus.Warnf("koji logout failed: %v", err)
		}
	}()

	return k.CGFailBuild(buildID, token)
}

func (impl *KojiFinalizeJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id().String())

	// initialize the result variable to be used to report status back to composer
	var kojiFinalizeJobResult = &worker.KojiFinalizeJobResult{}
	// initialize / declare variables to be used to report information back to Koji
	var args = &worker.KojiFinalizeJob{}
	var initArgs *worker.KojiInitJobResult

	// In all cases it is necessary to report result back to osbuild-composer worker API.
	defer func() {
		err := job.Update(kojiFinalizeJobResult)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}

		// Fail the Koji build if the job error is set and the necessary
		// information to identify the job are available.
		if kojiFinalizeJobResult.JobError != nil && initArgs != nil {
			/* #nosec G115 */
			buildID := int(initArgs.BuildID)
			// Make sure that signed integer conversion didn't underflow
			if buildID < 0 {
				logWithId.Errorf("BuildID integer underflow: %d", initArgs.BuildID)
				return
			}
			err = impl.kojiFail(args.Server, buildID, initArgs.Token)
			if err != nil {
				logWithId.Errorf("Failing Koji job failed: %v", err)
			}
		}
	}()

	err := job.Args(args)
	if err != nil {
		kojiFinalizeJobResult.JobError = clienterrors.New(clienterrors.ErrorParsingJobArgs, "Error parsing job args", err.Error())
		return err
	}

	var buildRoots []koji.BuildRoot
	var outputs []koji.BuildOutput
	// Extra info for each image output is stored using the image filename as the key
	imgOutputsExtraInfo := map[string]koji.ImageExtraInfo{}
	manifestOutputsExtraInfo := map[string]*koji.ManifestExtraInfo{}

	var osbuildResults []worker.OSBuildJobResult
	initArgs, osbuildResults, err = extractDynamicArgs(job)
	if err != nil {
		kojiFinalizeJobResult.JobError = clienterrors.New(clienterrors.ErrorParsingDynamicArgs, "Error parsing dynamic args", err.Error())
		return err
	}

	// Check the dependencies early.
	if hasFailedDependency(*initArgs, osbuildResults) {
		kojiFinalizeJobResult.JobError = clienterrors.New(clienterrors.ErrorKojiFailedDependency, "At least one job dependency failed", nil)
		return nil
	}

	for i, buildResult := range osbuildResults {
		buildRPMs := make([]koji.RPM, 0)
		// collect packages from stages in build pipelines
		for _, plName := range buildResult.PipelineNames.Build {
			buildPipelineMd := buildResult.OSBuildOutput.Metadata[plName]
			buildRPMs = append(buildRPMs, koji.OSBuildMetadataToRPMs(buildPipelineMd)...)
		}
		// this dedupe is usually not necessary since we generally only have
		// one rpm stage in one build pipeline, but it's not invalid to have
		// multiple
		buildRPMs = koji.DeduplicateRPMs(buildRPMs)

		kojiTargetResults := buildResult.TargetResultsByName(target.TargetNameKoji)
		// Only a single Koji target is allowed per osbuild job
		if len(kojiTargetResults) != 1 {
			kojiFinalizeJobResult.JobError = clienterrors.New(clienterrors.ErrorKojiFinalize, "Exactly one Koji target result is expected per osbuild job", nil)
			return nil
		}

		kojiTargetResult := kojiTargetResults[0]
		var kojiTargetOSBuildArtifact *koji.OsbuildArtifact
		if kojiTargetResult.OsbuildArtifact != nil {
			kojiTargetOSBuildArtifact = &koji.OsbuildArtifact{
				ExportFilename: kojiTargetResult.OsbuildArtifact.ExportFilename,
				ExportName:     kojiTargetResult.OsbuildArtifact.ExportName,
			}
		}

		kojiTargetOptions := kojiTargetResult.Options.(*target.KojiTargetResultOptions)

		buildRoots = append(buildRoots, koji.BuildRoot{
			ID: uint64(i),
			Host: koji.Host{
				Os:   buildResult.HostOS,
				Arch: buildResult.Arch,
			},
			ContentGenerator: koji.ContentGenerator{
				Name:    "osbuild",
				Version: buildResult.OSBuildVersion,
			},
			Container: koji.Container{
				Type: "none",
				Arch: buildResult.Arch,
			},
			Tools: []koji.Tool{},
			RPMs:  buildRPMs,
		})

		// collect packages from stages in payload pipelines
		imageRPMs := make([]koji.RPM, 0)
		for _, plName := range buildResult.PipelineNames.Payload {
			payloadPipelineMd := buildResult.OSBuildOutput.Metadata[plName]
			imageRPMs = append(imageRPMs, koji.OSBuildMetadataToRPMs(payloadPipelineMd)...)
		}

		// deduplicate
		imageRPMs = koji.DeduplicateRPMs(imageRPMs)

		imgOutputExtraInfo := koji.ImageExtraInfo{
			Arch:            buildResult.Arch,
			BootMode:        buildResult.ImageBootMode,
			OSBuildArtifact: kojiTargetOSBuildArtifact,
			OSBuildVersion:  buildResult.OSBuildVersion,
		}

		// The image filename is now set in the KojiTargetResultOptions.
		// For backward compatibility, if the filename is not set in the
		// options, use the filename from the KojiTargetOptions.
		imageFilename := kojiTargetOptions.Image.Filename
		if imageFilename == "" {
			imageFilename = args.KojiFilenames[i]
		}

		// If there are any non-Koji target results in the build,
		// add them to the image output extra metadata.
		nonKojiTargetResults := buildResult.TargetResultsFilterByName([]target.TargetName{target.TargetNameKoji})
		for _, result := range nonKojiTargetResults {
			imgOutputExtraInfo.UploadTargetResults = append(imgOutputExtraInfo.UploadTargetResults, result)
		}

		imgOutputsExtraInfo[imageFilename] = imgOutputExtraInfo

		// Image output
		outputs = append(outputs, koji.BuildOutput{
			BuildRootID:  uint64(i),
			Filename:     imageFilename,
			FileSize:     kojiTargetOptions.Image.Size,
			Arch:         buildResult.Arch,
			ChecksumType: koji.ChecksumType(kojiTargetOptions.Image.ChecksumType),
			Checksum:     kojiTargetOptions.Image.Checksum,
			Type:         koji.BuildOutputTypeImage,
			RPMs:         imageRPMs,
			Extra: &koji.BuildOutputExtra{
				ImageOutput: imgOutputExtraInfo,
			},
		})

		// OSBuild manifest output
		// TODO: Condition below is present for backward compatibility with old workers which don't upload the manifest.
		// TODO: Remove the condition it in the future.
		if kojiTargetOptions.OSBuildManifest != nil {
			manifestExtraInfo := koji.ManifestExtraInfo{
				Arch: buildResult.Arch,
			}

			if kojiTargetOptions.OSBuildManifestInfo != nil {
				manifestInfo := &koji.ManifestInfo{
					OSBuildComposerVersion: kojiTargetOptions.OSBuildManifestInfo.OSBuildComposerVersion,
				}
				for _, composerDep := range kojiTargetOptions.OSBuildManifestInfo.OSBuildComposerDeps {
					dep := &koji.OSBuildComposerDepModule{
						Path:    composerDep.Path,
						Version: composerDep.Version,
					}
					if composerDep.Replace != nil {
						dep.Replace = &koji.OSBuildComposerDepModule{
							Path:    composerDep.Replace.Path,
							Version: composerDep.Replace.Version,
						}
					}
					manifestInfo.OSBuildComposerDeps = append(manifestInfo.OSBuildComposerDeps, dep)
				}
				manifestExtraInfo.Info = manifestInfo
			}

			manifestOutputsExtraInfo[kojiTargetOptions.OSBuildManifest.Filename] = &manifestExtraInfo

			outputs = append(outputs, koji.BuildOutput{
				BuildRootID:  uint64(i),
				Filename:     kojiTargetOptions.OSBuildManifest.Filename,
				FileSize:     kojiTargetOptions.OSBuildManifest.Size,
				Arch:         buildResult.Arch,
				ChecksumType: koji.ChecksumType(kojiTargetOptions.OSBuildManifest.ChecksumType),
				Checksum:     kojiTargetOptions.OSBuildManifest.Checksum,
				Type:         koji.BuildOutputTypeManifest,
				Extra: &koji.BuildOutputExtra{
					ImageOutput: manifestExtraInfo,
				},
			})
		}

		// Build log output
		// TODO: Condition below is present for backward compatibility with old workers which don't upload the log.
		// TODO: Remove the condition it in the future.
		if kojiTargetOptions.Log != nil {
			outputs = append(outputs, koji.BuildOutput{
				BuildRootID:  uint64(i),
				Filename:     kojiTargetOptions.Log.Filename,
				FileSize:     kojiTargetOptions.Log.Size,
				Arch:         "noarch", // log file is not architecture dependent
				ChecksumType: koji.ChecksumType(kojiTargetOptions.Log.ChecksumType),
				Checksum:     kojiTargetOptions.Log.Checksum,
				Type:         koji.BuildOutputTypeLog,
			})
		}

		// SBOM documents output
		if len(kojiTargetOptions.SbomDocs) > 0 {
			for _, sbomDoc := range kojiTargetOptions.SbomDocs {
				outputs = append(outputs, koji.BuildOutput{
					BuildRootID:  uint64(i),
					Filename:     sbomDoc.Filename,
					FileSize:     sbomDoc.Size,
					Arch:         buildResult.Arch,
					ChecksumType: koji.ChecksumType(sbomDoc.ChecksumType),
					Checksum:     sbomDoc.Checksum,
					Type:         koji.BuildOutputTypeSbomDoc,
					// NB: The extra metadata are not added to the build extra metadata
					// because it does not contain any useful information for SBOM documents.
					Extra: &koji.BuildOutputExtra{
						ImageOutput: koji.SbomDocExtraInfo{
							Arch: buildResult.Arch,
						},
					},
				})
			}
		}
	}

	// Make sure StartTime cannot overflow the int64 conversion
	if args.StartTime > math.MaxInt64 {
		return fmt.Errorf("StartTime integer overflow: %d", args.StartTime)
	}
	/* #nosec G115 */
	startTime := int64(args.StartTime)
	build := koji.Build{
		BuildID:   initArgs.BuildID,
		TaskID:    args.TaskID,
		Name:      args.Name,
		Version:   args.Version,
		Release:   args.Release,
		StartTime: startTime,
		EndTime:   time.Now().Unix(),
		Extra: koji.BuildExtra{
			TypeInfo: koji.TypeInfoBuild{
				Image: imgOutputsExtraInfo,
			},
			Manifest: manifestOutputsExtraInfo,
		},
	}

	err = impl.kojiImport(args.Server, build, buildRoots, outputs, args.KojiDirectory, initArgs.Token)
	if err != nil {
		kojiFinalizeJobResult.JobError = clienterrors.New(clienterrors.ErrorKojiFinalize, err.Error(), nil)
		return err
	}

	return nil
}

// Extracts dynamic args of the koji-finalize job. Returns an error if they
// cannot be unmarshalled.
func extractDynamicArgs(job worker.Job) (*worker.KojiInitJobResult, []worker.OSBuildJobResult, error) {
	var kojiInitResult worker.KojiInitJobResult
	err := job.DynamicArgs(0, &kojiInitResult)
	if err != nil {
		return nil, nil, err
	}

	osbuildResults := make([]worker.OSBuildJobResult, job.NDynamicArgs()-1)

	for i := 1; i < job.NDynamicArgs(); i++ {
		err = job.DynamicArgs(i, &osbuildResults[i-1])
		if err != nil {
			return nil, nil, err
		}
	}

	return &kojiInitResult, osbuildResults, nil
}

// Returns true if any of koji-finalize dependencies failed.
func hasFailedDependency(kojiInitResult worker.KojiInitJobResult, osbuildResults []worker.OSBuildJobResult) bool {
	if kojiInitResult.JobError != nil {
		return true
	}

	for _, r := range osbuildResults {
		// No `OSBuildOutput` implies failure: either osbuild crashed or
		// rejected the input (manifest or command line arguments)
		if r.OSBuildOutput == nil || !r.OSBuildOutput.Success || r.JobError != nil {
			return true
		}
	}
	return false
}
