import re

def count_tokens_approx(text: str) -> int:
    """
    Very rough approximation of tokens (words + punctuation).
    """
    return len(re.findall(r'\w+|[^\w\s]', text))

def preprocess_function(func_data: dict) -> dict:
    """
    Preprocess the function source before sending to LLM.
    Strips excessive whitespace and computes token counts.
    """
    source = func_data.get("function_source", "")
    
    # Strip obvious banners or excessive empty lines
    lines = source.split('\n')
    cleaned_lines = [line.rstrip() for line in lines if line.strip() != ""]
    cleaned_source = "\n".join(cleaned_lines)
    
    token_count = count_tokens_approx(cleaned_source)
    
    # Truncate if insanely large (e.g., > 4000 approx tokens). 
    # Just an arbitrary limit to keep LLM context happy.
    if token_count > 4000:
        # Keep beginning and end
        half_limit = 1800 # roughly 1800 words/symbols
        # We'll just slice the characters relative to length to be safe
        split_idx = len(cleaned_source) // 2
        
        # This is a very rough truncation for safety
        cleaned_source = cleaned_source[:split_idx] + "\n\n/* [TRUNCATED] */\n\n" + cleaned_source[-split_idx:]
    
    func_data["function_source"] = cleaned_source
    func_data["token_count"] = token_count
    
    return func_data
