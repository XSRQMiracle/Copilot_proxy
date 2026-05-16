"""
Gemini API 协议兼容适配器。

接收 Gemini API 格式请求 (/v1beta/models/{model}:generateContent 等)，
转换为 OpenAI chat/completions 格式发送到 Copilot API，
再将 Copilot 响应转换回 Gemini 格式。
"""

import json
import re
from typing import Any
import uuid

import requests
from flask import Response, request

from adapters.base import BaseAdapter
from adapters.common import get_fallback_model, is_model_not_supported, set_fallback_model, try_fallback
from auth import get_copilot_token
from config import COPILOT_API_BASE
from proxy import build_headers, forward_request

STREAM_END_MARKER = 'data: [DONE]'

FINISH_REASON_MAP: dict[str | None, str] = {
    'stop': 'STOP',
    'length': 'MAX_TOKENS',
    'content_filter': 'SAFETY',
}

GEMINI_ROLE_MAP: dict[str, str] = {
    'user': 'user',
    'model': 'assistant',
}


def parse_gemini_path(gemini_path: str) -> tuple[str, str] | None:
    """
    从 Gemini 路径中提取 model 名称和 action。
    例如: 'claude-sonnet-4:generateContent' → ('claude-sonnet-4', 'generateContent')
          'publishers/google/models/gemini-pro:streamGenerateContent' → ('gemini-pro', 'streamGenerateContent')
    """
    last_colon = gemini_path.rfind(':')
    if last_colon == -1:
        return None

    action = gemini_path[last_colon + 1:]
    if action not in ('generateContent', 'streamGenerateContent'):
        return None

    model_part = gemini_path[:last_colon]
    model_match = re.search(r'(?:^|/)models/([^:]+)$', model_part)
    if model_match:
        model = model_match.group(1)
    else:
        model = model_part

    return (model, action)


def _gemini_contents_to_openai_messages(contents: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """将 Gemini contents 数组转换为 OpenAI messages。"""
    messages = []
    for item in contents:
        role = item.get('role', 'user')
        parts = item.get('parts', [])
        texts = []
        for part in parts:
            if isinstance(part, dict) and 'text' in part:
                texts.append(part['text'])
        content = '\n'.join(texts) if texts else ''
        messages.append({'role': GEMINI_ROLE_MAP.get(role, role), 'content': content})
    return messages


def gemini_to_openai(model: str, payload: dict[str, Any], stream: bool = False) -> dict[str, Any]:
    """将 Gemini 请求转换为 OpenAI chat/completions 格式。"""
    messages = []

    system_inst = payload.get('systemInstruction', {}) or {}
    if isinstance(system_inst, dict):
        system_parts = system_inst.get('parts', [])
        system_texts = [p.get('text', '') for p in system_parts if isinstance(p, dict)]
        if system_texts:
            messages.append({'role': 'system', 'content': '\n'.join(system_texts)})

    contents = payload.get('contents', [])
    messages.extend(_gemini_contents_to_openai_messages(contents))

    gc = payload.get('generationConfig', {}) or {}

    oai_payload: dict[str, Any] = {
        'model': model,
        'messages': messages,
        'stream': stream,
    }

    if 'maxOutputTokens' in gc:
        oai_payload['max_tokens'] = gc['maxOutputTokens']
    if 'temperature' in gc:
        oai_payload['temperature'] = gc['temperature']
    if 'topP' in gc:
        oai_payload['top_p'] = gc['topP']
    if 'stopSequences' in gc:
        oai_payload['stop'] = gc['stopSequences']

    return oai_payload


def _map_gemini_finish(reason: str | None) -> str:
    return FINISH_REASON_MAP.get(reason, 'STOP')


def openai_to_gemini_response(oai_data: dict[str, Any]) -> dict[str, Any]:
    """将 OpenAI chat/completions 响应转换为 Gemini generateContent 格式。"""
    choices = oai_data.get('choices', [{}])
    choice = choices[0] if choices else {}
    message = choice.get('message', {})
    content_text = message.get('content', '') or ''
    finish = _map_gemini_finish(choice.get('finish_reason'))

    oai_usage = oai_data.get('usage', {}) or {}

    return {
        'candidates': [
            {
                'content': {
                    'role': 'model',
                    'parts': [{'text': content_text}],
                },
                'finishReason': finish,
                'safetyRatings': [],
                'index': 0,
            }
        ],
        'usageMetadata': {
            'promptTokenCount': oai_usage.get('prompt_tokens', 0),
            'candidatesTokenCount': oai_usage.get('completion_tokens', 0),
            'totalTokenCount': oai_usage.get('total_tokens', 0),
        },
        'modelVersion': oai_data.get('model', ''),
    }


def openai_sse_to_gemini_sse() -> Response:
    """将 OpenAI SSE 流实时转换为 Gemini SSE 流。"""
    copilot_url = f'{COPILOT_API_BASE}/chat/completions'
    headers = build_headers('application/json')
    raw_body = request.get_data()
    body_str = raw_body.decode('utf-8') if raw_body else '{}'

    try:
        payload = json.loads(body_str)
    except json.JSONDecodeError:
        return Response(json.dumps({'error': {'message': '无效的 JSON'}}), status=400, mimetype='application/json')

    gemini_path_match = parse_gemini_path(request.path[len('/v1beta/models/'):])
    if not gemini_path_match:
        return Response(json.dumps({'error': {'message': '无效的请求路径'}}), status=400, mimetype='application/json')
    model, _ = gemini_path_match

    oai_payload = gemini_to_openai(model, payload)
    oai_payload['stream'] = True
    new_body = json.dumps(oai_payload).encode()

    upstream = forward_request('POST', copilot_url, headers, new_body, stream=True)

    if upstream.status_code == 400 and is_model_not_supported(upstream):
        fb = try_fallback('POST', copilot_url, headers, new_body)
        if fb is not None:
            upstream = fb

    has_safety = 'safetySettings' in payload

    sc: int = upstream.status_code
    if sc != 200:
        error_text = upstream.text[:500]
        print(f'[Gemini] API 返回 {sc}: {error_text}')
        return _gemini_error_response(sc, error_text)

    def generate():
        for chunk_bytes in upstream.iter_content(chunk_size=1024):
            if not chunk_bytes:
                continue

            for line in chunk_bytes.decode('utf-8', errors='replace').splitlines():
                line = line.strip()
                if not line:
                    continue
                if line == STREAM_END_MARKER:
                    break
                if not line.startswith('data: '):
                    continue

                data_str = line[6:]
                if data_str == '[DONE]':
                    break

                try:
                    chunk = json.loads(data_str)
                except json.JSONDecodeError:
                    continue

                choices = chunk.get('choices', [{}])
                delta = choices[0].get('delta', {}) if choices else {}
                finish = choices[0].get('finish_reason') if choices else None
                text = delta.get('content', '')

                gemini_chunk: dict = {
                    'candidates': [
                        {
                            'index': 0,
                            'content': {
                                'role': 'model',
                                'parts': [{'text': text}] if text else [],
                            },
                        }
                    ]
                }

                if finish:
                    gemini_chunk['candidates'][0]['finishReason'] = _map_gemini_finish(finish)

                yield f'data: {json.dumps(gemini_chunk)}\n\n'

    resp = Response(
        generate(),
        mimetype='text/event-stream',
        headers={
            'Cache-Control': 'no-cache',
            'X-Accel-Buffering': 'no',
        },
    )

    if has_safety:
        resp.headers['X-Gemini-Warning'] = 'safety_settings_ignored'

    return resp


def _gemini_missing_contents() -> tuple[dict[str, Any], int]:
    return {'error': {'message': 'Missing required field: contents', 'code': 400}}, 400


def _gemini_invalid_path() -> tuple[dict[str, Any], int]:
    return {'error': {'message': 'Invalid request path', 'code': 400}}, 400


def _gemini_error_response(status_code: int, message: str) -> Response:
    body = {
        'error': {
            'code': status_code,
            'message': message,
            'status': 'UNAVAILABLE' if status_code >= 500 else 'INVALID_ARGUMENT',
        }
    }
    return Response(json.dumps(body), status=status_code, mimetype='application/json')


class GeminiAdapter(BaseAdapter):

    def handle_request(self, path: str) -> Response | tuple[dict[str, Any], int]:
        if get_copilot_token() is None:
            return {'error': {'message': 'Copilot token 未就绪，请检查授权状态', 'code': 503}}, 503

        try:
            payload = request.get_json(force=True)
        except Exception:
            return {'error': {'message': 'Invalid JSON body', 'code': 400}}, 400

        if not payload or 'contents' not in payload:
            return _gemini_missing_contents()

        full_path = f'v1beta/models/{path}'
        gemini_path_match = parse_gemini_path(path)
        if not gemini_path_match:
            return _gemini_invalid_path()
        model, action = gemini_path_match

        stream = action == 'streamGenerateContent'

        has_safety = 'safetySettings' in payload

        if stream:
            return self._handle_stream(payload, model, has_safety)

        return self._handle_non_stream(payload, model, has_safety)

    def _handle_non_stream(
        self, payload: dict[str, Any], model: str, has_safety: bool
    ) -> Response | tuple[dict[str, Any], int]:
        oai_payload = gemini_to_openai(model, payload)
        copilot_url = f'{COPILOT_API_BASE}/chat/completions'
        headers = build_headers('application/json')
        body = json.dumps(oai_payload).encode()

        try:
            resp = forward_request('POST', copilot_url, headers, body)

            if resp.status_code == 400 and is_model_not_supported(resp):
                fb = try_fallback('POST', copilot_url, headers, body)
                if fb is not None:
                    resp = fb

            if resp.status_code != 200:
                error_text = resp.text[:500]
                print(f'[Gemini] API 返回 {resp.status_code}: {error_text}')
                sc: int = resp.status_code
                return _gemini_error_response(sc, error_text)

            oai_data = resp.json()
            gemini_resp = openai_to_gemini_response(oai_data)
            flask_resp = Response(json.dumps(gemini_resp), mimetype='application/json')

            if has_safety:
                flask_resp.headers['X-Gemini-Warning'] = 'safety_settings_ignored'

            return flask_resp
        except requests.exceptions.Timeout:
            return {'error': {'message': '请求超时', 'code': 504}}, 504
        except Exception as e:
            print(f'[✗] Gemini 代理错误: {e}')
            return {'error': {'message': str(e), 'code': 502}}, 502

    def _handle_stream(self, payload: dict[str, Any], model: str, has_safety: bool) -> Response:
        oai_payload = gemini_to_openai(model, payload)
        oai_payload['stream'] = True
        copilot_url = f'{COPILOT_API_BASE}/chat/completions'
        headers = build_headers('application/json')
        body = json.dumps(oai_payload).encode()

        upstream = forward_request('POST', copilot_url, headers, body, stream=True)

        if upstream.status_code == 400 and is_model_not_supported(upstream):
            fb = try_fallback('POST', copilot_url, headers, body)
            if fb is not None:
                upstream = fb

        sc: int = upstream.status_code
        if sc != 200:
            error_text = upstream.text[:500]
            print(f'[Gemini] API 返回 {sc}: {error_text}')
            return _gemini_error_response(sc, error_text)

        def generate():
            for chunk_bytes in upstream.iter_content(chunk_size=1024):
                if not chunk_bytes:
                    continue

                for line in chunk_bytes.decode('utf-8', errors='replace').splitlines():
                    line = line.strip()
                    if not line:
                        continue
                    if line == STREAM_END_MARKER:
                        break
                    if not line.startswith('data: '):
                        continue

                    data_str = line[6:]
                    if data_str == '[DONE]':
                        break

                    try:
                        chunk = json.loads(data_str)
                    except json.JSONDecodeError:
                        continue

                    choices = chunk.get('choices', [{}])
                    delta = choices[0].get('delta', {}) if choices else {}
                    finish = choices[0].get('finish_reason') if choices else None
                    text = delta.get('content', '')

                    gemini_chunk: dict[str, Any] = {
                        'candidates': [
                            {
                                'index': 0,
                                'content': {
                                    'role': 'model',
                                    'parts': [{'text': text}] if text else [],
                                },
                            }
                        ]
                    }

                    if finish:
                        gemini_chunk['candidates'][0]['finishReason'] = _map_gemini_finish(finish)

                    yield f'data: {json.dumps(gemini_chunk)}\n\n'

        resp = Response(
            generate(),
            mimetype='text/event-stream',
            headers={
                'Cache-Control': 'no-cache',
                'X-Accel-Buffering': 'no',
            },
        )

        if has_safety:
            resp.headers['X-Gemini-Warning'] = 'safety_settings_ignored'

        return resp
