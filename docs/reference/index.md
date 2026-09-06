# Reference

Material that supports the rest of this site rather than leading it: how the
bare-metal cluster was actually built, what to do when it breaks, the rules the
documentation itself follows, and the design system the platform's own
interfaces are written against.

## What is here

| Section | What it holds | How far to trust it |
|---|---|---|
| [Bare-metal bootstrap](../bootstrapping/index.md) | Prerequisites, configuration, and the staged build of the live cluster. The Terraform that establishes Vault's root of trust is documented beside the code in [`bootstrap/`](https://github.com/yu-min3/kensan-lab/blob/main/bootstrap/), not here | **A record, not an installer.** The steps reflect the running system; the full sequence has not been exercised from a blank machine |
| [Operations](../runbooks/index.md) | Runbooks, recovery procedures, and incident write-ups | Procedures actually used on this cluster, written after the fact they describe |
| [Documentation layout](doc-layout.md) | Which layer each fact belongs to, and why | The rule this repository is written to; enforced in review |
| [Design system](../design/index.md) | Whetstone tokens, components, and adoption status | The contract every UI in the repository is built against |
| [Articles](../articles.md) | Zenn / dev.to deep dives | Point-in-time. They capture the journey and go stale; this site is the current state |
| [Archive](../roadmap/storage-longhorn.md) | Finished plans kept for the record | Historical. Superseded by the architecture pages |

## Two ways in, two levels of assurance

The distinction that shapes this site is not how much is written down — it is
what has been checked.

| Path | Verification | What it can promise |
|---|---|---|
| [Try it in one command](../getting-started/try-it-with-kind.md) | Pull requests that touch the platform stand this environment up on CI and sign in to it | That it works. If it stops working, a build turns red |
| [Bare-metal bootstrap](../bootstrapping/index.md) | None. Written from the cluster that is running | That this is how it was built. Environment differences are yours to close |

Start with the first. Read the second as evidence, not as instructions.
