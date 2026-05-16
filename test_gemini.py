import json
from time import time

import requests


BASE_URL = "http://127.0.0.1:15432"


def main() -> None:
    model = "gemini-3-flash-preview"
    url = f"{BASE_URL}/v1beta/models/{model}:streamGenerateContent"
    payload = {
        "contents": [
            {
                "role": "user",
                "parts": [
                    {
                        "text": "深度探究一下为什么元首比猫猫更让申老师讨厌？"
                    }
                ],
            }
        ],
        "generationConfig": {
            "maxOutputTokens": 1024,
            "temperature": 0.7,
        },
    }

    headers = {
        "Content-Type": "application/json",
        "x-goog-api-key": "dummy",
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

            if not line.startswith("data: "):
                continue

            data = line[6:]
            if data == "[DONE]":
                break

            try:
                chunk = json.loads(data)
            except json.JSONDecodeError:
                continue

            candidates = chunk.get("candidates", []) or []
            if not candidates:
                continue

            content = candidates[0].get("content", {}) or {}
            parts = content.get("parts", []) or []
            for part in parts:
                if isinstance(part, dict):
                    text = part.get("text")
                    if text:
                        print(text, end="")

    end_time = time()
    print(f"\n\n---\n\n[+] 耗时: {end_time - start_time:.2f}秒")


if __name__ == "__main__":
    main()
