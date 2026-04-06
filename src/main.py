import argparse
import os
import time
import json
from pathlib import Path

from extract_functions import extract_functions
from preprocess_functions import preprocess_function
from score_functions import score_function
from store_results import append_to_jsonl, export_summary
from dotenv import load_dotenv

load_dotenv()

def process_module(module_dir: str, model_name: str, verbose: bool = False):
    json_path = os.path.join(module_dir, "decompiled_main.json")
    c_path = os.path.join(module_dir, "decompiled_main.c")
    jsonl_output_path = os.path.join(module_dir, "stage1_scores.jsonl")
    
    # 1. Load GUIDs
    target_guids = set()
    with open(json_path, "r", encoding="utf-8") as f:
        guid_data = json.load(f)
        for g in guid_data.get("guids", []):
            target_guids.add(g["name"])
            
    print(f"[{os.path.basename(module_dir)}] Loaded {len(target_guids)} distinct GUID interfaces.")
    
    # 2. Extract All Functions
    all_functions_dict = {}
    functions = extract_functions(c_path)
    for func in functions:
        func_preprocessed = preprocess_function(func)
        all_functions_dict[func_preprocessed["function_name"]] = func_preprocessed
        
    print(f"[{os.path.basename(module_dir)}] Parsed {len(all_functions_dict)} functions into memory map.")
    
    # 3. Filter for Starting Points
    starting_points = []
    for func_name, func_data in all_functions_dict.items():
        source = func_data.get("function_source", "")
        for guid in target_guids:
            if guid in source:
                starting_points.append(func_data)
                break
                
    print(f"[{os.path.basename(module_dir)}] Found {len(starting_points)} GUID-linked Starting Points.")
    if not starting_points:
        print(f"[{os.path.basename(module_dir)}] No attack surfaces linked to GUIDs. Skipping scoring.")
        return
        
    # 4. Interactive Scoring phase
    for idx, func in enumerate(starting_points, 1):
        if func.get("token_count", 0) < 10:
            func["score_1_to_100"] = 0
            func["reason_summary"] = "Skipped: Token count too low."
            append_to_jsonl(func, jsonl_output_path)
            continue
            
        print(f"[{os.path.basename(module_dir)}] Scoring {idx}/{len(starting_points)}: {func['function_name']}")
        func_scored = score_function(func, all_functions_dict, model_name=model_name, verbose=verbose)
        append_to_jsonl(func_scored, jsonl_output_path)
        
    # Generate the pristine llm_evaluations.json mapping file
    export_summary(jsonl_output_path, module_dir)


def main():
    parser = argparse.ArgumentParser(description="Stage 1: Module-Aware Function Scoring Pipeline")
    parser.add_argument("--input", "-i", type=str, required=True, help="Root workspace or specific module folder.")
    parser.add_argument("--model", "-m", type=str, default="claude-haiku-4-5-20251001", help="Model to use for scoring.")
    parser.add_argument("--verbose", "-v", action="store_true", help="Print the prompts being sent to the LLM.")
    parser.add_argument("--limit", "-l", type=int, default=None, help="Maximum number of module directories to analyze.")
    args = parser.parse_args()
    
    if not os.environ.get("ANTHROPIC_API_KEY"):
        print("Error: ANTHROPIC_API_KEY environment variable not found. Please add it to your .env file.")
        return

    workspace_dir = args.input
    print(f"Scanning workspace for EDK2 Modules: {workspace_dir}")
    
    modules_found = []
    # Identify module boundaries
    for root, dirs, files in os.walk(workspace_dir):
        if "decompiled_main.c" in files and "decompiled_main.json" in files:
            modules_found.append(root)
            
    if not modules_found:
        print("Error: No modules found. Are you sure this folder contains 'decompiled_main.c' and '.json' files?")
        return
            
    print(f"Detected {len(modules_found)} valid EDK2 Modules.")
    
    if args.limit and len(modules_found) > args.limit:
        print(f"Limiting execution to the first {args.limit} modules.\n")
        modules_found = modules_found[:args.limit]
    else:
        print("")
    
    start_time = time.time()
    
    for i, mod_dir in enumerate(modules_found, 1):
        print(f"=== Processing Module {i}/{len(modules_found)}: {os.path.basename(mod_dir)} ===")
        try:
            process_module(mod_dir, args.model, args.verbose)
        except Exception as e:
            print(f"Error processing module {mod_dir}: {e}")
        print("")
        
    elapsed = time.time() - start_time
    print(f"Pipeline completed perfectly in {elapsed:.2f} seconds.")

if __name__ == "__main__":
    main()
