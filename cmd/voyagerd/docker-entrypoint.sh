#!/bin/sh
set -e

# Generate config from template
envsubst < /etc/voyager/voyagerd.template.yaml > /etc/voyager/voyagerd.yaml

# Set permissions for non-root user
chown voyager:voyager /etc/voyager/voyagerd.yaml

# Graceful shutdown
trap 'kill -TERM $PID; wait $PID' TERM INT

# Start application
dumb-init -- /app/voyagerd --config /etc/voyager/voyagerd.yaml &
PID=$!
wait $PID