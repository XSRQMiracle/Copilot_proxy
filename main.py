#!/usr/bin/env python3
import logging
import os
import sys
import threading

from flask import Flask

import auth
import fallback
import models
from adapters.openai import OpenAIAdapter, get_fallback_model, set_fallback_model
from config import PROXY_PORT, TOKEN_FILE

app = Flask(__name__)
openai_adapter = OpenAIAdapter()


@app.route('/<path:path>', methods=['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS'])
def proxy_route(path):
    return openai_adapter.handle_request(path)


@app.route('/', methods=['GET'])
def health():
    return {
        "status": "running",
        "copilot_token_ready": auth.get_copilot_token() is not None,
        "proxy_port": PROXY_PORT
    }


@app.route('/fallback', methods=['GET'])
def fallback_route():
    return {"fallback_model": get_fallback_model()}


def print_continue_config():
    print(f"\n{'=' * 50}")
    print(f"  Copilot Proxy 已启动!")
    print(f"  地址: http://localhost:{PROXY_PORT}")
    print(f"{'=' * 50}")
    print(f"按 Ctrl+C 停止代理")
    print(f"{'=' * 50}\n")


def main():
    print(r"""
   ____            _ _       _     ____
  / ___|___  _ __ (_) | ___ | |_  |  _ \ _ __ _____  ___   _
 | |   / _ \| '_ \| | |/ _ \| __| | |_) | '__/ _ \ \/ / | | |
 | |__| (_) | |_) | | | (_) | |_  |  __/| | | (_) >  <| |_| |
  \____\___/| .__/|_|_|\___/ \__| |_|   |_|  \___/_/\_\\__, |
            |_|                                         |___/
    """)

    # 1. 尝试加载已保存的 token
    token = auth.load_token()
    auth.set_github_token(token)

    saved_token = auth.get_github_token()
    if saved_token:
        print(f"[~] 发现已保存的 Token: {saved_token[:10]}...{saved_token[-4:]}")
        print(f"[~] 正在验证 Token 有效性...")

        if auth.refresh_copilot_token():
            print(f"[✓] Token 有效!")
        else:
            print(f"[!] Token 已失效，需要重新授权")
            auth.set_github_token(None)

    # 2. 如果没有有效 token，进行 OAuth 授权
    if auth.get_github_token() is None:
        auth.set_github_token(auth.device_auth())
        auth.save_token(auth.get_github_token())

        print(f"\n[~] 正在获取 Copilot Token...")
        if not auth.refresh_copilot_token():
            print(f"\n[✗] 无法获取 Copilot Token")
            print(f"  可能的原因:")
            print(f"  1. 你的 GitHub 账号没有 Copilot 订阅")
            print(f"  2. 你没有 Copilot Pro / 教育版 / 免费版")
            print(f"  请确认: https://github.com/settings/copilot")

            if os.path.exists(TOKEN_FILE):
                os.remove(TOKEN_FILE)
            sys.exit(1)

    # 3. 启动后台 token 刷新
    t = threading.Thread(target=auth.token_refresh_loop, daemon=True)
    t.start()

    # 4. 打印配置说明
    print_continue_config()

    # 4.5 选择并打印 fallback 模型
    try:
        if auth.get_copilot_token():
            fm = fallback.choose_fallback_model(
                models_url=models.get_remote_models_url(),
                headers=models.build_models_headers()
            )
        else:
            fm = fallback.choose_fallback_model(models_url=models.get_local_models_url())

        if fm:
            set_fallback_model(fm)
            print(f"[~] 已选择回退模型: {get_fallback_model()}")
        else:
            print(f"[~] 未能找到合适的回退模型")
    except Exception as e:
        print(f"[!] 回退模型选择失败: {e}")

    # 5. 启动 Flask 代理
    log = logging.getLogger('werkzeug')
    log.setLevel(logging.WARNING)

    try:
        app.run(host='0.0.0.0', port=PROXY_PORT)
    except KeyboardInterrupt:
        print("\n\n[~] 代理已停止，下次运行会自动使用已保存的 Token")


if __name__ == '__main__':
    main()
