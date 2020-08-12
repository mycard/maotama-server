#!/bin/bash
export ADDRESS=$(dig whoami.cloudflare ch txt @1.0.0.1 +short | sed 's/\"//g')
exec "$@"
