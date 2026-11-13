"""Acquire the suite's OS lock and transfer its descriptor to the test runner."""
import fcntl
import json
import os
import sys
import uuid

lock_path, execution_id, *command = sys.argv[1:]
if execution_id == "--verify":
    probe = os.open(lock_path, os.O_RDWR | os.O_NOFOLLOW)
    try:
        fcntl.flock(probe, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        # Re-locking the inherited open description succeeds only for its owner.
        fcntl.flock(3, fcntl.LOCK_EX | fcntl.LOCK_NB)
        sys.exit(0)
    sys.exit("Suite lock is not held")

fd = os.open(lock_path, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
try:
    fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
except BlockingIOError:
    sys.exit("Test suite already holds the isolated environment lock")

owner = {"pid": os.getpid(), "token": uuid.uuid4().hex, "executionId": execution_id}
os.ftruncate(fd, 0)
os.write(fd, json.dumps(owner).encode())
os.set_inheritable(fd, True)
os.environ["XIAOPI_SUITE_LOCK"] = json.dumps({**owner, "fd": fd})
os.execv(command[0], command)
