# askl-golang-indexer

Create askl protobuf indexes for Go packages.

## Docker

Build the image:

```sh
docker build -t askl-golang-indexer .
```

Run the indexer with a source volume and a writable output volume:

```sh
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -v "$SRC_DIR:/workspace:ro" \
  -v "$OUT_DIR:/out" \
  askl-golang-indexer \
  --include-git-files \
  --project kubernetes \
  --path /workspace/cmd/kubelet \
  --index /out/index.pb
```

Notes:
- `--include-git-files` expects the source volume to include `.git`.
- The output volume must be writable by the container user.
- `--path` is repeatable and supports globs (for example `--path /workspace/cmd/*`).
- Positional args can be used in addition to `--path`.
- `--root` overrides the inferred project root stored in the index.

## Script

`generate_index_docker.sh` runs the container with the same volume layout. If you do not pass `--path`, it defaults to `/workspace`. The output index file is the second positional argument.

```sh
./generate_index_docker.sh --image askl-golang-indexer:latest \
  "$SRC_DIR" "$OUT_DIR/index.pb" \
  --include-git-files \
  --project kubernetes \
  --path /workspace/cmd/kubelet
```
