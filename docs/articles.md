# Articles

Deep-dive write-ups on Zenn. These are **point-in-time records** — they describe how something was built or solved at a specific moment, and may go out of sync with the current platform state.

For the current state of the platform, the docs in this site are the source of truth. The articles below capture the *story* — design tradeoffs, what was tried first, what broke, and how it was fixed.

<div class="admonition note" markdown>
<p class="admonition-title">These articles are in Japanese</p>

The author writes long-form on [Zenn](https://zenn.dev/yuu7751), a Japanese engineering platform. Code blocks, diagrams, and command output are language-independent, and machine translation handles the prose reasonably well.

</div>

## The platform, introduced

- [CNCF全冠しても、まだまだ初心者だった話 〜全載せホームラボ公開〜](https://zenn.dev/yuu7751/articles/a160a8b857d640) (2026-03) — the kensan-lab introduction: why a Golden Kubestronaut built a homelab with the full CNCF stack on it, and what that actually taught.

## Networking & Ingress

- [自宅からポート開放なしで認証付きアプリを公開 — Cloudflare Tunnel + Access](https://zenn.dev/yuu7751/articles/9df7ce4f1f4830) (2026-03) — exposing an authenticated app from home with no inbound port: Cloudflare Tunnel for the connection, Access for the gate. The setup behind [Cloudflare Tunnel & Access](architecture/cloudflare-tunnel.md).

## Auth & Identity

- [KeycloakでCondition - User Roleを使った認証＋認可フロー構築](https://zenn.dev/yuu7751/articles/49669d360856d8) (2025-08) — building an authn + authz flow with Keycloak's Condition - User Role. Predates the current oauth2-proxy `ext_authz` design described in [Auth](architecture/auth.md).

## Bare metal & cluster build

- [セキュアなKubernetes統合開発基盤の構築（ベアメタルサーバー編）](https://zenn.dev/yuu7751/articles/dcded99d7c7a7d) (2025-09) — standing up a secure Kubernetes development platform on bare metal, written while the cluster this repository describes was being built.

## The application

- [Kensan 〜 使うほど賢くなるAIエージェントを作った話](https://zenn.dev/yuu7751/articles/f14bd01d1d04a8) (2026-02) — the **legacy** kensan: React + Go microservices, Google ADK agents, and an Iceberg lakehouse. That version was retired in July 2026 ([ADR-017](adr/017-kensan-legacy-removal.md)); the article stands as a record of what it was.

## Background — the certifications behind this

- [【日本で12人目】Kubernetes未経験から1年で15資格全冠した記録【Golden Kubestronaut】](https://zenn.dev/yuu7751/articles/e6bbaa11aac218) (2026-01) — one year from no Kubernetes experience to all 15 certifications.
- [業務未経験から半年でKubestronaut達成（KCNA/KCSA/CKA/CKAD/CKS全冠）勉強法まとめ](https://zenn.dev/yuu7751/articles/24f509769b97b5) (2025-10) — the first five, and how they were studied for.

The remaining certification guides (PCA / CGoA / CBA / CCA / ICA / LFCS / CNPA / CAPA / OTCA / KCA) are on the [Zenn profile](https://zenn.dev/yuu7751).

---

## Not yet written

Storage (Longhorn), Vault, GitOps, and observability have no article yet — those subjects currently live only in this site's [Architecture](architecture/infrastructure.md) section and the [ADRs](adr/index.md).

## How these are written

- Articles are snapshots. When the platform evolves past what one describes, a note is added pointing at the relevant docs page rather than rewriting the article
- Code blocks in articles are excerpts — see the [repository](https://github.com/yu-min3/kensan-lab) for current configs
