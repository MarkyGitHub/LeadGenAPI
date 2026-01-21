"""
Lightweight mock Customer API for live async e2e testing.

Endpoints:
- POST /lead/receive/fake/USER_ID  -> stores payload, returns 200
- GET  /_last                      -> returns last payload
- POST /_reset                     -> clears stored payload
- GET  /_health                    -> returns 200
"""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Optional


LAST_REQUEST: Optional[dict] = None


class Handler(BaseHTTPRequestHandler):
    def _send_json(self, status_code: int, payload: dict) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):  # noqa: N802
        if self.path == "/_health":
            return self._send_json(200, {"status": "ok"})

        if self.path == "/_last":
            return self._send_json(200, {"last": LAST_REQUEST})

        return self._send_json(404, {"error": "not_found"})

    def do_POST(self):  # noqa: N802
        global LAST_REQUEST

        if self.path == "/_reset":
            LAST_REQUEST = None
            return self._send_json(200, {"status": "reset"})

        if self.path.startswith("/lead/receive/"):
            length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(length).decode("utf-8") if length else ""
            try:
                payload = json.loads(raw) if raw else {}
            except json.JSONDecodeError:
                payload = {"_raw": raw}

            LAST_REQUEST = {
                "path": self.path,
                "headers": {k.lower(): v for k, v in self.headers.items()},
                "payload": payload,
            }
            return self._send_json(200, {"status": "received"})

        return self._send_json(404, {"error": "not_found"})

    def log_message(self, format, *args):  # noqa: A003
        # Silence default logging to keep test output clean.
        return


def main() -> None:
    server = HTTPServer(("0.0.0.0", 8080), Handler)
    server.serve_forever()


if __name__ == "__main__":
    main()