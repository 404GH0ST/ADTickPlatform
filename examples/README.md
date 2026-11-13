# Example challenges

The HTTP, LFI, and RCE packages follow the same access rule: capture a current
flag from another team and submit it to the platform. An accepted capture
opens SSH on your own instance of that challenge. Proof files and a separate
unlock submission are no longer part of the examples.

- [HTTP](sample-http-challenge/README.md): explicit `/leak` route for platform smoke testing.
- [LFI](sample-lfi-challenge/README.md): path traversal in the template renderer.
- [RCE](sample-rce-challenge/README.md): command injection in diagnostics, executed as `nobody`.

The LFI and RCE packages include exploit scripts. Point them at an enemy service;
the platform rejects your own flags. The checker must first store a live flag.

## Verify all packages in isolation

Requires Docker and Python 3.10 or newer:

```bash
python3 scripts/smoke-example-packages.py
```

Run this from the repository root. It builds all six images with temporary tags,
starts disposable service containers on loopback ports, and checks:

- checker `validate`, `put`, `get`, and `check` phases;
- two flag rotations, current-flag disclosure, and rejection of the old flag;
- token-authenticated flag replacement being detected with reserved checker exit code 42;
- LFI/RCE exploit authentication and submission against a strict API fixture,
  including custom flag prefixes and duplicate rejection;
- `tcpdump` installation and denial of live capture/raw sockets when guarded capture is disabled;
- locked root accounts, a running SSH daemon, controller-compatible credential
  application, and stored-flag persistence after restart;
- absence of the legacy `/unlock` endpoint.

The runner cleans up its containers and image tags. It does not reset or modify a
platform stack. Its API fixture checks the exploit wire format; authoritative flag
validation and SSH grants are covered by the game-core and API gateway tests.

The HTTP package also supports `make smoke-sample-challenge-docker` for a full
platform smoke on a dedicated development stack. That existing smoke resets the
match database; use the isolated runner above to check packages alongside a live event.

For direct live capture under production kernel guards, run
`./scripts/smoke-live-capture.sh` and
`python3 deploy/security/direct-capture/test.py`. See the
[guarded capture deployment guide](../deploy/security/direct-capture/README.md).
