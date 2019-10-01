# runtime-utils

Utilities for configuring containers/image runtimes on OpenShift, such as cri-o,
buildah, and podman.

## FAQ

**Should my code be here or library-go?**

If your code is reused by multiple OpenShift components and does _not_ depend on
[containers/image](https://github.com/containers/image), consider submitting a
pull request to [library-go](https://github.com/openshift/library-go).
If your code does depend on [containers/image](https://github.com/containers/image)
and is reused by multiple OpenShift components, then submit a PR here.
