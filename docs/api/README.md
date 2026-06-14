# CarWatch API reference

[`openapi.yaml`](openapi.yaml) is a hand-authored OpenAPI 3.1 description of the
`/api/v1` surface, derived from the route table in `internal/api/api.go`
(`Routes()`). It documents the four auth tiers (authenticated, optional-auth,
guest, catalog), shared error/pagination components, and the request/response
schemas for the user-facing endpoints.

## Validate / preview

```bash
# lint (valid with style-only warnings)
npx @redocly/cli lint docs/api/openapi.yaml

# preview docs locally
npx @redocly/cli preview-docs docs/api/openapi.yaml
```

## Scope notes

- Endpoint groups mounted conditionally (admin, notifications, push,
  saved/hidden, scheduler status) are documented as the full surface.
- A few internal admin responses use permissive (`additionalProperties: true`)
  schemas rather than asserting an exact shape; tighten these when building
  contract tests against them.
- Keep this file in sync when adding or changing routes in `Routes()`.
