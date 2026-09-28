# ketchup-plan — CLI 1.10.1 post-install verification findings

## DONE
- v1.10.1 installed and verified: version, npm dist-tag, published README text, GH release assets
- `--project` enforced: setup without it exits 1
- Name resolution works: `--project quorum` → bc674142-… , confirmation line states the pin
- Unknown name fails closed: "workspace not found: <name>", existing .quorum.json left untouched
- END-TO-END: active workspace is howie, scratch repo pinned to quorum → `brief` read quorum's
  keyframe and `record-metric` landed in quorum::metrics (00:55:32) while howie::metrics stayed
  at 00:42:39. The pin beats the active workspace on both the read and the write path.

## TODO
- [ ] Burst 1: `setup -h` marks --project required, as it already does for --server-url [depends: none]
- [ ] Burst 2: `brief`'s description stops calling it "the active project" [depends: none]
