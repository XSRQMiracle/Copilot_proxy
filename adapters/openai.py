import json

import requests
from flask import request

import fallback
from adapters.base import BaseAdapter
from auth import get_copilot_token
from config import PROXY_PORT, COPILOT_API_BASE
from proxy import build_headers, build_response, forward_request

_fallback_model = None


def get_fallback_model():
    return _fallback_model


def set_fallback_model(model):
    global _fallback_model
    _fallback_model = model


def _clean_body(raw_body):
    if not raw_body:
        return raw_body
    try:
        data = json.loads(raw_body)
        for key in ['api_key', 'api_base']:
            data.pop(key, None)
        return json.dumps(data).encode()
    except Exception:
        return raw_body


def _is_model_not_supported(resp):
    try:
        err_json = resp.json()
        err = err_json.get('error', {}) if isinstance(err_json, dict) else {}
        code = err.get('code')
        msg = err.get('message', '')
    except Exception:
        code = None
        msg = resp.text or ''
    return code == 'model_not_supported' or 'not supported' in (msg or '').lower()


class OpenAIAdapter(BaseAdapter):

    def handle_request(self, path):
        if get_copilot_token() is None:
            return {"error": "Copilot token 未就绪，请检查授权状态"}, 503

        normalized_path = path[3:] if path.startswith('v1/') else path
        url = f"{COPILOT_API_BASE}/{normalized_path}"

        headers = build_headers(request.headers.get('Content-Type', 'application/json'))
        body = _clean_body(request.get_data())

        try:
            resp = forward_request(
                method=request.method,
                url=url,
                headers=headers,
                data=body,
            )

            if resp.status_code != 200:
                error_text = resp.text[:500]
                print(f"[!] API 返回 {resp.status_code}: {error_text}")

            if resp.status_code == 400 and _is_model_not_supported(resp):
                fallback_resp = self._try_fallback(url, headers, body)
                if fallback_resp is not None:
                    resp = fallback_resp

            return build_response(resp)
        except requests.exceptions.Timeout:
            return {"error": "请求超时"}, 504
        except Exception as e:
            print(f"[✗] 代理错误: {e}")
            return {"error": str(e)}, 502

    def _try_fallback(self, url, headers, body):
        global _fallback_model

        try:
            requested_model = None
            new_body = body
            if body:
                text_body = body.decode() if isinstance(body, (bytes, bytearray)) else body
                parsed = json.loads(text_body)
                if isinstance(parsed, dict) and parsed.get('model'):
                    requested_model = parsed.get('model')
                    if not _fallback_model:
                        set_fallback_model(
                            fallback.choose_fallback_model(
                                models_url=f'http://localhost:{PROXY_PORT}/v1/models'
                            )
                        )
                    parsed['model'] = _fallback_model
                    new_body = json.dumps(parsed).encode()

            if not _fallback_model:
                print(f"[!] 模型不可用: {requested_model or 'unknown'}，且未找到可用 fallback")
                return None

            print(f"[!] 模型不可用: {requested_model or 'unknown'}，回退到: {_fallback_model}")

            resp2 = forward_request(
                method=request.method,
                url=url,
                headers=headers,
                data=new_body,
            )
            if resp2.status_code != 200:
                error_text = resp2.text[:500]
                print(f"[!] 回退尝试仍返回 {resp2.status_code}: {error_text}")
            return resp2
        except Exception as e:
            print(f"[!] 回退重试失败: {e}")
            return None
