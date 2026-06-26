---
description: Record an operational/execution decision to flmnt (deploys, infra, incidents)
argument-hint: <the operational decision, as a fact>
---
Record this operational/execution decision (deploy, ops procedure, infra/SG change, incident fix, runtime config): **$ARGUMENTS**

Call `record_operational_decision(content='<decision as a fact>', causal_refs=[ids that informed it, if any])`. It targets the project `::operational` stream automatically (no `stream_id`) and has NO causal-ref gate. Keep operational decisions out of `::domain` so planning causal density stays clean. For product/architecture/strategy decisions use `/flmnt-decision` instead.
