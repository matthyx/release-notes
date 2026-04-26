# Golden test data

`golden_kubescape-operator-1.30.5.md` is the actual release body for
`kubescape-operator-1.30.5` published at:

  https://github.com/kubescape/helm-charts/releases/tag/kubescape-operator-1.30.5

## Strip policy

GitHub appends an auto-generated `## New Contributors` section to release
bodies at creation time. That section is **not** reproducible from the public
API and is **not** part of this tool's output. The fixture script
(`scripts/fetch_golden.sh`) strips it from the live body before committing the
golden file.

## Line endings

The fixture is normalized to LF (`\n`) only. CRLF is stripped on fetch and
the renderer is required to emit LF only.

## Regenerating

```
GH_TOKEN=... ./scripts/fetch_golden.sh
```
