# Release Notes v0.5.14

This release addresses security vulnerability CVE-2025-58058. It implements a
number of mitigation for a resource leak problem. It needs to only to be updated
if lzma.NewWriter is used.
