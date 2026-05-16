import os

PROXY_PORT = 15432
TOKEN_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), '.copilot_token.json')
CLIENT_ID = 'Iv1.b507a08c87ecfe98'  # VS Code 官方 Client ID

VSCODE_HEADERS = {
    'Editor-Version': 'vscode/1.96.0',
    'Editor-Plugin-Version': 'copilot/1.246.0',
    'User-Agent': 'GithubCopilot/1.246.0',
    'Accept': 'application/json',
}

COPILOT_API_BASE = 'https://api.githubcopilot.com'
