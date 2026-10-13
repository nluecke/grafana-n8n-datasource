# Publishing checklist

Not part of the PoC. Steps from the local archive to a signed plugin: first privately signed for the
Grafana Cloud stack `nluecke.grafana.net`, later in the public catalog. Grafana Cloud does not load unsigned plugins.

## Decisions (2026-10-13)

- [x] **Plugin ID `nluecke-n8n-datasource`**, grafana.com organisation `nluecke`, Grafana Cloud stack
      `https://nluecke.grafana.net`. The signing token must come from the organisation matching the first part
      of the plugin ID (`nluecke`).
- [x] **Open source** on GitHub: [nluecke/grafana-n8n-datasource](https://github.com/nluecke/grafana-n8n-datasource), Apache-2.0.
- [x] **Signature:** private for `nluecke.grafana.net` first, community signature (public catalog) later.
- [ ] **Version.** Currently `1.0.0` from the scaffold; consider `0.1.0` for the first release and maintain `CHANGELOG.md`.

## Next step: private signature for nluecke.grafana.net

Goal: run the plugin on `https://nluecke.grafana.net` without a public catalog listing, to complete the
Grafana Cloud row of the test matrix.

1. [x] **Clarified 2026-10-13:** the Grafana Cloud stack offers no way to install an "offline" (uploaded) plugin.
       The private signature is therefore only useful for self-hosted Grafana instances listed in `--rootUrls`;
       for `nluecke.grafana.net` the plugin has to come from the catalog (see "Public catalog" below).
2. [x] Create an access policy token on grafana.com in the `nluecke` organisation: realm = the organisation,
       scope `plugins:write`. Store it only as repository secret `GRAFANA_ACCESS_POLICY_TOKEN`
       (`gh secret set GRAFANA_ACCESS_POLICY_TOKEN`) or as a local environment variable, never in the repository.
3. [x] Sign: `npm run sign` signs for `https://nluecke.grafana.net` (`--rootUrls` in `package.json`; the value
       must match Grafana's `root_url`; add further URLs comma-separated). The CI workflow runs the same step
       automatically once the secret exists, so the build artifact is signed. Re-sign every version.
       Locally: `mage -v && npm run build && GRAFANA_ACCESS_POLICY_TOKEN=… npm run sign` → `dist/MANIFEST.txt`.
4. [ ] Package: `cp -r dist nluecke-n8n-datasource && zip -qr nluecke-n8n-datasource-<version>.zip nluecke-n8n-datasource`.
5. [ ] Install on `nluecke.grafana.net` (way from step 1), then test:
   - [ ] data source against n8n reachable from Grafana Cloud (n8n Cloud, or a self-hosted n8n with public URL),
   - [ ] self-hosted n8n on a private network via Private Data Source Connect (PDC agent next to n8n,
         _Private data source connect_ enabled under _Additional settings_). Verified locally with `make pdc`;
         for a test on Grafana Cloud before release, Grafana asks plugin authors to contact
         `integrations+pdc@grafana.com` with plugin version and stack,
   - [ ] import the dashboard from the data source's _Dashboards_ tab,
   - [ ] one alert rule on the error rate (`executionStats`, metric `errorRate`, bucket `range`).
6. [ ] Record results in `docs/poc-findings.md` (Phase 7 matrix).

## Public catalog (community signature) – the route to nluecke.grafana.net

After approval the plugin gets a community signature (not bound to a root URL) and is installable from the
catalog on Grafana Cloud and on any self-hosted Grafana.

- [x] Logo: own neutral logo (`src/img/logo.svg`), not n8n's logo. n8n's brand guidelines
      (https://n8n.io/brandguidelines/) only define how the logo looks, they grant no usage rights to third
      parties. Permission requested separately; swap the logo in a later version if granted (keep n8n's rules:
      unmodified, min. 100 px wide, clear space).
- [x] Disclaimer "not affiliated with n8n GmbH" in README and plugin description.
- [x] Screenshots without load-test data (`src/img/screenshot-dashboard.png`, `src/img/screenshot-config.png`).
- [x] `sign` script without `--rootUrls`. Signing before approval fails with
      `409 Field is required: rootUrls`, so CI and release only sign when the repository variable
      `SIGN_PLUGIN` is `true`.
- [x] Release workflow with build provenance attestation (`attestation: true`).
- [x] Release: `v1.0.0` submitted, automated checks flagged Go/x-net vulnerabilities and gosec false positives;
      fixed in `v1.0.2` (draft GitHub release with zip, SHA1, build attestation; validator: only "unsigned").
      `v1.0.1` is a tag without release (release build stopped at the Go minimum). Release tags `v*` are immutable.
- [ ] Publish the v1.0.2 draft release and update the submission (URL, SHA1).
- [ ] Submit on grafana.com → _My Plugins_ → _Submit plugin_ (texts: `docs/catalog-submission.md`).
- [ ] After approval: `gh variable set SIGN_PLUGIN --body true -R nluecke/grafana-n8n-datasource`, release the
      signed version, install from the catalog on `nluecke.grafana.net`, run the Grafana Cloud tests above
      (n8n Cloud, PDC, dashboard, alert rule) and record them in `docs/poc-findings.md`.
