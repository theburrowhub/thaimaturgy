"""A complete, deliberately small stateful rules package."""

ACTION_ID = "simple_d6.check"
EVENT_TYPE = "simple_d6.check_resolved"

def manifest():
    return {
        "id": "simple-d6",
        "name": "Simple d6",
        "description": "Minimal stateful Starlark example for the external rules package protocol.",
        "version": "0.1.0",
        "protocol_version": "1.0.0",
        "runtime": {
            "kind": "starlark",
            "entrypoint": "main.star",
        },
        "capabilities": [ACTION_ID],
    }

def initial_state():
    return {
        "schema_version": 1,
        "attempts": 0,
        "last": {},
    }

def list_actions(request):
    return [{
        "id": ACTION_ID,
        "label": "Make a d6 check",
        "description": "Roll 1d6, add a modifier, and compare the total with a target.",
        "input_schema": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "modifier": {
                    "type": "integer",
                    "minimum": -20,
                    "maximum": 20,
                    "default": 0,
                },
                "target": {
                    "type": "integer",
                    "minimum": 1,
                    "maximum": 30,
                },
            },
            "required": ["target"],
        },
        "tags": ["check", "example"],
    }]

def _reject(step_id, code, message):
    return {
        "id": step_id,
        "kind": "reject",
        "reject": {
            "code": code,
            "message": message,
        },
    }

def _has_only_keys(value, keys):
    if type(value) != "dict" or len(value) != len(keys):
        return False
    for key in value:
        if key not in keys:
            return False
    return True

def start(request):
    intent = request["intent"]
    if intent["action_id"] != ACTION_ID:
        return _reject(intent["id"], "unknown.action", "This package does not offer that action.")

    arguments = intent["arguments"]
    if not _has_only_keys(arguments, ["modifier", "target"]) and not _has_only_keys(arguments, ["target"]):
        return _reject(intent["id"], "invalid.arguments", "Expected target and optional modifier only.")
    modifier = arguments.get("modifier", 0)
    target = arguments.get("target")
    if type(modifier) != "int" or modifier < -20 or modifier > 20:
        return _reject(intent["id"], "invalid.arguments", "modifier must be an integer from -20 to 20.")
    if type(target) != "int" or target < 1 or target > 30:
        return _reject(intent["id"], "invalid.arguments", "target must be an integer from 1 to 30.")

    return {
        "id": intent["id"],
        "kind": "need_random",
        "continuation": {
            "phase": "await_random",
            "intent_id": intent["id"],
            "modifier": modifier,
            "target": target,
        },
        "need_random": {
            "method": "dice.roll",
            "specification": {
                "count": 1,
                "sides": 6,
            },
        },
    }

def _random_result(request):
    pending = request["pending"]
    continuation = pending["state"]
    if pending["kind"] != "need_random" or not _has_only_keys(continuation, ["phase", "intent_id", "modifier", "target"]):
        fail("invalid random continuation")
    if continuation["phase"] != "await_random":
        fail("invalid random continuation phase")
    response = request["response"]["data"]
    if not _has_only_keys(response, ["rolls"]):
        fail("dice.roll response must contain rolls only")
    rolls = response["rolls"]
    if type(rolls) != "list" or len(rolls) != 1 or type(rolls[0]) != "int" or rolls[0] < 1 or rolls[0] > 6:
        fail("dice.roll response must contain one d6 result")

    roll = rolls[0]
    total = roll + continuation["modifier"]
    result = {
        "intent_id": continuation["intent_id"],
        "roll": roll,
        "modifier": continuation["modifier"],
        "target": continuation["target"],
        "total": total,
        "success": total >= continuation["target"],
    }
    return {
        "id": pending["step_id"],
        "kind": "emit",
        "continuation": {
            "phase": "await_emit",
            "result": result,
        },
        "emit": {
            "events": [{
                "type": EVENT_TYPE,
                "schema_version": 1,
                "data": result,
            }],
        },
    }

def _emission_result(request):
    pending = request["pending"]
    continuation = pending["state"]
    if pending["kind"] != "emit" or not _has_only_keys(continuation, ["phase", "result"]):
        fail("invalid emission continuation")
    if continuation["phase"] != "await_emit" or not _valid_result(continuation["result"]):
        fail("invalid emission continuation state")

    acknowledgement = request["response"]["data"]
    if not _has_only_keys(acknowledgement, ["base_revision", "revision"]):
        fail("invalid emission acknowledgement")
    revision = request["snapshot"]["revision"]
    if acknowledgement["revision"] != revision or acknowledgement["base_revision"] + 1 != revision:
        fail("emission acknowledgement does not match the committed snapshot")
    state = request["snapshot"]["state"]
    if _state_diagnostic(state) != None or state["last"] != continuation["result"]:
        fail("emitted event is not present in the committed snapshot")

    result = continuation["result"]
    success = result["success"]
    return {
        "id": pending["step_id"],
        "kind": "complete",
        "complete": {
            "outcome": "simple_d6.check.success" if success else "simple_d6.check.failure",
            "result": {
                "intent_id": result["intent_id"],
                "roll": result["roll"],
                "modifier": result["modifier"],
                "target": result["target"],
                "total": result["total"],
                "success": success,
                "attempts": state["attempts"],
            },
        },
    }

def resume(request):
    kind = request["pending"]["kind"]
    if kind == "need_random":
        return _random_result(request)
    if kind == "emit":
        return _emission_result(request)
    fail("simple-d6 cannot resume this step kind")

def project(request):
    return {"view": request["snapshot"]["state"]}

def explain(request):
    if request["reference"] == ACTION_ID:
        return {"text": "Roll one six-sided die, add the modifier, and succeed when the total reaches the target."}
    return {"text": "No package-specific explanation is available for that reference."}

def _valid_result(result):
    keys = ["intent_id", "roll", "modifier", "target", "total", "success"]
    if not _has_only_keys(result, keys):
        return False
    if type(result["intent_id"]) != "string" or len(result["intent_id"]) == 0:
        return False
    if type(result["roll"]) != "int" or result["roll"] < 1 or result["roll"] > 6:
        return False
    if type(result["modifier"]) != "int" or result["modifier"] < -20 or result["modifier"] > 20:
        return False
    if type(result["target"]) != "int" or result["target"] < 1 or result["target"] > 30:
        return False
    if type(result["total"]) != "int" or result["total"] != result["roll"] + result["modifier"]:
        return False
    return type(result["success"]) == "bool" and result["success"] == (result["total"] >= result["target"])

def _state_diagnostic(state):
    if not _has_only_keys(state, ["schema_version", "attempts", "last"]):
        return "state must contain schema_version, attempts, and last only"
    if type(state["schema_version"]) != "int" or state["schema_version"] != 1:
        return "unsupported state schema_version"
    attempts = state["attempts"]
    if type(attempts) != "int" or attempts < 0 or attempts > 2147483647:
        return "attempts must be a non-negative 32-bit integer"
    last = state["last"]
    if type(last) != "dict":
        return "last must be an object"
    if len(last) == 0:
        if attempts != 0:
            return "a non-zero attempt count requires a last result"
        return None
    if attempts == 0 or not _valid_result(last):
        return "last must be a valid result after the first attempt"
    return None

def validate_state(request):
    return _state_diagnostic(request["snapshot"]["state"])

def reduce(request):
    state = request["snapshot"]["state"]
    diagnostic = _state_diagnostic(state)
    if diagnostic != None:
        fail(diagnostic)
    events = request["events"]
    if len(events) != 1:
        fail("simple-d6 reductions require exactly one event")
    event = events[0]
    if event["type"] != EVENT_TYPE or event["schema_version"] != 1 or not _valid_result(event["data"]):
        fail("invalid simple-d6 event")
    if state["attempts"] == 2147483647:
        fail("attempt counter is exhausted")
    return {"state": {
        "schema_version": 1,
        "attempts": state["attempts"] + 1,
        "last": event["data"],
    }}

def migrate(request):
    source = request["from"]
    if source["id"] != "simple-d6" or source["protocol_version"] != "1.0.0":
        fail("simple-d6 only accepts state from the same package and protocol")
    diagnostic = _state_diagnostic(request["state"])
    if diagnostic != None:
        fail(diagnostic)
    return {"state": request["state"]}
