# grafana/dsconfig – Feedback from the n8n datasource

Versions used: `github.com/grafana/dsconfig/dsconfig` v0.0.12, `github.com/grafana/dsconfig/schema` v0.0.13 (2026-10-13).
Schema: `pkg/schema/dsconfig.json`, test: `pkg/schema/dsconfig_test.go`.

Overall: `schema.RunPluginTests` is easy to adopt (one test function, eight checks). A deliberate drift
(removing `maxPages` from the schema) is caught by `JSONDataMatchesStruct` and `SchemaArtifactInSync`.

## Issue / PR candidates

| #   | Type     | Finding                                                                                                                                                                                                                                   | Proposal                                                                                                            |
| --- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| 1   | docs     | `PUBLISH-SCHEMA.md` places the schema at `pkg/schema/dsconfig.json`; the main README and `schema.md` do not mention a location. Plugin authors following other guides (e.g. putting it next to `plugin.json` in `src/`) end up elsewhere. | State the canonical location once in the README and link `PUBLISH-SCHEMA.md` from the top-level README.             |
| 2   | bug/docs | README example 2 shows `x-dsconfig-depends-on` / `x-dsconfig-required-when` on `secureValues`. For our `tlsCACert` (with `dependsOn` and `requiredWhen`) the generated `schema.gen.json` contains only `key` and `description`.           | Either emit the extensions as documented or update the README example.                                              |
| 3   | docs     | README example 1 shows `"format": "uri"` for the root `url`; the generated spec for our `url` field has `pattern` but no `format`.                                                                                                        | Align example with actual converter output (or document when `format` is derived).                                  |
| 4   | feature  | Base field packs only support `exclude`. Using `plugin_sdk_settings` for just `url` + TLS options means excluding ~15 fields.                                                                                                             | Add `include` to `BaseFieldRef`.                                                                                    |
| 5   | feature  | `role` vocabulary has `auth.apiKey.value` but no way to declare the HTTP header name the key is sent in (`X-N8N-API-KEY`).                                                                                                                | Allow a constant header name for `auth.apiKey.*` roles, or document `auth.apiKey.key` usage for fixed header names. |
| 6   | feature  | No secret-aware UI hint: `ui.component` has `input` / `textarea` but nothing that maps to `SecretInput` / `SecretTextArea`. A renderer would have to infer it from `target: secureJsonData`.                                              | Document that inference rule, or add `secret` / `secretTextarea` components.                                        |
| 7   | docs     | `ui` and `label` are not part of the generated SDK artifacts. Fine, but not stated; it is unclear which consumer should read them.                                                                                                        | Document which artifacts carry UI metadata.                                                                         |
| 8   | DX       | `go test ./...` from the plugin root also walks `node_modules` (a Go package ships inside `flatted`). Not a dsconfig bug, but the `go:generate` hint in `PUBLISH-SCHEMA.md` suggests `go test ./...`.                                     | Recommend `go test ./pkg/...`.                                                                                      |
| 9   | DX       | Struct fields that must not be part of jsonData (root `URL`, decrypted secrets) need `json:"-"`, because `JSONDataMatchesStruct` compares all json tags of the struct with the schema's jsonData fields. Not mentioned in the docs.       | Mention in `PUBLISH-SCHEMA.md` (Change 3) that the settings struct must contain exactly the jsonData keys.          |

## Things that worked well

- Field names chosen to match the SDK conventions (`tlsSkipVerify`, `tlsAuthWithCACert`, `tlsCACert`) let
  `DataSourceInstanceSettings.HTTPClientOptions` handle TLS and PDC without extra code.
- `SecureValuesMatchLoadSettings` keeps the `SecureKeys` list honest; we export it from the settings package
  so the loader and the test share one list.

## Update 2026-10-13: a schema consumer exists in @grafana/plugin-ui

`@grafana/plugin-ui` 0.17.6 contains `fetchConfigSchema(pluginType)` and `DatasourceConfigWizard`. The schema is
loaded from `public/plugins/{pluginType}/schema/dsconfig.json`, i.e. exactly the file `PUBLISH-SCHEMA.md` (Change 5)
copies into the plugin build. So the README statement "no React UI renderer" is outdated.

| #   | Type    | Finding                                                                                                                                                                                | Proposal                                                                                                                       |
| --- | ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| 10  | docs    | Neither the dsconfig README nor `PUBLISH-SCHEMA.md` mention that `@grafana/plugin-ui` consumes the published schema (`DatasourceConfigWizard`), nor which fields/UI hints it supports. | Link the consumer from the dsconfig README and document the supported subset (`ui.component`, `groups`, `dependsOn`, secrets). |
| 11  | feature | `role: auth.apiKey.value` plus a fixed header name would let the wizard render API-key auth for plugins like this one without a hand-written editor (see #5).                          | Same as #5.                                                                                                                    |
