# Publishing checklist

Route to a signed plugin on Grafana Cloud: public catalog with a community signature. Grafana Cloud only
installs plugins from the catalog; a private signature (`--rootUrls`) only helps self-hosted Grafana
instances, so it is not used.

## Setup (done)

- [x] Plugin ID `nluecke-n8n-datasource`, grafana.com organisation `nluecke` (the signing token must come
      from the organisation matching the first part of the plugin ID), test stack `https://nluecke.grafana.net`.
- [x] Open source on GitHub: [nluecke/grafana-n8n-datasource](https://github.com/nluecke/grafana-n8n-datasource), Apache-2.0.
- [x] Access policy token in the `nluecke` organisation (scope `plugins:write`), stored only as repository
      secret `GRAFANA_ACCESS_POLICY_TOKEN`.
- [x] `npm run sign` without `--rootUrls`. Signing before approval fails with `409 Field is required: rootUrls`,
      so CI and release only sign when the repository variable `SIGN_PLUGIN` is `true`.
- [x] Release workflow builds the archive with build provenance attestation and creates a draft release.
      Release tags `v*` are immutable: a failed release needs a new version.
- [x] Own neutral logo (`src/img/logo.svg`). n8n's brand guidelines (https://n8n.io/brandguidelines/) grant
      no usage rights; permission requested separately. If granted, keep n8n's rules (unmodified, min. 100 px
      wide, clear space).
- [x] Disclaimer "not affiliated with n8n GmbH" in README and plugin description; screenshots without load-test data.

## Catalog submission

- [x] v1.0.0 submitted; automated checks flagged Go/x-net vulnerabilities and gosec false positives, fixed in
      v1.0.2 (v1.0.1 is a tag without release).
- [x] v1.0.2 released and the submission updated (texts and checksums: `docs/catalog-submission.md`).
- [ ] v1.0.3 (Go 1.26.10 for GO-2026-6809, golang.org/x/net v0.62.0): release, verify, update the submission.
- [ ] Wait for Grafana's review.

## After approval

1. [ ] `gh variable set SIGN_PLUGIN --body true -R nluecke/grafana-n8n-datasource`, then release a signed version.
2. [ ] Install the plugin from the catalog on `nluecke.grafana.net` and test:
   - [ ] data source against an n8n reachable from Grafana Cloud (n8n Cloud, or self-hosted with a public URL),
   - [ ] self-hosted n8n on a private network through Private Data Source Connect (PDC agent next to n8n,
         _Private data source connect_ under _Additional settings_; verified locally with `make pdc`),
   - [ ] import the dashboard from the data source's _Dashboards_ tab,
   - [ ] an alert rule on the error rate (`executionStats`, metric `errorRate`, bucket `range`).
3. [ ] Record the results in the Phase 7 matrix of `docs/poc-findings.md`.

If Grafana asks for a commercial signature instead of a community one (the plugin targets n8n, a commercial
product), decide between a subscription and private signatures for self-hosted users.
