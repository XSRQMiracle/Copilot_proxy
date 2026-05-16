from auth import get_copilot_token
from config import PROXY_PORT, VSCODE_HEADERS


def build_models_headers():
    token = get_copilot_token()
    if not token:
        return None
    return {**VSCODE_HEADERS, 'Authorization': f'Bearer {token}'}


def get_remote_models_url():
    return 'https://api.githubcopilot.com/models'


def get_local_models_url():
    return f'http://localhost:{PROXY_PORT}/v1/models'
