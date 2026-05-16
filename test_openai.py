from openai import OpenAI
from time import time

client = OpenAI(
    base_url = "http://127.0.0.1:15432/v1/",
    api_key = "dummy"
)

start_time = time()
completion = client.chat.completions.create(
    model="gpt-4.1",
    messages=[{"role":"user","content": f"为我分析一下，为什么猫猫比元首可爱？"}],
    stream=True
)

for chunk in completion:
    if not getattr(chunk, "choices", None):
        continue
    reasoning = getattr(chunk.choices[0].delta, "reasoning_content", None)
    if reasoning:
        print(reasoning, end="")
    if chunk.choices and chunk.choices[0].delta.content is not None:
        print(chunk.choices[0].delta.content, end="")

end_time = time()
print(f"\n\n---\n\n[+] 耗时: {end_time - start_time:.2f}秒")