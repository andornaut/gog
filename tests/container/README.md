# Container tooling

Native project checks run in stock containers through `make check`. Dependencies
are installed in container scratch. The checkout is read-only and full checks
export no artifacts. CI copies only its coverage report after checks finish.

WORKAROUND: Scoped npm overrides keep Markdown lint dependencies patched for
[re2js](https://github.com/advisories/GHSA-gg6h-894w-4m9c) and
[KaTeX](https://github.com/advisories/GHSA-238p-pmpm-9mq7).
Remove the overrides when the upstream CLI dependency graph resolves patched
versions without them.
