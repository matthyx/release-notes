Kubescape is an E2E Kubernetes cluster security platform

## What's Changed
* Update values.yaml by @Naor-Armo in https://github.com/kubescape/helm-charts/pull/799
* https://github.com/kubescape/kubevuln/compare/v0.3.105...v0.3.109
  * Bump github.com/go-git/go-git/v5 from 5.16.2 to 5.16.5 by @dependabot[bot] in https://github.com/kubescape/kubevuln/pull/328
  * strip unnecessary fields from SBOM to reduce size by @matthyx in https://github.com/kubescape/kubevuln/pull/327
  * fix test expectations by @matthyx in https://github.com/kubescape/kubevuln/pull/329
  * use fixed StripSBOM from storage v0.0.247 by @matthyx in https://github.com/kubescape/kubevuln/pull/330
* https://github.com/kubescape/storage/compare/v0.0.239...v0.0.247
  * add permissions by @bvolovat in https://github.com/kubescape/storage/pull/281
  * Fix OpenAPI model names to use dot-notation instead of slash-notation by @matthyx in https://github.com/kubescape/storage/pull/283
  * disable slug channel by @shanyl9 in https://github.com/kubescape/storage/pull/285
  * Fix key not found by @jnathangreeg in https://github.com/kubescape/storage/pull/287
  * Implement StripSBOM function to reduce SBOM size by clearing unnecessary fields by @matthyx in https://github.com/kubescape/storage/pull/288
  * Preserve relationships in StripSBOM function (needed by kubevuln's filterSBOM) by @matthyx in https://github.com/kubescape/storage/pull/289
* https://github.com/kubescape/node-agent/compare/v0.3.42...v0.3.47
  * Strip unused SBOM fields to reduce object size by ~52% by @slashben in https://github.com/kubescape/node-agent/pull/720
  * use fixed StripSBOM from storage v0.0.247 by @matthyx in https://github.com/kubescape/node-agent/pull/726
  * bump github.com/goradd/maps v1.3.0 by @matthyx in https://github.com/kubescape/node-agent/pull/727
* https://github.com/kubescape/synchronizer/compare/v0.0.131...v0.0.132
  * fix: bump k8s-interface to v0.0.203 for OCI bare OCID detection by @rotemamsa in https://github.com/kubescape/synchronizer/pull/140

**Full Changelog**: https://github.com/kubescape/helm-charts/compare/kubescape-operator-1.30.4...kubescape-operator-1.30.5
