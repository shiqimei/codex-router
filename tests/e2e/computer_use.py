#!/usr/bin/env python3
"""Native Superior Computer Use regression. No alternate UI automation fallback.

Prepare a disposable TextEdit document; execute the emitted prompt in Superior
using its real CUA tool; verify the file effect, owner, and actual tool trace.
A blocked/unfinished native run exits nonzero and never counts as a pass.
"""
from __future__ import annotations
import argparse, datetime as dt, hashlib, json, pathlib, re, secrets, subprocess, sys, time

ROOT = pathlib.Path(__file__).resolve().parents[2]
APP = pathlib.Path.home() / 'Applications/CodexRouter.app'
PHASES = ('primary', 'provider')
FORBIDDEN = re.compile(r'\b(exec_command|write_file|writeFile(?:Sync)?|osascript|os\.system|subprocess|selenium|playwright|CGEvent|System Events)\b', re.I)
NATIVE = re.compile(r'mcp__cua_repl(?:[._]|$)', re.I)
ERRORS = re.compile(r'Computer Use server error[^\n]*|(?:accessibility|screen recording|screen capture) permission (?:denied|not granted|required)[^\n]*|(?:Could not|Failed to|Unable to) connect[^\n]*(?:Computer Use|native|CUA)[^\n]*|(?:unauthorized|untrusted) (?:caller|client)[^\n]*|Screenshot unavailable[^\n]*|(?:ReferenceError: )?cua is not defined[^\n]*', re.I)

def now(): return dt.datetime.now(dt.timezone.utc).isoformat()
def after(timestamp, started):
    try: return dt.datetime.fromisoformat(timestamp.replace('Z','+00:00')) >= dt.datetime.fromisoformat(started.replace('Z','+00:00'))
    except (ValueError, AttributeError): return False

def save(path, value): path.write_text(json.dumps(value, indent=2) + '\n')
def signed(path):
    if not path.exists(): return {'path': str(path), 'exists': False, 'valid': False}
    verify = subprocess.run(['codesign', '--verify', '--deep', '--strict', str(path)], capture_output=True, text=True)
    details = subprocess.run(['codesign', '-dvv', str(path)], capture_output=True, text=True).stderr
    fields = {}
    for line in details.splitlines():
        if line.startswith(('Identifier=', 'TeamIdentifier=', 'Signature=')):
            key, value = line.split('=', 1); fields[key] = value
    return {'path': str(path), 'exists': True, 'valid': verify.returncode == 0, **fields}

def preflight():
    helper = APP / 'Contents/Resources/cua_node/lib/node_modules/@oai/sky/Codex Computer Use.app'
    return {'app': signed(APP), 'bundledHelper': signed(helper)}

def prepare(run_dir, phase):
    run_dir.mkdir(parents=True, exist_ok=True)
    path = run_dir / 'manifest.json'
    manifest = json.loads(path.read_text()) if path.exists() else {'runId': secrets.token_hex(6), 'phases': {}}
    target = run_dir / f'Superior Computer Use {manifest["runId"]}.txt'
    if not target.exists(): target.write_text('Superior Computer Use regression target.\nOnly edit this disposable document.\n')
    expected = f'SUPERIOR CUA {manifest["runId"]} {phase.upper()} {secrets.token_hex(4)}'
    prompt = (
        f'Computer Use regression {manifest["runId"]}, phase {phase}. Use the actual native Computer Use tool in this Superior task. '
        f'The disposable TextEdit document at {target} is the only permitted edit target. '
        'Use cua_repl with cua.getApp("TextEdit") (or its documented native equivalent), inspect the accessibility state and obtain a real screenshot. '
        'Select this document by its title. Replace its contents with exactly the following line and save it with Command-S: '
        f'{expected}. The final period is punctuation, not part of the line. '
        'Then get fresh accessibility state and another screenshot to verify the saved document. '
        'Do not use a shell, filesystem write, AppleScript, browser extension, or any alternate automation to perform this test. '
        'If native Computer Use or its permissions are unavailable, report the exact error and stop; do not claim success or use a fallback.'
    )
    # Re-running a phase cannot accidentally reuse a previous pass.
    invalidated = PHASES if phase == 'primary' else ('provider',)
    for old_phase in invalidated:
        result_path = run_dir / f'{old_phase}-result.json'
        if result_path.exists():
            (run_dir / 'attempts').mkdir(exist_ok=True)
            result_path.rename(run_dir / 'attempts' / f'{old_phase}-{time.time_ns()}.json')
    if phase == 'primary': manifest['phases'] = {}
    manifest.update({'target': str(target), 'preflight': preflight()})
    manifest['phases'][phase] = {'startedAt': now(), 'expected': expected, 'prompt': prompt}
    save(path, manifest)
    (run_dir / f'{phase}-prompt.txt').write_text(prompt + '\n')
    print(json.dumps({'runDir': str(run_dir), 'target': str(target), 'phase': phase, 'prompt': prompt}, indent=2))

def trace_evidence(records, started):
    calls, outputs = [], []
    native_ids = set()
    image_count = 0
    for record in records:
        if not after(record.get('timestamp', ''), started): continue
        payload = record.get('payload', {})
        if record.get('type') != 'response_item' or not isinstance(payload, dict): continue
        if payload.get('type') in ('function_call', 'custom_tool_call'):
            args = payload.get('arguments', payload.get('input', ''))
            name = '.'.join(filter(None,[payload.get('namespace'),payload.get('name')]))
            arguments = args if isinstance(args,str) else json.dumps(args)
            calls.append({'name':name,'arguments':arguments})
            if NATIVE.search(name) or NATIVE.search(arguments):native_ids.add(payload.get('call_id'))
        if payload.get('type') in ('function_call_output', 'custom_tool_call_output'):
            output = payload.get('output', '')
            if payload.get('call_id') in native_ids and isinstance(output,list):
                image_count += sum(1 for item in output if isinstance(item,dict) and item.get('type') in ('input_image','image') and (item.get('image_url') or item.get('data')))
            outputs.append(output if isinstance(output, str) else json.dumps(output))
    code = '\n'.join(c['name'] + '\n' + c['arguments'] for c in calls)
    text = '\n'.join(outputs)
    forbidden = sorted(set(FORBIDDEN.findall(code)))
    return {'toolCalls': len(calls), 'nativeToolCalled': bool(native_ids), 'screenshotsReturned': image_count,
            'nativeAppSelected': bool(re.search(r'getApp\s*\(', code)),
            'screenshotRequested': bool(re.search(r'get(?:AXStateAnd)?Screenshot\s*\(', code)),
            'nativeEditRequested': bool(re.search(r'\.(?:typeText|paste|setValue)\s*\(', code)),
            'saveRequested': bool(re.search(r'pressKey\s*\([^)]*(?:super\+s|meta\+s|cmd\+s|command\+s)', code, re.I)),
            'forbiddenCalls': forbidden, 'nativeErrors': [m.group(0)[:400] for m in ERRORS.finditer(text)],
            'traceSHA256': hashlib.sha256((code + '\n' + text).encode()).hexdigest()}

def evaluate(evidence, actual, expected, target_visible=True):
    checks = {key: bool(evidence[key]) for key in ('nativeToolCalled', 'nativeAppSelected', 'screenshotRequested', 'nativeEditRequested', 'saveRequested')}
    checks['beforeAndAfterImagesReturned'] = evidence['screenshotsReturned'] >= 2
    checks['noAlternateAutomation'] = not evidence['forbiddenCalls']
    checks['savedExpectedText'] = actual.strip() == expected
    checks['documentObserved'] = target_visible
    checks['nativeCallSucceeded'] = not evidence['nativeErrors']
    status = 'pass' if all(checks.values()) else 'blocked' if evidence['nativeErrors'] and not evidence['forbiddenCalls'] else 'fail'
    return {'status': status, 'checks': checks, 'evidence': evidence}

def verify(run_dir, phase, thread_id):
    manifest = json.loads((run_dir / 'manifest.json').read_text())
    state = json.loads((pathlib.Path.home() / '.superior/state.json').read_text())
    owner = state.get('threadOwner', {}).get(thread_id)
    account = next((a for a in state['accounts'] if a['id'] == owner), None)
    if not account: raise ValueError('thread owner is unknown; select a real Superior regression thread')
    paths = list(pathlib.Path(account['codexHome']).glob(f'sessions/**/*{thread_id}*.jsonl'))
    if len(paths) != 1: raise ValueError(f'expected one authoritative rollout, found {len(paths)}')
    records = [json.loads(line) for line in paths[0].read_text().splitlines() if line.strip()]
    config = manifest['phases'][phase]
    evidence = trace_evidence(records, config['startedAt'])
    # Text must be observed in a tool result, not merely asserted in assistant prose.
    outputs = '\n'.join(json.dumps(r.get('payload', {}).get('output', '')) for r in records
                        if r.get('type') == 'response_item' and after(r.get('timestamp', ''), config['startedAt'])
                        and r.get('payload', {}).get('type') in ('function_call_output', 'custom_tool_call_output'))
    result = evaluate(evidence, pathlib.Path(manifest['target']).read_text(), config['expected'],
                      pathlib.Path(manifest['target']).stem in outputs or config['expected'] in outputs)
    result.update({'phase': phase, 'threadId': thread_id, 'ownerId': owner, 'ownerKind': account.get('kind', 'subscription'), 'checkedAt': now()})
    right_owner = (phase == 'primary' and owner == 'primary') or (phase == 'provider' and account.get('kind') == 'provider')
    result['checks']['expectedConnectionKind'] = right_owner
    if not right_owner: result['status'] = 'fail'
    # The second phase must continue the very same thread across the handoff.
    if phase == 'provider':
        first_path = run_dir / 'primary-result.json'
        first = json.loads(first_path.read_text()) if first_path.exists() else {}
        result['checks']['sameThreadAfterSwitch'] = first.get('threadId') == thread_id and first.get('status') == 'pass'
        if not result['checks']['sameThreadAfterSwitch']: result['status'] = 'fail'
    save(run_dir / f'{phase}-result.json', result)
    print(json.dumps(result, indent=2))
    return 0 if result['status'] == 'pass' else 2

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('action', choices=['prepare', 'verify'])
    p.add_argument('--run-dir', type=pathlib.Path, required=True)
    p.add_argument('--phase', choices=PHASES, required=True)
    p.add_argument('--thread-id')
    a = p.parse_args(); run_dir = a.run_dir.expanduser().resolve()
    if a.action == 'prepare': prepare(run_dir, a.phase); return 0
    if not a.thread_id: p.error('--thread-id is required for verification')
    return verify(run_dir, a.phase, a.thread_id)
if __name__ == '__main__': sys.exit(main())
