# kensan demo workspace

A small, **fictional** workspace used for screenshots and for trying kensan locally.
It contains no real personal data — every goal, task, note, and diary entry is made up.

Run kensan against it:

```bash
# Build the SPA once if apps/kensan/frontend/dist is missing:
cd apps/kensan/frontend && npm install && npm run build

cd ../backend
KENSAN_DATA_DIR=../demo-workspace \
KENSAN_STATIC_DIR=../frontend/dist \
KENSAN_ADDR=:8899 \
go run ./cmd/kensan
# open http://localhost:8899/
```

The dashboard screenshot in [`docs/showcase.md`](../../docs/showcase.md) is captured from this data.
