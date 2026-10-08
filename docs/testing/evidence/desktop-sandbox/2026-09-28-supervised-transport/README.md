# Explicit supervised process transport

Bridge source: `e8787a420df14cd7725f0744ef5b674ea433f3ba`. Status: explicit Bridge transport integration passed; Nexus default activation and release acceptance remain incomplete.

All Go commands used `GOWORK=off` on the current macOS arm64 host. Helper was built from the same Bridge source. No Windows native validation or cross-compilation was performed.

- `go test -race -count=1 ./internal/transport ./client ./supervision`: package regression passed; opt-in native cases run separately below.
- `NEXUS_SUPERVISION_TEST_HELPER=/tmp/nexus-supervised-transport-bootstrap go test -race -json -count=1 -timeout=120s ./internal/transport -run '^TestSupervisedTransportNative$'`: all eight mandatory test names passed without skip. Cases exercise JSON input/output and stderr, nonzero exit status, setsid descendant holding output, forced Close, retained typed cleanup failures, independent registration/retirement of every admission probe, and cancellation cleanup.
- `CGO_ENABLED=0 go test -count=1 ./internal/transport ./client -run 'Test(Supervised|ProcessSupervision)'`: option and admission contracts passed without cgo.
- `go vet ./internal/transport ./client ./supervision`: passed.

The probe CLI and Host are controlled fixtures, not real Claude authentication or Nexus database integration. Temporary fixture paths are not production protected paths. The separate Nexus Host adapter has its own real SQLite/native evidence. Connecting both through Nexus Manager startup, generation binding, protected App paths, reboot recovery and packaging remains outstanding. This batch does not establish full SDK/file/network isolation, all supported macOS versions or signed clean-host acceptance.

Nexus after pin: `go test -count=1 ./internal/runtime ./internal/runtime/clientopts` and the architecture gate passed. This validates the dependency update, not default supervision activation.
