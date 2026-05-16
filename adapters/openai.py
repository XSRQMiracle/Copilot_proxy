import requests
from flask import request

from adapters.base import BaseAdapter
from adapters.common import clean_body, is_model_not_supported, try_fallback
from auth import get_copilot_token
from config import COPILOT_API_BASE
from proxy import build_headers, build_response, forward_request


class OpenAIAdapter(BaseAdapter):

    def handle_request(self, path):
        if get_copilot_token() is None:
            return {"error": "Copilot token 未就绪，请检查授权状态"}, 503

        normalized_path = path[3:] if path.startswith('v1/') else path
        url = f"{COPILOT_API_BASE}/{normalized_path}"

        headers = build_headers(request.headers.get('Content-Type', 'application/json'))
        body = clean_body(request.get_data())

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

            if resp.status_code == 400 and is_model_not_supported(resp):
                fallback_resp = try_fallback(request.method, url, headers, body)
                if fallback_resp is not None:
                    resp = fallback_resp

            return build_response(resp)
        except requests.exceptions.Timeout:
            return {"error": "请求超时"}, 504
        except Exception as e:
            print(f"[✗] 代理错误: {e}")
            return {"error": str(e)}, 502
