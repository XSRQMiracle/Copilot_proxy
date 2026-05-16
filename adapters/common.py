import json

import requests
import fallback as fallback_module
from config import PROXY_PORT
from proxy import forward_request

_fallback_model: str | None = None


def get_fallback_model() -> str | None:
    return _fallback_model


def set_fallback_model(model: str | None) -> None:
    global _fallback_model
    _fallback_model = model


def is_model_not_supported(resp: requests.Response) -> bool:
    try:
        err_json = resp.json()
        err = err_json.get('error', {}) if isinstance(err_json, dict) else {}
        code = err.get('code')
        msg = err.get('message', '')
    except Exception:
        code = None
        msg = resp.text or ''
    return code == 'model_not_supported' or 'not supported' in (msg or '').lower()


def try_fallback(
    method: str,
    url: str,
    headers: dict[str, str],
    body: bytes | None,
) -> requests.Response | None:
    global _fallback_model

    try:
        requested_model: str | None = None
        new_body = body
        if body:
            text_body = body.decode() if isinstance(body, (bytes, bytearray)) else body
            parsed = json.loads(text_body)
            if isinstance(parsed, dict) and parsed.get('model'):
                requested_model = parsed.get('model')
                if not _fallback_model:
                    set_fallback_model(
                        fallback_module.choose_fallback_model(
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
            method=method,
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


def clean_body(raw_body: bytes | None) -> bytes | None:
    if not raw_body:
        return raw_body
    try:
        data = json.loads(raw_body)
        for key in ['api_key', 'api_base']:
            data.pop(key, None)
        return json.dumps(data).encode()
    except Exception:
        return raw_body
