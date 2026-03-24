AI File Reader
==============

A Claude Code session can keep wanting to issue `sed -n 'START,ENDp'` commands
which trigger security checks.  Or use `find` in pipelines with chunking and
`ls` invocations or `grep` on the results.

This is a small Go tool which can be used by Claude to perform various actions
to read from the local file-system in areas it is configured to allow reading
from, via a config file.

The goal is to make most data reading operations something which can be
performed safely, without tripping shell safety invocation safe-guards, by
exposing the contents of the file-system through a search/transformation
layer.
