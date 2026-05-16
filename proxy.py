import requests
from flask import Response

from config import VSCODE_HEADERS
from auth import get_copilot_token

EXCLUDED_RESPONSE_HEADERS = [
    'content-encoding',
    'connection',
    'transfer-encoding',
    'content-length',
]


def build_headers(content_type):
    return {
        **VSCODE_HEADERS,
        'Content-Type': content_type or 'application/json',
        'Authorization': f'Bearer {get_copilot_token()}',
        'Copilot-Integration-Id': 'vscode-chat',
    }


def forward_request(method, url, headers, data, stream=True, timeout=120):
    return requests.request(
        method=method,
        url=url,
        headers=headers,
        data=data,
        stream=stream,
        timeout=timeout,
    )


def build_response(resp):
    response_headers = {
        k: v for k, v in resp.headers.items()
        if k.lower() not in EXCLUDED_RESPONSE_HEADERS
    }

    return Response(
        resp.iter_content(chunk_size=1024),
        status=resp.status_code,
        headers=response_headers,
        direct_passthrough=True,
    )
