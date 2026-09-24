#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""RichPMS OAuth client and stdio bridge for Codex, using Python 3.9+."""

import argparse
import base64
import hashlib
import json
import os
try:
    # Unix：跨进程文件锁；Windows 无 fcntl，退化为无锁（单进程 CLI 场景可接受）
    import fcntl
except ImportError:  # pragma: no cover - Windows
    fcntl = None
from pathlib import Path
import secrets
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import webbrowser
from http.server import BaseHTTPRequestHandler, HTTPServer

# Force UTF-8 on the data streams regardless of the shell locale: WSL/POSIX C-locale Pythons
# otherwise decode stdin as ASCII and silently turn Chinese work content into '?' before send.
for _stream in (sys.stdin, sys.stdout, sys.stderr):
    if _stream is not None and hasattr(_stream, "reconfigure"):
        _stream.reconfigure(encoding="utf-8", errors="replace")

# Keep authorization endpoints explicit: the server's discovery routes fail in Codex.
BASE = os.environ.get("RICHPM_AI_BASE", "http://192.168.7.30:8080").rstrip("/")
AUTH_BASE = f"{BASE}/api/ai"
MCP_URL = f"{AUTH_BASE}/mcp"
PORT = int(os.environ.get("RICHPM_CALLBACK_PORT", "8379"))
REDIRECT = f"http://127.0.0.1:{PORT}/callback"
TIMEOUT_S = int(os.environ.get("RICHPM_AUTH_TIMEOUT", "300"))
SCOPES = "report:read report:write"
# Credentials live outside the repository, in an owner-only directory and file.
# The default file name follows the identity (RICHPM_CLIENT_ID): a preset identity such as
# nexus gets its own cache, so channels never overwrite each other's credentials.
_PRESET_IDENTITY = os.environ.get("RICHPM_CLIENT_ID", "").strip().lower()
_DEFAULT_AUTH_FILE = (".codex", "richpm",
                      f"{_PRESET_IDENTITY}.json" if _PRESET_IDENTITY else "oauth.json")
AUTH_FILE = Path(os.environ.get(
    "RICHPM_AUTH_FILE", str(Path.home().joinpath(*_DEFAULT_AUTH_FILE))
)).expanduser()


class HttpError(RuntimeError):
    """Expose the HTTP status without copying server response bodies or credentials."""

    def __init__(self, status, message):
        super().__init__(message)
        self.status = status


class CallbackHandler(BaseHTTPRequestHandler):
    """Captures the OAuth callback code and returns a browser-friendly page."""

    def do_GET(self):
        """Accept one callback only when state and any supplied issuer match."""
        parsed = urllib.parse.urlparse(self.path)
        params = urllib.parse.parse_qs(parsed.query, keep_blank_values=True)
        if parsed.path != "/callback":
            self.send_response(404)
            self.end_headers()
            return
        state = self.server.oauth_state
        states = params.get("state", [])
        valid_state = (len(states) == 1 and secrets.compare_digest(
            states[0].encode("utf-8"), state["state"].encode("utf-8")))
        valid_issuer = "iss" not in params or params["iss"] == [AUTH_BASE]
        codes, errors = params.get("code", []), params.get("error", [])
        valid_result = ((len(codes) == 1 and bool(codes[0]) and not errors)
                        or (len(errors) == 1 and bool(errors[0]) and not codes))
        if not valid_state or not valid_issuer or not valid_result or state["event"].is_set():
            self.send_response(400)
            self.end_headers()
            return
        state["code"] = codes[0] if codes else None
        state["error"] = errors[0] if errors else None
        state["event"].set()
        body = (
            '<meta charset="utf-8"><body style="font-family:system-ui;'
            'background:#f6f8fb;display:flex;align-items:center;justify-content:center;'
            'height:100vh;margin:0"><div style="background:#fff;padding:32px 40px;'
            'border-radius:10px;box-shadow:0 8px 28px rgba(20,33,61,.12)">'
            "<h2>RichPMS 授权已返回</h2><p>可以关闭此页，回到 Codex 继续。</p>"
            "</div></body>"
        ).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        try:
            self.wfile.write(body)
        except BrokenPipeError:
            pass

    def log_message(self, *args):
        """Suppress access logs because callback URLs contain authorization codes."""
        pass


def http_json(url, payload=None, headers=None, form=False):
    """Sends JSON or form HTTP requests and returns parsed JSON plus headers."""
    req_headers = dict(headers or {})
    data = None
    if payload is not None:
        if form:
            data = urllib.parse.urlencode(payload).encode("utf-8")
            req_headers.setdefault("Content-Type", "application/x-www-form-urlencoded")
        else:
            data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
            req_headers.setdefault("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(urllib.request.Request(url, data=data, headers=req_headers), timeout=30) as resp:
            ctype = resp.headers.get("Content-Type", "")
            if "text/event-stream" in ctype:
                expected_id = payload.get("id") if isinstance(payload, dict) else None
                block = []
                for line in resp:
                    line = line.decode("utf-8", errors="replace")
                    block.append(line)
                    if not line.strip():
                        try:
                            parsed = parse_sse("".join(block), expected_id)
                            return parsed, resp.headers
                        except LookupError:
                            block.clear()
                parsed = parse_sse("".join(block), expected_id)
            else:
                raw = resp.read().decode("utf-8", errors="replace")
                parsed = json.loads(raw) if raw.strip() else None
            return parsed, resp.headers
    except urllib.error.HTTPError as exc:
        exc.close()
        path = urllib.parse.urlsplit(url).path
        raise HttpError(exc.code, f"HTTP {exc.code} from {path}") from None
    except urllib.error.URLError:
        raise RuntimeError("Cannot connect to RichPMS; check the intranet connection.") from None


class MissingSseResponse(RuntimeError, LookupError):
    """The SSE block contains no response matching the outstanding request."""


def parse_sse(raw, expected_id=None):
    """Parse LF/CRLF SSE events, skipping heartbeats and unrelated notifications."""
    for block in raw.replace("\r\n", "\n").replace("\r", "\n").split("\n\n"):
        lines = [line[5:].strip() for line in block.splitlines() if line.startswith("data:")]
        if lines:
            text = "\n".join(lines)
            if text:
                result = json.loads(text)
                if isinstance(result, dict) and "id" in result:
                    if expected_id is None or result["id"] == expected_id:
                        return result
    raise MissingSseResponse("RichPMS returned no matching SSE response.")


def register_client():
    """Registers a public OAuth client for the exact callback URL."""
    payload = {
        "client_name": "Codex RichPMS bridge",
        "redirect_uris": [REDIRECT],
        "grant_types": ["authorization_code", "refresh_token"],
        "response_types": ["code"],
        "token_endpoint_auth_method": "none",
        "scope": SCOPES,
    }
    resp, _ = http_json(f"{AUTH_BASE}/oauth/register", payload)
    client_id = resp.get("client_id") if isinstance(resp, dict) else None
    if not client_id:
        raise RuntimeError("Registration did not return a client ID.")
    return client_id


def authorize(client_id):
    """Run explicit browser PKCE authorization and return the token response."""
    verifier = secrets.token_urlsafe(64)[:86]
    challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).rstrip(b"=").decode()
    state = secrets.token_urlsafe(18)
    query = urllib.parse.urlencode({
        "response_type": "code",
        "client_id": client_id,
        "redirect_uri": REDIRECT,
        "scope": SCOPES,
        "state": state,
        "code_challenge": challenge,
        "code_challenge_method": "S256",
    })
    server = HTTPServer(("127.0.0.1", PORT), CallbackHandler)
    oauth_state = {"state": state, "code": None, "error": None, "event": threading.Event()}
    server.oauth_state = oauth_state
    threading.Thread(target=server.serve_forever, daemon=True).start()
    auth_url = f"{AUTH_BASE}/oauth/authorize?{query}"
    print(f"AUTH_URL={auth_url}", file=sys.stderr, flush=True)
    try:
        if os.environ.get("RICHPM_AI_NO_BROWSER") != "1":
            webbrowser.open(auth_url)
        print(f"WAITING_FOR_AUTH_SECONDS={TIMEOUT_S}", file=sys.stderr, flush=True)
        if not oauth_state["event"].wait(TIMEOUT_S):
            raise RuntimeError("Authorization timed out; run --login again.")
    finally:
        server.shutdown()
        server.server_close()
    if oauth_state["error"]:
        raise RuntimeError("Authorization was denied; run --login again.")
    if not oauth_state["code"]:
        raise RuntimeError("authorization callback did not include code")
    token_resp, _ = http_json(f"{AUTH_BASE}/oauth/token", {
        "grant_type": "authorization_code",
        "code": oauth_state["code"],
        "client_id": client_id,
        "redirect_uri": REDIRECT,
        "code_verifier": verifier,
    }, form=True)
    token = token_resp.get("access_token") if isinstance(token_resp, dict) else None
    if not token:
        raise RuntimeError("Token endpoint did not return an access token.")
    return token_resp


class TokenManager:
    """Persist OAuth credentials privately and serialize rotating-token refreshes."""

    def __init__(self, path=AUTH_FILE):
        """Accept an isolated cache path for tests or a separate RichPMS server."""
        self.path = Path(path)

    def prepare_directory(self):
        """Create the dedicated cache directory with owner-only access."""
        if self.path.parent.is_symlink() or self.path.is_symlink():
            raise RuntimeError("Credential cache must not be a symbolic link.")
        self.path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.path.parent.chmod(0o700)

    def load(self):
        """Read only a well-formed cache for the configured authorization server."""
        try:
            with self.path.open(encoding="utf-8") as stream:
                record = json.load(stream)
        except FileNotFoundError:
            return None
        except (ValueError, OSError):
            raise RuntimeError("Cannot read OAuth cache; run --login to reauthorize.") from None
        if not isinstance(record, dict) or record.get("server") != AUTH_BASE:
            raise RuntimeError("OAuth cache belongs to another server; use a separate auth file.")
        self.path.chmod(0o600)
        return record

    def save(self, record):
        """Replace the cache atomically; never leave partial rotated credentials."""
        self.prepare_directory()
        descriptor, temporary = tempfile.mkstemp(prefix=".oauth-", dir=self.path.parent)
        try:
            with os.fdopen(descriptor, "w", encoding="utf-8") as stream:
                json.dump(record, stream)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, self.path)
        finally:
            if os.path.exists(temporary):
                os.unlink(temporary)

    def token_record(self, client_id, response, previous=None):
        """Normalize expiry and retain the old refresh token if not rotated."""
        if not isinstance(response, dict) or not response.get("access_token"):
            raise RuntimeError("OAuth response did not contain an access token.")
        return {"server": AUTH_BASE, "client_id": client_id,
                "access_token": response["access_token"],
                "refresh_token": response.get("refresh_token") or (previous or {}).get("refresh_token"),
                "expires_at": time.time() + float(response.get("expires_in", 3600))}

    def get_token(self, force_refresh=False, interactive=False):
        """Reuse/refresh authorization; only explicit interactive calls open a browser."""
        self.prepare_directory()
        lock_path = self.path.with_suffix(".lock")
        descriptor = os.open(lock_path,
                             os.O_CREAT | os.O_RDWR | (os.O_NOFOLLOW if hasattr(os, "O_NOFOLLOW") else 0),
                             0o600)
        with os.fdopen(descriptor, "w") as lock:
            if fcntl is not None:
                fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
            record = self.load()
            if record and not force_refresh and record.get("access_token"):
                if float(record.get("expires_at", 0)) > time.time() + 60:
                    return record["access_token"]
            if record and record.get("refresh_token"):
                try:
                    response, _ = http_json(f"{AUTH_BASE}/oauth/token", {
                        "grant_type": "refresh_token", "client_id": record["client_id"],
                        "refresh_token": record["refresh_token"],
                    }, form=True)
                    record = self.token_record(record["client_id"], response, record)
                    self.save(record)
                    return record["access_token"]
                except HttpError as exc:
                    if exc.status not in (400, 401, 403):
                        raise
                    if not interactive:
                        raise RuntimeError("OAuth refresh expired; run the bridge with --login.") from None
            if not interactive:
                raise RuntimeError("RichPMS authorization required; run the bridge with --login.")
            # RICHPM_CLIENT_ID: use a preset identity (e.g. nexus) instead of dynamic registration,
            # so grant pages, governance, and audits attribute this channel separately (468).
            client_id = os.environ.get("RICHPM_CLIENT_ID") or register_client()
            record = self.token_record(client_id, authorize(client_id))
            self.save(record)
            return record["access_token"]


class McpClient:
    """Minimal streamable-HTTP MCP client for RichPMS tools."""

    def __init__(self, token_manager):
        """Keep session/protocol state local while sharing cached authorization."""
        self.token_manager = token_manager
        self.next_id = 1
        self.session_id = None
        self.protocol_version = None

    def rpc(self, method, params=None, expect_response=True):
        """Build CLI requests with monotonically increasing request IDs."""
        msg = {"jsonrpc": "2.0", "method": method}
        if expect_response:
            msg["id"] = self.next_id
            self.next_id += 1
        if params is not None:
            msg["params"] = params
        return self.forward(msg)

    def forward(self, msg):
        """Preserve RPC IDs; retry one HTTP 401, never an uncertain transport failure."""
        headers = {"Accept": "application/json, text/event-stream"}
        if self.session_id:
            headers["Mcp-Session-Id"] = self.session_id
        if self.protocol_version:
            headers["MCP-Protocol-Version"] = self.protocol_version
        for attempt in range(2):
            headers["Authorization"] = "Bearer " + self.token_manager.get_token(force_refresh=bool(attempt))
            try:
                result, resp_headers = http_json(MCP_URL, msg, headers=headers)
                break
            except HttpError as exc:
                if exc.status != 401 or attempt:
                    raise
        if resp_headers.get("Mcp-Session-Id"):
            self.session_id = resp_headers.get("Mcp-Session-Id")
        if "id" not in msg:
            return None
        if not isinstance(result, dict) or result.get("id") != msg["id"]:
            raise RuntimeError("RichPMS returned an invalid JSON-RPC response.")
        if msg["method"] == "initialize" and isinstance(result.get("result"), dict):
            self.protocol_version = result["result"].get("protocolVersion")
        return result

    def initialize(self):
        """Negotiate the upstream protocol before issuing CLI tool requests."""
        result = self.rpc("initialize", {
            "protocolVersion": "2025-03-26",
            "capabilities": {},
            "clientInfo": {"name": "codex-richpm-bridge", "version": "1.1.0"},
        })
        if "error" in result or not isinstance(result.get("result"), dict):
            raise RuntimeError("RichPMS MCP initialization failed.")
        self.rpc("notifications/initialized", expect_response=False)
        return result

    def call_tool(self, name, arguments):
        """Call the named server tool; write approval remains the caller's responsibility."""
        return self.rpc("tools/call", {"name": name, "arguments": arguments})


def serve_stdio(client, source, output):
    """Bridge newline-delimited MCP messages without banners or secrets on stdout."""
    for line in source:
        if not line.strip():
            continue
        request = None
        try:
            request = json.loads(line)
            if (not isinstance(request, dict) or request.get("jsonrpc") != "2.0"
                    or not isinstance(request.get("method"), str)):
                raise ValueError("Invalid request")
            result = client.forward(request)
        except Exception as exc:
            if isinstance(request, dict) and "method" in request and "id" not in request:
                continue
            message = str(exc) if isinstance(exc, RuntimeError) else "RichPMS bridge request failed."
            result = {"jsonrpc": "2.0", "id": request.get("id") if isinstance(request, dict) else None,
                      "error": {"code": -32603, "message": message}}
        if result is not None:
            output.write(json.dumps(result, ensure_ascii=False) + "\n")
            output.flush()


def main():
    """Provide explicit login, one-shot queries, legacy interaction, and MCP stdio."""
    parser = argparse.ArgumentParser(description=__doc__)
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument("--login", action="store_true", help="Authorize in the browser and cache credentials")
    modes.add_argument("--stdio", action="store_true", help="Serve Codex MCP over stdin/stdout")
    modes.add_argument("--call", metavar="TOOL", help="Call a RichPMS tool using cached authorization")
    modes.add_argument("--tools", action="store_true", help="List upstream MCP tools")
    parser.add_argument("--arguments", default="{}", help="JSON object for --call")
    parser.add_argument("--confirm-write", action="store_true", help="Use only after the user approves the exact rows")
    args = parser.parse_args()
    manager = TokenManager()
    if args.login:
        manager.get_token(interactive=True)
        identity = _PRESET_IDENTITY or "dynamic (DCR)"
        print(f"AUTH_OK (identity: {identity}; credentials cached at {AUTH_FILE})")
        return
    client = McpClient(manager)
    if args.stdio:
        serve_stdio(client, sys.stdin, sys.stdout)
        return
    if args.call:
        arguments = json.loads(args.arguments)
        if not isinstance(arguments, dict):
            parser.error("--arguments must be a JSON object")
        if args.call in ("save_daily_report_draft", "submit_daily_report") and not args.confirm_write:
            parser.error("Review exact rows with the user before using --confirm-write")
        client.initialize()
        result = client.call_tool(args.call, arguments)
        print(json.dumps(result, ensure_ascii=False))
        if "error" in result or result.get("result", {}).get("isError"):
            raise SystemExit(1)
        return
    manager.get_token(interactive=not args.tools)
    client.initialize()
    tools_result = client.rpc("tools/list")
    if args.tools:
        print(json.dumps(tools_result, ensure_ascii=False))
        return
    print("READY", flush=True)
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        if line == "exit":
            print("BYE")
            return
        try:
            request = json.loads(line)
            tool_name = request["tool"]
            arguments = request.get("arguments", {})
            result = client.call_tool(tool_name, arguments)
            print("CALL_RESULT=" + json.dumps(result, ensure_ascii=False), flush=True)
        except Exception as exc:
            message = str(exc) if isinstance(exc, RuntimeError) else "RichPMS request failed."
            print("CALL_ERROR=" + message, flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        message = str(exc) if isinstance(exc, RuntimeError) else f"RichPMS failed ({type(exc).__name__})."
        print("FATAL=" + message, file=sys.stderr)
        sys.exit(2)
