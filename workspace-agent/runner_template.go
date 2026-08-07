package main

// PythonRunnerTemplate is the Python harness script executed to run uploaded Playwright scripts over CDP.
const PythonRunnerTemplate = `
import asyncio
import json
import os
import sys
import time
import traceback
from playwright.async_api import async_playwright

class ScriptHelpers:
    def __init__(self):
        pass

    def log(self, message, level="INFO"):
        print(f"[SCRIPT_LOG] {str(level).upper()}|{message}", flush=True)

    def info(self, message):
        self.log(message, "INFO")

    def warn(self, message):
        self.log(message, "WARN")

    def error(self, message):
        self.log(message, "ERROR")

    async def sleep(self, seconds):
        await asyncio.sleep(float(seconds))

def script_print(*args, sep=" ", end="\n", **kwargs):
    msg = sep.join(str(a) for a in args)
    if end and end not in ("", "\n"):
        msg += end.rstrip("\n")
    print(f"[SCRIPT_LOG] STDOUT|{msg}", flush=True)

async def main():
    if len(sys.argv) < 3:
        print("[SCRIPT_LOG] ERROR|Invalid arguments to runner", flush=True)
        sys.exit(1)

    user_script_path = sys.argv[1]
    inputs_json = sys.argv[2]
    cdp_url = os.environ.get("CHROME_CDP_URL", "http://127.0.0.1:9222")

    try:
        inputs = json.loads(inputs_json) if inputs_json else {}
    except Exception as exc:
        print(f"[SCRIPT_LOG] ERROR|Failed to parse inputs JSON: {exc}", flush=True)
        inputs = {}

    try:
        with open(user_script_path, "r", encoding="utf-8") as f:
            code = f.read()
    except Exception as exc:
        print(f"[SCRIPT_LOG] ERROR|Failed to read script file: {exc}", flush=True)
        sys.exit(1)

    helpers = ScriptHelpers()
    globals_ns = {
        "__name__": "__shadowops_user_script__",
        "__builtins__": __builtins__,
        "asyncio": asyncio,
        "json": json,
        "time": time,
        "helpers": helpers,
        "print": script_print,
    }

    try:
        compiled = compile(code, user_script_path, "exec")
        exec(compiled, globals_ns, globals_ns)
    except Exception as exc:
        tb = traceback.format_exc()
        print(f"[SCRIPT_LOG] ERROR|Syntax / Compilation error: {exc}", flush=True)
        print(f"[SCRIPT_RESULT] {json.dumps({'status': 'failed', 'error': str(exc), 'traceback': tb})}", flush=True)
        sys.exit(1)

    run_fn = globals_ns.get("run")
    if not callable(run_fn):
        err_msg = "Uploaded script must define 'async def run(page, context, browser, inputs, helpers)'"
        print(f"[SCRIPT_LOG] ERROR|{err_msg}", flush=True)
        print(f"[SCRIPT_RESULT] {json.dumps({'status': 'failed', 'error': err_msg})}", flush=True)
        sys.exit(1)

    async with async_playwright() as p:
        try:
            browser = await p.chromium.connect_over_cdp(cdp_url)
        except Exception as exc:
            err_msg = f"Failed to connect to browser CDP at {cdp_url}: {exc}"
            print(f"[SCRIPT_LOG] ERROR|{err_msg}", flush=True)
            print(f"[SCRIPT_RESULT] {json.dumps({'status': 'failed', 'error': err_msg})}", flush=True)
            sys.exit(1)

        contexts = browser.contexts
        if not contexts:
            context = await browser.new_context()
        else:
            context = contexts[0]

        pages = context.pages
        if not pages:
            page = await context.new_page()
        else:
            page = pages[-1]

        try:
            res = await run_fn(page, context, browser, inputs, helpers)
            if res is None:
                res = {"status": "succeeded", "ok": True}
            elif isinstance(res, dict) and "status" not in res:
                res["status"] = "succeeded"
                res["ok"] = True
            print(f"[SCRIPT_RESULT] {json.dumps(res)}", flush=True)
        except asyncio.CancelledError:
            print("[SCRIPT_LOG] WARN|Script cancelled by user", flush=True)
            print(f"[SCRIPT_RESULT] {json.dumps({'status': 'cancelled', 'error': 'Script cancelled by user'})}", flush=True)
            sys.exit(0)
        except Exception as exc:
            tb = traceback.format_exc()
            print(f"[SCRIPT_LOG] ERROR|Execution exception: {exc}", flush=True)
            print(f"[SCRIPT_RESULT] {json.dumps({'status': 'failed', 'error': str(exc), 'traceback': tb})}", flush=True)
            sys.exit(1)

if __name__ == "__main__":
    asyncio.run(main())
`
