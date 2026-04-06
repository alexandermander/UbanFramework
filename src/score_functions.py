import json
import os
import anthropic
from datetime import datetime, timezone

def score_function(func_data: dict, all_functions_dict: dict, model_name: str = "claude-3-haiku-20240307", verbose: bool = False) -> dict:
    """
    Evaluates a function iteratively using Anthropic API. If Claude requests context 
    (other functions), it fetches them from all_functions_dict and continues the chat up to 5 jumps.
    """
    source = func_data.get("function_source", "")
    func_name = func_data.get("function_name", "Unknown")
    MAX_JUMPS = 5

    system_prompt = """You are an expert security researcher specializing in EDK2 UEFI firmware binaries and analyzing decompiled pseudo-C code.
Task: Evaluate the EDK2 firmware function for potential security vulnerabilities.

RULES:
- You must reply ONLY with a valid JSON object matching the schema below. Do not output markdown code blocks (like ```json). Just the raw JSON.
- If you need to see the source code of another function called within this code before you can make a safe judgment, put its name in `request_context`.
- If you have enough information to score the function, leave `request_context` empty and provide a `score_1_to_100`.

SCHEMA:
{
  "request_context": ["function_name1", "function_name2"], 
  "score_1_to_100": int,
  "confidence": float (0.0 to 1.0),
  "reason_summary": string,
  "suspicion_tags": [string]
}"""

    initial_user_prompt = f"Function Name: {func_name}\nCode:\n{source}"

    messages = [
        {"role": "user", "content": initial_user_prompt}
    ]

    # Initialize default failure outputs
    func_data["score_1_to_100"] = 0
    func_data["confidence"] = 0.0
    func_data["reason_summary"] = "Error or Max Depth Exceeded"
    func_data["suspicion_tags"] = []
    func_data["analysis_timestamp"] = datetime.now(timezone.utc).isoformat()
    func_data["stage"] = "stage1_scoring_interactive_anthropic"
    
    jumps = 0
    requested_history = set()
    
    # Initialize Anthropic client (automatically picks up os.environ["ANTHROPIC_API_KEY"])
    client = anthropic.Anthropic()
    
    while jumps <= MAX_JUMPS:
        if verbose:
            print(f"\n[VERBOSE] === Sending to Claude ({len(messages)} messages total, Jump {jumps}/{MAX_JUMPS}) ===")
            print(json.dumps(messages[-1], indent=2))
            print("=" * 60)

        try:
            response = client.messages.create(
                model=model_name,
                max_tokens=1024,
                system=system_prompt,
                messages=messages
            )

            message_response = response.content[0].text.strip()
            messages.append({"role": "assistant", "content": message_response})
            
            # Clean possible markdown from the response just in case
            if message_response.startswith("```json"):
                message_response = message_response[7:]
            if message_response.startswith("```"):
                message_response = message_response[3:]
            if message_response.endswith("```"):
                message_response = message_response[:-3]
            message_response = message_response.strip()

            structured_data = json.loads(message_response)
            requests_list = structured_data.get("request_context", [])

            # Filter out requests we already fulfilled to avoid loops
            new_requests = [rq for rq in requests_list if rq not in requested_history]

            if not new_requests:
                func_data["score_1_to_100"] = structured_data.get("score_1_to_100", 0)
                func_data["confidence"] = structured_data.get("confidence", 0.0)
                func_data["reason_summary"] = structured_data.get("reason_summary", "No reason provided")
                func_data["suspicion_tags"] = structured_data.get("suspicion_tags", [])

                if jumps > 0:
                    func_data["suspicion_tags"].append(f"required_{jumps}_context_jumps")
                return func_data

            jumps += 1
            if jumps > MAX_JUMPS:
                func_data["reason_summary"] = f"Max jumps ({MAX_JUMPS}) exceeded while requesting: {new_requests}"
                return func_data

            context_reply = "Here is the context you requested:\n\n"
            found_any = False
            for req_func in new_requests:
                requested_history.add(req_func)
                if req_func in all_functions_dict:
                    context_reply += f"--- Function: {req_func} ---\n"
                    context_reply += all_functions_dict[req_func].get("function_source", "") + "\n\n"
                    found_any = True
                else:
                    context_reply += f"--- Function: {req_func} ---\n[NOT FOUND IN EXTRACTED FILES]\n\n"

            if not found_any:
                context_reply = "None of the requested functions were found in the codebase. Please provide a score based on what you have."

            messages.append({"role": "user", "content": context_reply})

        except anthropic.NotFoundError as e:
            print(f"\n[FATAL ERROR] The model '{model_name}' does not exist or your API key doesn't have access to it! Aborting pipeline.")
            import sys
            sys.exit(1)
        except anthropic.AuthenticationError as e:
            print(f"\n[FATAL ERROR] Anthropic rejected your API key! Aborting pipeline.")
            import sys
            sys.exit(1)
        except Exception as e:
            func_data["reason_summary"] = f"Exception during scoring loop: {str(e)}"
            return func_data

    return func_data
