# M0 reports

`results.json` contains per-fixture backend and conversion evidence generated
with:

```sh
go run ./cmd/m0report -runs 5 -out reports/m0/results.json
```

The command performs one warmup, then records the mean wall time and allocated
bytes across the requested serial repetitions. These measurements are
comparative spike evidence, not stable performance promises.

`decision.md` will select keep, fork, hybridize, or replace after all M0 work
packages pass their gates.

No release-support claim is implied by a backend accepting a fixture.
