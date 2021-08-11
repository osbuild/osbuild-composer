# Workers: heartbeat

Workers check in with composer every 15 seconds to see if their job hasn't been
cancelled. We can use this to introduce a heartbeat. If the worker fails to
check in for over 2 minutes, composer assumes the worker crashed or was stopped,
marking the job as failed.

This will mitigate the issue where jobs who had their worker crash or stopped,
would remain in a 'building' state forever.
