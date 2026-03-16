#!/usr/bin/env bash

cleanup() {
    echo -e "\n[!] Execution interrupted by user. Cleaning up..."
    exit 1
}

trap cleanup SIGINT

OUTPATH="out_decomplied_files"

MY_SCRIPTS=$(readlink -f "$(dirname "$0")")
PROJECT_DIR="$MY_SCRIPTS/ghidra_projects"

mkdir -p "$OUTPATH"
mkdir -p "$PROJECT_DIR"

GHIDRA_HEADLESS="/opt/ghidra_12.0.4_PUBLIC/support/analyzeHeadless"
UEFI_SCRIPTS="/opt/ghidra-firmware-utils/ghidra_scripts"

COUNT=0
MAX=5

for bin_efi in ./only_pe_files/*; do

    if [ "$COUNT" -ge "$MAX" ]; then
        echo "[+] Reached limit ($MAX files). Stopping."
        break
    fi

    FILE_INFO=$(file -b "$bin_efi")

    if [[ "$FILE_INFO" != *"PE32"* ]]; then
        continue
    fi

    echo "Detected PE file..."

    name=$(basename "$bin_efi")
    name="${name%_*}"

    mkdir -p "$OUTPATH/$name"
    cp "$bin_efi" "$OUTPATH/$name/main.efi"

    OUTPUT_FILE="$OUTPATH/$name/decompiled_main.c"
    INPUT_FILE=$(readlink -f "$bin_efi")

    analyzeHeadless "$PROJECT_DIR" efi_analysis \
      -import "$INPUT_FILE" \
      -overwrite \
      -scriptPath "$MY_SCRIPTS;$UEFI_SCRIPTS" \
      -postScript UEFIHelper.java \
      -postScript ExportDecompiled.java "$OUTPUT_FILE"

    COUNT=$((COUNT+1))

done
