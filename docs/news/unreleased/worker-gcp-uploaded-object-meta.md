# Worker: Set image name as custom metadata on the file uploaded to GCP Storage

Worker osbuild jobs with GCP upload target now set the chosen image name as
custom metadata on the uploaded object. This makes finding the uploaded
object using the image name possible. The behavior is useful mainly
for cleaning up cloud resources in case of unexpected failures.
