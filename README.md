# SLSA GitHub Actions Demo

A proof-of-concept SLSA provenance generator for GitHub Actions.

## Background

[SLSA](https://github.com/slsa-framework/slsa) is a framework intended to codify
and promote secure software supply-chain practices. SLSA helps trace software
artifacts (e.g. binaries) back to the build and source control systems that
produced them using in-toto's
[Attestation](https://github.com/in-toto/attestation/blob/main/spec/README.md)
metadata format.

## Description

This proof-of-concept GitHub Action demonstrates an initial SLSA integration
conformant with SLSA Level 1. This provenance can be uploaded to the native
artifact store or to any other artifact repository.

While there are no integrity guarantees on the produced provenance at L1,
publishing artifact provenance in a common format opens up opportunities for
automated analysis and auditing. Additionally, moving build definitions into
source control and onto well-supported, secure build systems represents a marked
improvement from the ecosystem's current state.

### Security and Support

This is demo repo and is not intended to be used in production contexts. As
such, we cannot make any commitments of future support.

## Example

To see an example of the action... in action, see the [example action](.github/workflows/example-publish.yml)
and [example provenance](examples/build.provenance) in this repository.

## Usage

The GitHub action has the following user configuration

| Input | Default | Description |
| ----- | ------- | ----------- |
|`artifact_path` | *`none`* | Path to build artifact or directory of build artifacts |
|`output_path` | `build.provenance` | Path to write build provenance file |

To try out this provenance generator, add the following snippet to your GitHub
Actions workflow:

```
      - name: Generate provenance
        uses: slsa-framework/github-actions-demo@v0.1
        with:
          artifact_path: <path-to-artifact>
```
In this example we use the default output path `build.provenance`, you can
upload the build provenance to the workflow run result with the
`actions/upload-artifact` github action
```
      - name: Upload provenance
        uses: actions/upload-artifact@v2
        with:
          name: build-provenance
          path: build.provenance
```
