# Data Governance Baseline

The scaffold provides a backend `datapolicy` catalog primitive. Each registered field records:

- classification: `public`, `internal`, `confidential`, `restricted`, `personal_information`, or `important_data`
- owner, processing purpose, residency, retention days, and field tags
- a dynamic masking strategy
- export approval and watermark requirements

`personal_information` and `restricted` fields force masking, export approval, and watermarking. Unregistered fields cannot be masked or exported through the catalog. Business repositories and DTOs must register explicit policies instead of guessing sensitivity from field names.

This is a `Built-in` policy boundary, not a claim that a project has completed its data inventory, personal-information impact assessment, important-data identification, or regulatory filing. A production application must connect the catalog to platform management, query serialization, export approval, retention jobs, deletion evidence, and its organization policy.
