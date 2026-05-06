Execute cmd/scavium-faucet/.agent/step20.3.0.md following cmd/scavium-faucet/.agent/rules.md and cmd/scavium-faucet/.agent/commands.md.

Hard requirements:
- Treat the current repository as the only source of truth.
- Do not assume undocumented structure or behavior.
- Before editing, report:
  1. Files read
  2. Files to modify/create
  3. Minimal implementation plan
- Keep changes minimal, backward-compatible, and production-safe.
- Do not perform broad refactors.
- Do not introduce heavy dependencies.
- Update documentation incrementally if behavior, scope, or operator workflow changes.
- If something is unclear or missing in the step file, do not assume: explicitly state the uncertainty.
- After implementation, provide:
  1. Complete list of modified/created files
  2. Validation commands executed
  3. Test/build results
  4. Full git commands:
     - git checkout -b <branch>
     - git add <files>
     - git commit -m "<message>"
     - git checkout main
     - git merge <branch>
     - git branch -d <branch>
