#!/bin/sh
set -e

# The /workspace volume is shared with the scanner and may be created root-owned
# by Docker. Fix its ownership as root, then drop to the unprivileged app user so
# the API process itself never runs as root.
if [ -d /workspace ]; then
	chown app:app /workspace 2>/dev/null || true
fi

exec su-exec app:app "$@"
