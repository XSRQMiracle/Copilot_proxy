import json
from time import time

import requests


BASE_URL = "http://127.0.0.1:15432"


def main() -> None:
    url = f"{BASE_URL}/v1/messages"
    payload = {
        "model": "claude-haiku-4.5",
        "max_tokens": 1024,
        "stream": True,
        "messages": [
            {
                "role": "user",
                "content": "深度探究一下水母和元首那个更可爱？",
            }
        ],
    }

    headers = {
        "Content-Type": "application/json",
        "x-api-key": "dummy",
        "anthropic-version": "2023-06-01",
    }

    start_time = time()
    with requests.post(url, headers=headers, json=payload, stream=True, timeout=300) as response:
        response.raise_for_status()
        response.encoding = "utf-8"

        for raw_line in response.iter_lines(decode_unicode=True):
            if not raw_line:
                continue

            line = raw_line.strip()
            if not line:
                continue

            if line.startswith("data: "):
                data = line[6:]
                if data == "[DONE]":
                    break

                try:
                    event = json.loads(data)
                except json.JSONDecodeError:
                    continue

                event_type = event.get("type")
                if event_type == "content_block_delta":
                    delta = event.get("delta", {}) or {}
                    if delta.get("type") == "text_delta":
                        print(delta.get("text", ""), end="")
                elif event_type == "message_stop":
                    break

    end_time = time()
    print(f"\n\n---\n\n[+] 耗时: {end_time - start_time:.2f}秒")


if __name__ == "__main__":
    main()
