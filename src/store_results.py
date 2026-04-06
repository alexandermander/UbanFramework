import json
import csv
import os
from pathlib import Path

def append_to_jsonl(function_data: dict, filepath: str):
    """
    Appends a single JSON dictionary to a JSONL file.
    """
    with open(filepath, 'a', encoding='utf-8') as f:
        f.write(json.dumps(function_data) + '\n')

def export_summary(jsonl_filepath: str, output_dir: str):
    """
    Reads the JSONL file and generates a CSV and a Markdown summary.
    """
    if not os.path.exists(jsonl_filepath):
        print(f"Error: No jsonl file found at {jsonl_filepath}")
        return
        
    records = []
    with open(jsonl_filepath, 'r', encoding='utf-8') as f:
        for line in f:
            if line.strip():
                try:
                    records.append(json.loads(line))
                except json.JSONDecodeError:
                    pass
                    
    if not records:
        print("No valid records found to summarize.")
        return

    # Sort records by score descending
    records.sort(key=lambda x: x.get("score_1_to_100", 0), reverse=True)
    
    # 1. Export JSON list
    json_path = Path(output_dir) / "llm_evaluations.json"
    
    # Strip out the massive function_source strictly from the summary JSON so it's clean and easy to look at
    clean_records = []
    for r in records:
        clean_r = r.copy()
        clean_r.pop("function_source", None)
        clean_records.append(clean_r)
        
    with open(json_path, 'w', encoding='utf-8') as f:
        json.dump(clean_records, f, indent=2)
            
    # 2. Export Markdown (Top 100)
    md_path = Path(output_dir) / "summary.md"
    top_records = records[:100]
    
    with open(md_path, 'w', encoding='utf-8') as f:
        f.write("# Stage 1 Scoring Summary (Top 100)\n\n")
        f.write(f"Total functions tracked: {len(records)}\n\n")
        
        f.write("| Rank | Score | Function | File | Line | Tags | Reason |\n")
        f.write("|---|---|---|---|---|---|---|\n")
        
        for idx, r in enumerate(top_records, 1):
            score = r.get("score_1_to_100", 0)
            func = r.get("function_name", "Unknown")
            file_p = Path(r.get("file_path", "")).name
            line = r.get("function_start_line", 0)
            tags = ", ".join(r.get("suspicion_tags", []))
            reason = r.get("reason_summary", "").replace('\n', ' ')
            
            # Truncate reason if too long
            if len(reason) > 100:
                reason = reason[:97] + "..."
                
            f.write(f"| {idx} | {score} | `{func}` | `{file_p}` | {line} | {tags} | {reason} |\n")
