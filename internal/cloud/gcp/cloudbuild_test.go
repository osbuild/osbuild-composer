package gcp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudbuildResourcesFromBuildLog(t *testing.T) {
	testCases := []struct {
		buildLog  string
		resources cloudbuildBuildResources
	}{
		{
			buildLog: `2021/03/15 18:07:56 starting build "dba8bd1a-79b7-4060-a99d-c334760cba18"

FETCHSOURCE
BUILD
Pulling image: gcr.io/compute-image-tools/gce_vm_image_import:release
release: Pulling from compute-image-tools/gce_vm_image_import
0b8bbb5f50a4: Pulling fs layer
7efaa022ad36: Pulling fs layer
e5303db5f8f9: Pulling fs layer
688d304ec274: Pulling fs layer
e969b3a22ab3: Pulling fs layer
cd7b8272632b: Pulling fs layer
2175d0ddd745: Pulling fs layer
69fbd73b475e: Pulling fs layer
7a5922a992b2: Pulling fs layer
688d304ec274: Waiting
e969b3a22ab3: Waiting
cd7b8272632b: Waiting
2175d0ddd745: Waiting
69fbd73b475e: Waiting
7a5922a992b2: Waiting
e5303db5f8f9: Verifying Checksum
e5303db5f8f9: Download complete
688d304ec274: Verifying Checksum
688d304ec274: Download complete
e969b3a22ab3: Verifying Checksum
e969b3a22ab3: Download complete
0b8bbb5f50a4: Verifying Checksum
0b8bbb5f50a4: Download complete
cd7b8272632b: Verifying Checksum
cd7b8272632b: Download complete
69fbd73b475e: Verifying Checksum
69fbd73b475e: Download complete
7efaa022ad36: Verifying Checksum
7efaa022ad36: Download complete
7a5922a992b2: Verifying Checksum
7a5922a992b2: Download complete
2175d0ddd745: Verifying Checksum
2175d0ddd745: Download complete
0b8bbb5f50a4: Pull complete
7efaa022ad36: Pull complete
e5303db5f8f9: Pull complete
688d304ec274: Pull complete
e969b3a22ab3: Pull complete
cd7b8272632b: Pull complete
2175d0ddd745: Pull complete
69fbd73b475e: Pull complete
7a5922a992b2: Pull complete
Digest: sha256:d39e2c0e6a7113d989d292536e9d14e927de838cb21a24c61eb7d44fef1fa51d
Status: Downloaded newer image for gcr.io/compute-image-tools/gce_vm_image_import:release
gcr.io/compute-image-tools/gce_vm_image_import:release
[import-image]: 2021-03-12T17:29:05Z Creating Google Compute Engine disk from gs://images-bkt-us/random-object-1234
[inflate]: 2021-03-12T17:29:05Z Validating workflow
[inflate]: 2021-03-12T17:29:05Z Validating step "setup-disks"
[inflate]: 2021-03-12T17:29:06Z Validating step "import-virtual-disk"
[inflate]: 2021-03-12T17:29:06Z Validating step "wait-for-signal"
[inflate]: 2021-03-12T17:29:06Z Validating step "cleanup"
[inflate]: 2021-03-12T17:29:06Z Validation Complete
[inflate]: 2021-03-12T17:29:06Z Workflow Project: ascendant-braid-303513
[inflate]: 2021-03-12T17:29:06Z Workflow Zone: us-central1-c
[inflate]: 2021-03-12T17:29:06Z Workflow GCSPath: gs://ascendant-braid-303513-daisy-bkt-us-central1/gce-image-import-2021-03-12T17:29:03Z-qllpn
[inflate]: 2021-03-12T17:29:06Z Daisy scratch path: https://console.cloud.google.com/storage/browser/ascendant-braid-303513-daisy-bkt-us-central1/gce-image-import-2021-03-12T17:29:03Z-qllpn/daisy-inflate-20210312-17:29:05-1wghm
[inflate]: 2021-03-12T17:29:06Z Uploading sources
[inflate]: 2021-03-12T17:29:07Z Running workflow
[inflate]: 2021-03-12T17:29:07Z Running step "setup-disks" (CreateDisks)
[inflate.setup-disks]: 2021-03-12T17:29:07Z CreateDisks: Creating disk "disk-importer-inflate-1wghm".
[inflate.setup-disks]: 2021-03-12T17:29:07Z CreateDisks: Creating disk "disk-qllpn".
[inflate.setup-disks]: 2021-03-12T17:29:07Z CreateDisks: Creating disk "disk-inflate-scratch-1wghm".
[inflate]: 2021-03-12T17:29:08Z Step "setup-disks" (CreateDisks) successfully finished.
[inflate]: 2021-03-12T17:29:08Z Running step "import-virtual-disk" (CreateInstances)
[inflate.import-virtual-disk]: 2021-03-12T17:29:08Z CreateInstances: Creating instance "inst-importer-inflate-1wghm".
[inflate]: 2021-03-12T17:29:18Z Step "import-virtual-disk" (CreateInstances) successfully finished.
[inflate.import-virtual-disk]: 2021-03-12T17:29:18Z CreateInstances: Streaming instance "inst-importer-inflate-1wghm" serial port 1 output to https://storage.cloud.google.com/ascendant-braid-303513-daisy-bkt-us-central1/gce-image-import-2021-03-12T17:29:03Z-qllpn/daisy-inflate-20210312-17:29:05-1wghm/logs/inst-importer-inflate-1wghm-serial-port1.log
[inflate]: 2021-03-12T17:29:18Z Running step "wait-for-signal" (WaitForInstancesSignal)
[inflate.wait-for-signal]: 2021-03-12T17:29:18Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": watching serial port 1, SuccessMatch: "ImportSuccess:", FailureMatch: ["ImportFailed:" "WARNING Failed to download metadata script" "Failed to download GCS path" "Worker instance terminated"] (this is not an error), StatusMatch: "Import:".
[inflate.wait-for-signal]: 2021-03-12T17:29:28Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": StatusMatch found: "Import: Ensuring disk-inflate-scratch-1wghm has capacity of 3 GB in projects/550072179371/zones/us-central1-c."
[inflate.wait-for-signal]: 2021-03-12T17:29:28Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": StatusMatch found: "Import: /dev/sdb is attached and ready."
[inflate.wait-for-signal]: 2021-03-12T17:29:48Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": StatusMatch found: "Import: Copied image from gs://images-bkt-us/random-object-1234 to /daisy-scratch/random-object-1234:"
[inflate.wait-for-signal]: 2021-03-12T17:29:48Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": StatusMatch found: "Import: Importing /daisy-scratch/random-object-1234 of size 2GB to disk-qllpn in projects/550072179371/zones/us-central1-c."
[inflate.wait-for-signal]: 2021-03-12T17:29:48Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": StatusMatch found: "Import: <serial-output key:'target-size-gb' value:'2'>"
[inflate.wait-for-signal]: 2021-03-12T17:29:48Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": StatusMatch found: "Import: <serial-output key:'source-size-gb' value:'2'>"
[inflate.wait-for-signal]: 2021-03-12T17:29:48Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": StatusMatch found: "Import: <serial-output key:'import-file-format' value:'vmdk'>"
[inflate.wait-for-signal]: 2021-03-12T17:29:48Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": StatusMatch found: "Import: Ensuring disk-qllpn has capacity of 2 GB in projects/550072179371/zones/us-central1-c."
[inflate.wait-for-signal]: 2021-03-12T17:29:48Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": StatusMatch found: "Import: /dev/sdc is attached and ready."
[debug]: 2021-03-12T17:29:52Z Started checksum calculation.
[shadow-disk-checksum]: 2021-03-12T17:29:53Z Validating workflow
[shadow-disk-checksum]: 2021-03-12T17:29:53Z Validating step "create-disks"
[shadow-disk-checksum]: 2021-03-12T17:29:53Z Validating step "create-instance"
[shadow-disk-checksum]: 2021-03-12T17:29:54Z Validating step "wait-for-checksum"
[shadow-disk-checksum]: 2021-03-12T17:29:54Z Validation Complete
[shadow-disk-checksum]: 2021-03-12T17:29:54Z Workflow Project: ascendant-braid-303513
[shadow-disk-checksum]: 2021-03-12T17:29:54Z Workflow Zone: us-central1-c
[shadow-disk-checksum]: 2021-03-12T17:29:54Z Workflow GCSPath: gs://ascendant-braid-303513-daisy-bkt-us-central1/gce-image-import-2021-03-12T17:29:03Z-qllpn
[shadow-disk-checksum]: 2021-03-12T17:29:54Z Daisy scratch path: https://console.cloud.google.com/storage/browser/ascendant-braid-303513-daisy-bkt-us-central1/gce-image-import-2021-03-12T17:29:03Z-qllpn/daisy-shadow-disk-checksum-20210312-17:29:53-r3qxv
[shadow-disk-checksum]: 2021-03-12T17:29:54Z Uploading sources
[shadow-disk-checksum]: 2021-03-12T17:29:54Z Running workflow
[shadow-disk-checksum]: 2021-03-12T17:29:54Z Running step "create-disks" (CreateDisks)
[shadow-disk-checksum.create-disks]: 2021-03-12T17:29:54Z CreateDisks: Creating disk "disk-shadow-disk-checksum-shadow-disk-checksum-r3qxv".
[shadow-disk-checksum]: 2021-03-12T17:29:55Z Step "create-disks" (CreateDisks) successfully finished.
[shadow-disk-checksum]: 2021-03-12T17:29:55Z Running step "create-instance" (CreateInstances)
[shadow-disk-checksum.create-instance]: 2021-03-12T17:29:55Z CreateInstances: Creating instance "inst-shadow-disk-checksum-shadow-disk-checksum-r3qxv".
[shadow-disk-checksum]: 2021-03-12T17:30:03Z Error running workflow: step "create-instance" run error: operation failed &{ClientOperationId: CreationTimestamp: Description: EndTime:2021-03-12T09:30:02.764-08:00 Error:0xc000454460 HttpErrorMessage:FORBIDDEN HttpErrorStatusCode:403 Id:4817697984863411195 InsertTime:2021-03-12T09:29:56.910-08:00 Kind:compute#operation Name:operation-1615570195674-5bd5a3f9f770d-10b61845-7bf8f2ab OperationType:insert Progress:100 Region: SelfLink:https://www.googleapis.com/compute/v1/projects/ascendant-braid-303513/zones/us-central1-c/operations/operation-1615570195674-5bd5a3f9f770d-10b61845-7bf8f2ab StartTime:2021-03-12T09:29:56.913-08:00 Status:DONE StatusMessage: TargetId:2023576705161370619 TargetLink:https://www.googleapis.com/compute/v1/projects/ascendant-braid-303513/zones/us-central1-c/instances/inst-shadow-disk-checksum-shadow-disk-checksum-r3qxv User:550072179371@cloudbuild.gserviceaccount.com Warnings:[] Zone:https://www.googleapis.com/compute/v1/projects/ascendant-braid-303513/zones/us-central1-c ServerResponse:{HTTPStatusCode:200 Header:map[Cache-Control:[private] Content-Type:[application/json; charset=UTF-8] Date:[Fri, 12 Mar 2021 17:30:03 GMT] Server:[ESF] Vary:[Origin X-Origin Referer] X-Content-Type-Options:[nosniff] X-Frame-Options:[SAMEORIGIN] X-Xss-Protection:[0]]} ForceSendFields:[] NullFields:[]}:
Code: QUOTA_EXCEEDED
Message: Quota 'CPUS' exceeded.  Limit: 24.0 in region us-central1.
[shadow-disk-checksum]: 2021-03-12T17:30:03Z Workflow "shadow-disk-checksum" cleaning up (this may take up to 2 minutes).
[shadow-disk-checksum]: 2021-03-12T17:30:03Z Workflow "shadow-disk-checksum" finished cleanup.
[inflate.wait-for-signal]: 2021-03-12T17:30:38Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": StatusMatch found: "Import: <serial-output key:'disk-checksum' value:'5ed32cb4d9d9dd17cba6da1b3903edaa  --d29c6650c73d602034fea42869a545ae  --75a1e608e6f1c50758f4fee5a7d8e3d0  --75a1e608e6f1c50758f4fee5a7d8e3d0  -'>"
[inflate.wait-for-signal]: 2021-03-12T17:30:38Z WaitForInstancesSignal: Instance "inst-importer-inflate-1wghm": SuccessMatch found "ImportSuccess: Finished import."
[inflate]: 2021-03-12T17:30:38Z Step "wait-for-signal" (WaitForInstancesSignal) successfully finished.
[inflate]: 2021-03-12T17:30:38Z Running step "cleanup" (DeleteResources)
[inflate.cleanup]: 2021-03-12T17:30:38Z DeleteResources: Deleting instance "inst-importer".
[inflate]: 2021-03-12T17:31:05Z Step "cleanup" (DeleteResources) successfully finished.
[inflate]: 2021-03-12T17:31:05Z Serial-output value -> disk-checksum:5ed32cb4d9d9dd17cba6da1b3903edaa  --d29c6650c73d602034fea42869a545ae  --75a1e608e6f1c50758f4fee5a7d8e3d0  --75a1e608e6f1c50758f4fee5a7d8e3d0  -
[inflate]: 2021-03-12T17:31:05Z Serial-output value -> target-size-gb:2
[inflate]: 2021-03-12T17:31:05Z Serial-output value -> source-size-gb:2
[inflate]: 2021-03-12T17:31:05Z Serial-output value -> import-file-format:vmdk
[inflate]: 2021-03-12T17:31:05Z Workflow "inflate" cleaning up (this may take up to 2 minutes).
[inflate]: 2021-03-12T17:31:07Z Workflow "inflate" finished cleanup.
[import-image]: 2021-03-12T17:31:07Z Finished creating Google Compute Engine disk
[import-image]: 2021-03-12T17:31:07Z Creating image "image-dea5f2fb8b6e3826653cfc02ef648c8f8a3be5cb0501aeb75dcd6147"
PUSH
DONE
`,
			resources: cloudbuildBuildResources{
				zone: "us-central1-c",
				computeInstances: []string{
					"inst-importer-inflate-1wghm",
					"inst-shadow-disk-checksum-shadow-disk-checksum-r3qxv",
				},
				computeDisks: []string{
					"disk-qllpn",
					"disk-importer-inflate-1wghm",
					"disk-inflate-scratch-1wghm",
					"disk-shadow-disk-checksum-shadow-disk-checksum-r3qxv",
				},
				storageCacheDir: struct {
					bucket string
					dir    string
				}{
					bucket: "ascendant-braid-303513-daisy-bkt-us-central1",
					dir:    "gce-image-import-2021-03-12T17:29:03Z-qllpn",
				},
			},
		},
		{
			buildLog: `starting build "cbf2e886-a81f-4761-9d39-0a03c3579996"

FETCHSOURCE
BUILD
Pulling image: gcr.io/compute-image-tools/gce_vm_image_import:release
release: Pulling from compute-image-tools/gce_vm_image_import
0b8bbb5f50a4: Pulling fs layer
7efaa022ad36: Pulling fs layer
e5303db5f8f9: Pulling fs layer
688d304ec274: Pulling fs layer
e969b3a22ab3: Pulling fs layer
cd7b8272632b: Pulling fs layer
2175d0ddd745: Pulling fs layer
69fbd73b475e: Pulling fs layer
7a5922a992b2: Pulling fs layer
688d304ec274: Waiting
e969b3a22ab3: Waiting
cd7b8272632b: Waiting
2175d0ddd745: Waiting
69fbd73b475e: Waiting
7a5922a992b2: Waiting
e5303db5f8f9: Verifying Checksum
e5303db5f8f9: Download complete
688d304ec274: Download complete
e969b3a22ab3: Verifying Checksum
e969b3a22ab3: Download complete
0b8bbb5f50a4: Verifying Checksum
0b8bbb5f50a4: Download complete
cd7b8272632b: Verifying Checksum
cd7b8272632b: Download complete
69fbd73b475e: Verifying Checksum
69fbd73b475e: Download complete
7a5922a992b2: Verifying Checksum
7a5922a992b2: Download complete
7efaa022ad36: Verifying Checksum
7efaa022ad36: Download complete
2175d0ddd745: Verifying Checksum
2175d0ddd745: Download complete
0b8bbb5f50a4: Pull complete
7efaa022ad36: Pull complete
e5303db5f8f9: Pull complete
688d304ec274: Pull complete
e969b3a22ab3: Pull complete
cd7b8272632b: Pull complete
2175d0ddd745: Pull complete
69fbd73b475e: Pull complete
7a5922a992b2: Pull complete
Digest: sha256:d39e2c0e6a7113d989d292536e9d14e927de838cb21a24c61eb7d44fef1fa51d
Status: Downloaded newer image for gcr.io/compute-image-tools/gce_vm_image_import:release
gcr.io/compute-image-tools/gce_vm_image_import:release
[import-image]: 2021-03-12T16:52:00Z Creating Google Compute Engine disk from gs://images-bkt-us/random-object-1234
[inflate]: 2021-03-12T16:52:01Z Validating workflow
[inflate]: 2021-03-12T16:52:01Z Validating step "setup-disks"
[inflate]: 2021-03-12T16:52:01Z Validating step "import-virtual-disk"
[inflate]: 2021-03-12T16:52:01Z Validating step "wait-for-signal"
[inflate]: 2021-03-12T16:52:01Z Validating step "cleanup"
[inflate]: 2021-03-12T16:52:01Z Validation Complete
[inflate]: 2021-03-12T16:52:01Z Workflow Project: ascendant-braid-303513
[inflate]: 2021-03-12T16:52:01Z Workflow Zone: us-central1-c
[inflate]: 2021-03-12T16:52:01Z Workflow GCSPath: gs://ascendant-braid-303513-daisy-bkt-us-central1/gce-image-import-2021-03-12T16:51:59Z-74mbx
[inflate]: 2021-03-12T16:52:01Z Daisy scratch path: https://console.cloud.google.com/storage/browser/ascendant-braid-303513-daisy-bkt-us-central1/gce-image-import-2021-03-12T16:51:59Z-74mbx/daisy-inflate-20210312-16:52:01-trs8w
[inflate]: 2021-03-12T16:52:01Z Uploading sources
[inflate]: 2021-03-12T16:52:01Z Running workflow
[inflate]: 2021-03-12T16:52:01Z Running step "setup-disks" (CreateDisks)
[inflate.setup-disks]: 2021-03-12T16:52:01Z CreateDisks: Creating disk "disk-74mbx".
[inflate.setup-disks]: 2021-03-12T16:52:01Z CreateDisks: Creating disk "disk-importer-inflate-trs8w".
[inflate.setup-disks]: 2021-03-12T16:52:01Z CreateDisks: Creating disk "disk-inflate-scratch-trs8w".
[inflate]: 2021-03-12T16:52:03Z Step "setup-disks" (CreateDisks) successfully finished.
[inflate]: 2021-03-12T16:52:03Z Running step "import-virtual-disk" (CreateInstances)
[inflate.import-virtual-disk]: 2021-03-12T16:52:03Z CreateInstances: Creating instance "inst-importer-inflate-trs8w".
[inflate]: 2021-03-12T16:52:17Z Step "import-virtual-disk" (CreateInstances) successfully finished.
[inflate.import-virtual-disk]: 2021-03-12T16:52:17Z CreateInstances: Streaming instance "inst-importer-inflate-trs8w" serial port 1 output to https://storage.cloud.google.com/ascendant-braid-303513-daisy-bkt-us-central1/gce-image-import-2021-03-12T16:51:59Z-74mbx/daisy-inflate-20210312-16:52:01-trs8w/logs/inst-importer-inflate-trs8w-serial-port1.log
[inflate]: 2021-03-12T16:52:17Z Running step "wait-for-signal" (WaitForInstancesSignal)
[inflate.wait-for-signal]: 2021-03-12T16:52:17Z WaitForInstancesSignal: Instance "inst-importer-inflate-trs8w": watching serial port 1, SuccessMatch: "ImportSuccess:", FailureMatch: ["ImportFailed:" "WARNING Failed to download metadata script" "Failed to download GCS path" "Worker instance terminated"] (this is not an error), StatusMatch: "Import:".
[inflate.wait-for-signal]: 2021-03-12T16:52:28Z WaitForInstancesSignal: Instance "inst-importer-inflate-trs8w": StatusMatch found: "Import: Ensuring disk-inflate-scratch-trs8w has capacity of 3 GB in projects/550072179371/zones/us-central1-c."
[inflate.wait-for-signal]: 2021-03-12T16:52:28Z WaitForInstancesSignal: Instance "inst-importer-inflate-trs8w": StatusMatch found: "Import: /dev/sdb is attached and ready."
[debug]: 2021-03-12T16:52:38Z Started checksum calculation.
[shadow-disk-checksum]: 2021-03-12T16:52:38Z Validating workflow
[shadow-disk-checksum]: 2021-03-12T16:52:38Z Validating step "create-disks"
CANCELLED
ERROR: context canceled`,
			resources: cloudbuildBuildResources{
				zone: "us-central1-c",
				computeInstances: []string{
					"inst-importer-inflate-trs8w",
				},
				computeDisks: []string{
					"disk-74mbx",
					"disk-importer-inflate-trs8w",
					"disk-inflate-scratch-trs8w",
				},
				storageCacheDir: struct {
					bucket string
					dir    string
				}{
					bucket: "ascendant-braid-303513-daisy-bkt-us-central1",
					dir:    "gce-image-import-2021-03-12T16:51:59Z-74mbx",
				},
			},
		},
		{
			buildLog: `starting build "4f351d2a-5c07-4555-8319-ee7a8e514da1"

FETCHSOURCE
BUILD
Pulling image: gcr.io/compute-image-tools/gce_vm_image_import:release
release: Pulling from compute-image-tools/gce_vm_image_import
2e1eb53387e5: Pulling fs layer
95cc589e8a63: Pulling fs layer
3b6aa88d2880: Pulling fs layer
4cf16dbc31d9: Pulling fs layer
45c52af3bf12: Pulling fs layer
ae477f711ff8: Pulling fs layer
abf85f8ba3ed: Pulling fs layer
a06a66dd712b: Pulling fs layer
e6263716b8ac: Pulling fs layer
4cf16dbc31d9: Waiting
45c52af3bf12: Waiting
ae477f711ff8: Waiting
abf85f8ba3ed: Waiting
a06a66dd712b: Waiting
e6263716b8ac: Waiting
3b6aa88d2880: Verifying Checksum
3b6aa88d2880: Download complete
4cf16dbc31d9: Verifying Checksum
4cf16dbc31d9: Download complete
45c52af3bf12: Verifying Checksum
45c52af3bf12: Download complete
ae477f711ff8: Verifying Checksum
ae477f711ff8: Download complete
95cc589e8a63: Verifying Checksum
95cc589e8a63: Download complete
a06a66dd712b: Download complete
e6263716b8ac: Verifying Checksum
e6263716b8ac: Download complete
2e1eb53387e5: Verifying Checksum
2e1eb53387e5: Download complete
abf85f8ba3ed: Verifying Checksum
abf85f8ba3ed: Download complete
2e1eb53387e5: Pull complete
95cc589e8a63: Pull complete
3b6aa88d2880: Pull complete
4cf16dbc31d9: Pull complete
45c52af3bf12: Pull complete
ae477f711ff8: Pull complete
abf85f8ba3ed: Pull complete
a06a66dd712b: Pull complete
e6263716b8ac: Pull complete
Digest: sha256:90ccc6b8c1239f14690ade311b2a85c6e75931f903c614b4556c1024d54783b5
Status: Downloaded newer image for gcr.io/compute-image-tools/gce_vm_image_import:release
gcr.io/compute-image-tools/gce_vm_image_import:release
[import-image]: 2021-02-17T12:42:09Z Creating Google Compute Engine disk from gs://thozza-images/f32-image.vhd
[inflate]: 2021-02-17T12:42:09Z Validating workflow
[inflate]: 2021-02-17T12:42:09Z Validating step "setup-disks"
[inflate]: 2021-02-17T12:42:10Z Validating step "import-virtual-disk"
[inflate]: 2021-02-17T12:42:10Z Validating step "wait-for-signal"
[inflate]: 2021-02-17T12:42:10Z Validating step "cleanup"
[inflate]: 2021-02-17T12:42:10Z Validation Complete
[inflate]: 2021-02-17T12:42:10Z Workflow Project: ascendant-braid-303513
[inflate]: 2021-02-17T12:42:10Z Workflow Zone: europe-west1-b
[inflate]: 2021-02-17T12:42:10Z Workflow GCSPath: gs://ascendant-braid-303513-daisy-bkt-eu/gce-image-import-2021-02-17T12:42:05Z-lz55d
[inflate]: 2021-02-17T12:42:10Z Daisy scratch path: https://console.cloud.google.com/storage/browser/ascendant-braid-303513-daisy-bkt-eu/gce-image-import-2021-02-17T12:42:05Z-lz55d/daisy-inflate-20210217-12:42:09-p57zp
[inflate]: 2021-02-17T12:42:10Z Uploading sources
[inflate]: 2021-02-17T12:42:11Z Running workflow
[inflate]: 2021-02-17T12:42:11Z Running step "setup-disks" (CreateDisks)
[inflate.setup-disks]: 2021-02-17T12:42:11Z CreateDisks: Creating disk "disk-lz55d".
[inflate.setup-disks]: 2021-02-17T12:42:11Z CreateDisks: Creating disk "disk-importer-inflate-p57zp".
[inflate.setup-disks]: 2021-02-17T12:42:11Z CreateDisks: Creating disk "disk-inflate-scratch-p57zp".
[inflate]: 2021-02-17T12:42:13Z Step "setup-disks" (CreateDisks) successfully finished.
[inflate]: 2021-02-17T12:42:13Z Running step "import-virtual-disk" (CreateInstances)
[inflate.import-virtual-disk]: 2021-02-17T12:42:13Z CreateInstances: Creating instance "inst-importer-inflate-p57zp".
[inflate.import-virtual-disk]: 2021-02-17T12:42:23Z CreateInstances: Streaming instance "inst-importer-inflate-p57zp" serial port 1 output to https://storage.cloud.google.com/ascendant-braid-303513-daisy-bkt-eu/gce-image-import-2021-02-17T12:42:05Z-lz55d/daisy-inflate-20210217-12:42:09-p57zp/logs/inst-importer-inflate-p57zp-serial-port1.log
[inflate]: 2021-02-17T12:42:23Z Step "import-virtual-disk" (CreateInstances) successfully finished.
[inflate]: 2021-02-17T12:42:23Z Running step "wait-for-signal" (WaitForInstancesSignal)
[inflate.wait-for-signal]: 2021-02-17T12:42:23Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": watching serial port 1, SuccessMatch: "ImportSuccess:", FailureMatch: ["ImportFailed:" "WARNING Failed to download metadata script" "Failed to download GCS path" "Worker instance terminated"] (this is not an error), StatusMatch: "Import:".
[inflate.wait-for-signal]: 2021-02-17T12:42:34Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": StatusMatch found: "Import: Ensuring disk-inflate-scratch-p57zp has capacity of 6 GB in projects/550072179371/zones/europe-west1-b."
[inflate.wait-for-signal]: 2021-02-17T12:42:34Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": StatusMatch found: "Import: /dev/sdb is attached and ready."
[inflate.wait-for-signal]: 2021-02-17T12:43:43Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": StatusMatch found: "Import: Copied image from gs://thozza-images/f32-image.vhd to /daisy-scratch/f32-image.vhd:"
[inflate.wait-for-signal]: 2021-02-17T12:43:43Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": StatusMatch found: "Import: Importing /daisy-scratch/f32-image.vhd of size 5GB to disk-lz55d in projects/550072179371/zones/europe-west1-b."
[inflate.wait-for-signal]: 2021-02-17T12:43:43Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": StatusMatch found: "Import: <serial-output key:'target-size-gb' value:'5'>"
[inflate.wait-for-signal]: 2021-02-17T12:43:43Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": StatusMatch found: "Import: <serial-output key:'source-size-gb' value:'5'>"
[inflate.wait-for-signal]: 2021-02-17T12:43:43Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": StatusMatch found: "Import: <serial-output key:'import-file-format' value:'raw'>"
[inflate.wait-for-signal]: 2021-02-17T12:43:43Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": StatusMatch found: "Import: Ensuring disk-lz55d has capacity of 5 GB in projects/550072179371/zones/europe-west1-b."
[inflate.wait-for-signal]: 2021-02-17T12:43:43Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": StatusMatch found: "Import: /dev/sdc is attached and ready."
[inflate.wait-for-signal]: 2021-02-17T12:44:13Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": StatusMatch found: "Import: <serial-output key:'disk-checksum' value:'a6e310ede3ecdc9de88cd402b4709b6a  --20765c2763c49ae251ce0e7762499abf  --75a1e608e6f1c50758f4fee5a7d8e3d0  --75a1e608e6f1c50758f4fee5a7d8e3d0  -'>"
[inflate.wait-for-signal]: 2021-02-17T12:44:13Z WaitForInstancesSignal: Instance "inst-importer-inflate-p57zp": SuccessMatch found "ImportSuccess: Finished import."
[inflate]: 2021-02-17T12:44:13Z Step "wait-for-signal" (WaitForInstancesSignal) successfully finished.
[inflate]: 2021-02-17T12:44:13Z Running step "cleanup" (DeleteResources)
[inflate.cleanup]: 2021-02-17T12:44:13Z DeleteResources: Deleting instance "inst-importer".
[inflate]: 2021-02-17T12:44:32Z Step "cleanup" (DeleteResources) successfully finished.
[inflate]: 2021-02-17T12:44:32Z Serial-output value -> target-size-gb:5
[inflate]: 2021-02-17T12:44:32Z Serial-output value -> source-size-gb:5
[inflate]: 2021-02-17T12:44:32Z Serial-output value -> import-file-format:raw
[inflate]: 2021-02-17T12:44:32Z Serial-output value -> disk-checksum:a6e310ede3ecdc9de88cd402b4709b6a  --20765c2763c49ae251ce0e7762499abf  --75a1e608e6f1c50758f4fee5a7d8e3d0  --75a1e608e6f1c50758f4fee5a7d8e3d0  -
[inflate]: 2021-02-17T12:44:32Z Workflow "inflate" cleaning up (this may take up to 2 minutes).
[inflate]: 2021-02-17T12:44:38Z Workflow "inflate" finished cleanup.
[import-image]: 2021-02-17T12:44:38Z Finished creating Google Compute Engine disk
[import-image]: 2021-02-17T12:44:38Z Inspecting disk for OS and bootloader
[inspect]: 2021-02-17T12:44:38Z Validating workflow
[inspect]: 2021-02-17T12:44:38Z Validating step "run-inspection"
[inspect]: 2021-02-17T12:44:39Z Validating step "wait-for-signal"
[inspect]: 2021-02-17T12:44:39Z Validating step "cleanup"
[inspect]: 2021-02-17T12:44:39Z Validation Complete
[inspect]: 2021-02-17T12:44:39Z Workflow Project: ascendant-braid-303513
[inspect]: 2021-02-17T12:44:39Z Workflow Zone: europe-west1-b
[inspect]: 2021-02-17T12:44:39Z Workflow GCSPath: gs://ascendant-braid-303513-daisy-bkt-eu/gce-image-import-2021-02-17T12:42:05Z-lz55d
[inspect]: 2021-02-17T12:44:39Z Daisy scratch path: https://console.cloud.google.com/storage/browser/ascendant-braid-303513-daisy-bkt-eu/gce-image-import-2021-02-17T12:42:05Z-lz55d/daisy-inspect-20210217-12:44:38-t6wt4
[inspect]: 2021-02-17T12:44:39Z Uploading sources
[inspect]: 2021-02-17T12:44:42Z Running workflow
[inspect]: 2021-02-17T12:44:42Z Running step "run-inspection" (CreateInstances)
[inspect.run-inspection]: 2021-02-17T12:44:42Z CreateInstances: Creating instance "run-inspection-inspect-t6wt4".
[inspect]: 2021-02-17T12:44:51Z Step "run-inspection" (CreateInstances) successfully finished.
[inspect.run-inspection]: 2021-02-17T12:44:51Z CreateInstances: Streaming instance "run-inspection-inspect-t6wt4" serial port 1 output to https://storage.cloud.google.com/ascendant-braid-303513-daisy-bkt-eu/gce-image-import-2021-02-17T12:42:05Z-lz55d/daisy-inspect-20210217-12:44:38-t6wt4/logs/run-inspection-inspect-t6wt4-serial-port1.log
[inspect]: 2021-02-17T12:44:51Z Running step "wait-for-signal" (WaitForInstancesSignal)
[inspect.wait-for-signal]: 2021-02-17T12:44:51Z WaitForInstancesSignal: Instance "run-inspection-inspect-t6wt4": watching serial port 1, SuccessMatch: "Success:", FailureMatch: ["Failed:" "WARNING Failed to download metadata script" "Failed to download GCS path"] (this is not an error), StatusMatch: "Status:".
[inspect.wait-for-signal]: 2021-02-17T12:45:51Z WaitForInstancesSignal: Instance "run-inspection-inspect-t6wt4": StatusMatch found: "Status: <serial-output key:'inspect_pb' value:'CgkaAjMyKAIwoB84AQ=='>"
[inspect.wait-for-signal]: 2021-02-17T12:45:51Z WaitForInstancesSignal: Instance "run-inspection-inspect-t6wt4": SuccessMatch found "Success: Done!"
[inspect]: 2021-02-17T12:45:51Z Step "wait-for-signal" (WaitForInstancesSignal) successfully finished.
[inspect]: 2021-02-17T12:45:51Z Running step "cleanup" (DeleteResources)
[inspect.cleanup]: 2021-02-17T12:45:51Z DeleteResources: Deleting instance "run-inspection".
[inspect]: 2021-02-17T12:46:07Z Step "cleanup" (DeleteResources) successfully finished.
[inspect]: 2021-02-17T12:46:07Z Serial-output value -> inspect_pb:CgkaAjMyKAIwoB84AQ==
[inspect]: 2021-02-17T12:46:07Z Workflow "inspect" cleaning up (this may take up to 2 minutes).
[inspect]: 2021-02-17T12:46:11Z Workflow "inspect" finished cleanup.
[debug]: 2021-02-17T12:46:11Z Detection results: os_release:{major_version:"32"  architecture:X64  distro_id:FEDORA}  os_count:1
[import-image]: 2021-02-17T12:46:11Z Inspection result=os_release:{distro:"fedora"  major_version:"32"  architecture:X64  distro_id:FEDORA}  elapsed_time_ms:93747  os_count:1
[import-image]: 2021-02-17T12:46:13Z Could not detect operating system. Please re-import with the operating system specified. For more information, see https://cloud.google.com/compute/docs/import/importing-virtual-disks#bootable
ERROR
ERROR: build step 0 "gcr.io/compute-image-tools/gce_vm_image_import:release" failed: step exited with non-zero status: 1`,
			resources: cloudbuildBuildResources{
				zone: "europe-west1-b",
				computeInstances: []string{
					"inst-importer-inflate-p57zp",
					"run-inspection-inspect-t6wt4",
				},
				computeDisks: []string{
					"disk-lz55d",
					"disk-importer-inflate-p57zp",
					"disk-inflate-scratch-p57zp",
				},
				storageCacheDir: struct {
					bucket string
					dir    string
				}{
					bucket: "ascendant-braid-303513-daisy-bkt-eu",
					dir:    "gce-image-import-2021-02-17T12:42:05Z-lz55d",
				},
			},
		},
		{
			buildLog: `starting build "21eb22bb-b92e-41f7-972d-35e75dae2a2c"

FETCHSOURCE
BUILD
Pulling image: gcr.io/compute-image-tools/gce_vm_image_import:release
release: Pulling from compute-image-tools/gce_vm_image_import
a352db2f02b6: Pulling fs layer
8e0ed4351c49: Pulling fs layer
7ef2d30124da: Pulling fs layer
7558c9498dac: Pulling fs layer
ac1adc2a272f: Pulling fs layer
1bfe08dab915: Pulling fs layer
d7a35a584f97: Pulling fs layer
14ef19cde991: Pulling fs layer
e3a2159de935: Pulling fs layer
7558c9498dac: Waiting
ac1adc2a272f: Waiting
1bfe08dab915: Waiting
d7a35a584f97: Waiting
14ef19cde991: Waiting
e3a2159de935: Waiting
7ef2d30124da: Verifying Checksum
7ef2d30124da: Download complete
7558c9498dac: Verifying Checksum
7558c9498dac: Download complete
ac1adc2a272f: Verifying Checksum
ac1adc2a272f: Download complete
a352db2f02b6: Verifying Checksum
a352db2f02b6: Download complete
8e0ed4351c49: Verifying Checksum
8e0ed4351c49: Download complete
14ef19cde991: Verifying Checksum
14ef19cde991: Download complete
1bfe08dab915: Verifying Checksum
1bfe08dab915: Download complete
e3a2159de935: Verifying Checksum
e3a2159de935: Download complete
d7a35a584f97: Verifying Checksum
d7a35a584f97: Download complete
a352db2f02b6: Pull complete
8e0ed4351c49: Pull complete
7ef2d30124da: Pull complete
7558c9498dac: Pull complete
ac1adc2a272f: Pull complete
1bfe08dab915: Pull complete
d7a35a584f97: Pull complete
14ef19cde991: Pull complete
e3a2159de935: Pull complete
Digest: sha256:63ab233c04139087154f27797efbebc9e55302f465ffb56e2dce34c2b5bf5d8a
Status: Downloaded newer image for gcr.io/compute-image-tools/gce_vm_image_import:release
gcr.io/compute-image-tools/gce_vm_image_import:release
[import-image]: 2021-04-27T08:08:40Z The resource 'image-8c27cb332db33890146b290a3989198d829ae456dabf96e5a4461147' already exists. Please pick an image name that isn't already used.
ERROR
ERROR: build step 0 "gcr.io/compute-image-tools/gce_vm_image_import:release" failed: step exited with non-zero status: 1`,
			resources: cloudbuildBuildResources{},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("log #%d", i), func(t *testing.T) {
			resources, err := cloudbuildResourcesFromBuildLog(tc.buildLog)
			require.NoError(t, err)
			require.NotNil(t, resources)

			require.Equal(t, resources.zone, tc.resources.zone)
			require.ElementsMatch(t, resources.computeDisks, tc.resources.computeDisks)
			require.ElementsMatch(t, resources.computeInstances, tc.resources.computeInstances)
			require.Equal(t, resources.storageCacheDir.bucket, tc.resources.storageCacheDir.bucket)
			require.Equal(t, resources.storageCacheDir.dir, tc.resources.storageCacheDir.dir)
		})
	}
}
