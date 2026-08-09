#!/usr/bin/env python3
"""Generate MP3 narration through the configured Hermes Magnific MCP server.

Reads JSON from stdin:
  {
    "text": "...",
    "outputPath": "/path/out.mp3",
    "voiceId": 467,
    "model": "eleven_v3",
    "stability": 0.15,
    "similarityBoost": 0.35,
    "speed": 0.95,
    "useSpeakerBoost": true
  }

Prints {"audioPath":"/path/out.mp3"}. Signed URLs are downloaded immediately
and never printed.
"""
import asyncio
import json
import pathlib
import sys
import urllib.request

sys.path.insert(0, str(pathlib.Path.home() / ".hermes" / "hermes-agent"))
from tools.mcp_tool import _connect_server, _load_mcp_config  # type: ignore


def _identifier(result):
    sc = getattr(result, "structuredContent", None)
    if isinstance(sc, dict):
        creation = sc.get("creation")
        if not creation and isinstance(sc.get("result"), dict):
            creation = sc["result"].get("creation")
        if isinstance(creation, dict) and creation.get("identifier"):
            return creation["identifier"]
        if sc.get("identifier"):
            return sc["identifier"]
    raw = "\n".join(getattr(b, "text", "") for b in (getattr(result, "content", None) or []) if getattr(b, "text", None))
    if raw.strip():
        start = raw.find("{")
        obj = json.loads(raw[start:] if start >= 0 else raw)
        creation = obj.get("creation") or obj.get("result", {}).get("creation")
        if creation and creation.get("identifier"):
            return creation["identifier"]
        if obj.get("identifier"):
            return obj["identifier"]
    raise RuntimeError("Magnific audio_tts did not return a creation identifier")


def _wait_results(result):
    sc = getattr(result, "structuredContent", None)
    if isinstance(sc, dict) and "results" in sc:
        return sc["results"]
    raw = "\n".join(getattr(b, "text", "") for b in (getattr(result, "content", None) or []) if getattr(b, "text", None))
    if raw.strip():
        start = raw.find("{")
        return json.loads(raw[start:] if start >= 0 else raw).get("results", [])
    return []


def _url(item):
    result = item.get("results") or item.get("result") or {}
    if isinstance(result, dict):
        return result.get("url") or result.get("downloadUrl") or result.get("audioUrl")
    return None


async def main():
    payload = json.load(sys.stdin)
    text = str(payload.get("text") or "").strip()
    if not text:
        raise SystemExit("empty text")
    output = pathlib.Path(payload["outputPath"]).expanduser()
    output.parent.mkdir(parents=True, exist_ok=True)

    cfg = _load_mcp_config().get("magnific")
    if not cfg:
        raise SystemExit("Hermes Magnific MCP config missing; run: hermes mcp add magnific --url https://mcp.magnific.com --auth oauth && hermes mcp login magnific")
    server = await _connect_server("magnific", cfg)
    try:
        args = {
            "text": text,
            "voiceId": int(payload["voiceId"]),
            "model": payload.get("model") or "eleven_v3",
            "stability": float(payload.get("stability", 0.15)),
            "similarityBoost": float(payload.get("similarityBoost", 0.35)),
            "speed": float(payload.get("speed", 0.95)),
            "useSpeakerBoost": bool(payload.get("useSpeakerBoost", True)),
            "visible": False,
        }
        created = await server.session.call_tool("audio_tts", arguments=args)
        ident = _identifier(created)
        while True:
            waited = await server.session.call_tool("creations_wait", arguments={"identifiers": [ident], "timeoutSeconds": 25})
            items = _wait_results(waited)
            if not items:
                continue
            item = items[0]
            status = (item.get("status") or "").lower()
            if status == "completed":
                url = _url(item)
                if not url:
                    raise RuntimeError("completed Magnific creation has no audio URL")
                tmp = output.with_suffix(output.suffix + ".tmp")
                with urllib.request.urlopen(url, timeout=120) as resp, tmp.open("wb") as fh:
                    fh.write(resp.read())
                if tmp.stat().st_size <= 0:
                    raise RuntimeError("downloaded empty audio")
                tmp.replace(output)
                print(json.dumps({"audioPath": str(output)}, ensure_ascii=False))
                return
            if status in {"failed", "error", "cancelled"}:
                raise RuntimeError(f"Magnific creation failed: {status}")
    finally:
        await server.shutdown()


if __name__ == "__main__":
    asyncio.run(main())
