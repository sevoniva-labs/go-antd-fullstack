# Data Governance Baseline

The scaffold provides a backend `datapolicy` catalog and a persisted platform management boundary. Each registered field records:

- classification: `public`, `internal`, `confidential`, `restricted`, `personal_information`, or `important_data`
- owner, processing purpose, residency, retention days, and field tags
- a dynamic masking strategy
- export approval and watermark requirements

`personal_information` and `restricted` fields force masking, export approval, and watermarking. Unregistered fields cannot be masked or exported through the catalog. Business repositories and DTOs must register explicit policies instead of guessing sensitivity from field names.

The platform exposes `GET /api/v1/admin/data-policies` and approval-controlled `PUT /api/v1/admin/data-policies/{field_key}` for organization-scoped policy management. `POST /api/v1/admin/data-exports/authorize` validates the registered fields before claiming a one-time approval execution ticket; the whole change/export authorization is written through the reliable audit transaction boundary. PostgreSQL and MySQL migrations are included.

This is a `Built-in` policy boundary, not a claim that a project has completed its data inventory, personal-information impact assessment, important-data identification, or regulatory filing. A production application must still connect the catalog to query serialization, the actual export executor, retention jobs, deletion evidence, watermarked file generation, and its organization policy. Automatic classification and unregistered-field inference remain intentionally unsupported.
