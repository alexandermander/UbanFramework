import os
from pathlib import Path

def discover_pseudo_c_files(input_dir: str):
    """
    Recursively walks the input directory and yields paths to typical C/pseudo-C files.
    """
    input_path = Path(input_dir)
    if not input_path.exists() or not input_path.is_dir():
        print(f"Warning: Input directory '{input_dir}' does not exist or is not a directory.")
        return

    # Decompiled output can sometimes use .c, .h, or .cpp.
    valid_extensions = {".c", ".h", ".cpp", ".cc"}

    for root, _, files in os.walk(input_path):
        for file in files:
            file_path = Path(root) / file
            if file_path.suffix.lower() in valid_extensions:
                yield str(file_path)
