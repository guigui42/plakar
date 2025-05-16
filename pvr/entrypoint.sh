#!/bin/bash

HOST=""
SNAPSHOT=""
SNAPSHOT_PATH=""
CMD=""

# Parse arguments
while [ "$#" -gt 0 ]; do
    case "$1" in
        -host)
            HOST="$2"
            shift 2
            ;;
        -snapshot)
            SNAPSHOT="$2"
            shift 2
            ;;
        -path)
            SNAPSHOT_PATH="$2"
            shift 2
            ;;
        --)
            shift
            CMD="$@"
            break
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# Set default command if none provided
if [ -z "$CMD" ]; then
    CMD="bash"
fi

# Validate required args
if [ -z "$HOST" ]; then
    echo "Error: -host is required." >&2
    exit 1
fi

KLOSET_MOUNTPOINT="/mnt/kloset"
SNAPSHOT_MOUNTPOINT="/mnt/snapshot"

mkdir -p "$KLOSET_MOUNTPOINT"

# Suppress plakar security check message
plakar -disable-security-check >/dev/null

# Start plakar agent in the foreground and detach it from the terminal.
# We can't run it in the background because plakar agent refuses to launch when syslog is not available (issue #1047).
setsid plakar agent -foreground 2>/dev/null &

# Wait for plakar agent to start
while ! test -S ~/.cache/plakar/agent.sock; do
    sleep 1
done

# Run plakar mount in background, detached from terminal
setsid plakar at "$HOST" mount "$KLOSET_MOUNTPOINT" &

# Wait until mountpoint is established (max 30s)
tries=0
while ! mountpoint -q "$KLOSET_MOUNTPOINT"; do
    sleep 1
    tries=$((tries + 1))
    if [ "$tries" -ge 30 ]; then
        echo "Error: mount did not succeed within 30 seconds." >&2
        exit 1
    fi
done

# Use latest snapshot if not specified
if [ -z "$SNAPSHOT" ]; then
    echo "No snapshot specified, resolving latest"
    SNAPSHOT=$(plakar -no-agent at "$HOST" ls | awk '{print $2; exit}')
fi

matches=($(find "$KLOSET_MOUNTPOINT" -maxdepth 1 -type d -printf "%f\n" | grep "^$SNAPSHOT"))

if [ "${#matches[@]}" -eq 0 ]; then
    echo "No snapshot directory starting with '$SNAPSHOT' found in $KLOSET_MOUNTPOINT" >&2
    exit 1
elif [ "${#matches[@]}" -gt 1 ]; then
    echo "Multiple snapshot directories starting with '$SNAPSHOT' found:" >&2
    printf '  - %s\n' "${matches[@]}" >&2
    exit 1
fi

# Create symlink
ln -sfn "$KLOSET_MOUNTPOINT/${matches[0]}" "$SNAPSHOT_MOUNTPOINT"

cd "$SNAPSHOT_MOUNTPOINT"/"$SNAPSHOT_PATH"

# Execute the command
exec $CMD