# Audio lab instructions

This is a learning module on an experimental branch. The learner owns the
functions in `exercises/` and their prose in `notes/`.

- Leave exercise bodies unfinished unless the user explicitly asks you to
  implement one. Do not solve exercises to make the test suite green.
- Deliberately failing learner tests are expected. Do not skip, weaken, or
  remove them to hide unfinished work.
- Use `go test ./internal/... ./cmd/...` for infrastructure checks.
  `go vet ./...` also works with unfinished exercise bodies.
- The `./audiolab` launcher rebuilds before running, so experiments incorporate
  current learner code. Keep generated artifacts under ignored `work/`.
- Reports must use learner-generated samples and learner measurements.
  Pending functions must stay visibly pending, with no reference fallback.
- Keep this module independent of the parent MCU and free of third-party
  dependencies unless the user requests a scope change.
