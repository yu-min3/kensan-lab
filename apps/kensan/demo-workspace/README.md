# kensan demo workspace

A small, **fictional** workspace used for screenshots and for trying kensan locally.
It contains no real personal data — every goal, task, note, and diary entry is made up.

Run kensan against it:

```bash
cd apps/kensan/backend
KENSAN_DATA_DIR=../demo-workspace \
KENSAN_STATIC_DIR=../frontend/dist \
KENSAN_ADDR=:8899 \
go run ./cmd/kensan
# open http://localhost:8899/
```

(Build the SPA once with `make -C apps/kensan build` if `frontend/dist` is missing.)

The dashboard screenshot in [`docs/showcase.md`](../../docs/showcase.md) is captured from this data.
