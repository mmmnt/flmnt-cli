---
description: Attest that recalled flmnt context changed the outcome (record_attestation)
argument-hint: [what context helped / what you verified]
---
Record a context attestation — use sparingly, only when recalled flmnt context genuinely changed what you did.

Call `record_attestation(kind='<short kind, e.g. context_applied / verified_against_code>', note='<what the recalled context was + how it changed the outcome: $ARGUMENTS>')`. It emits a `ContextAttestation` metric (value=1) to the workspace `::metrics` stream, signaling when flmnt memory deepened the work. Don't attest routinely — only on a real assist.
