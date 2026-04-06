import argparse
import json
import os
import re
import sys

from extract_functions import extract_functions

def get_called_functions(source_code: str, all_function_names: set) -> list:
    """Read the raw C string and extract any tokens that perfectly match known function names."""
    tokens = re.findall(r'\b[a-zA-Z_][a-zA-Z0-9_]*\b', source_code)
    called = []
    for t in tokens:
        if t in all_function_names and t not in called:
            called.append(t)
    return called

def list_guid_functions(c_file_path: str, json_file_path: str):
    """List all functions that reference GUIDs from the module's decompiled_main.json."""
    if not os.path.exists(json_file_path):
        print(f"Error: No GUID file found at {json_file_path}")
        sys.exit(1)

    with open(json_file_path, "r", encoding="utf-8") as f:
        guid_data = json.load(f)
    target_guids = {g["name"] for g in guid_data.get("guids", [])}

    print(f"Loaded {len(target_guids)} GUIDs from {os.path.basename(json_file_path)}\n")

    functions = extract_functions(c_file_path)

    print(f"{'#':<4} {'Function Name':<30} {'Lines':<15} {'Matched GUIDs'}")
    print("-" * 90)

    idx = 0
    for func in functions:
        source = func.get("function_source", "")
        matched = [g for g in target_guids if g in source]
        if matched:
            idx += 1
            name = func["function_name"]
            lines = f"{func['function_start_line']}-{func['function_end_line']}"
            guids_str = ", ".join(matched)
            print(f"{idx:<4} {name:<30} {lines:<15} {guids_str}")

    if idx == 0:
        print("No functions reference any GUIDs in this module.")

def trace_function(c_file_path: str, function_name: str, max_depth: int):
    """Print the source of a function and recursively trace its dependencies."""
    functions = extract_functions(c_file_path)
    all_funcs = {f["function_name"]: f for f in functions}

    if function_name not in all_funcs:
        print(f"Error: Function '{function_name}' was not found!")
        sys.exit(1)

    print(f"Starting tree trace for {function_name} (Max limit: {max_depth} functions)\n")

    queue = [function_name]
    visited = set()

    while queue and len(visited) < max_depth:
        current_func_name = queue.pop(0)

        if current_func_name in visited:
            continue

        visited.add(current_func_name)
        func_data = all_funcs[current_func_name]
        source = func_data.get("function_source", "")

        print("-" * 80)
        print(f"Source Code for: {current_func_name}  (Dependency #{len(visited)})")
        print(f"Lines: {func_data.get('function_start_line')} to {func_data.get('function_end_line')}")
        print("-" * 80)
        print(source)
        print("-" * 80 + "\n")

        called_functions = get_called_functions(source, set(all_funcs.keys()))
        for child in called_functions:
            if child not in visited and child not in queue:
                queue.append(child)

    if queue:
        print(f"Reached the max depth limit of {max_depth}! Did not print: {', '.join(queue)}")

def main():
    parser = argparse.ArgumentParser(description="Inspect functions from a decompiled EDK2 module.")
    parser.add_argument("-t", "--workspace", type=str, required=True, help="Workspace directory (e.g., out_decomplied_files)")
    parser.add_argument("-a", "--module", type=str, required=True, help="Module directory name (e.g., AbtDxe)")
    parser.add_argument("-f", "--function", type=str, default=None, help="Function name to inspect. If omitted, lists all GUID-linked functions.")
    parser.add_argument("-d", "--max-depth", type=int, default=10, help="Max number of dependent functions to print (default 10)")
    args = parser.parse_args()

    c_file_path = os.path.join(args.workspace, args.module, "decompiled_main.c")
    json_file_path = os.path.join(args.workspace, args.module, "decompiled_main.json")

    if not os.path.exists(c_file_path):
        print(f"Error: Could not find C file at {c_file_path}")
        sys.exit(1)

    if args.function:
        trace_function(c_file_path, args.function, args.max_depth)
    else:
        list_guid_functions(c_file_path, json_file_path)

if __name__ == "__main__":
    main()
