#!/usr/bin/env python3
"""Validate an operator-owned registry/credential JSON packet from stdin.

Run before writing registry JSON and credential data to the secret store.
This checks configuration shape, not approval. It never writes or prints secrets.
The native Backstage loader accepts the registry JSON as a YAML config overlay.
"""
import json
import re
import sys

MAX_PACKET = 256 * 1024
ENV = re.compile(r"BACKSTAGE_SERVICE_[A-Z][A-Z0-9_]*_TOKEN")
SUBJECT = re.compile(r"[A-Za-z0-9_.:-]{1,128}")
PLUGIN = re.compile(r"[a-z][a-z0-9-]{0,63}")


def exact(value, keys):
    if not isinstance(value, dict) or set(value) != set(keys):
        raise ValueError("shape")


def unique_object(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError("duplicate key")
        value[key] = item
    return value


def validate(packet):
    exact(packet, ["registry", "credentials"])
    registry, credentials = packet["registry"], packet["credentials"]
    exact(registry, ["backend"])
    exact(registry["backend"], ["auth"])
    exact(registry["backend"]["auth"], ["externalAccess"])
    entries = registry["backend"]["auth"]["externalAccess"]
    if not isinstance(entries, list) or len(entries) > 32 or not isinstance(credentials, dict):
        raise ValueError("registry")
    refs = set()
    for entry in entries:
        exact(entry, ["type", "options", "accessRestrictions"])
        if entry["type"] != "static":
            raise ValueError("unsupported authentication")
        options = entry["options"]
        exact(options, ["subject", "token"])
        if not isinstance(options["subject"], str) or not SUBJECT.fullmatch(options["subject"]):
            raise ValueError("subject")
        placeholder = options["token"]
        if not isinstance(placeholder, str) or not placeholder.startswith("${") or not placeholder.endswith("}"):
            raise ValueError("token must be an environment reference")
        ref = placeholder[2:-1]
        if not ENV.fullmatch(ref) or ref in refs:
            raise ValueError("credential reference")
        refs.add(ref)
        restrictions = entry["accessRestrictions"]
        if not isinstance(restrictions, list) or not 1 <= len(restrictions) <= 8:
            raise ValueError("plugin restrictions")
        plugins = set()
        for restriction in restrictions:
            exact(restriction, ["plugin"])
            plugin = restriction["plugin"]
            if not isinstance(plugin, str) or not PLUGIN.fullmatch(plugin) or plugin in plugins:
                raise ValueError("plugin")
            plugins.add(plugin)
    if set(credentials) != refs:
        raise ValueError("credential set")
    for token in credentials.values():
        if not isinstance(token, str) or not 32 <= len(token) <= 4096 or any(ord(c) < 33 or ord(c) > 126 for c in token):
            raise ValueError("credential")
    return len(entries)


def main():
    try:
        data = sys.stdin.buffer.read(MAX_PACKET + 1)
        if len(data) > MAX_PACKET:
            raise ValueError("packet limit")
        packet = json.loads(data, object_pairs_hook=unique_object)
        count = validate(packet)
    except (ValueError, TypeError, KeyError, UnicodeError, RecursionError):
        # Native YAML/JSON errors can contain source excerpts. Never print them.
        print('{"valid":false}')
        return 1
    print(json.dumps({"valid": True, "registrations": count}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
