# Releasing `lifxland`

Semantic-version tags trigger `.github/workflows/release.yml`. The workflow:

1. validates the tag and runs the complete Go test suite;
2. cross-compiles `lifxland` with `CGO_ENABLED=0`;
3. creates macOS, Linux, Raspberry Pi, and Windows archives;
4. generates `SHA256SUMS`;
5. creates a draft GitHub Release with generated notes.

To prepare a release, ensure `main` is pushed and its test workflow is green,
then create and push the tag:

```sh
git tag v0.8.0
git push origin v0.8.0
```

After the release workflow succeeds, open the draft GitHub Release. Review or
replace the generated notes, confirm that all six archives and `SHA256SUMS` are
attached, and publish it.

The release workflow intentionally creates a draft: tagging remains a simple
Git operation, while publishing the user-visible release and its notes stays an
explicit maintainer decision. Re-running a successful tag workflow replaces
the attached assets without overwriting manually edited release notes.
