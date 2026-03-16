#!/usr/bin/env bash

mkdir -p only_pe_files

find . -type f -name body.bin -size +8k -print0 |
while IFS= read -r -d '' one_file; do
    temp=$(echo "$one_file" | awk -F/ '{split($(NF-2),a," "); print a[2]"_"$NF}')
    cp "$one_file" "only_pe_files/$temp" || exit 1
done
