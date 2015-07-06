This directory contains vendored dependencies for this library. No types from the vendored
dependencies must leave the actual package.

This vendoring is different to Godeps as it uses the full import paths and is also effective
if the expfmt package is imported from somewhere else. The vendored library are part of the
library code itself.