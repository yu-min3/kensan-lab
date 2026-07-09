# Incident: the m4neo silent-freeze series — and the watchdog that was never armed

## Summary

| Item | Detail |
|------|------|
| Occurred | 3 freezes: 2026-05-15 (#1), 2026-05-29 (#2), 2026-06-24 (#3) |
| Detected | #2 after **8 days**; #3 after **34 hours** (both by a human noticing, not an alert) |
| Impact | The high-performance node freezes silently. Everything pinned to it stops: Prometheus / Loki / Tempo / Keycloak, and in #2 a Terminating-stuck ArgoCD application-controller **halted GitOps sync for the whole cluster for 8 days** |
| Root cause | Freeze trigger: unconfirmed (leading hypothesis: NVMe idle power-state hang). Non-recovery: the hardware-watchdog module was **blacklisted by an Ubuntu auto-generated config at boot**, so the arm-on-freeze reset never existed |

This is a series post-mortem: the same node froze three times, and the most valuable lesson came not from the freezes themselves but from **why the defenses installed after #2 did nothing during #3**.

## The three freezes

| # | Date | Downtime | Signature | Note |
|---|------|----------|-----------|------|
| 1 | 2026-05-15 | hours | NIC (r8169) deadlock suspected | first occurrence; recovery measures designed but **not implemented** |
| 2 | 2026-05-29 | **8 days** | ping OK (3ms), SSH times out at banner exchange — kernel network stack alive, userland entirely dead. Journal cuts off with zero warning | watchdog + panic sysctls deployed on recovery day |
| 3 | 2026-06-24 | 34 hours | identical to #2: log stops mid-line during routine apt automation, no hung-task / OOM / panic traces | **the defenses from #2 did not fire** — see below |

## Timeline (#3, the instructive one)

| When (JST) | Event |
|------|---------|
| 06-06 15:32 | Watchdog + panic-sysctl config deployed after #2, verified **live** (manual modprobe). No reboot afterwards |
| 06-24 11:13 | Journal cuts off abruptly during apt-daily / PackageKit automation — silent freeze |
| 06-24 11:15 | kubelet heartbeat stops; node goes NotReady |
| 06-24 → 06-25 | 34 hours unresponsive. No NotReady alert existed, so nobody noticed |
| 06-25 21:52 | Manual power cycle; node returns |
| 06-25 | Full RCA of "why didn't the watchdog fire" + hypothesis falsification with Prometheus/SMART data |

## Why both defense layers failed

Two layers were believed to be protecting the node. Both were bypassed.

### Layer 1: kernel panic sysctls — wrong failure class

`softlockup_panic` / `hardlockup_panic` / `unknown_nmi_panic` were all live and correct. But this freeze is a **D-state hang (all processes stuck waiting on I/O)** — no CPU spins in-kernel, so none of the lockup detectors trigger, and no panic ever fires. This failure class can only be caught by a hardware watchdog.

### Layer 2: hardware watchdog — configured, never loaded

| Step | State |
|---------|------|
| Config files (modules-load / `RuntimeWatchdogSec=30s` / panic sysctls) | ✅ all present and correct |
| `sp5100_tco` module | ❌ **skipped at boot — listed in Ubuntu's auto-generated blacklist** (`/lib/modprobe.d/blacklist_linux_*.conf`) |
| `/dev/watchdog` device | ❌ `No such file or directory` |
| systemd holding the watchdog | ❌ `No systemd watchdog enabled` |
| The hardware itself | ✅ manual `modprobe` loads instantly; `wdctl` shows `SP5100 TCO timer` |

The hardware was perfectly capable. The 30-second auto-reset was **petting a device that didn't exist**.

### Why the 2026-06-06 "verification" missed it

The node was never rebooted between 06-06 and the freeze. The verification after #2 was a **live check on a manually-loaded module** — it never validated that the configuration survives a boot, which is exactly where the blacklist strikes.

> 🔑 **Live verification ≠ boot verification.** Any measure that must persist across reboots is only verified once you reboot and inspect the booted state.

## Hypothesis falsification (#3 aftermath, with data)

Three plausible causes were tested against Prometheus and SMART data instead of gut feeling:

| Hypothesis | Verdict | Evidence |
|------|------|------|
| Overheating | ❌ falsified | CPU at 56°C at freeze time, flat for 35h prior. The 88°C reading was a post-recovery load spike |
| I/O overload | ❌ falsified | disk busy 1–3%, writes 0.4MB/s, I/O queue 0 right before the freeze — effectively idle |
| SSD wear / failure | ❌ falsified | SMART PASSED, media_errors 0, wear 1%, spare 100% (telemetry itself buggy on this budget OEM drive) |

**The surviving pattern**: all three freezes happened in the morning (06:00 / 09:02 / 11:13) — first activity after a night of idling. #3 came 10 seconds after apt-daily completed. Not "apt is heavy" but "**hardware that sank into idle power states can't wake for the morning's first I/O**".

**Leading (unconfirmed) suspect**: NVMe APST (autonomous power-state transitions) on a budget OEM drive that the kernel already flags with `platform quirk: setting simple suspend`.

## Response

| Measure | Status |
|------|------|
| Watchdog loaded via initramfs (`/etc/initramfs-tools/modules`), defeating the blacklist ordering; timeout raised to **120s** (30s risks false resets) | armed live; boot persistence to be verified on next reboot |
| NVMe APST disabled (`nvme_core.default_ps_max_latency_us=0` in GRUB) | configured; takes effect on next reboot |
| **netconsole** — kernel log streamed over UDP to another node, so the freeze moment finally gets captured (the journal physically cannot record it: journald is itself stuck on I/O) | implemented, end-to-end tested |
| NotReady alert (the only fix for "34 hours before anyone noticed") | tracked as follow-up; see [cluster health monitoring](../architecture/cluster-health-monitoring.md) |

## Lessons

1. **Live verification ≠ boot verification.** Persistence must be verified in the booted state — the entire #3 outage traces to skipping one reboot test
2. **Implement countermeasures all the way.** The #1 report designed the watchdog fix; it sat unimplemented until #2 forced it. A designed-but-not-deployed defense is a false sense of security
3. **A watchdog recovers you; it does not notify you.** Auto-reset and alerting are separate problems — the 8-day and 34-hour silences are an alerting failure, and they directly motivated layer ③ (off-cluster dead-man's switch) of the [health-monitoring design](../architecture/cluster-health-monitoring.md)
4. **Log-less failures need out-of-band logging.** When journald itself is frozen, only netconsole (or similar) can capture the trigger
5. **Falsify hypotheses with data.** "It must be overheating" survived until Prometheus said otherwise; every reflexive explanation for this freeze died on contact with the metrics

## Related

- Single-node concentration risk (everything above rode on one node): scheduling rules in [`.claude/rules/kubernetes-cluster.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/kubernetes-cluster.md)
- The monitoring layers this incident shaped: [cluster-health-monitoring.md](../architecture/cluster-health-monitoring.md) / [observability architecture](../architecture/observability.md)
