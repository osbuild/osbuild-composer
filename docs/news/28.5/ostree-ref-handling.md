# Improve OSTree Repository URL and Ref parsing

If the OSTree Repository URL did not end in a `/` the parsing would fail with a less-than-useful error message.  This has been fixed. Error messages for different failure cases have also been improved.
