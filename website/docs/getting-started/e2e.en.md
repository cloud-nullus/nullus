# End to end (E2E)

This guide takes you through the whole process in one pass, starting from an empty Linux or macOS
laptop: **bring up a kind cluster → install a stack → run a pipeline → back up → delete →
restore → continue the pipeline**.

<div class="nullus-cards" markdown>

[**Open the local E2E guide (Korean) →**<span>15 steps + 3 appendices. Check off each step in the left rail as you go.</span>](../guides/local-e2e.html)

</div>

## What the guide covers

| | Step | Content |
|---|------|------|
| **Prepare** | 1 | Prerequisites — required tools and minimum specs |
| | 2 | Bring up the platform |
| | 3 | Log in and check the cluster |
| **Stack** | 4 | Install a GitLab + Argo CD + SonarQube stack |
| | 5 | Open the tools — reach all 8 tools by domain |
| | 6 | API connection settings — GitLab address and backup |
| **CI/CD** | 7 | Choose a CI/CD template |
| | 8 | Create and run a pipeline |
| | 9 | Check the results and record fingerprints |
| **Backup and restore** | 10 | Back up |
| | 11 | Delete |
| | 12 | Restore |
| | 13 | Rewire after the restore |
| **Continue** | 14 | Continue the pipeline |
| | 15 | Clean up |
| **Appendices** | — | Troubleshooting · Using SonarQube · Known limitations |

## This guide reflects real runs

The commands and expected values in the guide are not estimates: **they were confirmed by
actually running everything from start to finish.** These are the values measured in the most
recent verification run.

| Item | Measured |
|------|--------|
| Platform startup | 68 s · smoke tests 16/16 |
| Stack install complete | 740 s · 39 pods · 0 unhealthy |
| Tool access | All 8 respond |
| Backup | 76 s · 1,069,537,942 bytes · 11 integrity checks passed |
| Namespace deletion | 86 s (no manual intervention) |
| Restore | 71 s · PVCs 6/6 |
| Pipeline rerun | All 4 jobs succeeded · quality gate passed |
| Cleanup | 15 s — 0 clusters, containers, or volumes left |

!!! tip "Before you start"
    Step 2 of the guide uses `sudo` once: to reach the tools by domain without a port, you need
    entries in both the host's `/etc/hosts` and CoreDNS.
    `./scripts/setup-local-domain.sh --apply-hosts` does that. If you leave out this option,
    logging in at step 3 does not work.

!!! warning "Some steps delete things"
    Step 11 **actually deletes** namespaces, and step 15 deletes the kind cluster and even the
    Docker volumes. On a machine with a cluster you already use, skip step 15 or read its
    commands first.

## A shorter path

If you just want to bring up the platform rather than go through the whole process,
[Run locally](run-local.md) is shorter. To check the prerequisites first, see
[Prerequisites](prerequisites.md).
