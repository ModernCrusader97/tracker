#!/bin/bash
cd /home/claude/service/tracker
export $(grep -v '^#' .env | xargs)
exec ./bin/server
