import os

def extract_functions(file_path: str):
    """
    Heuristic parser to extract functions from pseudo-C files.
    Returns a list of dictionaries containing function data.
    """
    functions = []
    
    if not os.path.exists(file_path):
        return functions
        
    try:
        with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
            lines = f.readlines()
    except Exception as e:
        print(f"Error reading {file_path}: {e}")
        return functions

    in_function = False
    brace_depth = 0
    current_func_start = 0
    current_func_name = "Unknown"
    current_body_lines = []

    # Extremely rudimentary heuristic for signatures
    # Look for a line that isn't empty, doesn't start with '#', 
    # doesn't end in ';', and has a space then a name then '('
    
    # We will mainly track brace depth. If we find an opening '{' at depth 0, 
    # we assume we are starting a function or block.
    
    for i, line in enumerate(lines, start=1):
        stripped = line.strip()
        
        # If we are not inside a function, look for a new function
        if not in_function:
            # If we see an opening brace, we start collecting
            if '{' in line:
                # We assume the signature was on lines prior to this brace.
                # A very basic approach is to look backwards 1-2 lines for a signature.
                # But for now, we just start recording.
                in_function = True
                brace_depth += line.count('{') - line.count('}')
                current_func_start = i
                
                # Attempt to guess function name by looking at lines before
                sig_candidate = "".join([l.strip() for l in lines[max(0, i-3):i]])
                if '(' in sig_candidate and ')' in sig_candidate:
                    # extract what's before the '(' as name
                    before_paren = sig_candidate.split('(')[0].strip()
                    parts = before_paren.split()
                    if len(parts) > 0:
                        current_func_name = parts[-1]
                        # Remove pointers
                        current_func_name = current_func_name.lstrip('*')
                else:
                    current_func_name = f"func_at_line_{i}"
                
                # If we actually had the signature on earlier lines, let's include them in the body
                # For simplicity, we just start the body from wherever we think the block starts, 
                # or a couple lines above to capture the signature
                start_idx = max(0, i-3) if in_function else i-1
                current_body_lines = lines[start_idx:i] # The lines leading up to the brace
                
            continue
            
        # We are inside a function
        current_body_lines.append(line)
        brace_depth += line.count('{') - line.count('}')
        
        if brace_depth <= 0:
            # Function has ended
            func_text = "".join(current_body_lines)
            functions.append({
                "function_name": current_func_name,
                "function_start_line": current_func_start,
                "function_end_line": i,
                "function_source": func_text,
                "file_path": file_path
            })
            
            # Reset state
            in_function = False
            brace_depth = 0
            current_body_lines = []
            
    return functions
