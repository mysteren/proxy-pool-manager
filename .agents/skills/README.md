# Agent Skills

Skills for Zed's coding agent in `proxy-pool-manager`. They are **English
"recipe + pointers"** documents: short, actionable steps that route the agent to
the RU source of truth in `docs/` and to the root `AGENTS.md` /
`PROJECT-BLUEPRINT.md`, instead of restating them.

## What a skill is

A skill is a directory under `.agents/skills/` containing a `SKILL.md` file:

- **frontmatter** — YAML between `---` with `name` (must match the directory name;
  lowercase with single hyphens) and an English `description`;
- **body** — the instructions the agent follows when the skill is loaded.

```markdown
---
name: my-skill
description: Specific trigger + what the skill does.
---

# My Skill

...steps...
```

## How Zed loads them

Zed auto-loads a skill when its `description` matches the current task. A skill
can also be referenced explicitly (e.g. "read `proxy-tester/SKILL.md`").

## Ground rules

- Keep a skill short (roughly 30–80 lines; `project-conventions` may be longer).
- **No invented facts.** If a detail is not in `docs/` or the code, do not write it.
- If a fact already lives in `docs/`, **point to it** (`Full detail: docs/...`)
  instead of copying the prose.
- A skill's `name` must equal its directory name; descriptions are English.

## Read first

- `AGENTS.md` — always-on repo rules.
- `PROJECT-BLUEPRINT.md` — portable project overview.
- `docs/` — RU source of truth for humans.
- `project-conventions` — the enforceable checklist for every code change.

## Index

| Skill | Use when |
|-------|----------|
| [project-conventions](project-conventions/SKILL.md) | Any code change — always-on conventions |
| [project-bootstrap](project-bootstrap/SKILL.md) | Wails / Tailwind / shadcn / layout / theme infra |
| [proxy-source-fetcher](proxy-source-fetcher/SKILL.md) | Loading and parsing proxy sources |
| [proxy-storage](proxy-storage/SKILL.md) | SQLite schema, CRUD, filters, export |
| [proxy-tester](proxy-tester/SKILL.md) | Latency / HTTP validation / speed / geo testing |
| [ui-proxy-table](ui-proxy-table/SKILL.md) | Proxies and MTProto tables |
| [ui-source-manager](ui-source-manager/SKILL.md) | Sources page |
| [ui-theme-manager](ui-theme-manager/SKILL.md) | Light / dark theme |
