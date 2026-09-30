"""Observe native macOS session boundaries using only this probe's children."""
import errno
import contextlib
import json
import os
import pathlib
import select
import signal
import subprocess
import sys
import time


def child():
    time.sleep(30)


def root():
    descendant = subprocess.Popen(
        [sys.executable, __file__, "child"], start_new_session=True,
        stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    print(json.dumps({"rootPid": os.getpid(), "rootSession": os.getsid(0),
                      "childPid": descendant.pid, "childSession": os.getsid(descendant.pid)}), flush=True)
    sys.stdin.readline()


def alive(pid):
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False


def probe():
    facts = {"platform": sys.platform, "scope": "native Unix session and kqueue behavior; not full-tree acceptance"}
    process = subprocess.Popen([sys.executable, __file__, "root"], start_new_session=True,
                               stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
    identity = None
    try:
        identity = json.loads(process.stdout.readline())
        assert identity["rootPid"] == process.pid
        assert identity["rootSession"] == process.pid
        assert identity["childSession"] == identity["childPid"]
        process.stdin.write("exit\n")
        process.stdin.flush()
        facts["rootExitCode"] = process.wait(timeout=5)
        facts.update(identity)
        try:
            os.killpg(process.pid, signal.SIGKILL)
            facts["originalGroupKill"] = "signal_sent"
        except ProcessLookupError:
            facts["originalGroupKill"] = "group_not_found"
        facts["detachedChildSurvived"] = alive(identity["childPid"])
        assert facts["detachedChildSurvived"]
        with contextlib.closing(select.kqueue()) as queue:
            # NOTE_TRACK is 0x1 in the locally installed sys/event.h.
            request = select.kevent(os.getpid(), filter=select.KQ_FILTER_PROC,
                                   flags=select.KQ_EV_ADD | 0x0040,  # EV_RECEIPT in sys/event.h
                                   fflags=select.KQ_NOTE_EXIT | select.KQ_NOTE_FORK | 0x1)
            receipt = queue.control([request], 1, 0)[0]
            facts["kqueueTrack"] = {"errorEvent": bool(receipt.flags & select.KQ_EV_ERROR),
                                    "errno": receipt.data, "errorName": errno.errorcode.get(receipt.data)}
    finally:
        if process.poll() is None:
            process.kill()
            process.wait(timeout=5)
        if identity is not None and alive(identity["childPid"]):
            assert os.getsid(identity["childPid"]) == identity["childSession"]
            os.killpg(identity["childPid"], signal.SIGKILL)
            deadline = time.monotonic() + 5
            while alive(identity["childPid"]) and time.monotonic() < deadline:
                time.sleep(0.01)
            facts["probeChildGoneAfterExplicitCleanup"] = not alive(identity["childPid"])
        if process.stdin is not None:
            process.stdin.close()
        if process.stdout is not None:
            process.stdout.close()
    assert facts["probeChildGoneAfterExplicitCleanup"], "probe child did not terminate"
    print(json.dumps(facts, indent=2))


if __name__ == "__main__":
    {"root": root, "child": child}.get(sys.argv[1] if len(sys.argv) > 1 else "", probe)()
