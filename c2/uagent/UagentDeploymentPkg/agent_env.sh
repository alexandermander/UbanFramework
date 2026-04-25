#!/usr/bin/env bash

# Source this file from a shell:
#   source /home/alexa/Documents/SanderStuff/aau/cyber2/edk2/TestDevPkg/agent_env.sh

_testdevpkg_this_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export TESTDEVPKG_DIR="${_testdevpkg_this_dir}"
export EDK2_SOURCE_DIR="${_testdevpkg_this_dir}/../source"
export USERVER_DIR="/home/alexa/Documents/SanderStuff/UbanFramework/c2/userver/"

testdevpkg_edksetup() {
  if [ ! -d "${EDK2_SOURCE_DIR}" ]; then
    echo "EDK2 source directory not found: ${EDK2_SOURCE_DIR}" >&2
    return 1
  fi

  cd "${EDK2_SOURCE_DIR}" || return 1
  # shellcheck disable=SC1091
  source edksetup.sh
}

testdevpkg_build() {
  if [ $# -ne 1 ]; then
    echo "usage: testdevpkg_build <ModuleName.inf>" >&2
    return 1
  fi

  build -p TestDevPkg/TestDevPkg.dsc -m "TestDevPkg/$1" -a X64 -t GCC5
}

testdevpkg_build_sample() {
  testdevpkg_build VariableDumpToServer.inf
}

testdevpkg_efi_path() {
  if [ $# -ne 1 ]; then
    echo "usage: testdevpkg_efi_path <ModuleBaseName>" >&2
    return 1
  fi

  printf '%s\n' "${EDK2_SOURCE_DIR}/Build/TestDevPkg/DEBUG_GCC5/X64/$1.efi"
}
