#!/bin/bash
set -x

ENV_FILE=${1:-.env}

while IFS="" read p
do
  eval export $p
done < <(grep -v '^#' "$ENV_FILE" | grep -v '^$')

make dev
