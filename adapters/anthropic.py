"""
Anthropic Messages API 协议兼容适配器。

接收 Anthropic Messages API 格式请求 (/v1/messages)，
转换为 OpenAI chat/completions 格式发送到 Copilot API，
再将 Copilot 响应转换回 Anthropic 格式。
"""

import json
from typing import Any
import uuid

import requests
from flask import Response, request

from adapters.base import BaseAdapter
from adapters.common import clean_body, get_fallback_model, is_model_not_supported, set_fallback_model, try_fallback
from auth import get_copilot_token
from config import COPILOT_API_BASE
from proxy import build_headers, forward_request

STREAM_END_MARKER = 'data: [DONE]\n\n'

FINISH_REASON_MAP: dict[str | None, str] = {
    'stop': 'end_turn',
    'length': 'max_tokens',
    'content_filter': 'content_filter',
}

ROLE_MAP: dict[str, str] = {
    'user': 'user',
    'assistant': 'assistant',
}


def _convert_anthropic_messages(anthropic_msg: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Anthropic messages → OpenAI messages (role/content pairs)."""
    result = []
    for msg in anthropic_msg:
        role = msg.get('role', 'user')
        content = msg.get('content', '')
        if isinstance(content, list):
            texts = []
            for block in content:
                if isinstance(block, dict) and block.get('type') == 'text':
                    texts.append(block.get('text', ''))
            content = '\n'.join(texts)
        result.append({'role': ROLE_MAP.get(role, role), 'content': content})
    return result


def anthropic_to_openai(payload: dict[str, Any]) -> dict[str, Any]:
    """将 Anthropic Messages API 请求转换为 OpenAI chat/completions 格式。"""
    messages = []

    system = payload.get('system')
    if system:
        messages.append({'role': 'system', 'content': system})

    anthropic_msgs = payload.get('messages', [])
    messages.extend(_convert_anthropic_messages(anthropic_msgs))

    oai_payload: dict[str, Any] = {
        'model': payload.get('model', ''),
        'messages': messages,
        'max_tokens': payload.get('max_tokens', 4096),
        'stream': payload.get('stream', False),
    }

    if 'temperature' in payload:
        oai_payload['temperature'] = payload['temperature']
    if 'top_p' in payload:
        oai_payload['top_p'] = payload['top_p']
    if 'stop_sequences' in payload:
        oai_payload['stop'] = payload['stop_sequences']
    if 'metadata' in payload:
        oai_payload['metadata'] = payload['metadata']

    return oai_payload


def _map_finish_reason(oai_reason: str | None) -> str:
    return FINISH_REASON_MAP.get(oai_reason, 'end_turn')


def openai_to_anthropic_response(oai_data: dict[str, Any]) -> dict[str, Any]:
    """将 OpenAI chat/completions 响应转换为 Anthropic Messages API 格式。"""
    choices = oai_data.get('choices', [{}])
    choice = choices[0] if choices else {}
    message = choice.get('message', {})
    content_text = message.get('content', '') or ''
    finish = _map_finish_reason(choice.get('finish_reason'))

    oai_usage = oai_data.get('usage', {}) or {}
    anthropic_usage = {
        'input_tokens': oai_usage.get('prompt_tokens', 0),
        'output_tokens': oai_usage.get('completion_tokens', 0),
    }

    return {
        'id': f"msg_{oai_data.get('id', str(uuid.uuid4().hex[:24]))}",
        'type': 'message',
        'role': 'assistant',
        'content': [{'type': 'text', 'text': content_text}],
        'model': oai_data.get('model', ''),
        'stop_reason': finish,
        'stop_sequence': None,
        'usage': anthropic_usage,
    }


def _build_anthropic_usage(prompt_tokens: int = 0, completion_tokens: int = 0) -> dict[str, Any]:
    return {'input_tokens': prompt_tokens, 'output_tokens': completion_tokens}


def openai_sse_to_anthropic_sse() -> Response:
    """将 OpenAI SSE 流实时转换为 Anthropic SSE 事件流。"""
    url = request.url.replace('/v1/messages', '/v1/chat/completions')
    copilot_url = f"{COPILOT_API_BASE}/chat/completions"

    headers = build_headers(request.headers.get('Content-Type', 'application/json'))
    raw_body = clean_body(request.get_data())
    body_str = raw_body.decode('utf-8') if raw_body else '{}'
    oai_body = anthropic_to_openai(json.loads(body_str))
    oai_body['stream'] = True
    new_body = json.dumps(oai_body).encode()

    upstream = forward_request('POST', copilot_url, headers, new_body, stream=True)

    if upstream.status_code == 400 and is_model_not_supported(upstream):
        fb = try_fallback('POST', copilot_url, headers, new_body)
        if fb is not None:
            upstream = fb

    if upstream.status_code != 200:
        error_text = upstream.text[:500]
        print(f"[Anthropic] API 返回 {upstream.status_code}: {error_text}")
        return _anthropic_error_response(upstream.status_code, error_text)

    def generate():
        message_id = f"msg_{uuid.uuid4().hex[:24]}"
        model_name = oai_body.get('model', '')
        has_started = False
        prompt_tokens = 0
        completion_tokens = 0
        stop_reason = None

        for chunk_bytes in upstream.iter_content(chunk_size=1024):
            if not chunk_bytes:
                continue

            for line in chunk_bytes.decode('utf-8', errors='replace').splitlines():
                line = line.strip()
                if not line or line == STREAM_END_MARKER.strip():
                    continue
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

                if not has_started:
                    has_started = True
                    usage = chunk.get('usage', {}) or {}
                    prompt_tokens = usage.get('prompt_tokens', 0)
                    completion_tokens = usage.get('completion_tokens', 0)

                    msg_start = {
                        'type': 'message_start',
                        'message': {
                            'id': message_id,
                            'type': 'message',
                            'role': 'assistant',
                            'content': [],
                            'model': model_name,
                            'stop_reason': None,
                            'stop_sequence': None,
                            'usage': _build_anthropic_usage(prompt_tokens, 0),
                        },
                    }
                    yield f"event: message_start\ndata: {json.dumps(msg_start)}\n\n"

                    content_block = {'type': 'text', 'text': ''}
                    cb_start = {
                        'type': 'content_block_start',
                        'index': 0,
                        'content_block': content_block,
                    }
                    yield f"event: content_block_start\ndata: {json.dumps(cb_start)}\n\n"

                text = delta.get('content', '')
                if text:
                    cb_delta = {
                        'type': 'content_block_delta',
                        'index': 0,
                        'delta': {'type': 'text_delta', 'text': text},
                    }
                    yield f"event: content_block_delta\ndata: {json.dumps(cb_delta)}\n\n"

                if finish:
                    stop_reason = _map_finish_reason(finish)
                    yield f"event: content_block_stop\ndata: {json.dumps({'type': 'content_block_stop', 'index': 0})}\n\n"

                    msg_delta = {
                        'type': 'message_delta',
                        'delta': {'stop_reason': stop_reason, 'stop_sequence': None},
                        'usage': _build_anthropic_usage(0, completion_tokens),
                    }
                    yield f"event: message_delta\ndata: {json.dumps(msg_delta)}\n\n"

                    yield f"event: message_stop\ndata: {json.dumps({'type': 'message_stop'})}\n\n"

        if not has_started:
            yield _anthropic_sse_error(503, '上游无响应')

    return Response(
        generate(),
        mimetype='text/event-stream',
        headers={
            'Cache-Control': 'no-cache',
            'X-Accel-Buffering': 'no',
        },
    )


def _anthropic_error_response(status_code: int, message: str) -> Response:
    body = {
        'type': 'error',
        'error': {
            'type': 'api_error',
            'message': message,
        },
    }
    return Response(json.dumps(body), status=status_code, mimetype='application/json')


def _anthropic_sse_error(status_code: int, message: str) -> str:
    err = {
        'type': 'error',
        'error': {'type': 'api_error', 'message': message},
    }
    return f"event: error\ndata: {json.dumps(err)}\n\n"


class AnthropicAdapter(BaseAdapter):

    def handle_request(self, path: str) -> Response | tuple[dict[str, Any], int]:
        if get_copilot_token() is None:
            return {'error': 'Copilot token 未就绪，请检查授权状态'}, 503

        try:
            payload = request.get_json(force=True)
        except Exception:
            return {'error': '无效的 JSON 请求体'}, 400

        if not payload or 'messages' not in payload:
            return {'error': '缺少必填字段: messages'}, 400

        stream = payload.get('stream', False)

        if stream:
            return openai_sse_to_anthropic_sse()

        oai_payload = anthropic_to_openai(payload)
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
                print(f'[Anthropic] API 返回 {resp.status_code}: {error_text}')
                sc: int = resp.status_code
                return _anthropic_error_response(sc, error_text)

            oai_data = resp.json()
            anthropic_resp = openai_to_anthropic_response(oai_data)
            return Response(json.dumps(anthropic_resp), mimetype='application/json')
        except requests.exceptions.Timeout:
            return {'error': '请求超时'}, 504
        except Exception as e:
            print(f'[✗] Anthropic 代理错误: {e}')
            return {'error': str(e)}, 502
