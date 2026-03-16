#!/usr/bin/env python3

import argparse
import os
import shutil
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path


def is_pe32(path: Path) -> bool:
    try:
        out = subprocess.check_output(["file", "-b", str(path)], text=True)
    except Exception:
        return False
    return "PE32" in out


def derive_name(path: Path) -> str:
    name = path.name
    if "_" in name:
        name = name.rsplit("_", 1)[0]
    else:
        name = path.stem
    return name


def run_one(
    bin_path: Path,
    out_root: Path,
    project_root: Path,
    ghidra_headless: Path,
    script_path: str,
) -> tuple[Path, int, float]:
    name = derive_name(bin_path)
    out_dir = out_root / name
    out_dir.mkdir(parents=True, exist_ok=True)

    (out_dir / "main.efi").write_bytes(bin_path.read_bytes())
    output_file = out_dir / "decompiled_main.c"

    project_name = f"efi_analysis_{name}_{int(time.time() * 1000)}_{os.getpid()}"
    project_dir = project_root

    cmd = [
        str(ghidra_headless),
        str(project_dir),
        project_name,
        "-import",
        str(bin_path.resolve()),
        "-overwrite",
        "-scriptPath",
        script_path,
        "-postScript",
        "UEFIHelper.java",
        "-postScript",
        "ExportDecompiled.java",
        str(output_file),
    ]

    start = time.time()
    try:
        result = subprocess.run(cmd, check=False)
        return_code = result.returncode
    finally:
        project_path = project_root / project_name
        if project_path.exists():
            shutil.rmtree(project_path, ignore_errors=True)
    elapsed = time.time() - start
    return bin_path, return_code, elapsed


def main() -> int:
    parser = argparse.ArgumentParser(description="Parallel Ghidra headless decompile.")
    parser.add_argument("--jobs", type=int, default=max(1, os.cpu_count() // 2))
    parser.add_argument("--outdir", default="out_decomplied_files")
    parser.add_argument("--project-root", default="ghidra_projects")
    parser.add_argument("--input-dir", default="getallbin")
    parser.add_argument(
        "--ghidra-headless",
        default="/opt/ghidra/support/analyzeHeadless",
    )
    parser.add_argument(
        "--uefi-scripts",
        default="/opt/ghidra/Ghidra/Extensions/ghidra-firmware-utils/ghidra_scripts",
    )
    args = parser.parse_args()

    repo_root = Path(__file__).resolve().parent
    out_root = (repo_root / args.outdir).resolve()
    out_root.mkdir(parents=True, exist_ok=True)

    project_root = (repo_root / args.project_root).resolve()
    project_root.mkdir(parents=True, exist_ok=True)

    input_dir = (repo_root / args.input_dir).resolve()
    ghidra_headless = Path(args.ghidra_headless)

    script_path = f"{repo_root};{args.uefi_scripts}"

    bins = [p for p in input_dir.iterdir() if p.is_file() and is_pe32(p)]
    if not bins:
        print("No PE32 files found.")
        return 1

    failures = 0
    with ThreadPoolExecutor(max_workers=args.jobs) as executor:
        futures = [
            executor.submit(
                run_one,
                bin_path,
                out_root,
                project_root,
                ghidra_headless,
                script_path,
            )
            for bin_path in bins
        ]
        for fut in as_completed(futures):
            bin_path, code, elapsed = fut.result()
            if code != 0:
                failures += 1
            status = "OK" if code == 0 else f"FAIL({code})"
            print(f"{status} {bin_path.name} ({elapsed:.1f}s)")

    if failures:
        print(f"Completed with {failures} failure(s).")
        return 1

    print("Completed successfully.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
