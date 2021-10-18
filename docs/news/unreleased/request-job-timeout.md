# Timeout when requesting jobs

When workers request a new job they make a blocking call to the `/api/worker/v1/jobs`
endpoint. There are cases however where a polling approach is more useful, for instance when idle
connections get terminated after a certain period of time.

The new `request_job_timeout` option under the worker config section allows for a timeout on the
`/api/worker/v1/jobs` endpoint. It's a string with `"0"` as default, any string which is parseable
by `time.Duration.ParseDuration()` is allowed however, for instance `"10s"`.

Because this is an expected timeout, "204 No Content" will be returned by the worker server in case
of such a timeout. The worker client will simply poll again straight away.

To maintain backwards compatilibity the default behaviour is still a blocking connection without
timeout.
