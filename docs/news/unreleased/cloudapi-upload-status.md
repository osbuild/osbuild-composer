# Cloud API: include upload target-specific options in `UploadStatus`

The `UploadStatus` now includes additional information in its `options` property.
The information is specific to the chosen target Cloud provider and it is necessary
to successfully identify the built and shared OS image by the end user. Currently
this information is returned for both supported targets, **AWS** and **GCP**.

Information included for **AWS** target:

- AMI
- Region

Information included for **GCP** target:

- Image name
- Image's source Project ID
