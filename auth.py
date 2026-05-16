import json
import os
import sys
import time
import webbrowser

import requests

from config import CLIENT_ID, TOKEN_FILE, VSCODE_HEADERS

github_token = None
copilot_token = None


def get_github_token():
    return github_token


def set_github_token(token):
    global github_token
    github_token = token


def get_copilot_token():
    return copilot_token


def save_token(token):
    with open(TOKEN_FILE, 'w') as f:
        json.dump({'github_token': token}, f)
    print(f"[✓] Token 已保存到 {TOKEN_FILE}")


def load_token():
    if os.path.exists(TOKEN_FILE):
        try:
            with open(TOKEN_FILE, 'r') as f:
                data = json.load(f)
                return data.get('github_token')
        except Exception:
            return None
    return None


def device_auth():
    print("\n" + "=" * 50)
    print("  GitHub Copilot 授权")
    print("=" * 50)

    print("\n[1/3] 正在请求设备验证码...")
    r = requests.post(
        'https://github.com/login/device/code',
        headers={'Accept': 'application/json'},
        data={'client_id': CLIENT_ID, 'scope': 'read:user'}
    )
    data = r.json()

    if 'user_code' not in data:
        print(f"[✗] 请求失败: {data}")
        sys.exit(1)

    user_code = data['user_code']
    device_code = data['device_code']
    interval = data.get('interval', 5)
    expires_in = data.get('expires_in', 900)

    print(f"\n[2/3] 请在浏览器中完成授权:")
    print(f"")
    print(f"  ┌─────────────────────────────┐")
    print(f"  │                             │")
    print(f"  │     Code:  {user_code}        │")
    print(f"  │                             │")
    print(f"  └─────────────────────────────┘")
    print(f"")
    print(f"  打开: https://github.com/login/device")
    print(f"  输入上面的验证码并授权")
    print(f"")

    try:
        webbrowser.open('https://github.com/login/device')
        print("  (已自动打开浏览器)")
    except Exception:
        print("  (请手动打开上面的链接)")

    print(f"\n[3/3] 等待授权中... (超时: {expires_in}秒)")

    start_time = time.time()
    while time.time() - start_time < expires_in:
        time.sleep(interval)

        r = requests.post(
            'https://github.com/login/oauth/access_token',
            headers={'Accept': 'application/json'},
            data={
                'client_id': CLIENT_ID,
                'device_code': device_code,
                'grant_type': 'urn:ietf:params:oauth:grant-type:device_code'
            }
        )
        result = r.json()

        error = result.get('error')
        if error == 'authorization_pending':
            elapsed = int(time.time() - start_time)
            print(f"  等待中... ({elapsed}秒)", end='\r')
            continue
        elif error == 'slow_down':
            interval += 5
            continue
        elif error == 'expired_token':
            print("\n[✗] 验证码已过期，请重新运行脚本")
            sys.exit(1)
        elif error == 'access_denied':
            print("\n[✗] 授权被拒绝")
            sys.exit(1)
        elif 'access_token' in result:
            token = result['access_token']
            print(f"\n\n[✓] 授权成功!")
            print(f"  Token: {token[:10]}...{token[-4:]}")
            return token
        else:
            print(f"\n[✗] 未知响应: {result}")
            sys.exit(1)

    print("\n[✗] 授权超时")
    sys.exit(1)


def refresh_copilot_token():
    global copilot_token

    headers = {**VSCODE_HEADERS, 'Authorization': f'token {github_token}'}

    try:
        r = requests.get(
            'https://api.github.com/copilot_internal/v2/token',
            headers=headers
        )
        data = r.json()

        if 'token' in data:
            copilot_token = data['token']
            expires_at = data.get('expires_at', 0)
            expire_time = time.strftime('%H:%M:%S', time.localtime(expires_at))
            print(f"[✓] Copilot Token 刷新成功 (过期时间: {expire_time})")
            return True
        else:
            msg = data.get('message', str(data))
            print(f"[✗] Copilot Token 刷新失败: {msg}")
            return False
    except Exception as e:
        print(f"[✗] 请求异常: {e}")
        return False


def token_refresh_loop():
    while True:
        time.sleep(1500)
        print(f"\n[~] 自动刷新 Copilot Token...")
        refresh_copilot_token()
