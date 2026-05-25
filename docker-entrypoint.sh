#!/bin/sh

set -e

if [ "$1" = "/app/sso-server" ]; then
    echo "Applying database migrations..."
    /app/sso-migrate up
fi

exec "$@"
